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
	log.Printf("[DEBUG] CreateRun called with %d bytes of YAML data", len(req.YamlPayload))

	runID, err := generateID()
	if err != nil {
		log.Printf("[ERROR] Failed to generate run ID: %v", err)
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}
	log.Printf("[DEBUG] Generated run ID: %s", runID)

	test, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		log.Printf("[ERROR] Failed to parse YAML: %v", err)
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	log.Printf("[DEBUG] Successfully parsed YAML for test: %s", test.Name)

	log.Printf("[DEBUG] Creating run info for ID: %s", runID)
	runInfo := &RunInfo{
		ID:        runID,
		Status:    "PENDING",
		StartedAt: time.Now(),
	}

	log.Printf("[DEBUG] Attempting to acquire mutex lock for run %s", runID)
	e.mu.Lock()
	log.Printf("[DEBUG] Acquired mutex lock for run %s", runID)
	e.runs[runID] = runInfo
	e.mu.Unlock()
	log.Printf("[DEBUG] Released mutex lock for run %s", runID)
	log.Printf("[DEBUG] Created run info for ID: %s", runID)

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("test-%s", runID),
		TaskQueue: "test-workflows",
	}

	log.Printf("[DEBUG] Starting workflow for run %s with test: %s", runID, test.Name)
	log.Printf("[DEBUG] Workflow options: ID=%s, TaskQueue=%s", workflowOptions.ID, workflowOptions.TaskQueue)

	execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test)
	if err != nil {
		log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
		log.Printf("[DEBUG] Attempting to acquire mutex lock for error handling of run %s", runID)
		e.mu.Lock()
		log.Printf("[DEBUG] Acquired mutex lock for error handling of run %s", runID)
		runInfo.Status = "FAILED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()
		log.Printf("[DEBUG] Released mutex lock for error handling of run %s", runID)
		e.addLog(runID, fmt.Sprintf("Failed to start workflow: %v", err))
		return nil, fmt.Errorf("failed to start workflow: %w", err)
	}

	log.Printf("[DEBUG] Successfully started workflow for run %s with execution ID: %s", runID, execution.GetID())

	log.Printf("[DEBUG] Attempting to acquire mutex lock for workflow update of run %s", runID)
	e.mu.Lock()
	log.Printf("[DEBUG] Acquired mutex lock for workflow update of run %s", runID)
	runInfo.Status = "RUNNING"
	runInfo.WorkflowID = execution.GetID()
	runInfo.RunID = execution.GetRunID()
	e.mu.Unlock()
	log.Printf("[DEBUG] Released mutex lock for workflow update of run %s", runID)

	e.addLog(runID, fmt.Sprintf("Starting test: %s", test.Name))

	// Start monitoring in a separate goroutine
	go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())

	log.Printf("[DEBUG] CreateRun completed successfully for run %s", runID)
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

// func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
// 	e.mu.RLock()
// 	defer e.mu.RUnlock()

// 	response := &generated.ListRunsResponse{
// 		Runs: make([]*generated.RunSummary, 0, len(e.runs)),
// 	}

// 	for _, runInfo := range e.runs {
// 		summary := &generated.RunSummary{
// 			RunId:     runInfo.ID,
// 			Status:    runInfo.Status,
// 			StartedAt: runInfo.StartedAt.Format(time.RFC3339),
// 		}

// 		if !runInfo.EndedAt.IsZero() {
// 			summary.EndedAt = runInfo.EndedAt.Format(time.RFC3339)
// 		}

// 		response.Runs = append(response.Runs, summary)
// 	}

// 	return response, nil
// }

func (e *Engine) monitorWorkflow(runID, workflowID, workflowRunID string) {
	// Create a context with timeout for monitoring
	// TODO: Make this time limit configurable
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	log.Printf("[DEBUG] Starting to monitor workflow %s for run %s", workflowID, runID)
	workflowRun := e.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

	// Create a channel to handle the workflow result
	resultChan := make(chan error, 1)

	// Start a goroutine to handle the blocking Get operation
	go func() {
		var result interface{}
		err := workflowRun.Get(ctx, &result)
		resultChan <- err
	}()

	// Wait for either the result or context timeout
	select {
	case err := <-resultChan:
		log.Printf("[DEBUG] Got workflow result for run %s", runID)
		log.Printf("[DEBUG] Attempting to acquire mutex lock for workflow completion of run %s", runID)
		e.mu.Lock()
		log.Printf("[DEBUG] Acquired mutex lock for workflow completion of run %s", runID)

		runInfo, exists := e.runs[runID]
		if !exists {
			log.Printf("[ERROR] Run not found during monitoring: %s", runID)
			e.mu.Unlock()
			log.Printf("[DEBUG] Released mutex lock for workflow completion of run %s (run not found)", runID)
			return
		}

		runInfo.EndedAt = time.Now()

		if err != nil {
			log.Printf("[ERROR] Workflow failed for run %s: %v", runID, err)
			runInfo.Status = "FAILED"
			e.mu.Unlock()
			log.Printf("[DEBUG] Released mutex lock for workflow completion of run %s (before adding error log)", runID)
			e.addLog(runID, fmt.Sprintf("Workflow failed: %v", err))
		} else {
			log.Printf("[DEBUG] Workflow completed successfully for run %s", runID)
			runInfo.Status = "PASSED"
			e.mu.Unlock()
			log.Printf("[DEBUG] Released mutex lock for workflow completion of run %s (before adding success log)", runID)
			e.addLog(runID, "Workflow completed successfully")
		}
	case <-ctx.Done():
		log.Printf("[DEBUG] Monitoring timed out for run %s", runID)
		log.Printf("[DEBUG] Attempting to acquire mutex lock for workflow timeout of run %s", runID)
		e.mu.Lock()
		log.Printf("[DEBUG] Acquired mutex lock for workflow timeout of run %s", runID)
		if runInfo, exists := e.runs[runID]; exists {
			runInfo.Status = "TIMEOUT"
			runInfo.EndedAt = time.Now()
			e.mu.Unlock()
			log.Printf("[DEBUG] Released mutex lock for workflow timeout of run %s (before adding timeout log)", runID)
			e.addLog(runID, "Workflow monitoring timed out")
		} else {
			e.mu.Unlock()
			log.Printf("[DEBUG] Released mutex lock for workflow timeout of run %s (run not found)", runID)
		}
	}
}

func (e *Engine) addLog(runID, message string) {
	log.Printf("[DEBUG] Attempting to acquire mutex lock for adding log to run %s", runID)
	e.mu.Lock()
	log.Printf("[DEBUG] Acquired mutex lock for adding log to run %s", runID)
	defer func() {
		e.mu.Unlock()
		log.Printf("[DEBUG] Released mutex lock for adding log to run %s", runID)
	}()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, message)
	log.Printf("[DEBUG] Added log to run %s: %s", runID, message)
}

func generateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}
