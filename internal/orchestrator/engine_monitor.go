package orchestrator

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
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
	endedAt := time.Now().UTC()

	// Determine status and error message from workflow result
	var status string
	var errMsg *string
	var cleanErr string

	if workflowErr != nil {
		if workflowErr.Error() == "workflow monitoring timeout" {
			status = "TIMEOUT"
		} else {
			status = "FAILED"
			cleanErr = interpreter.ExtractCleanError(workflowErr)
			errMsg = &cleanErr
		}
	} else {
		status = "PASSED"
	}

	// CRITICAL: Always persist DB update first, regardless of in-memory state.
	// This ensures scheduled runs (and any runs where engine may have restarted)
	// get their terminal status persisted correctly.
	if e.runStore != nil {
		if err := e.runStore.UpdateRunTestByWorkflowID(context.Background(), workflowID, status, errMsg, endedAt, 0); err != nil {
			// Only log as error if it's not a "not found" error (which can happen for local runs without DB)
			slog.Error("updateTestStatus: failed to persist run_test status", "workflow_id", workflowID, "status", status, "error", err)
		} else {
			slog.Debug("updateTestStatus: persisted run_test status", "workflow_id", workflowID, "status", status)
		}
	}

	// Now update in-memory state (for streaming logs, etc.)
	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		// Run not in memory - this can happen after engine restart.
		// DB was already updated above, so just trigger run completion check via DB.
		slog.Warn("updateTestStatus: run not in memory, checking DB for run completion", "run_id", runID, "workflow_id", workflowID)
		e.checkIfRunFinishedFromDB(runID)
		return
	}

	testInfo, exists := runInfo.Tests[workflowID]
	if !exists {
		e.mu.Unlock()
		// Test not in memory but run exists - this is unusual but handle it
		slog.Warn("updateTestStatus: test not in memory", "run_id", runID, "workflow_id", workflowID)
		e.checkIfRunFinished(runID)
		return
	}

	// Update in-memory state
	testInfo.EndedAt = endedAt
	testInfo.Status = status
	durationMs := endedAt.Sub(testInfo.StartedAt).Milliseconds()
	orgID := runInfo.OrganizationID
	testID := testInfo.TestID
	testName := testInfo.Name
	e.mu.Unlock()

	// Log based on status
	if workflowErr != nil {
		if status == "TIMEOUT" {
			log.Printf("[WARN] Test timed out: %s", testName)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", testName), "red", true)
		} else {
			log.Printf("[ERROR] Test failed: %s - %s", testName, cleanErr)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %s", testName, cleanErr), "red", true)
		}
	} else {
		log.Printf("[INFO] Test passed: %s", testName)
		e.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testName), "green", true)
	}

	// Update duration now that we have it from in-memory state
	if orgID != uuid.Nil && e.runStore != nil {
		// Re-update with correct duration (the initial update above used 0)
		if err := e.runStore.UpdateRunTestByWorkflowID(context.Background(), workflowID, status, errMsg, endedAt, durationMs); err != nil {
			slog.Debug("updateTestStatus: failed to update duration", "workflow_id", workflowID, "error", err)
		}

		// Update test last_run if we have a resolved test_id
		if testID != uuid.Nil {
			if err := e.runStore.UpdateTestLastRun(context.Background(), testID, runID, status, endedAt, durationMs); err != nil {
				slog.Debug("updateTestStatus: failed to update test last_run", "test_id", testID, "error", err)
			}
		}
	}

	e.checkIfRunFinished(runID)
}

func (e *Engine) addLog(runID, message, color string, bold bool) {
	e.addLogWithContext(runID, message, color, bold, "", "")
}

func (e *Engine) addLogWithContext(runID, message, color string, bold bool, testName, stepName string) {
	e.addLogWithWorkflowContext(runID, "", message, color, bold, testName, stepName)
}

func (e *Engine) addLogWithWorkflowContext(runID, workflowID, message, color string, bold bool, testName, stepName string) {
	var orgID uuid.UUID

	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
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
	orgID = runInfo.OrganizationID
	e.mu.Unlock()

	if orgID == uuid.Nil || e.runStore == nil {
		return
	}

	// Prepare log entry with optional run_test_id linking
	logEntry := persistence.RunLog{
		RunID:    runID,
		Level:    "INFO",
		Message:  message,
		Metadata: map[string]interface{}{
			"color":     color,
			"bold":      bold,
			"test_name": testName,
			"step_name": stepName,
		},
		LoggedAt: time.Now().UTC(),
	}

	// Resolve run_test_id from workflow_id if provided
	if workflowID != "" {
		if runTest, err := e.runStore.GetRunTestByWorkflowID(context.Background(), workflowID); err == nil {
			logEntry.RunTestID = uuid.NullUUID{Valid: true, UUID: runTest.ID}
		}
	}

	if _, err := e.runStore.InsertRunLog(context.Background(), logEntry); err != nil {
		slog.Error("addLogWithWorkflowContext: failed to persist run log", "run_id", runID, "error", err)
	}
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
		suiteID := runInfo.SuiteID
		scheduleID := runInfo.ScheduleID
		scheduleType := runInfo.ScheduleType
		endTime := runInfo.EndedAt
		e.mu.Unlock()
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. All %d tests passed.", runName, counts.Total), "n/a", true)
		if orgID != uuid.Nil && e.runStore != nil {
			if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
				RunID:          runID,
				OrganizationID: orgID,
				Status:         stringPtr("PASSED"),
				EndedAt:        timePtr(endTime),
				Totals:         makeRunTotals(counts),
			}); err != nil {
				slog.Error("checkIfRunFinished: failed to persist PASSED state", "run_id", runID, "error", err)
			}
			// Update suite last_run
			if suiteID != uuid.Nil {
				if err := e.runStore.UpdateSuiteLastRun(context.Background(), suiteID, runID, "PASSED", endTime); err != nil {
					slog.Debug("checkIfRunFinished: failed to update suite last_run", "suite_id", suiteID, "error", err)
				}
			}
			// Update schedule last_run if this was a scheduled run
			if scheduleID != uuid.Nil {
				switch scheduleType {
				case "project":
					if err := e.runStore.UpdateProjectScheduleLastRun(context.Background(), scheduleID, runID, "PASSED", endTime); err != nil {
						slog.Debug("checkIfRunFinished: failed to update project schedule last_run", "schedule_id", scheduleID, "error", err)
					}
				case "suite":
					if err := e.runStore.UpdateSuiteScheduleLastRun(context.Background(), scheduleID, runID, "PASSED", endTime); err != nil {
						slog.Debug("checkIfRunFinished: failed to update suite schedule last_run", "schedule_id", scheduleID, "error", err)
					}
				}
			}
		}
		return
	}

	runInfo.Status = "FAILED"
	runInfo.EndedAt = time.Now().UTC()
	orgID := runInfo.OrganizationID
	suiteID := runInfo.SuiteID
	scheduleID := runInfo.ScheduleID
	scheduleType := runInfo.ScheduleType
	endTime := runInfo.EndedAt
	e.mu.Unlock()

	if counts.TimedOut == 0 {
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total), "n/a", true)
	} else {
		e.addLog(runID, fmt.Sprintf("Test run: \"%s\" finished. %d/%d tests passed, %d/%d tests failed, %d/%d tests timed out.", runName, counts.Passed, counts.Total, counts.Failed, counts.Total, counts.TimedOut, counts.Total), "n/a", true)
	}

	if orgID != uuid.Nil && e.runStore != nil {
		if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
			RunID:          runID,
			OrganizationID: orgID,
			Status:         stringPtr("FAILED"),
			EndedAt:        timePtr(endTime),
			Totals:         makeRunTotals(counts),
		}); err != nil {
			slog.Error("checkIfRunFinished: failed to persist FAILED state", "run_id", runID, "error", err)
		}
		// Update suite last_run
		if suiteID != uuid.Nil {
			if err := e.runStore.UpdateSuiteLastRun(context.Background(), suiteID, runID, "FAILED", endTime); err != nil {
				slog.Debug("checkIfRunFinished: failed to update suite last_run", "suite_id", suiteID, "error", err)
			}
		}
		// Update schedule last_run if this was a scheduled run
		if scheduleID != uuid.Nil {
			switch scheduleType {
			case "project":
				if err := e.runStore.UpdateProjectScheduleLastRun(context.Background(), scheduleID, runID, "FAILED", endTime); err != nil {
					slog.Debug("checkIfRunFinished: failed to update project schedule last_run", "schedule_id", scheduleID, "error", err)
				}
			case "suite":
				if err := e.runStore.UpdateSuiteScheduleLastRun(context.Background(), scheduleID, runID, "FAILED", endTime); err != nil {
					slog.Debug("checkIfRunFinished: failed to update suite schedule last_run", "schedule_id", scheduleID, "error", err)
				}
			}
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

// checkIfRunFinishedFromDB checks run completion status directly from the database.
// This is used when in-memory state is not available (e.g., after engine restart).
func (e *Engine) checkIfRunFinishedFromDB(runID string) {
	if e.runStore == nil {
		return
	}

	ctx := context.Background()

	// Get all run_tests for this run from DB
	runTests, err := e.runStore.ListRunTests(ctx, runID)
	if err != nil {
		slog.Error("checkIfRunFinishedFromDB: failed to list run_tests", "run_id", runID, "error", err)
		return
	}

	if len(runTests) == 0 {
		slog.Debug("checkIfRunFinishedFromDB: no run_tests found", "run_id", runID)
		return
	}

	// Count statuses from DB
	counts := TestStatusCounts{Total: len(runTests)}
	for _, rt := range runTests {
		switch rt.Status {
		case "PASSED":
			counts.Passed++
		case "FAILED":
			counts.Failed++
		case "TIMEOUT":
			counts.TimedOut++
		case "PENDING", "RUNNING":
			counts.Pending++
		}
	}

	// If any tests still pending/running, run is not finished
	if counts.Pending > 0 {
		slog.Debug("checkIfRunFinishedFromDB: run still has pending tests", "run_id", runID, "pending", counts.Pending)
		return
	}

	// All tests complete - determine final status
	endTime := time.Now().UTC()
	var finalStatus string
	if counts.Failed == 0 && counts.TimedOut == 0 {
		finalStatus = "PASSED"
	} else {
		finalStatus = "FAILED"
	}

	slog.Info("checkIfRunFinishedFromDB: all tests complete, updating run status",
		"run_id", runID,
		"status", finalStatus,
		"passed", counts.Passed,
		"failed", counts.Failed,
		"timeout", counts.TimedOut,
	)

	// Get run record to find org_id
	// We need to use a method that doesn't require org_id first
	runTests2, err := e.runStore.ListRunTests(ctx, runID)
	if err != nil || len(runTests2) == 0 {
		slog.Error("checkIfRunFinishedFromDB: cannot determine org_id", "run_id", runID)
		return
	}

	// Use the UpdateRunByID method to update run status directly
	if err := e.runStore.UpdateRunStatusByID(ctx, runID, finalStatus, endTime, makeRunTotals(counts)); err != nil {
		slog.Error("checkIfRunFinishedFromDB: failed to update run status", "run_id", runID, "error", err)
	}
}
