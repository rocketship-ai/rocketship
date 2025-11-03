package orchestrator

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
)

func (e *Engine) monitorWorkflow(runID, workflowID, workflowRunID string) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	slog.Debug("Starting to monitor workflow", "workflow_id", workflowID, "run_id", runID)
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
			cleanErr := interpreter.ExtractCleanError(workflowErr)
			log.Printf("[ERROR] Test failed: %s - %s", testInfo.Name, cleanErr)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %s", testInfo.Name, cleanErr), "red", true)
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
	e.addLogWithContext(runID, message, color, bold, "", "")
}

func (e *Engine) addLogWithContext(runID, message, color string, bold bool, testName, stepName string) {
	e.mu.Lock()
	defer e.mu.Unlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		log.Printf("[WARN] Run %s not found when trying to add log", runID)
		return
	}

	runInfo.Logs = append(runInfo.Logs, LogLine{
		Msg:      message,
		Color:    color,
		Bold:     bold,
		TestName: testName,
		StepName: stepName,
	})
}

func (e *Engine) checkIfRunFinished(runID string) {
	counts, err := e.getTestStatusCounts(runID)
	if err != nil {
		log.Printf("[ERROR] Failed to get test status counts for run %s: %v", runID, err)
		return
	}

	if counts.Pending > 0 {
		return
	}

	hasFailure := counts.Failed > 0 || counts.TimedOut > 0
	e.mu.RLock()
	if runInfo, exists := e.runs[runID]; exists && runInfo.SuiteInitFailed {
		hasFailure = true
	}
	e.mu.RUnlock()

	e.triggerSuiteCleanup(runID, hasFailure)

	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		log.Printf("[ERROR] Run not found when checking if finished: %s", runID)
		return
	}

	runName := runInfo.Name

	if counts.Failed == 0 && counts.TimedOut == 0 {
		runInfo.Status = "PASSED"
		runInfo.EndedAt = time.Now().UTC()
		orgID := runInfo.OrganizationID
		endTime := runInfo.EndedAt
		e.mu.Unlock()
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, counts.Total), "n/a", true)
		if orgID != uuid.Nil {
			if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
				RunID:          runID,
				OrganizationID: orgID,
				Status:         stringPtr("PASSED"),
				EndedAt:        timePtr(endTime),
				Totals:         makeRunTotals(counts),
			}); err != nil {
				slog.Error("checkIfRunFinished: failed to persist PASSED state", "run_id", runID, "error", err)
			}
		}
		return
	}

	runInfo.Status = "FAILED"
	runInfo.EndedAt = time.Now().UTC()
	orgID := runInfo.OrganizationID
	endTime := runInfo.EndedAt
	e.mu.Unlock()

	if counts.TimedOut == 0 {
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total), "n/a", true)
	} else {
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total, counts.TimedOut, counts.Total), "n/a", true)
	}

	if orgID != uuid.Nil {
		if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
			RunID:          runID,
			OrganizationID: orgID,
			Status:         stringPtr("FAILED"),
			EndedAt:        timePtr(endTime),
			Totals:         makeRunTotals(counts),
		}); err != nil {
			slog.Error("checkIfRunFinished: failed to persist FAILED state", "run_id", runID, "error", err)
		}
	}
}

func (e *Engine) getTestStatusCounts(runID string) (TestStatusCounts, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runInfo, exists := e.runs[runID]
	if !exists {
		return TestStatusCounts{}, fmt.Errorf("run not found: %s", runID)
	}

	counts := TestStatusCounts{Total: len(runInfo.Tests)}

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
