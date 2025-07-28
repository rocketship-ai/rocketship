package runs

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/types"
)

// GetRun implements the get run endpoint
func (h *Handler) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	slog.Debug("GetRun called", "run_id", req.RunId)

	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	// First check in-memory runs for active runs
	h.mu.RLock()
	defer h.mu.RUnlock()

	// Try exact match first
	if runInfo, exists := h.runs[req.RunId]; exists {
		slog.Debug("Found active run in memory", "run_id", req.RunId)
		return mapRunInfoToRunDetails(runInfo), nil
	}

	// If not found and the ID looks like a prefix (12 chars or less), search for prefix match
	if len(req.RunId) <= 12 {
		slog.Debug("Searching for run by prefix", "prefix", req.RunId)
		for fullID, runInfo := range h.runs {
			if strings.HasPrefix(fullID, req.RunId) {
				slog.Debug("Found run by prefix match", "prefix", req.RunId, "full_id", fullID)
				return mapRunInfoToRunDetails(runInfo), nil
			}
		}
	}

	// TODO: Query Temporal for historical run data
	// For now, return not found
	slog.Debug("Run not found in memory", "run_id", req.RunId)
	return nil, fmt.Errorf("run not found: %s", req.RunId)
}

// CancelRun cancels all workflows associated with a run
func (h *Handler) CancelRun(ctx context.Context, req *generated.CancelRunRequest) (*generated.CancelRunResponse, error) {
	runID := req.RunId

	h.mu.Lock()
	runInfo, exists := h.runs[runID]
	if !exists {
		h.mu.Unlock()
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", runID),
		}, nil
	}

	// Mark run as cancelled
	runInfo.Status = "CANCELLED"

	// Cancel all workflows in this run
	cancelledCount := 0
	errorCount := 0
	for testID, testInfo := range runInfo.Tests {
		slog.Debug("Cancelling workflow", "workflow_id", testInfo.WorkflowID, "test_id", testID)
		err := h.temporal.CancelWorkflow(context.Background(), testInfo.WorkflowID, "")
		if err != nil {
			slog.Debug("Failed to cancel workflow", "workflow_id", testInfo.WorkflowID, "error", err)
			errorCount++
		} else {
			slog.Debug("Successfully cancelled workflow", "workflow_id", testInfo.WorkflowID)
			cancelledCount++
		}
	}

	h.mu.Unlock()

	message := fmt.Sprintf("Cancelled %d workflows, %d errors", cancelledCount, errorCount)

	return &generated.CancelRunResponse{
		Success: true,
		Message: message,
	}, nil
}

// mapRunInfoToRunDetails converts in-memory RunInfo to RunDetails for GetRun response
func mapRunInfoToRunDetails(runInfo *types.RunInfo) *generated.GetRunResponse {
	tests := make([]*generated.TestDetails, 0, len(runInfo.Tests))

	for _, testInfo := range runInfo.Tests {
		duration := int64(0)
		if !testInfo.EndedAt.IsZero() {
			duration = testInfo.EndedAt.Sub(testInfo.StartedAt).Milliseconds()
		}

		tests = append(tests, &generated.TestDetails{
			TestId:     testInfo.WorkflowID,
			Name:       testInfo.Name,
			Status:     testInfo.Status,
			StartedAt:  testInfo.StartedAt.Format(time.RFC3339),
			EndedAt:    testInfo.EndedAt.Format(time.RFC3339),
			DurationMs: duration,
		})
	}

	duration := int64(0)
	if !runInfo.EndedAt.IsZero() {
		duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
	}

	return &generated.GetRunResponse{
		Run: &generated.RunDetails{
			RunId:      runInfo.ID,
			SuiteName:  runInfo.Name,
			Status:     runInfo.Status,
			StartedAt:  runInfo.StartedAt.Format(time.RFC3339),
			EndedAt:    runInfo.EndedAt.Format(time.RFC3339),
			DurationMs: duration,
			Context: &generated.RunContext{
				ProjectId:    runInfo.Context.ProjectID,
				Source:       runInfo.Context.Source,
				Branch:       runInfo.Context.Branch,
				CommitSha:    runInfo.Context.CommitSHA,
				Trigger:      runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:     runInfo.Context.Metadata,
			},
			Tests: tests,
		},
	}
}