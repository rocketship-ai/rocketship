package orchestrator

import (
	"crypto/rand"
	"encoding/hex"
	"sort"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
)

func generateID() (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func sortRuns(runs []*generated.RunSummary, orderBy string, ascending bool) {
	sort.Slice(runs, func(i, j int) bool {
		var result bool

		switch orderBy {
		case "duration":
			result = runs[i].DurationMs < runs[j].DurationMs
		case "ended_at":
			timeI, errI := time.Parse(time.RFC3339, runs[i].EndedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].EndedAt)
			if errI != nil || errJ != nil {
				result = runs[i].EndedAt < runs[j].EndedAt
			} else {
				result = timeI.Before(timeJ)
			}
		case "started_at":
			fallthrough
		default:
			timeI, errI := time.Parse(time.RFC3339, runs[i].StartedAt)
			timeJ, errJ := time.Parse(time.RFC3339, runs[j].StartedAt)
			if errI != nil || errJ != nil {
				result = runs[i].StartedAt < runs[j].StartedAt
			} else {
				result = timeI.Before(timeJ)
			}
		}

		if !ascending {
			result = !result
		}

		return result
	})
}

func mapRunInfoToRunDetails(runInfo *RunInfo) *generated.GetRunResponse {
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
