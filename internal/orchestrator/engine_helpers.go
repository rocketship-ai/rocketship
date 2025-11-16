package orchestrator

import (
	"crypto/rand"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
)

var (
	ulidEntropy = ulid.Monotonic(rand.Reader, 0)
	ulidMutex   sync.Mutex
)

func generateID() (string, error) {
	ulidMutex.Lock()
	defer ulidMutex.Unlock()

	id, err := ulid.New(ulid.Timestamp(time.Now()), ulidEntropy)
	if err != nil {
		return "", err
	}
	return strings.ToLower(id.String()), nil
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

func cloneStringMap(source map[string]string) map[string]string {
	if len(source) == 0 {
		return make(map[string]string)
	}
	result := make(map[string]string, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
}

func cloneInterfaceMap(source map[string]interface{}) map[string]interface{} {
	if len(source) == 0 {
		return make(map[string]interface{})
	}
	result := make(map[string]interface{}, len(source))
	for k, v := range source {
		result[k] = v
	}
	return result
}

func extractSavedValues(state map[string]string) map[string]string {
	if len(state) == 0 {
		return make(map[string]string)
	}
	result := make(map[string]string)
	for key, value := range state {
		result[key] = value
	}
	return result
}
