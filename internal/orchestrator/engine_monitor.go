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

	endedAt := time.Now().UTC()
	testInfo.EndedAt = endedAt
	durationMs := endedAt.Sub(testInfo.StartedAt).Milliseconds()
	orgID := runInfo.OrganizationID

	var status string
	var errMsg *string

	if workflowErr != nil {
		if workflowErr.Error() == "workflow monitoring timeout" {
			status = "TIMEOUT"
			testInfo.Status = status
			e.mu.Unlock()
			log.Printf("[WARN] Test timed out: %s", testInfo.Name)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" timed out", testInfo.Name), "red", true)
		} else {
			status = "FAILED"
			testInfo.Status = status
			e.mu.Unlock()
			cleanErr := interpreter.ExtractCleanError(workflowErr)
			errMsg = &cleanErr
			log.Printf("[ERROR] Test failed: %s - %s", testInfo.Name, cleanErr)
			e.addLog(runID, fmt.Sprintf("Test: \"%s\" failed: %s", testInfo.Name, cleanErr), "red", true)
		}
	} else {
		status = "PASSED"
		testInfo.Status = status
		e.mu.Unlock()
		log.Printf("[INFO] Test passed: %s", testInfo.Name)
		e.addLog(runID, fmt.Sprintf("Test: \"%s\" passed", testInfo.Name), "green", true)
	}

	// Persist run_test status update
	if orgID != uuid.Nil && e.runStore != nil {
		if err := e.runStore.UpdateRunTestByWorkflowID(context.Background(), workflowID, status, errMsg, endedAt, durationMs); err != nil {
			slog.Error("updateTestStatus: failed to persist run_test status", "workflow_id", workflowID, "error", err)
		}

		// Update test last_run if we have a resolved test_id
		if testInfo.TestID != uuid.Nil {
			if err := e.runStore.UpdateTestLastRun(context.Background(), testInfo.TestID, runID, status, endedAt, durationMs); err != nil {
				slog.Debug("updateTestStatus: failed to update test last_run", "test_id", testInfo.TestID, "error", err)
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
