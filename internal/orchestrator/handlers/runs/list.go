package runs

import (
	"context"
	"log/slog"
	"sort"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
)

// ListRuns implements the list runs endpoint - Phase 1 with in-memory data
func (h *Handler) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	slog.Debug("ListRuns called",
		"project_id", req.ProjectId,
		"source", req.Source,
		"branch", req.Branch,
		"status", req.Status,
		"limit", req.Limit)

	// For Phase 1, return in-memory runs with basic filtering
	// TODO: Integrate with Temporal search attributes

	h.mu.RLock()
	defer h.mu.RUnlock()

	runs := make([]*generated.RunSummary, 0)

	for _, runInfo := range h.runs {
		// Basic filtering by status
		if req.Status != "" && runInfo.Status != req.Status {
			continue
		}

		// Filter by project ID
		if req.ProjectId != "" && runInfo.Context.ProjectID != req.ProjectId {
			continue
		}

		// Filter by source
		if req.Source != "" && runInfo.Context.Source != req.Source {
			continue
		}

		// Filter by branch
		if req.Branch != "" && runInfo.Context.Branch != req.Branch {
			continue
		}

		// Filter by schedule name
		if req.ScheduleName != "" && runInfo.Context.ScheduleName != req.ScheduleName {
			continue
		}

		duration := int64(0)
		if !runInfo.EndedAt.IsZero() {
			duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
		}

		// Count test statuses
		var passed, failed, timeout int32
		for _, test := range runInfo.Tests {
			switch test.Status {
			case "PASSED":
				passed++
			case "FAILED":
				failed++
			case "TIMEOUT":
				timeout++
			}
		}

		runSummary := &generated.RunSummary{
			RunId:        runInfo.ID,
			SuiteName:    runInfo.Name,
			Status:       runInfo.Status,
			StartedAt:    runInfo.StartedAt.Format(time.RFC3339),
			EndedAt:      runInfo.EndedAt.Format(time.RFC3339),
			DurationMs:   duration,
			TotalTests:   int32(len(runInfo.Tests)),
			PassedTests:  passed,
			FailedTests:  failed,
			TimeoutTests: timeout,
			Context: &generated.RunContext{
				ProjectId:    runInfo.Context.ProjectID,
				Source:       runInfo.Context.Source,
				Branch:       runInfo.Context.Branch,
				CommitSha:    runInfo.Context.CommitSHA,
				Trigger:      runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:     runInfo.Context.Metadata,
			},
		}

		runs = append(runs, runSummary)
	}

	// Sort runs based on request parameters
	sortRuns(runs, req.OrderBy, !req.Descending)

	// Apply limit
	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(runs) > int(limit) {
		runs = runs[:limit]
	}

	result := &generated.ListRunsResponse{
		Runs:       runs,
		TotalCount: int32(len(runs)),
	}

	slog.Debug("Returning ListRuns response", "runs_count", len(runs))
	return result, nil
}

// sortRuns sorts a slice of RunSummary based on the specified field and direction
func sortRuns(runs []*generated.RunSummary, orderBy string, ascending bool) {
	sort.Slice(runs, func(i, j int) bool {
		var result bool

		switch orderBy {
		case "duration":
			result = runs[i].DurationMs < runs[j].DurationMs
		case "ended_at":
			// Parse time strings for comparison
			timeI, errI := time.Parse(time.RFC3339, runs[i].EndedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].EndedAt)
			if errI != nil || errJ != nil {
				// Fall back to string comparison if parsing fails
				result = runs[i].EndedAt < runs[j].EndedAt
			} else {
				result = timeI.Before(timeJ)
			}
		case "started_at":
		default:
			// Default to started_at
			timeI, errI := time.Parse(time.RFC3339, runs[i].StartedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].StartedAt)
			if errI != nil || errJ != nil {
				// Fall back to string comparison if parsing fails
				result = runs[i].StartedAt < runs[j].StartedAt
			} else {
				result = timeI.Before(timeJ)
			}
		}

		// Reverse the result if not ascending (i.e., descending)
		if !ascending {
			result = !result
		}

		return result
	})
}