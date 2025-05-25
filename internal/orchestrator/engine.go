package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/client"
)

func NewEngine(c client.Client) *Engine {
	return &Engine{
		temporal: c,
		runs:     make(map[string]*RunInfo),
	}
}

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	// Validate input
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.YamlPayload) == 0 {
		return nil, fmt.Errorf("YAML payload cannot be empty")
	}
	
	slog.Debug("CreateRun called", "payload_size", len(req.YamlPayload))

	runID, err := generateID()
	if err != nil {
		slog.Error("Failed to generate run ID", "error", err)
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	run, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		slog.Error("Failed to parse YAML", "error", err)
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}
	
	// Validate parsed run has tests
	if len(run.Tests) == 0 {
		return nil, fmt.Errorf("test run must contain at least one test")
	}
	
	slog.Debug("Starting run", "name", run.Name, "test_count", len(run.Tests))

	runInfo := &RunInfo{
		ID:        runID,
		Name:      run.Name,
		Status:    "PENDING",
		StartedAt: time.Now(),
		Tests:     make(map[string]*TestInfo),
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("Starting test run \"%s\"... 🚀", run.Name),
				Color: "purple",
				Bold:  true,
			},
		},
	}

	for _, test := range run.Tests {
		testID, err := generateID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate test ID: %v", err)
			return nil, fmt.Errorf("failed to generate test ID: %w", err)
		}
		testInfo := &TestInfo{
			WorkflowID: testID,
			Name:       test.Name,
			Status:     "PENDING",
			StartedAt:  time.Now(),
			RunID:      runID,
		}

		workflowOptions := client.StartWorkflowOptions{
			ID:        testID,
			TaskQueue: "test-workflows",
		}

		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, run.Vars)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
		// add a log line for the start of the test
		runInfo.Logs = append(runInfo.Logs, LogLine{
			Msg:   fmt.Sprintf("Running test: \"%s\"...", test.Name),
			Color: "n/a",
			Bold:  false,
		})
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

	// Initial validation and log copy
	e.mu.RLock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.RUnlock()
		return fmt.Errorf("run not found: %s", runID)
	}
	
	logs := make([]LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

	// Send initial logs
	for _, logMsg := range logs {
		if err := stream.Send(&generated.LogLine{
			Ts:    time.Now().Format(time.RFC3339),
			Msg:   logMsg.Msg,
			Color: logMsg.Color,
			Bold:  logMsg.Bold,
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
			// Get new logs and status in a single lock acquisition
			e.mu.RLock()
			runInfo, exists := e.runs[runID]
			if !exists {
				e.mu.RUnlock()
				return fmt.Errorf("run not found: %s", runID)
			}

			var newLogs []LogLine
			status := runInfo.Status
			
			if len(runInfo.Logs) > lastLogIndex {
				newLogs = make([]LogLine, len(runInfo.Logs)-lastLogIndex)
				copy(newLogs, runInfo.Logs[lastLogIndex:])
				lastLogIndex = len(runInfo.Logs)
			}
			e.mu.RUnlock()

			// Send new logs (outside of lock)
			for _, logMsg := range newLogs {
				if err := stream.Send(&generated.LogLine{
					Ts:    time.Now().Format(time.RFC3339),
					Msg:   logMsg.Msg,
					Color: logMsg.Color,
					Bold:  logMsg.Bold,
				}); err != nil {
					return err
				}
			}

			// Check if run is finished
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

	log.Printf("[DEBUG] Starting to monitor workflow %s for run %s", workflowID, runID)
	workflowRun := e.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

	resultChan := make(chan error, 1)
	go func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[ERROR] Panic in workflow monitoring goroutine: %v", r)
				resultChan <- fmt.Errorf("workflow monitoring panic: %v", r)
			}
		}()
		
		var result interface{}
		err := workflowRun.Get(ctx, &result)
		resultChan <- err
	}()

	select {
	case err := <-resultChan:
		e.updateTestStatus(runID, workflowID, err)
	case <-ctx.Done():
		log.Printf("[WARN] Monitoring timed out for workflow %s in run %s", workflowID, runID)
		e.updateTestStatus(runID, workflowID, fmt.Errorf("workflow monitoring timeout"))
	}
}

// updateTestStatus updates the test status and handles cleanup atomically
func (e *Engine) updateTestStatus(runID, workflowID string, workflowErr error) {
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Run not found during status update: %s", runID)
		return
	}

	testInfo, exists := runInfo.Tests[workflowID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Test not found during status update: %s in run %s", workflowID, runID)
		return
	}

	testInfo.EndedAt = time.Now()
	
	if workflowErr != nil {
		if workflowErr.Error() == "workflow monitoring timeout" {
			testInfo.Status = "TIMEOUT"
			e.mu.Unlock()
			log.Printf("[WARN] Test timed out: %s", testInfo.Name)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", testInfo.Name), "red", true)
		} else {
			testInfo.Status = "FAILED"
			e.mu.Unlock()
			log.Printf("[ERROR] Test failed: %s - %v", testInfo.Name, workflowErr)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %v", testInfo.Name, workflowErr), "red", true)
		}
	} else {
		testInfo.Status = "PASSED"
		e.mu.Unlock()
		log.Printf("[INFO] Test passed: %s", testInfo.Name)
		e.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testInfo.Name), "green", true)
	}
	
	e.checkIfRunFinished(runID)
}

func (e *Engine) addLog(runID, message, color string, bold bool) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, LogLine{
		Msg:   message,
		Color: color,
		Bold:  bold,
	})
}

func generateID() (string, error) {
	b := make([]byte, 8)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func (e *Engine) checkIfRunFinished(runID string) {
	counts, err := e.getTestStatusCounts(runID)
	if err != nil {
		log.Printf("[ERROR] Failed to get test status counts for run %s: %v", runID, err)
		return
	}
	
	// Only proceed if all tests are finished (no pending tests)
	if counts.Pending > 0 {
		return
	}
	
	// Get run name and update status atomically
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Run not found when checking if finished: %s", runID)
		return
	}
	
	runName := runInfo.Name
	
	// Determine final status and update
	if counts.Failed == 0 && counts.TimedOut == 0 {
		// All tests passed
		runInfo.Status = "PASSED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, counts.Total), "n/a", true)
	} else {
		// Some tests failed or timed out
		runInfo.Status = "FAILED"
		runInfo.EndedAt = time.Now()
		e.mu.Unlock()
		
		if counts.TimedOut == 0 {
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total), "n/a", true)
		} else {
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total, counts.TimedOut, counts.Total), "n/a", true)
		}
	}
}

// getTestStatusCounts returns all test status counts in a single lock acquisition
func (e *Engine) getTestStatusCounts(runID string) (TestStatusCounts, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	runInfo, exists := e.runs[runID]
	if !exists {
		return TestStatusCounts{}, fmt.Errorf("run not found: %s", runID)
	}
	
	counts := TestStatusCounts{
		Total: len(runInfo.Tests),
	}
	
	for _, testInfo := range runInfo.Tests {
		switch testInfo.Status {
		case "PASSED":
			counts.Passed++
		case "FAILED":
			counts.Failed++
		case "TIMEOUT":
			counts.TimedOut++
		case "PENDING":
			counts.Pending++
		}
	}
	
	return counts, nil
}

// Health implements the health check endpoint
func (e *Engine) Health(ctx context.Context, req *generated.HealthRequest) (*generated.HealthResponse, error) {
	return &generated.HealthResponse{
		Status: "ok",
	}, nil
}
