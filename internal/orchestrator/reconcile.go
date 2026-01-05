package orchestrator

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	enumspb "go.temporal.io/api/enums/v1"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
	"go.temporal.io/sdk/client"
)

const (
	// DefaultRunningGracePeriod is how long a run_test can be in RUNNING/PENDING before Temporal reconciliation
	// This is the fast-path threshold for checking Temporal status
	DefaultRunningGracePeriod = 3 * time.Minute
	// DefaultStaleRunThreshold is the safety-net threshold for force-failing truly stale runs
	// This is only used when Temporal queries fail or workflows are not found
	DefaultStaleRunThreshold = 2 * time.Hour
	// DefaultReconcileInterval is how often the reconciler checks for stale runs
	DefaultReconcileInterval = 1 * time.Minute
	// DefaultReconcileBatchSize is how many stale run_tests to process per reconciliation cycle
	DefaultReconcileBatchSize = 50
)

// Reconciler cleans up stale run statuses by querying Temporal for actual workflow status.
// This handles cases where the engine restarted or crashed during test execution,
// or where workflow completion updates were missed.
type Reconciler struct {
	engine             *Engine
	runningGracePeriod time.Duration // Grace period before Temporal reconciliation
	staleRunThreshold  time.Duration // Safety-net threshold for force-fail
	reconcileInterval  time.Duration
	batchSize          int
	stopCh             chan struct{}
	logger             *slog.Logger
}

// NewReconciler creates a new reconciler for cleaning up stale run statuses.
func NewReconciler(engine *Engine, logger *slog.Logger) *Reconciler {
	runningGrace := DefaultRunningGracePeriod
	if envGrace := os.Getenv("ROCKETSHIP_RECONCILE_RUNNING_GRACE_MINUTES"); envGrace != "" {
		if mins, err := strconv.Atoi(envGrace); err == nil && mins > 0 {
			runningGrace = time.Duration(mins) * time.Minute
		}
	}

	staleThreshold := DefaultStaleRunThreshold
	if envThreshold := os.Getenv("ROCKETSHIP_STALE_RUN_THRESHOLD_MINUTES"); envThreshold != "" {
		if mins, err := strconv.Atoi(envThreshold); err == nil && mins > 0 {
			staleThreshold = time.Duration(mins) * time.Minute
		}
	}

	reconcileInterval := DefaultReconcileInterval
	if envInterval := os.Getenv("ROCKETSHIP_RECONCILE_INTERVAL_MINUTES"); envInterval != "" {
		if mins, err := strconv.Atoi(envInterval); err == nil && mins > 0 {
			reconcileInterval = time.Duration(mins) * time.Minute
		}
	}

	batchSize := DefaultReconcileBatchSize
	if envBatch := os.Getenv("ROCKETSHIP_RECONCILE_BATCH_SIZE"); envBatch != "" {
		if size, err := strconv.Atoi(envBatch); err == nil && size > 0 {
			batchSize = size
		}
	}

	return &Reconciler{
		engine:             engine,
		runningGracePeriod: runningGrace,
		staleRunThreshold:  staleThreshold,
		reconcileInterval:  reconcileInterval,
		batchSize:          batchSize,
		stopCh:             make(chan struct{}),
		logger:             logger,
	}
}

// Start begins the reconciliation loop
func (r *Reconciler) Start() {
	go r.run()
	r.logger.Info("reconciler started",
		"running_grace", r.runningGracePeriod,
		"stale_threshold", r.staleRunThreshold,
		"interval", r.reconcileInterval,
		"batch_size", r.batchSize,
	)
}

// Stop gracefully shuts down the reconciler
func (r *Reconciler) Stop() {
	close(r.stopCh)
	r.logger.Info("reconciler stopped")
}

func (r *Reconciler) run() {
	// Run immediately on start to clean up any stale runs from before engine restart
	r.reconcile()

	ticker := time.NewTicker(r.reconcileInterval)
	defer ticker.Stop()

	for {
		select {
		case <-r.stopCh:
			return
		case <-ticker.C:
			r.reconcile()
		}
	}
}

func (r *Reconciler) reconcile() {
	if r.engine.runStore == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Phase 1: Fast reconciliation using Temporal Describe API
	// Find run_tests that have been PENDING/RUNNING longer than the grace period
	graceThreshold := time.Now().UTC().Add(-r.runningGracePeriod)
	staleTests, err := r.engine.runStore.ListStaleRunTests(ctx, graceThreshold, r.batchSize)
	if err != nil {
		r.logger.Error("reconcile: failed to list stale run_tests", "error", err)
	} else if len(staleTests) > 0 {
		r.logger.Info("reconcile: found stale run_tests to check against Temporal",
			"count", len(staleTests),
			"grace_threshold", r.runningGracePeriod,
		)

		// Track which runs need status recalculation
		affectedRuns := make(map[string]struct{})

		for _, test := range staleTests {
			if updated := r.reconcileRunTestWithTemporal(ctx, test); updated {
				affectedRuns[test.RunID] = struct{}{}
			}
		}

		// Recalculate run statuses for affected runs
		for runID := range affectedRuns {
			r.recalculateRunStatus(ctx, runID)
		}
	} else {
		r.logger.Debug("reconcile: no stale run_tests found")
	}

	// Phase 2: Safety-net for truly stale runs (fallback when Temporal queries fail)
	// This handles edge cases where workflows are completely gone from Temporal
	staleRunThreshold := time.Now().UTC().Add(-r.staleRunThreshold)
	staleRuns, err := r.engine.runStore.ListStaleRunningRuns(ctx, staleRunThreshold, r.batchSize)
	if err != nil {
		r.logger.Error("reconcile: failed to list stale runs for safety-net", "error", err)
		return
	}

	if len(staleRuns) > 0 {
		r.logger.Warn("reconcile: found truly stale runs to force-complete",
			"count", len(staleRuns),
			"stale_threshold", r.staleRunThreshold,
		)

		for _, run := range staleRuns {
			r.forceCompleteStaleRun(ctx, run.ID)
		}
	}
}

// reconcileRunTestWithTemporal queries Temporal for the actual workflow status and updates the DB.
// Returns true if the status was updated.
func (r *Reconciler) reconcileRunTestWithTemporal(ctx context.Context, test persistence.StaleRunTest) bool {
	if test.WorkflowID == "" {
		return false
	}

	// Query Temporal for workflow status using DescribeWorkflowExecution
	desc, err := r.engine.temporal.DescribeWorkflowExecution(ctx, test.WorkflowID, "")
	if err != nil {
		// Workflow not found in Temporal - it's either very old and cleaned up,
		// or was never started. Don't force-fail here; let the safety-net handle it.
		r.logger.Debug("reconcile: could not describe workflow",
			"workflow_id", test.WorkflowID,
			"test_name", test.Name,
			"error", err,
		)
		return false
	}

	if desc.WorkflowExecutionInfo == nil {
		r.logger.Warn("reconcile: workflow describe returned nil info",
			"workflow_id", test.WorkflowID,
			"test_name", test.Name,
		)
		return false
	}

	temporalStatus := desc.WorkflowExecutionInfo.Status
	dbStatus := mapTemporalStatusToDBStatus(temporalStatus)

	// If Temporal says it's still running, leave it alone
	if dbStatus == "" {
		r.logger.Debug("reconcile: workflow still running in Temporal",
			"workflow_id", test.WorkflowID,
			"test_name", test.Name,
		)
		return false
	}

	// Workflow has completed - update the DB
	endedAt := time.Now().UTC()
	if desc.WorkflowExecutionInfo.CloseTime != nil {
		endedAt = desc.WorkflowExecutionInfo.CloseTime.AsTime()
	}

	r.logger.Info("reconcile: updating run_test status from Temporal",
		"workflow_id", test.WorkflowID,
		"test_name", test.Name,
		"temporal_status", temporalStatus.String(),
		"db_status", dbStatus,
	)

	if err := r.engine.runStore.UpdateRunTestStatus(ctx, test.ID, dbStatus, endedAt); err != nil {
		r.logger.Error("reconcile: failed to update run_test status",
			"workflow_id", test.WorkflowID,
			"test_name", test.Name,
			"error", err,
		)
		return false
	}

	return true
}

// mapTemporalStatusToDBStatus maps Temporal workflow execution status to DB status.
// Returns empty string if the workflow is still running.
func mapTemporalStatusToDBStatus(status enumspb.WorkflowExecutionStatus) string {
	switch status {
	case enumspb.WORKFLOW_EXECUTION_STATUS_COMPLETED:
		return "PASSED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_FAILED:
		return "FAILED"
	case enumspb.WORKFLOW_EXECUTION_STATUS_TIMED_OUT:
		return "TIMEOUT"
	case enumspb.WORKFLOW_EXECUTION_STATUS_CANCELED, enumspb.WORKFLOW_EXECUTION_STATUS_TERMINATED:
		return "FAILED" // Treat cancelled/terminated as failed
	case enumspb.WORKFLOW_EXECUTION_STATUS_CONTINUED_AS_NEW:
		return "" // Still running (continued)
	case enumspb.WORKFLOW_EXECUTION_STATUS_RUNNING:
		return "" // Still running
	default:
		return "" // Unknown status, treat as running
	}
}

// recalculateRunStatus recalculates and updates a run's status based on its run_tests.
func (r *Reconciler) recalculateRunStatus(ctx context.Context, runID string) {
	runTests, err := r.engine.runStore.ListRunTests(ctx, runID)
	if err != nil {
		r.logger.Error("reconcile: failed to list run_tests for status recalculation",
			"run_id", runID,
			"error", err,
		)
		return
	}

	// Count statuses
	var passed, failed, timeout, pending int
	for _, rt := range runTests {
		switch rt.Status {
		case "PASSED":
			passed++
		case "FAILED":
			failed++
		case "TIMEOUT":
			timeout++
		case "PENDING", "RUNNING":
			pending++
		}
	}

	total := len(runTests)

	// Only update if all tests are in terminal state
	if pending > 0 {
		r.logger.Debug("reconcile: run still has pending tests",
			"run_id", runID,
			"pending", pending,
		)
		return
	}

	var finalStatus string
	if failed == 0 && timeout == 0 {
		finalStatus = "PASSED"
	} else {
		finalStatus = "FAILED"
	}

	endTime := time.Now().UTC()
	totals := TestStatusCounts{
		Total:    total,
		Passed:   passed,
		Failed:   failed,
		TimedOut: timeout,
	}

	r.logger.Info("reconcile: all tests complete, marking run as terminal",
		"run_id", runID,
		"status", finalStatus,
		"passed", passed,
		"failed", failed,
		"timeout", timeout,
	)

	if err := r.engine.runStore.UpdateRunStatusByID(ctx, runID, finalStatus, endTime, makeRunTotals(totals)); err != nil {
		r.logger.Error("reconcile: failed to update run status",
			"run_id", runID,
			"error", err,
		)
	}
}

// forceCompleteStaleRun is the safety-net that force-completes truly stale runs.
// This is only called for runs that are older than the stale threshold AND
// whose workflows couldn't be queried from Temporal.
func (r *Reconciler) forceCompleteStaleRun(ctx context.Context, runID string) {
	r.logger.Warn("reconcile: force-completing stale run (safety-net)",
		"run_id", runID,
		"stale_threshold", r.staleRunThreshold,
	)

	// Get all run_tests for this run
	runTests, err := r.engine.runStore.ListRunTests(ctx, runID)
	if err != nil {
		r.logger.Error("reconcile: failed to list run_tests for force-complete",
			"run_id", runID,
			"error", err,
		)
		return
	}

	// Count statuses
	var passed, failed, timeout, pending int
	for _, rt := range runTests {
		switch rt.Status {
		case "PASSED":
			passed++
		case "FAILED":
			failed++
		case "TIMEOUT":
			timeout++
		case "PENDING", "RUNNING":
			pending++
		}
	}

	total := len(runTests)
	endTime := time.Now().UTC()

	// Force-complete the stale run_tests
	if pending > 0 {
		if err := r.engine.runStore.ForceCompleteStaleRunTests(ctx, runID, "FAILED"); err != nil {
			r.logger.Error("reconcile: failed to force-complete stale run_tests",
				"run_id", runID,
				"error", err,
			)
			return
		}
	}

	// Update run status
	totals := TestStatusCounts{
		Total:    total,
		Passed:   passed,
		Failed:   failed + pending, // Pending tests are now failed
		TimedOut: timeout,
	}

	if err := r.engine.runStore.UpdateRunStatusByID(ctx, runID, "FAILED", endTime, makeRunTotals(totals)); err != nil {
		r.logger.Error("reconcile: failed to update run status",
			"run_id", runID,
			"error", err,
		)
	}
}

// ReconcileOnce runs a single reconciliation cycle (useful for testing or manual invocation)
func (r *Reconciler) ReconcileOnce() {
	r.reconcile()
}

// TemporalClient interface for dependency injection in tests
type TemporalClient interface {
	client.Client
}
