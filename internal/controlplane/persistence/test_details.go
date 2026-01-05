package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
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
	ProjectID            uuid.UUID `db:"project_id"`
	ProjectName          string    `db:"project_name"`
	ProjectRepoURL       string    `db:"project_repo_url"`
	ProjectPathScope     []string  `db:"-"` // Parsed from JSON
	ProjectPathScopeJSON string    `db:"project_path_scope"`
	ProjectDefaultBranch string    `db:"project_default_branch"`

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
	TotalCount  int            `db:"total_count"` // For COUNT(*) OVER() pagination
}

// TestRunsResult contains paginated test run results with total count
type TestRunsResult struct {
	Runs   []TestRunSummary
	Total  int
	Limit  int
	Offset int
}

// TestRunsParams contains query parameters for ListTestRuns
type TestRunsParams struct {
	Triggers        []string // Filter by trigger types (ci, manual, schedule)
	EnvironmentSlug string   // Filter by environment slug (e.g. "staging") - matches r.environment string
	Limit           int
	Offset          int
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
			p.repo_url as project_repo_url,
			COALESCE(p.path_scope, '[]'::jsonb)::text as project_path_scope,
			p.default_branch as project_default_branch,
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

	// Parse project path_scope from JSON
	if row.ProjectPathScopeJSON != "" && row.ProjectPathScopeJSON != "[]" {
		if err := json.Unmarshal([]byte(row.ProjectPathScopeJSON), &row.ProjectPathScope); err != nil {
			row.ProjectPathScope = []string{}
		}
	} else {
		row.ProjectPathScope = []string{}
	}

	return &row, nil
}

// TestIdentity contains the fields needed to identify a logical test across branches/projects
type TestIdentity struct {
	SuiteName string
	TestName  string
	RepoURL   string
	PathScope []string
}

// ListTestRuns returns recent run_tests for a logical test identity with filtering by trigger.
// This queries across all projects in the same repo/path_scope group to show runs from feature branches.
// Returns paginated results with total count for server-side pagination.
func (s *Store) ListTestRuns(ctx context.Context, orgID uuid.UUID, identity TestIdentity, params TestRunsParams) (TestRunsResult, error) {
	// Set defaults
	limit := params.Limit
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	offset := params.Offset
	if offset < 0 {
		offset = 0
	}

	// Get all project IDs that share the same repo/path_scope (the "project group")
	projectIDs, err := s.ListProjectIDsByRepoAndPathScope(ctx, orgID, identity.RepoURL, identity.PathScope)
	if err != nil {
		return TestRunsResult{}, fmt.Errorf("failed to get project group IDs: %w", err)
	}
	if len(projectIDs) == 0 {
		return TestRunsResult{Runs: []TestRunSummary{}, Total: 0, Limit: limit, Offset: offset}, nil
	}

	// Build the query matching by suite_name and test_name across the project group
	// This avoids relying on rt.test_id which may be null for some CI flows
	// Uses COUNT(*) OVER() for efficient total count in a single query
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
			r.initiator,
			COUNT(*) OVER() AS total_count
		FROM run_tests rt
		JOIN runs r ON rt.run_id = r.id
		WHERE r.project_id = ANY($1)
		  AND r.organization_id = $2
		  AND r.suite_name = $3
		  AND lower(rt.name) = lower($4)
	`

	args := []interface{}{pq.Array(projectIDs), orgID, identity.SuiteName, identity.TestName}
	argPos := 5

	// Add trigger filter if specified
	if len(params.Triggers) > 0 {
		query += fmt.Sprintf(" AND r.trigger = ANY($%d)", argPos)
		args = append(args, pq.Array(params.Triggers))
		argPos++
	}

	// Add environment filter if specified (by slug/string, not UUID)
	if params.EnvironmentSlug != "" {
		query += fmt.Sprintf(" AND r.environment = $%d", argPos)
		args = append(args, params.EnvironmentSlug)
		argPos++
	}

	// Order by execution time (prefer started_at, fall back to created_at) for better UX
	query += fmt.Sprintf(" ORDER BY COALESCE(rt.started_at, rt.created_at) DESC LIMIT $%d OFFSET $%d", argPos, argPos+1)
	args = append(args, limit, offset)

	var rows []TestRunSummary
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return TestRunsResult{}, fmt.Errorf("failed to list test runs: %w", err)
	}

	if rows == nil {
		rows = []TestRunSummary{}
	}

	// Extract total from first row (all rows have same total_count from COUNT(*) OVER())
	total := 0
	if len(rows) > 0 {
		total = rows[0].TotalCount
	}

	return TestRunsResult{
		Runs:   rows,
		Total:  total,
		Limit:  limit,
		Offset: offset,
	}, nil
}
