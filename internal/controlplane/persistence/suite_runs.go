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
// (projects that share the same repo/path_scope). Results are ordered by created_at DESC.
func (s *Store) ListRunsForSuiteGroup(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName string, limit int) ([]SuiteRunRow, error) {
	if limit <= 0 {
		limit = 200
	}
	if limit > 1000 {
		limit = 1000
	}

	if len(projectIDs) == 0 {
		return []SuiteRunRow{}, nil
	}

	const query = `
		SELECT
			id,
			status,
			branch,
			commit_sha,
			commit_message,
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
		FROM runs
		WHERE organization_id = $1
		  AND project_id = ANY($2)
		  AND suite_name = $3
		ORDER BY created_at DESC
		LIMIT $4
	`

	var runs []SuiteRunRow
	if err := s.db.SelectContext(ctx, &runs, query, orgID, pq.Array(projectIDs), suiteName, limit); err != nil {
		return nil, fmt.Errorf("failed to list runs for suite group: %w", err)
	}
	if runs == nil {
		runs = []SuiteRunRow{}
	}
	return runs, nil
}
