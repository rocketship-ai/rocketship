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

// SuiteRunRow represents a run for suite activity display
type SuiteRunRow struct {
	ID            string         `db:"id"`
	Status        string         `db:"status"`
	Branch        string         `db:"branch"`
	CommitSHA     sql.NullString `db:"commit_sha"`
	CommitMessage sql.NullString `db:"commit_message"`
	ConfigSource  string         `db:"config_source"`
	Environment   string         `db:"environment"`
	Initiator     string         `db:"initiator"`
	Trigger       string         `db:"trigger"`
	ScheduleName  string         `db:"schedule_name"`
	ScheduleID    uuid.NullUUID  `db:"schedule_id"`
	Source        string         `db:"source"`
	TotalTests    int            `db:"total_tests"`
	PassedTests   int            `db:"passed_tests"`
	FailedTests   int            `db:"failed_tests"`
	TimeoutTests  int            `db:"timeout_tests"`
	SkippedTests  int            `db:"skipped_tests"`
	CreatedAt     time.Time      `db:"created_at"`
	StartedAt     sql.NullTime   `db:"started_at"`
	EndedAt       sql.NullTime   `db:"ended_at"`
}

// ListProjectIDsByRepoAndPathScope returns all project IDs in an org that share
// the same repo URL and path_scope (i.e., represent the same .rocketship directory
// across different branches/refs).
func (s *Store) ListProjectIDsByRepoAndPathScope(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) ([]uuid.UUID, error) {
	// Convert pathScope to JSON for comparison
	pathScopeJSON, err := json.Marshal(pathScope)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal path_scope: %w", err)
	}

	const query = `
		SELECT id
		FROM projects
		WHERE organization_id = $1
		  AND repo_url = $2
		  AND path_scope = $3::jsonb
	`

	var ids []uuid.UUID
	if err := s.db.SelectContext(ctx, &ids, query, orgID, repoURL, string(pathScopeJSON)); err != nil {
		return nil, fmt.Errorf("failed to list project IDs by repo and path_scope: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// SuiteRunsFilter holds filter parameters for suite run queries
type SuiteRunsFilter struct {
	EnvironmentSlug string   // Filter by environment slug (empty = all)
	Triggers        []string // Filter by trigger types (empty = all)
	Search          string   // Search in commit message and SHA
}

// ListRunsForSuiteGroup returns runs for a suite across all projects in a group
// (projects that share the same repo/path_scope).
//
// Shows up to 3 branches:
//  1. The default branch (always shown if it has runs)
//  2. Up to 2 additional non-default branches with the most recent activity
//
// Each branch shows up to runsPerBranch runs (default 5).
// Results are ordered by: default branch first, then by latest run time desc, then by created_at desc.
// Use filter to apply environment, trigger, and search filters.
func (s *Store) ListRunsForSuiteGroup(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName, defaultBranch string, runsPerBranch int, filter SuiteRunsFilter) ([]SuiteRunRow, error) {
	if runsPerBranch <= 0 {
		runsPerBranch = 5
	}
	if runsPerBranch > 20 {
		runsPerBranch = 20
	}

	if len(projectIDs) == 0 {
		return []SuiteRunRow{}, nil
	}

	// Strategy:
	// 1. branch_stats: Get the latest run time for each branch
	// 2. branch_ranking: Rank non-default branches by recency; default branch gets rank 0
	// 3. top_branches: Select default branch + top 2 non-default branches
	// 4. ranked_runs: Get runs for selected branches, numbered within each branch
	// 5. Final SELECT: Filter to runsPerBranch runs per branch, ordered properly
	const query = `
		WITH branch_stats AS (
			SELECT
				branch,
				MAX(created_at) as latest_run,
				CASE WHEN branch = $5 THEN 1 ELSE 0 END as is_default
			FROM runs
			WHERE organization_id = $1
			  AND project_id = ANY($2)
			  AND suite_name = $3
			  AND ($6 = '' OR environment = $6)
			  AND ($7::text[] IS NULL OR trigger = ANY($7))
			  AND ($8 = '' OR COALESCE(commit_message, '') ILIKE '%' || $8 || '%' OR COALESCE(commit_sha, '') ILIKE '%' || $8 || '%')
			GROUP BY branch
		),
		branch_ranking AS (
			SELECT
				branch,
				latest_run,
				is_default,
				CASE
					WHEN is_default = 1 THEN 0
					ELSE ROW_NUMBER() OVER (
						PARTITION BY is_default
						ORDER BY latest_run DESC
					)
				END as branch_rank
			FROM branch_stats
		),
		top_branches AS (
			SELECT branch, latest_run
			FROM branch_ranking
			WHERE is_default = 1 OR branch_rank <= 2
		),
		ranked_runs AS (
			SELECT
				r.id,
				r.status,
				r.branch,
				r.commit_sha,
				r.commit_message,
				r.config_source,
				r.environment,
				r.initiator,
				r.trigger,
				r.schedule_name,
				r.schedule_id,
				r.source,
				r.total_tests,
				r.passed_tests,
				r.failed_tests,
				r.timeout_tests,
				r.skipped_tests,
				r.created_at,
				r.started_at,
				r.ended_at,
				ROW_NUMBER() OVER (PARTITION BY r.branch ORDER BY r.created_at DESC) as row_num,
				tb.latest_run as branch_latest_run
			FROM runs r
			INNER JOIN top_branches tb ON r.branch = tb.branch
			WHERE r.organization_id = $1
			  AND r.project_id = ANY($2)
			  AND r.suite_name = $3
			  AND ($6 = '' OR r.environment = $6)
			  AND ($7::text[] IS NULL OR r.trigger = ANY($7))
			  AND ($8 = '' OR COALESCE(r.commit_message, '') ILIKE '%' || $8 || '%' OR COALESCE(r.commit_sha, '') ILIKE '%' || $8 || '%')
		)
		SELECT
			id,
			status,
			branch,
			commit_sha,
			commit_message,
			config_source,
			environment,
			initiator,
			trigger,
			schedule_name,
			schedule_id,
			source,
			total_tests,
			passed_tests,
			failed_tests,
			timeout_tests,
			skipped_tests,
			created_at,
			started_at,
			ended_at
		FROM ranked_runs
		WHERE row_num <= $4
		ORDER BY
			CASE WHEN branch = $5 THEN 0 ELSE 1 END,
			branch_latest_run DESC,
			created_at DESC
	`

	// Convert triggers to pq.Array (nil if empty for SQL NULL)
	var triggersArg interface{}
	if len(filter.Triggers) > 0 {
		triggersArg = pq.Array(filter.Triggers)
	}

	var runs []SuiteRunRow
	if err := s.db.SelectContext(ctx, &runs, query, orgID, pq.Array(projectIDs), suiteName, runsPerBranch, defaultBranch, filter.EnvironmentSlug, triggersArg, filter.Search); err != nil {
		return nil, fmt.Errorf("failed to list runs for suite group: %w", err)
	}
	if runs == nil {
		runs = []SuiteRunRow{}
	}
	return runs, nil
}

// SuiteRunsBranchResult holds paginated results for branch drilldown view
type SuiteRunsBranchResult struct {
	Runs   []SuiteRunRow
	Total  int
	Limit  int
	Offset int
	Branch string
}

// ListRunsForSuiteBranch returns paginated runs for a specific branch with filters.
// Used for branch drilldown view with pagination.
func (s *Store) ListRunsForSuiteBranch(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName, branch string, filter SuiteRunsFilter, limit, offset int) (SuiteRunsBranchResult, error) {
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	if len(projectIDs) == 0 {
		return SuiteRunsBranchResult{Runs: []SuiteRunRow{}, Branch: branch, Limit: limit, Offset: offset}, nil
	}

	// Single query with COUNT(*) OVER() for total
	const query = `
		SELECT
			id,
			status,
			branch,
			commit_sha,
			commit_message,
			config_source,
			environment,
			initiator,
			trigger,
			schedule_name,
			schedule_id,
			source,
			total_tests,
			passed_tests,
			failed_tests,
			timeout_tests,
			skipped_tests,
			created_at,
			started_at,
			ended_at,
			COUNT(*) OVER() as total_count
		FROM runs
		WHERE organization_id = $1
		  AND project_id = ANY($2)
		  AND suite_name = $3
		  AND branch = $4
		  AND ($5 = '' OR environment = $5)
		  AND ($6::text[] IS NULL OR trigger = ANY($6))
		  AND ($7 = '' OR COALESCE(commit_message, '') ILIKE '%' || $7 || '%' OR COALESCE(commit_sha, '') ILIKE '%' || $7 || '%')
		ORDER BY created_at DESC
		LIMIT $8 OFFSET $9
	`

	// Row type includes total_count
	type runWithCount struct {
		SuiteRunRow
		TotalCount int `db:"total_count"`
	}

	// Convert triggers to pq.Array (nil if empty for SQL NULL)
	var triggersArg interface{}
	if len(filter.Triggers) > 0 {
		triggersArg = pq.Array(filter.Triggers)
	}

	var runsWithCount []runWithCount
	if err := s.db.SelectContext(ctx, &runsWithCount, query, orgID, pq.Array(projectIDs), suiteName, branch, filter.EnvironmentSlug, triggersArg, filter.Search, limit, offset); err != nil {
		return SuiteRunsBranchResult{}, fmt.Errorf("failed to list runs for suite branch: %w", err)
	}

	// Extract runs and total
	runs := make([]SuiteRunRow, len(runsWithCount))
	total := 0
	for i, r := range runsWithCount {
		runs[i] = r.SuiteRunRow
		total = r.TotalCount // Same for all rows
	}

	return SuiteRunsBranchResult{
		Runs:   runs,
		Total:  total,
		Limit:  limit,
		Offset: offset,
		Branch: branch,
	}, nil
}
