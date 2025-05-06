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
	ID         string
	Status     string
	StartedAt  time.Time
	EndedAt    time.Time
	WorkflowID string
	RunID      string
	Logs       []string
}

func NewEngine(c client.Client) *Engine {
	return &Engine{
		temporal: c,
		runs:     make(map[string]*RunInfo),
	}
}

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	runID, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	// TODO: Return full slice of tests, not just the first one
	test, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	runInfo := &RunInfo{
		ID:        runID,
		Status:    "PENDING",
		StartedAt: time.Now(),
	}

	e.mu.Lock()
	e.runs[runID] = runInfo
	e.mu.Unlock()

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-%s", runID),
		TaskQueue: "test-workflows",
	}

	log.Printf("Starting workflow for run %s with test: %s", runID, test.Name)
	e.addLog(runID, fmt.Sprintf("Starting test: %s", test.Name))

	execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test)
	if err != nil {
		e.mu.Lock()
		runInfo.Status = "FAILED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()
		e.addLog(runID, fmt.Sprintf("Failed to start workflow: %v", err))
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	e.mu.Lock()
	runInfo.Status = "RUNNING"
	runInfo.WorkflowID = execution.GetID()
	runInfo.RunID = execution.GetRunID()
	e.mu.Unlock()

	e.addLog(runID, fmt.Sprintf("Workflow started with ID: %s, RunID: %s", execution.GetID(), execution.GetRunID()))

	go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())

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

func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	response := &generated.ListRunsResponse{
		Runs: make([]*generated.RunSummary, 0, len(e.runs)),
	}

	for _, runInfo := range e.runs {
		summary := &generated.RunSummary{
			RunId:     runInfo.ID,
			Status:    runInfo.Status,
			StartedAt: runInfo.StartedAt.Format(time.RFC3339),
		}

		if !runInfo.EndedAt.IsZero() {
			summary.EndedAt = runInfo.EndedAt.Format(time.RFC3339)
		}

		response.Runs = append(response.Runs, summary)
	}

	return response, nil
}

func (e *Engine) monitorWorkflow(runID, workflowID, workflowRunID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	workflowRun := e.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

	var result interface{}
	err := workflowRun.Get(ctx, &result)

	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("Run not found: %s", runID)
		return
	}

	runInfo.EndedAt = time.Now()

	if err != nil {
		runInfo.Status = "FAILED"
		e.addLog(runID, fmt.Sprintf("Workflow failed: %v", err))
		log.Printf("Workflow failed for run %s: %v", runID, err)
	} else {
		runInfo.Status = "PASSED"
		e.addLog(runID, "Workflow completed successfully")
		log.Printf("Workflow completed successfully for run %s", runID)
	}
}

func (e *Engine) addLog(runID, message string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
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
