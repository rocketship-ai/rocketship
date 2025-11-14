package orchestrator

import (
	"database/sql"
	"strings"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// makeNullString creates a sql.NullString from a trimmed value
func makeNullString(value string) sql.NullString {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: trimmed, Valid: true}
}

// stringPtr creates a pointer to a string value
func stringPtr(value string) *string {
	if value == "" {
		return new(string)
	}
	v := value
	return &v
}

// timePtr creates a pointer to a time value
func timePtr(t time.Time) *time.Time {
	if t.IsZero() {
		return &time.Time{}
	}
	val := t
	return &val
}

// mapRunRecordToSummary converts a persistence RunRecord to a generated RunSummary
func mapRunRecordToSummary(rec persistence.RunRecord) *generated.RunSummary {
	start := rec.StartedAt.Time
	if !rec.StartedAt.Valid {
		start = rec.CreatedAt
	}

	var (
		endedAt  string
		duration int64
		endTime  time.Time
	)
	if rec.EndedAt.Valid {
		endTime = rec.EndedAt.Time
		endedAt = endTime.Format(time.RFC3339)
		if !start.IsZero() {
			duration = endTime.Sub(start).Milliseconds()
		}
	}

	commitSha := ""
	if rec.CommitSHA.Valid {
		commitSha = rec.CommitSHA.String
	}

	context := &generated.RunContext{
		Source:       rec.Source,
		Branch:       rec.Branch,
		CommitSha:    commitSha,
		Trigger:      rec.Trigger,
		ScheduleName: rec.ScheduleName,
		Metadata:     map[string]string{},
	}
	if rec.ProjectID.Valid {
		context.ProjectId = rec.ProjectID.UUID.String()
	}

	startedAt := ""
	if !start.IsZero() {
		startedAt = start.Format(time.RFC3339)
	}

	return &generated.RunSummary{
		RunId:        rec.ID,
		SuiteName:    rec.SuiteName,
		Status:       rec.Status,
		StartedAt:    startedAt,
		EndedAt:      endedAt,
		DurationMs:   duration,
		TotalTests:   int32(rec.TotalTests),
		PassedTests:  int32(rec.PassedTests),
		FailedTests:  int32(rec.FailedTests),
		TimeoutTests: int32(rec.TimeoutTests),
		Context:      context,
	}
}

// mapRunRecordToRunDetails converts a persistence RunRecord to a GetRunResponse
func mapRunRecordToRunDetails(rec persistence.RunRecord) *generated.GetRunResponse {
	start := rec.StartedAt.Time
	if !rec.StartedAt.Valid {
		start = rec.CreatedAt
	}

	startedAt := ""
	if !start.IsZero() {
		startedAt = start.Format(time.RFC3339)
	}

	var (
		endedAt  string
		duration int64
	)
	if rec.EndedAt.Valid {
		end := rec.EndedAt.Time
		endedAt = end.Format(time.RFC3339)
		if !start.IsZero() {
			duration = end.Sub(start).Milliseconds()
		}
	}

	commitSha := ""
	if rec.CommitSHA.Valid {
		commitSha = rec.CommitSHA.String
	}

	details := &generated.RunDetails{
		RunId:      rec.ID,
		SuiteName:  rec.SuiteName,
		Status:     rec.Status,
		StartedAt:  startedAt,
		EndedAt:    endedAt,
		DurationMs: duration,
		Context: &generated.RunContext{
			Source:       rec.Source,
			Branch:       rec.Branch,
			CommitSha:    commitSha,
			Trigger:      rec.Trigger,
			ScheduleName: rec.ScheduleName,
			Metadata:     map[string]string{},
		},
		Tests: []*generated.TestDetails{},
	}
	if rec.ProjectID.Valid {
		details.Context.ProjectId = rec.ProjectID.UUID.String()
	}

	return &generated.GetRunResponse{Run: details}
}

// makeRunTotals converts TestStatusCounts to persistence RunTotals
func makeRunTotals(counts TestStatusCounts) *persistence.RunTotals {
	return &persistence.RunTotals{
		Total:   counts.Total,
		Passed:  counts.Passed,
		Failed:  counts.Failed,
		Timeout: counts.TimedOut,
	}
}
