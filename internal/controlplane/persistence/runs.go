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

	var environmentID, scheduleID, commitMessage, scheduleType interface{}
	if run.EnvironmentID.Valid {
		environmentID = run.EnvironmentID.UUID
	}
	if run.ScheduleID.Valid {
		scheduleID = run.ScheduleID.UUID
	}
	if run.CommitMessage.Valid {
		commitMessage = run.CommitMessage.String
	}
	if run.ScheduleType.Valid {
		scheduleType = run.ScheduleType.String
	}

	const query = `
        INSERT INTO runs (
            id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
            config_source, source, branch, environment, commit_sha, bundle_sha,
            total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
            environment_id, schedule_id, commit_message,
            created_at, updated_at, started_at, ended_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21, $22, $23, NOW(), NOW(), $24, $25)
        RETURNING created_at, updated_at
    `

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		run.ID, run.OrganizationID, projectID, run.Status, run.SuiteName,
		run.Initiator, run.Trigger, run.ScheduleName, scheduleType, run.ConfigSource,
		run.Source, run.Branch, run.Environment, commitSHA, bundleSHA,
		run.TotalTests, run.PassedTests, run.FailedTests, run.TimeoutTests, run.SkippedTests,
		environmentID, scheduleID, commitMessage,
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
        RETURNING id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
                  config_source, source, branch, environment, commit_sha, bundle_sha,
                  total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
                  environment_id, schedule_id, commit_message,
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
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
               environment_id, schedule_id, commit_message,
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
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
               environment_id, schedule_id, commit_message,
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

// ListRunsByProject returns recent test runs for a specific project
func (s *Store) ListRunsByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
               environment_id, schedule_id, commit_message,
               created_at, updated_at, started_at, ended_at
        FROM runs
        WHERE project_id = $1
        ORDER BY created_at DESC
        LIMIT $2
    `
	var runs []RunRecord
	if err := s.db.SelectContext(ctx, &runs, query, projectID, limit); err != nil {
		return nil, fmt.Errorf("failed to list runs by project: %w", err)
	}
	if runs == nil {
		runs = []RunRecord{}
	}
	return runs, nil
}

// UpdateRunStatusByID updates a run's status directly by run_id, without requiring org_id.
// This is used for DB-only completion checks when in-memory state is not available.
func (s *Store) UpdateRunStatusByID(ctx context.Context, runID string, status string, endedAt time.Time, totals *RunTotals) error {
	if runID == "" {
		return errors.New("run id required")
	}
	if status == "" {
		return errors.New("status required")
	}

	var query string
	var args []interface{}

	if totals != nil {
		query = `
            UPDATE runs
            SET status = $2, ended_at = $3,
                total_tests = $4, passed_tests = $5, failed_tests = $6, timeout_tests = $7,
                updated_at = NOW()
            WHERE id = $1
        `
		args = []interface{}{runID, status, endedAt, totals.Total, totals.Passed, totals.Failed, totals.Timeout}
	} else {
		query = `
            UPDATE runs
            SET status = $2, ended_at = $3, updated_at = NOW()
            WHERE id = $1
        `
		args = []interface{}{runID, status, endedAt}
	}

	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("failed to update run status: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListStaleRunningRuns returns runs that are stuck in RUNNING status and are older than the specified time.
// This is used for reconciliation to clean up stale runs after engine restarts.
func (s *Store) ListStaleRunningRuns(ctx context.Context, olderThan time.Time, limit int) ([]RunRecord, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	const query = `
        SELECT id, organization_id, project_id, status, suite_name, initiator, trigger, schedule_name, schedule_type,
               config_source, source, branch, environment, commit_sha, bundle_sha,
               total_tests, passed_tests, failed_tests, timeout_tests, skipped_tests,
               environment_id, schedule_id, commit_message,
               created_at, updated_at, started_at, ended_at
        FROM runs
        WHERE status = 'RUNNING'
          AND created_at < $1
        ORDER BY created_at ASC
        LIMIT $2
    `
	var runs []RunRecord
	if err := s.db.SelectContext(ctx, &runs, query, olderThan, limit); err != nil {
		return nil, fmt.Errorf("failed to list stale running runs: %w", err)
	}
	if runs == nil {
		runs = []RunRecord{}
	}
	return runs, nil
}

// ForceCompleteStaleRunTests marks all PENDING/RUNNING run_tests for a run as complete.
// This is used during reconciliation when the parent run is being marked as complete.
// If status is "PASSED", only PENDING/RUNNING tests are marked as PASSED.
// If status is "FAILED", all non-terminal tests are marked as FAILED.
func (s *Store) ForceCompleteStaleRunTests(ctx context.Context, runID string, status string) error {
	if runID == "" {
		return errors.New("run id required")
	}
	if status == "" {
		return errors.New("status required")
	}

	const query = `
        UPDATE run_tests
        SET status = $2, ended_at = NOW()
        WHERE run_id = $1 AND status IN ('PENDING', 'RUNNING')
    `

	if _, err := s.db.ExecContext(ctx, query, runID, status); err != nil {
		return fmt.Errorf("failed to force complete stale run_tests: %w", err)
	}

	return nil
}

// StaleRunTest represents a run_test that is stuck in PENDING/RUNNING status
type StaleRunTest struct {
	ID         uuid.UUID `db:"id"`
	RunID      string    `db:"run_id"`
	WorkflowID string    `db:"workflow_id"`
	Name       string    `db:"name"`
	Status     string    `db:"status"`
	CreatedAt  time.Time `db:"created_at"`
}

// ListStaleRunTests returns run_tests that are stuck in PENDING/RUNNING status
// and are older than the specified time. This is used for Temporal-based reconciliation.
func (s *Store) ListStaleRunTests(ctx context.Context, olderThan time.Time, limit int) ([]StaleRunTest, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	const query = `
        SELECT id, run_id, workflow_id, name, status, created_at
        FROM run_tests
        WHERE status IN ('PENDING', 'RUNNING')
          AND created_at < $1
          AND workflow_id IS NOT NULL
          AND workflow_id != ''
        ORDER BY created_at ASC
        LIMIT $2
    `
	var tests []StaleRunTest
	if err := s.db.SelectContext(ctx, &tests, query, olderThan, limit); err != nil {
		return nil, fmt.Errorf("failed to list stale run_tests: %w", err)
	}
	if tests == nil {
		tests = []StaleRunTest{}
	}
	return tests, nil
}

// UpdateRunTestStatus updates a run_test's status and ended_at by ID.
// This is used for reconciliation when updating based on Temporal status.
func (s *Store) UpdateRunTestStatus(ctx context.Context, id uuid.UUID, status string, endedAt time.Time) error {
	if id == uuid.Nil {
		return errors.New("run test id required")
	}
	if status == "" {
		return errors.New("status required")
	}

	const query = `
        UPDATE run_tests
        SET status = $2, ended_at = $3
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, id, status, endedAt)
	if err != nil {
		return fmt.Errorf("failed to update run_test status: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}
