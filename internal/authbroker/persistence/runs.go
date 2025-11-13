package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// InsertRun creates a new test run record
func (s *Store) InsertRun(ctx context.Context, run RunRecord) (RunRecord, error) {
	if run.ID == "" {
		return RunRecord{}, errors.New("run id required")
	}
	if run.OrganizationID == uuid.Nil {
		return RunRecord{}, errors.New("organization id required")
	}

	var projectID interface{}
	if run.ProjectID.Valid {
		projectID = run.ProjectID.UUID
	}

	var commitSHA, bundleSHA interface{}
	if run.CommitSHA.Valid {
		commitSHA = run.CommitSHA.String
	}
	if run.BundleSHA.Valid {
		bundleSHA = run.BundleSHA.String
	}

	var startedAt, endedAt interface{}
	if run.StartedAt.Valid {
		startedAt = run.StartedAt.Time
	}
	if run.EndedAt.Valid {
		endedAt = run.EndedAt.Time
	}

	const query = `
        INSERT INTO runs (
            id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name,
            config_source, source, branch, environment, commit_sha, bundle_sha,
            total_tests, passed_tests, failed_tests, timeout_tests,
            created_at, updated_at, started_at, ended_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, NOW(), NOW(), $19, $20)
        RETURNING created_at, updated_at
    `

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		run.ID, run.OrganizationID, projectID, run.Status, run.SuiteName,
		run.Initiator, run.Trigger, run.ScheduleName, run.ConfigSource,
		run.Source, run.Branch, run.Environment, commitSHA, bundleSHA,
		run.TotalTests, run.PassedTests, run.FailedTests, run.TimeoutTests,
		startedAt, endedAt); err != nil {
		return RunRecord{}, fmt.Errorf("failed to insert run: %w", err)
	}

	run.CreatedAt = dest.CreatedAt
	run.UpdatedAt = dest.UpdatedAt
	return run, nil
}

// UpdateRun updates a test run record with new data
func (s *Store) UpdateRun(ctx context.Context, update RunUpdate) (RunRecord, error) {
	if update.RunID == "" {
		return RunRecord{}, errors.New("run id required")
	}
	if update.OrganizationID == uuid.Nil {
		return RunRecord{}, errors.New("organization id required")
	}

	sets := []string{}
	args := []interface{}{update.RunID, update.OrganizationID}
	argIdx := 3

	if update.Status != nil {
		sets = append(sets, fmt.Sprintf("status = $%d", argIdx))
		args = append(args, *update.Status)
		argIdx++
	}
	if update.StartedAt != nil {
		sets = append(sets, fmt.Sprintf("started_at = $%d", argIdx))
		args = append(args, *update.StartedAt)
		argIdx++
	}
	if update.EndedAt != nil {
		sets = append(sets, fmt.Sprintf("ended_at = $%d", argIdx))
		args = append(args, *update.EndedAt)
		argIdx++
	}
	if update.CommitSHA != nil {
		sets = append(sets, fmt.Sprintf("commit_sha = $%d", argIdx))
		args = append(args, *update.CommitSHA)
		argIdx++
	}
	if update.BundleSHA != nil {
		sets = append(sets, fmt.Sprintf("bundle_sha = $%d", argIdx))
		args = append(args, *update.BundleSHA)
		argIdx++
	}
	if update.Totals != nil {
		sets = append(sets, fmt.Sprintf("total_tests = $%d, passed_tests = $%d, failed_tests = $%d, timeout_tests = $%d",
			argIdx, argIdx+1, argIdx+2, argIdx+3))
		args = append(args, update.Totals.Total, update.Totals.Passed, update.Totals.Failed, update.Totals.Timeout)
	}

	if len(sets) == 0 {
		return RunRecord{}, errors.New("no fields to update")
	}

	// Build SET clause
	setsStr := ""
	for i, set := range sets {
		if i > 0 {
			setsStr += ", "
		}
		setsStr += set
	}

	query := fmt.Sprintf(`
        UPDATE runs
        SET %s, updated_at = NOW()
        WHERE id = $1 AND organization_id = $2
        RETURNING id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name,
                  config_source, source, branch, environment, commit_sha, bundle_sha,
                  total_tests, passed_tests, failed_tests, timeout_tests,
                  created_at, updated_at, started_at, ended_at
    `, setsStr)

	var run RunRecord
	if err := s.db.GetContext(ctx, &run, query, args...); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, sql.ErrNoRows
		}
		return RunRecord{}, fmt.Errorf("failed to update run: %w", err)
	}
	return run, nil
}

// GetRun retrieves a test run by ID
func (s *Store) GetRun(ctx context.Context, orgID uuid.UUID, runID string) (RunRecord, error) {
	const query = `
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests,
               created_at, updated_at, started_at, ended_at
        FROM runs
        WHERE organization_id = $1 AND id = $2
    `
	var run RunRecord
	if err := s.db.GetContext(ctx, &run, query, orgID, runID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RunRecord{}, sql.ErrNoRows
		}
		return RunRecord{}, fmt.Errorf("failed to get run: %w", err)
	}
	return run, nil
}

// ListRuns returns recent test runs for an organization
func (s *Store) ListRuns(ctx context.Context, orgID uuid.UUID, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests,
               created_at, updated_at, started_at, ended_at
        FROM runs
        WHERE organization_id = $1
        ORDER BY created_at DESC
        LIMIT $2
    `
	var runs []RunRecord
	if err := s.db.SelectContext(ctx, &runs, query, orgID, limit); err != nil {
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}
	if runs == nil {
		runs = []RunRecord{}
	}
	return runs, nil
}
