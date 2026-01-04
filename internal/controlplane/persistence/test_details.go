package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// TestDetailRow represents a single test with enriched step definitions
type TestDetailRow struct {
	// Test info
	ID          uuid.UUID      `db:"id"`
	Name        string         `db:"name"`
	Description sql.NullString `db:"description"`
	SourceRef   string         `db:"source_ref"`
	StepCount   int            `db:"step_count"`

	// Suite info
	SuiteID   uuid.UUID `db:"suite_id"`
	SuiteName string    `db:"suite_name"`

	// Project info
	ProjectID   uuid.UUID `db:"project_id"`
	ProjectName string    `db:"project_name"`

	// Parsed step summaries with enriched config
	StepSummaries []StepSummary `db:"-"`

	// Raw JSONB for parsing
	StepSummariesJSON string `db:"step_summaries_json"`

	// Timestamps
	CreatedAt time.Time `db:"created_at"`
	UpdatedAt time.Time `db:"updated_at"`
}

// TestRunSummary represents a run_test result for the test detail page sidebar
type TestRunSummary struct {
	ID          uuid.UUID      `db:"id"`
	RunID       string         `db:"run_id"`
	Status      string         `db:"status"`
	DurationMs  sql.NullInt64  `db:"duration_ms"`
	CreatedAt   time.Time      `db:"created_at"`
	StartedAt   sql.NullTime   `db:"started_at"`
	EndedAt     sql.NullTime   `db:"ended_at"`
	Trigger     string         `db:"trigger"`
	Environment string         `db:"environment"`
	Branch      string         `db:"branch"`
	CommitSHA   sql.NullString `db:"commit_sha"`
	Initiator   string         `db:"initiator"`
}

// TestRunsParams contains query parameters for ListTestRuns
type TestRunsParams struct {
	Triggers      []string // Filter by trigger types (ci, manual, schedule)
	EnvironmentID uuid.NullUUID
	Limit         int
	Offset        int
}

// GetTestDetail returns a test with its enriched step summaries and project/suite info
func (s *Store) GetTestDetail(ctx context.Context, orgID uuid.UUID, testID uuid.UUID) (*TestDetailRow, error) {
	const query = `
		SELECT
			t.id,
			t.name,
			t.description,
			t.source_ref,
			t.step_count,
			t.suite_id,
			s.name as suite_name,
			p.id as project_id,
			p.name as project_name,
			COALESCE(t.step_summaries, '[]'::jsonb)::text as step_summaries_json,
			t.created_at,
			t.updated_at
		FROM tests t
		JOIN suites s ON t.suite_id = s.id
		JOIN projects p ON s.project_id = p.id
		WHERE t.id = $1
		  AND p.organization_id = $2
		  AND t.is_active = true
		  AND s.is_active = true
		  AND p.is_active = true
	`

	var row TestDetailRow
	if err := s.db.GetContext(ctx, &row, query, testID, orgID); err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to get test detail: %w", err)
	}

	// Parse step summaries from JSON
	if row.StepSummariesJSON != "" && row.StepSummariesJSON != "[]" {
		if err := json.Unmarshal([]byte(row.StepSummariesJSON), &row.StepSummaries); err != nil {
			// Log error but don't fail - return empty step summaries
			row.StepSummaries = []StepSummary{}
		}
	} else {
		row.StepSummaries = []StepSummary{}
	}

	return &row, nil
}

// ListTestRuns returns recent run_tests for a specific test with filtering by trigger
func (s *Store) ListTestRuns(ctx context.Context, orgID uuid.UUID, testID uuid.UUID, params TestRunsParams) ([]TestRunSummary, error) {
	// Set defaults
	limit := params.Limit
	if limit <= 0 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}

	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	// Build the query with optional trigger filter
	query := `
		SELECT
			rt.id,
			rt.run_id,
			rt.status,
			rt.duration_ms,
			rt.created_at,
			rt.started_at,
			rt.ended_at,
			r.trigger,
			r.environment,
			r.branch,
			r.commit_sha,
			r.initiator
		FROM run_tests rt
		JOIN runs r ON rt.run_id = r.id
		JOIN projects p ON r.project_id = p.id
		WHERE rt.test_id = $1
		  AND p.organization_id = $2
	`

	args := []interface{}{testID, orgID}
	argPos := 3

	// Add trigger filter if specified
	if len(params.Triggers) > 0 {
		query += fmt.Sprintf(" AND r.trigger = ANY($%d)", argPos)
		args = append(args, params.Triggers)
		argPos++
	}

	// Add environment filter if specified
	if params.EnvironmentID.Valid {
		query += fmt.Sprintf(" AND r.environment_id = $%d", argPos)
		args = append(args, params.EnvironmentID.UUID)
		argPos++
	}

	// Order by execution time (prefer started_at, fall back to created_at) for better UX
	query += fmt.Sprintf(" ORDER BY COALESCE(rt.started_at, rt.created_at) DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	var rows []TestRunSummary
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, fmt.Errorf("failed to list test runs: %w", err)
	}

	return rows, nil
}
