package runs

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/rocketship-ai/rocketship/internal/orchestrator/types"
)

// monitorWorkflow monitors a workflow execution and updates test status
func (h *Handler) monitorWorkflow(runID, workflowID, workflowRunID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	slog.Debug("Starting to monitor workflow", "workflow_id", workflowID, "run_id", runID)
	workflowRun := h.temporal.GetWorkflow(ctx, workflowID, workflowRunID)

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
		h.updateTestStatus(runID, workflowID, err)
	case <-ctx.Done():
		log.Printf("[WARN] Monitoring timed out for workflow %s in run %s", workflowID, runID)
		h.updateTestStatus(runID, workflowID, fmt.Errorf("workflow monitoring timeout"))
	}
}

// updateTestStatus updates the test status and handles cleanup atomically
func (h *Handler) updateTestStatus(runID, workflowID string, workflowErr error) {
	h.mu.Lock()
	runInfo, exists := h.runs[runID]
	if !exists {
		h.mu.Unlock()
		log.Printf("[ERROR] Run not found during status update: %s", runID)
		return
	}

	testInfo, exists := runInfo.Tests[workflowID]
	if !exists {
		h.mu.Unlock()
		log.Printf("[ERROR] Test not found during status update: %s in run %s", workflowID, runID)
		return
	}

	testInfo.EndedAt = time.Now()

	if workflowErr != nil {
		if workflowErr.Error() == "workflow monitoring timeout" {
			testInfo.Status = "TIMEOUT"
			h.mu.Unlock()
			log.Printf("[WARN] Test timed out: %s", testInfo.Name)
			h.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", testInfo.Name), "red", true)
		} else {
			testInfo.Status = "FAILED"
			h.mu.Unlock()
			log.Printf("[ERROR] Test failed: %s - %v", testInfo.Name, workflowErr)
			h.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %v", testInfo.Name, workflowErr), "red", true)
		}
	} else {
		testInfo.Status = "PASSED"
		h.mu.Unlock()
		log.Printf("[INFO] Test passed: %s", testInfo.Name)
		h.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testInfo.Name), "green", true)
	}

	h.checkIfRunFinished(runID)
}

// checkIfRunFinished checks if all tests in a run are complete and updates run status
func (h *Handler) checkIfRunFinished(runID string) {
	counts, err := h.getTestStatusCounts(runID)
	if err != nil {
		log.Printf("[ERROR] Failed to get test status counts for run %s: %v", runID, err)
		return
	}

	// Only proceed if all tests are finished (no pending tests)
	if counts.Pending > 0 {
		return
	}

	// Get run name and update status atomically
	h.mu.Lock()
	runInfo, exists := h.runs[runID]
	if !exists {
		h.mu.Unlock()
		log.Printf("[ERROR] Run not found when checking if finished: %s", runID)
		return
	}

	runName := runInfo.Name

	// Determine final status and update
	if counts.Failed == 0 && counts.TimedOut == 0 {
		// All tests passed
		runInfo.Status = "PASSED"
		runInfo.EndedAt = time.Now()
		h.mu.Unlock()
		h.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, counts.Total), "n/a", true)
	} else {
		// Some tests failed or timed out
		runInfo.Status = "FAILED"
		runInfo.EndedAt = time.Now()
		h.mu.Unlock()

		if counts.TimedOut == 0 {
			h.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total), "n/a", true)
		} else {
			h.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total, counts.TimedOut, counts.Total), "n/a", true)
		}
	}
}

// getTestStatusCounts returns all test status counts in a single lock acquisition
func (h *Handler) getTestStatusCounts(runID string) (types.TestStatusCounts, error) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	runInfo, exists := h.runs[runID]
	if !exists {
		return types.TestStatusCounts{}, fmt.Errorf("run not found: %s", runID)
	}

	counts := types.TestStatusCounts{
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