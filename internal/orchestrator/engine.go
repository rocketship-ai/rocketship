package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/client"
)

type Engine struct {
	generated.UnimplementedEngineServer
	temporal client.Client
	runs     map[string]*RunInfo
	mu       sync.RWMutex
}
type RunInfo struct {
	Name      string
	ID        string
	Status    string
	StartedAt time.Time
	EndedAt   time.Time
	Tests     map[string]*TestInfo
	Logs      []string
}

type TestInfo struct {
	Name       string
	ID         string
	Status     string
	StartedAt  time.Time
	EndedAt    time.Time
	RunID      string
	WorkflowID string
}

func NewEngine(c client.Client) *Engine {
	return &Engine{
		temporal: c,
		runs:     make(map[string]*RunInfo),
	}
}

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	log.Printf("[DEBUG] CreateRun called with %d bytes of YAML data", len(req.YamlPayload))

	runID, err := generateID()
	if err != nil {
		log.Printf("[ERROR] Failed to generate run ID: %v", err)
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	run, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		log.Printf("[ERROR] Failed to parse YAML: %v", err)
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	log.Printf("[DEBUG] Starting run: %s", run.Name)

	runInfo := &RunInfo{
		Name:      run.Name,
		ID:        runID,
		Status:    "PENDING",
		StartedAt: time.Now(),
		Tests:     make(map[string]*TestInfo),
		Logs:      []string{},
	}

	for _, test := range run.Tests {
		testID, err := generateID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate test ID: %v", err)
			return nil, fmt.Errorf("failed to generate test ID: %w", err)
		}
		testInfo := &TestInfo{
			Name:       test.Name,
			ID:         testID,
			Status:     "PENDING",
			StartedAt:  time.Now(),
			RunID:      runID,
			WorkflowID: "",
		}
		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
		e.mu.Unlock()

		workflowOptions := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("run-%s-test-%s", runID, testID),
			TaskQueue: "test-workflows",
		}

		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID].WorkflowID = execution.GetID()
		e.mu.Unlock()

		go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())
	}

	e.mu.Lock()
	e.runs[runID] = runInfo
	e.mu.Unlock()

	return &generated.CreateRunResponse{
		RunId: runID,
	}, nil
}

func (e *Engine) StreamLogs(req *generated.LogStreamRequest, stream generated.Engine_StreamLogsServer) error {
	runID := req.RunId

	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	e.mu.RUnlock()

	if !exists {
		return fmt.Errorf("run not found: %s", runID)
	}

	e.mu.RLock()
	logs := make([]string, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

	for _, logMsg := range logs {
		if err := stream.Send(&generated.LogLine{
			Ts:  time.Now().Format(time.RFC3339),
			Msg: logMsg,
		}); err != nil {
			return err
		}
	}

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	lastLogIndex := len(logs)

	for {
		select {
		case <-ticker.C:
			e.mu.RLock()
			runInfo, exists := e.runs[runID]
			if !exists {
				e.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}

			if len(runInfo.Logs) > lastLogIndex {
				newLogs := runInfo.Logs[lastLogIndex:]
				lastLogIndex = len(runInfo.Logs)
				e.mu.RUnlock()

				for _, logMsg := range newLogs {
					if err := stream.Send(&generated.LogLine{
						Ts:  time.Now().Format(time.RFC3339),
						Msg: logMsg,
					}); err != nil {
						return err
					}
				}
			} else {
				e.mu.RUnlock()
			}

			e.mu.RLock()
			status := runInfo.Status
			e.mu.RUnlock()

			if status == "PASSED" || status == "FAILED" {
				return nil
			}
		case <-stream.Context().Done():
			return nil
		}
	}
}

func (e *Engine) monitorWorkflow(runID, workflowID, workflowRunID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("[DEBUG] Starting to monitor workflow %s", workflowID)
	workflowRun := e.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

	resultChan := make(chan error, 1)
	go func() {
		var result interface{}
		err := workflowRun.Get(ctx, &result)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		e.mu.Lock()
		runInfo, exists := e.runs[runID]
		if !exists {
			log.Printf("[ERROR] Run not found during monitoring: %s", runID)
			e.mu.Unlock()
			return
		}

		runInfo.EndedAt = time.Now()

		if err != nil {
			log.Printf("[ERROR] Workflow failed for run %s: %v", runID, err)
			runInfo.Status = "FAILED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Workflow failed: %v", err))
		} else {
			log.Printf("[DEBUG] Workflow completed successfully for run %s", runID)
			runInfo.Status = "PASSED"
			e.mu.Unlock()
			e.addLog(runID, "Workflow completed successfully")
		}
	case <-ctx.Done():
		log.Printf("[DEBUG] Monitoring timed out for run %s", runID)
		e.mu.Lock()
		if runInfo, exists := e.runs[runID]; exists {
			runInfo.Status = "TIMEOUT"
			runInfo.EndedAt = time.Now()
			e.mu.Unlock()
			e.addLog(runID, "Workflow monitoring timed out")
		} else {
			e.mu.Unlock()
		}
	}
}

func (e *Engine) addLog(runID, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, message)
}

func generateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
