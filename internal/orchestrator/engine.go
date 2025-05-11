package orchestrator

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
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
		ID:        runID,
		Name:      run.Name,
		Status:    "PENDING",
		StartedAt: time.Now(),
		Tests:     make(map[string]*TestInfo),
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("🚀🚀🚀 Starting test run \"%s\"... 🚀🚀🚀", run.Name),
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

		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
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
	logs := make([]LogLine, len(runInfo.Logs))
	copy(logs, runInfo.Logs)
	e.mu.RUnlock()

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
						Ts:    time.Now().Format(time.RFC3339),
						Msg:   logMsg.Msg,
						Color: logMsg.Color,
						Bold:  logMsg.Bold,
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
				// Run is finished, end the stream
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

		testInfo := runInfo.Tests[workflowID]
		testInfo.EndedAt = time.Now()

		if err != nil {
			log.Printf("[ERROR] Workflow failed for test %s: %v", testInfo.Name, err)
			testInfo.Status = "FAILED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %v", testInfo.Name, err), "red", true)
			e.checkIfRunFinished(runID)
		} else {
			log.Printf("[DEBUG] Workflow completed successfully for run %s", runID)
			testInfo.Status = "PASSED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testInfo.Name), "green", true)
			e.checkIfRunFinished(runID)
		}
	case <-ctx.Done():
		log.Printf("[DEBUG] Monitoring timed out for test ID %s", workflowID)
		e.mu.Lock()
		if runInfo, exists := e.runs[runID]; exists {
			runInfo.Tests[workflowID].Status = "TIMEOUT"
			runInfo.Tests[workflowID].EndedAt = time.Now()
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", runInfo.Tests[workflowID].Name), "red", true)
			e.checkIfRunFinished(runID)
		} else {
			e.mu.Unlock()
		}
	}
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
	if e.isRunFinished(runID) {
		// get run name
		e.mu.RLock()
		runName := e.runs[runID].Name
		e.mu.RUnlock()
		numTests := len(e.runs[runID].Tests)
		if numTests == e.numTestsPassed(runID) {
			e.mu.Lock()
			e.runs[runID].Status = "PASSED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, numTests), "green", true)
		} else if numTests == (e.numTestsPassed(runID) + e.numTestsFailed(runID)) {
			e.mu.Lock()
			e.runs[runID].Status = "FAILED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, e.numTestsPassed(runID), numTests, e.numTestsFailed(runID), numTests), "red", true)
		} else {
			// we have tests that timed out. Print # failed and # timed out
			e.mu.Lock()
			e.runs[runID].Status = "FAILED"
			e.mu.Unlock()
			e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, e.numTestsPassed(runID), numTests, e.numTestsFailed(runID), numTests, e.numTestsTimedOut(runID), numTests), "red", true)
		}
	}
}

// helper function that checks if any tests are still in PENDING status. isRunFinished()
func (e *Engine) isRunFinished(runID string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	for _, testInfo := range e.runs[runID].Tests {
		if testInfo.Status == "PENDING" {
			return false
		}
	}
	return true
}

// number of tests in run which are in status PASSED
func (e *Engine) numTestsPassed(runID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.runs[runID].Tests)
}

// number of tests in run which are in status FAILED
func (e *Engine) numTestsFailed(runID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.runs[runID].Tests)
}

func (e *Engine) numTestsTimedOut(runID string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return len(e.runs[runID].Tests)
}
