package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// OverviewMetrics contains aggregated metrics for the overview dashboard
type OverviewMetrics struct {
	// Now metrics (point-in-time or 24h window)
	FailingMonitors    int              `json:"failing_monitors"`
	FailingTests24h    int              `json:"failing_tests_24h"`
	RunsInProgress     int              `json:"runs_in_progress"`
	PassRate24h        *float64         `json:"pass_rate_24h"`        // nil if no runs
	MedianDurationMs24h *int64          `json:"median_duration_ms_24h"` // nil if no runs

	// Chart data
	PassRateOverTime   []PassRateDataPoint  `json:"pass_rate_over_time"`
	FailuresBySuite24h []SuiteFailureData   `json:"failures_by_suite_24h"`
}

// PassRateDataPoint represents a single day of pass rate data
type PassRateDataPoint struct {
	Date     string  `json:"date"`      // YYYY-MM-DD format
	PassRate float64 `json:"pass_rate"` // 0-100
	Volume   int     `json:"volume"`    // number of runs
}

// SuiteFailureData represents failure counts for a single suite
type SuiteFailureData struct {
	Suite    string `json:"suite"`
	Passes   int    `json:"passes"`
	Failures int    `json:"failures"`
}

// CountProjectsForOrg returns the number of projects in an organization
func (s *Store) CountProjectsForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `SELECT COUNT(*) FROM projects WHERE organization_id = $1`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count projects: %w", err)
	}
	return count, nil
}

// CountSuitesForOrg returns the total number of suites across all projects in an organization
func (s *Store) CountSuitesForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM suites s
		JOIN projects p ON p.id = s.project_id
		WHERE p.organization_id = $1
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count suites: %w", err)
	}
	return count, nil
}

// CountSuitesOnDefaultBranchForOrg returns the number of suites where config was discovered on the project's default branch
// This is determined by checking if the suite's file_path is set (meaning it was synced from the repo)
// In the future, we can add more sophisticated checks with suite.config JSONB
func (s *Store) CountSuitesOnDefaultBranchForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM suites s
		JOIN projects p ON p.id = s.project_id
		WHERE p.organization_id = $1 AND s.file_path IS NOT NULL
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count suites on default branch: %w", err)
	}
	return count, nil
}

// CountEnvsWithVarsForOrg returns the number of environments with at least one variable defined
func (s *Store) CountEnvsWithVarsForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM project_environments e
		JOIN projects p ON p.id = e.project_id
		WHERE p.organization_id = $1 AND jsonb_object_length(e.variables) > 0
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count envs with vars: %w", err)
	}
	return count, nil
}

// CountEnabledSchedulesForOrg returns the number of enabled schedules across all projects in an organization
func (s *Store) CountEnabledSchedulesForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM suite_schedules sc
		JOIN projects p ON p.id = sc.project_id
		WHERE p.organization_id = $1 AND sc.enabled = TRUE
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count enabled schedules: %w", err)
	}
	return count, nil
}

// CountActiveCITokensForOrg returns the number of active (non-revoked) CI tokens for an organization
func (s *Store) CountActiveCITokensForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM ci_tokens
		WHERE organization_id = $1 AND revoked_at IS NULL
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count active ci tokens: %w", err)
	}
	return count, nil
}

// GetOverviewMetrics returns aggregated metrics for the overview dashboard.
// It respects project and environment scoping based on user access.
func (s *Store) GetOverviewMetrics(ctx context.Context, orgID, userID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID, days int) (OverviewMetrics, error) {
	if days <= 0 {
		days = 7
	}
	if days > 30 {
		days = 30
	}

	// Get accessible project IDs
	accessibleIDs, err := s.ListAccessibleProjectIDs(ctx, orgID, userID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get accessible project IDs: %w", err)
	}

	// Intersect with requested projectIDs if provided
	var effectiveProjectIDs []uuid.UUID
	if len(projectIDs) > 0 {
		accessibleSet := make(map[uuid.UUID]bool, len(accessibleIDs))
		for _, id := range accessibleIDs {
			accessibleSet[id] = true
		}
		for _, id := range projectIDs {
			if accessibleSet[id] {
				effectiveProjectIDs = append(effectiveProjectIDs, id)
			}
		}
	} else {
		effectiveProjectIDs = accessibleIDs
	}

	// If no accessible projects, return zeros
	if len(effectiveProjectIDs) == 0 {
		return OverviewMetrics{
			PassRateOverTime:   []PassRateDataPoint{},
			FailuresBySuite24h: []SuiteFailureData{},
		}, nil
	}

	metrics := OverviewMetrics{
		PassRateOverTime:   []PassRateDataPoint{},
		FailuresBySuite24h: []SuiteFailureData{},
	}

	// Get "now" metrics in parallel using Go routines would be ideal,
	// but for simplicity we'll run them sequentially here

	// 1. Failing monitors (enabled schedules whose last run failed in last 24h)
	failingMonitors, err := s.getFailingMonitorsCount(ctx, orgID, effectiveProjectIDs, environmentID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get failing monitors: %w", err)
	}
	metrics.FailingMonitors = failingMonitors

	// 2. Failing tests in last 24h
	failingTests, err := s.getFailingTestsCount24h(ctx, orgID, effectiveProjectIDs, environmentID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get failing tests count: %w", err)
	}
	metrics.FailingTests24h = failingTests

	// 3. Runs in progress (RUNNING or PENDING)
	runsInProgress, err := s.getRunsInProgressCount(ctx, orgID, effectiveProjectIDs, environmentID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get runs in progress: %w", err)
	}
	metrics.RunsInProgress = runsInProgress

	// 4. Pass rate and median duration for last 24h
	passRate, medianDuration, err := s.getPassRateAndDuration24h(ctx, orgID, effectiveProjectIDs, environmentID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get pass rate: %w", err)
	}
	metrics.PassRate24h = passRate
	metrics.MedianDurationMs24h = medianDuration

	// 5. Pass rate over time (daily buckets)
	passRateOverTime, err := s.getPassRateOverTime(ctx, orgID, effectiveProjectIDs, environmentID, days)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get pass rate over time: %w", err)
	}
	metrics.PassRateOverTime = passRateOverTime

	// 6. Failures by suite (last 24h)
	failuresBySuite, err := s.getFailuresBySuite24h(ctx, orgID, effectiveProjectIDs, environmentID)
	if err != nil {
		return OverviewMetrics{}, fmt.Errorf("failed to get failures by suite: %w", err)
	}
	metrics.FailuresBySuite24h = failuresBySuite

	return metrics, nil
}

// getFailingMonitorsCount counts enabled schedules whose last run failed within 24h
func (s *Store) getFailingMonitorsCount(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID) (int, error) {
	// Count from both project_schedules and suite_schedules
	// A "failing monitor" is an enabled schedule whose last_run_at >= now()-24h
	// and last_run_status IN ('FAILED','TIMEOUT','CANCELLED')

	var envFilter string
	args := []interface{}{pq.Array(projectIDs), time.Now().Add(-24 * time.Hour)}
	if environmentID != nil {
		envFilter = " AND ps.environment_id = $3"
		args = append(args, *environmentID)
	}

	// Project schedules
	projectScheduleQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM project_schedules ps
		WHERE ps.project_id = ANY($1)
			AND ps.enabled = TRUE
			AND ps.last_run_at >= $2
			AND ps.last_run_status IN ('FAILED', 'TIMEOUT', 'CANCELLED')
			%s
	`, envFilter)

	var projectCount int
	if err := s.db.GetContext(ctx, &projectCount, projectScheduleQuery, args...); err != nil {
		return 0, fmt.Errorf("project schedules query failed: %w", err)
	}

	// Suite schedules - reset args for this query
	args = []interface{}{pq.Array(projectIDs), time.Now().Add(-24 * time.Hour)}
	envFilter = ""
	if environmentID != nil {
		envFilter = " AND ss.environment_id = $3"
		args = append(args, *environmentID)
	}

	suiteScheduleQuery := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM suite_schedules ss
		WHERE ss.project_id = ANY($1)
			AND ss.enabled = TRUE
			AND ss.last_run_at >= $2
			AND ss.last_run_status IN ('FAILED', 'TIMEOUT', 'CANCELLED')
			%s
	`, envFilter)

	var suiteCount int
	if err := s.db.GetContext(ctx, &suiteCount, suiteScheduleQuery, args...); err != nil {
		return 0, fmt.Errorf("suite schedules query failed: %w", err)
	}

	return projectCount + suiteCount, nil
}

// getFailingTestsCount24h counts failed+timeout tests from runs in last 24h
func (s *Store) getFailingTestsCount24h(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID) (int, error) {
	var envFilter string
	args := []interface{}{orgID, pq.Array(projectIDs), time.Now().Add(-24 * time.Hour)}
	if environmentID != nil {
		envFilter = " AND r.environment_id = $4"
		args = append(args, *environmentID)
	}

	query := fmt.Sprintf(`
		SELECT COALESCE(SUM(r.failed_tests + r.timeout_tests), 0)::int
		FROM runs r
		WHERE r.organization_id = $1
			AND r.project_id = ANY($2)
			AND r.status IN ('PASSED', 'FAILED', 'CANCELLED', 'TIMEOUT')
			AND r.ended_at >= $3
			%s
	`, envFilter)

	var count int
	if err := s.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

// getRunsInProgressCount counts runs that are RUNNING or PENDING
func (s *Store) getRunsInProgressCount(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID) (int, error) {
	var envFilter string
	args := []interface{}{orgID, pq.Array(projectIDs)}
	if environmentID != nil {
		envFilter = " AND r.environment_id = $3"
		args = append(args, *environmentID)
	}

	query := fmt.Sprintf(`
		SELECT COUNT(*)
		FROM runs r
		WHERE r.organization_id = $1
			AND r.project_id = ANY($2)
			AND r.status IN ('RUNNING', 'PENDING')
			%s
	`, envFilter)

	var count int
	if err := s.db.GetContext(ctx, &count, query, args...); err != nil {
		return 0, err
	}
	return count, nil
}

// getPassRateAndDuration24h computes pass rate and median duration for runs in last 24h
func (s *Store) getPassRateAndDuration24h(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID) (*float64, *int64, error) {
	var envFilter string
	args := []interface{}{orgID, pq.Array(projectIDs), time.Now().Add(-24 * time.Hour)}
	if environmentID != nil {
		envFilter = " AND r.environment_id = $4"
		args = append(args, *environmentID)
	}

	query := fmt.Sprintf(`
		SELECT
			100.0 * SUM(r.passed_tests) / NULLIF(SUM(r.total_tests - r.skipped_tests), 0) AS pass_rate,
			percentile_cont(0.5) WITHIN GROUP (ORDER BY EXTRACT(EPOCH FROM (r.ended_at - r.started_at)) * 1000)::bigint AS median_duration_ms
		FROM runs r
		WHERE r.organization_id = $1
			AND r.project_id = ANY($2)
			AND r.status IN ('PASSED', 'FAILED', 'CANCELLED', 'TIMEOUT')
			AND r.started_at IS NOT NULL
			AND r.ended_at IS NOT NULL
			AND r.ended_at >= $3
			%s
	`, envFilter)

	var result struct {
		PassRate         sql.NullFloat64 `db:"pass_rate"`
		MedianDurationMs sql.NullInt64   `db:"median_duration_ms"`
	}
	if err := s.db.GetContext(ctx, &result, query, args...); err != nil {
		return nil, nil, err
	}

	var passRate *float64
	var medianDuration *int64
	if result.PassRate.Valid {
		passRate = &result.PassRate.Float64
	}
	if result.MedianDurationMs.Valid {
		medianDuration = &result.MedianDurationMs.Int64
	}
	return passRate, medianDuration, nil
}

// getPassRateOverTime returns daily pass rate and volume for the last N days
func (s *Store) getPassRateOverTime(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID, days int) ([]PassRateDataPoint, error) {
	var envFilter string
	args := []interface{}{orgID, pq.Array(projectIDs), time.Now().AddDate(0, 0, -days)}
	if environmentID != nil {
		envFilter = " AND r.environment_id = $4"
		args = append(args, *environmentID)
	}

	query := fmt.Sprintf(`
		SELECT
			date_trunc('day', r.ended_at)::date AS day,
			100.0 * SUM(r.passed_tests) / NULLIF(SUM(r.total_tests - r.skipped_tests), 0) AS pass_rate,
			COUNT(*)::int AS volume
		FROM runs r
		WHERE r.organization_id = $1
			AND r.project_id = ANY($2)
			AND r.status IN ('PASSED', 'FAILED', 'CANCELLED', 'TIMEOUT')
			AND r.started_at IS NOT NULL
			AND r.ended_at IS NOT NULL
			AND r.ended_at >= $3
			%s
		GROUP BY date_trunc('day', r.ended_at)::date
		ORDER BY day ASC
	`, envFilter)

	rows := []struct {
		Day      time.Time       `db:"day"`
		PassRate sql.NullFloat64 `db:"pass_rate"`
		Volume   int             `db:"volume"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	result := make([]PassRateDataPoint, 0, len(rows))
	for _, row := range rows {
		passRate := 0.0
		if row.PassRate.Valid {
			passRate = row.PassRate.Float64
		}
		result = append(result, PassRateDataPoint{
			Date:     row.Day.Format("2006-01-02"),
			PassRate: passRate,
			Volume:   row.Volume,
		})
	}

	return result, nil
}

// getFailuresBySuite24h returns top suites by failures in the last 24h
func (s *Store) getFailuresBySuite24h(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, environmentID *uuid.UUID) ([]SuiteFailureData, error) {
	var envFilter string
	args := []interface{}{orgID, pq.Array(projectIDs), time.Now().Add(-24 * time.Hour)}
	if environmentID != nil {
		envFilter = " AND r.environment_id = $4"
		args = append(args, *environmentID)
	}

	query := fmt.Sprintf(`
		SELECT
			r.suite_name AS suite,
			COALESCE(SUM(r.passed_tests), 0)::int AS passes,
			COALESCE(SUM(r.failed_tests + r.timeout_tests), 0)::int AS failures
		FROM runs r
		WHERE r.organization_id = $1
			AND r.project_id = ANY($2)
			AND r.status IN ('PASSED', 'FAILED', 'CANCELLED', 'TIMEOUT')
			AND r.ended_at >= $3
			%s
		GROUP BY r.suite_name
		HAVING SUM(r.failed_tests + r.timeout_tests) > 0
		ORDER BY failures DESC
		LIMIT 10
	`, envFilter)

	var rows []SuiteFailureData
	if err := s.db.SelectContext(ctx, &rows, query, args...); err != nil {
		return nil, err
	}

	if rows == nil {
		rows = []SuiteFailureData{}
	}
	return rows, nil
}
