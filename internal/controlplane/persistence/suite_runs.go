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
// Applies two filters for a clean UI:
//  1. Limit to 5 most recent runs per branch
//  2. Only show branches that have a run in the last 24 hours (default branch is always shown)
//
// Results are ordered by: default branch first, then by latest run time desc, then by created_at desc.
func (s *Store) ListRunsForSuiteGroup(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName, defaultBranch string, runsPerBranch int) ([]SuiteRunRow, error) {
	if runsPerBranch <= 0 {
		runsPerBranch = 5
	}
	if runsPerBranch > 20 {
		runsPerBranch = 20
	}

	if len(projectIDs) == 0 {
		return []SuiteRunRow{}, nil
	}

	// Use a CTE with window functions to:
	// 1. Rank runs within each branch by created_at DESC
	// 2. Track the latest run time per branch for freshness filtering
	// Then filter to keep:
	// - At most runsPerBranch runs per branch
	// - Only branches with a run in the last 24 hours (or the default branch)
	const query = `
		WITH ranked_runs AS (
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
				ROW_NUMBER() OVER (PARTITION BY branch ORDER BY created_at DESC) as row_num,
				MAX(created_at) OVER (PARTITION BY branch) as branch_latest_run
			FROM runs
			WHERE organization_id = $1
			  AND project_id = ANY($2)
			  AND suite_name = $3
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
		  AND (
		      branch = $5
		      OR branch_latest_run > NOW() - INTERVAL '24 hours'
		  )
		ORDER BY
			CASE WHEN branch = $5 THEN 0 ELSE 1 END,
			branch_latest_run DESC,
			created_at DESC
	`

	var runs []SuiteRunRow
	if err := s.db.SelectContext(ctx, &runs, query, orgID, pq.Array(projectIDs), suiteName, runsPerBranch, defaultBranch); err != nil {
		return nil, fmt.Errorf("failed to list runs for suite group: %w", err)
	}
	if runs == nil {
		runs = []SuiteRunRow{}
	}
	return runs, nil
}
