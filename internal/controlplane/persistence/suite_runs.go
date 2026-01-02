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

// ListRunsForSuiteGroup returns runs for a suite across all projects in a group
// (projects that share the same repo/path_scope).
//
// Shows up to 3 branches:
//  1. The default branch (always shown if it has runs)
//  2. Up to 2 additional non-default branches with the most recent activity
//
// Each branch shows up to runsPerBranch runs (default 5).
// Results are ordered by: default branch first, then by latest run time desc, then by created_at desc.
// If environmentID is provided, only runs with that environment_id are returned.
func (s *Store) ListRunsForSuiteGroup(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName, defaultBranch string, runsPerBranch int, environmentID uuid.NullUUID) ([]SuiteRunRow, error) {
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
			  AND ($6::uuid IS NULL OR environment_id = $6)
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
			  AND ($6::uuid IS NULL OR r.environment_id = $6)
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

	var runs []SuiteRunRow
	if err := s.db.SelectContext(ctx, &runs, query, orgID, pq.Array(projectIDs), suiteName, runsPerBranch, defaultBranch, environmentID); err != nil {
		return nil, fmt.Errorf("failed to list runs for suite group: %w", err)
	}
	if runs == nil {
		runs = []SuiteRunRow{}
	}
	return runs, nil
}
