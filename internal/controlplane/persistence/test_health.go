package persistence

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// TestHealthRow represents a single test in the test health table
type TestHealthRow struct {
	// Test info
	TestID     uuid.UUID `db:"test_id"`
	TestName   string    `db:"test_name"`
	StepCount  int       `db:"step_count"`
	TestPlugins []string `db:"-"` // Extracted from step_summaries

	// Suite info
	SuiteID   uuid.UUID `db:"suite_id"`
	SuiteName string    `db:"suite_name"`

	// Project info
	ProjectID   uuid.UUID `db:"project_id"`
	ProjectName string    `db:"project_name"`

	// Latest schedule results (newest first, up to 20)
	// Values: "success", "failed", "pending", "running"
	LatestResults []string `db:"-"` // Extracted from JSONB array

	// Success percentage (0-100), null if no data
	SuccessPercent sql.NullInt32 `db:"success_percent"`

	// Last scheduled run time
	LastRunAt sql.NullTime `db:"last_run_at"`

	// Next scheduled run time
	NextRunAt sql.NullTime `db:"next_run_at"`

	// Is the latest scheduled result live (pending/running)?
	IsLive bool `db:"is_live"`

	// Raw JSONB fields for parsing
	StepSummariesJSON  string `db:"step_summaries_json"`
	LatestResultsJSON  string `db:"latest_results_json"`
}

// TestHealthSuiteOption represents a suite option for the filter dropdown
type TestHealthSuiteOption struct {
	ID   uuid.UUID `db:"id"`
	Name string    `db:"name"`
}

// TestHealthParams contains query parameters for ListTestHealth
type TestHealthParams struct {
	ProjectIDs    []uuid.UUID    // Filter by project IDs (empty = all accessible)
	EnvironmentID uuid.NullUUID  // Filter by environment (optional)
	SuiteIDs      []uuid.UUID    // Filter by suite IDs (optional)
	Plugins       []string       // Filter by plugins (OR semantics, optional)
	Search        string         // Search test name (ILIKE, optional)
	Limit         int            // Max rows (default 100, max 500)
}

// ListTestHealth returns test health data for the Test Health page.
// Only includes schedulable tests (default-branch discovery).
// Applies all filters at SQL level for scalability.
func (s *Store) ListTestHealth(ctx context.Context, orgID, userID uuid.UUID, params TestHealthParams) ([]TestHealthRow, []TestHealthSuiteOption, error) {
	// Get accessible project IDs
	accessibleIDs, err := s.ListAccessibleProjectIDs(ctx, orgID, userID)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get accessible project IDs: %w", err)
	}
	if len(accessibleIDs) == 0 {
		return []TestHealthRow{}, []TestHealthSuiteOption{}, nil
	}

	// If specific project IDs requested, intersect with accessible IDs
	var projectIDs []uuid.UUID
	if len(params.ProjectIDs) > 0 {
		idSet := make(map[uuid.UUID]struct{}, len(accessibleIDs))
		for _, id := range accessibleIDs {
			idSet[id] = struct{}{}
		}
		for _, id := range params.ProjectIDs {
			if _, ok := idSet[id]; ok {
				projectIDs = append(projectIDs, id)
			}
		}
		if len(projectIDs) == 0 {
			return []TestHealthRow{}, []TestHealthSuiteOption{}, nil
		}
	} else {
		projectIDs = accessibleIDs
	}

	// Set defaults
	limit := params.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	// Build the main query
	// This is a complex query that:
	// 1. Joins projects, suites, tests (all on default branch, active)
	// 2. Uses LATERAL to get latest N scheduled run_tests results
	// 3. Computes next_run_at from schedules (project or suite override)
	// 4. Applies all filters

	// Note: For next_run_at calculation:
	// - If env filter present: use suite_schedules override if exists, else project_schedules for that env
	// - If no env filter: compute MIN across all envs
	// We'll simplify for v1: if env filter present, use that env's schedule; otherwise use earliest next_run_at

	const query = `
		WITH test_base AS (
			SELECT
				t.id as test_id,
				t.name as test_name,
				t.step_count,
				COALESCE(t.step_summaries, '[]'::jsonb) as step_summaries,
				s.id as suite_id,
				s.name as suite_name,
				p.id as project_id,
				p.name as project_name
			FROM tests t
			JOIN suites s ON t.suite_id = s.id
			JOIN projects p ON s.project_id = p.id
			WHERE p.id = ANY($1)
			  AND lower(t.source_ref) = lower(p.default_branch)
			  AND lower(s.source_ref) = lower(p.default_branch)
			  AND t.is_active = true
			  AND s.is_active = true
			  AND p.is_active = true
			  AND (COALESCE(CARDINALITY($4::uuid[]), 0) = 0 OR s.id = ANY($4::uuid[]))
			  AND ($5::text = '' OR t.name ILIKE '%' || $5 || '%')
		),
		-- Get latest scheduled run results for each test (up to 20)
		test_with_results AS (
			SELECT
				tb.*,
				COALESCE(lr.statuses, ARRAY[]::text[]) as latest_results,
				lr.last_run_at,
				COALESCE(lr.total_runs, 0) as total_runs,
				COALESCE(lr.passed_runs, 0) as passed_runs,
				COALESCE((ARRAY_LENGTH(lr.statuses, 1) > 0 AND lr.statuses[1] IN ('PENDING', 'RUNNING')), false) as is_live
			FROM test_base tb
			LEFT JOIN LATERAL (
				SELECT
					array_agg(recent.status ORDER BY recent.created_at DESC) as statuses,
					MAX(recent.created_at) as last_run_at,
					COUNT(*)::int as total_runs,
					COUNT(*) FILTER (WHERE recent.status = 'PASSED')::int as passed_runs
				FROM (
					SELECT
						rt.status,
						rt.created_at
					FROM run_tests rt
					JOIN runs r ON rt.run_id = r.id
					WHERE rt.test_id = tb.test_id
					  AND r.trigger = 'schedule'
					  AND ($2::uuid IS NULL OR r.environment_id = $2)
					ORDER BY rt.created_at DESC
					LIMIT 20
				) recent
			) lr ON true
		),
		-- Calculate next_run_at from schedules
		test_with_schedule AS (
			SELECT
				twr.*,
				CASE
					WHEN $2::uuid IS NOT NULL THEN
						-- Environment specified: use suite override if exists, else project schedule
						COALESCE(
							(SELECT ss.next_run_at FROM suite_schedules ss WHERE ss.suite_id = twr.suite_id AND ss.environment_id = $2 AND ss.enabled = true),
							(SELECT ps.next_run_at FROM project_schedules ps WHERE ps.project_id = twr.project_id AND ps.environment_id = $2 AND ps.enabled = true)
						)
					ELSE
						-- No environment: get earliest next_run across all envs
						(
							SELECT MIN(next_run)
							FROM (
								-- Project schedules for this project where no suite override exists
								SELECT ps.next_run_at as next_run
								FROM project_schedules ps
								WHERE ps.project_id = twr.project_id AND ps.enabled = true
								  AND NOT EXISTS (
									SELECT 1 FROM suite_schedules ss
									WHERE ss.suite_id = twr.suite_id AND ss.environment_id = ps.environment_id AND ss.enabled = true
								  )
								UNION ALL
								-- Suite schedule overrides
								SELECT ss.next_run_at as next_run
								FROM suite_schedules ss
								WHERE ss.suite_id = twr.suite_id AND ss.enabled = true
							) all_schedules
						)
				END as next_run_at
			FROM test_with_results twr
		)
		SELECT
			test_id,
			test_name,
			step_count,
			step_summaries::text as step_summaries_json,
			suite_id,
			suite_name,
			project_id,
			project_name,
			COALESCE(array_to_json(latest_results)::text, '[]') as latest_results_json,
			CASE WHEN total_runs > 0 THEN ROUND((passed_runs::numeric / total_runs) * 100)::int ELSE NULL END as success_percent,
			last_run_at,
			next_run_at,
			is_live
		FROM test_with_schedule
		ORDER BY
			CASE WHEN next_run_at IS NOT NULL THEN 0 ELSE 1 END ASC,
			last_run_at DESC NULLS LAST,
			test_name ASC
		LIMIT $3
	`

	// Convert params to SQL-compatible values
	var envID interface{}
	if params.EnvironmentID.Valid {
		envID = params.EnvironmentID.UUID
	}

	// Always pass an array for suiteIDs (empty array when no filter)
	suiteIDsSlice := params.SuiteIDs
	if suiteIDsSlice == nil {
		suiteIDsSlice = []uuid.UUID{}
	}
	suiteIDs := pq.Array(suiteIDsSlice)

	search := strings.TrimSpace(params.Search)

	type rawRow struct {
		TestID             uuid.UUID      `db:"test_id"`
		TestName           string         `db:"test_name"`
		StepCount          int            `db:"step_count"`
		StepSummariesJSON  string         `db:"step_summaries_json"`
		SuiteID            uuid.UUID      `db:"suite_id"`
		SuiteName          string         `db:"suite_name"`
		ProjectID          uuid.UUID      `db:"project_id"`
		ProjectName        string         `db:"project_name"`
		LatestResultsJSON  string         `db:"latest_results_json"`
		SuccessPercent     sql.NullInt32  `db:"success_percent"`
		LastRunAt          sql.NullTime   `db:"last_run_at"`
		NextRunAt          sql.NullTime   `db:"next_run_at"`
		IsLive             bool           `db:"is_live"`
	}

	var rawRows []rawRow
	if err := s.db.SelectContext(ctx, &rawRows, query, pq.Array(projectIDs), envID, limit, suiteIDs, search); err != nil {
		return nil, nil, fmt.Errorf("failed to list test health: %w", err)
	}

	// Post-process: parse JSONB fields and apply plugin filter
	rows := make([]TestHealthRow, 0, len(rawRows))
	suiteSet := make(map[uuid.UUID]string)

	for _, raw := range rawRows {
		row := TestHealthRow{
			TestID:         raw.TestID,
			TestName:       raw.TestName,
			StepCount:      raw.StepCount,
			SuiteID:        raw.SuiteID,
			SuiteName:      raw.SuiteName,
			ProjectID:      raw.ProjectID,
			ProjectName:    raw.ProjectName,
			SuccessPercent: raw.SuccessPercent,
			LastRunAt:      raw.LastRunAt,
			NextRunAt:      raw.NextRunAt,
			IsLive:         raw.IsLive,
		}

		// Parse step_summaries to extract plugins
		row.TestPlugins = parsePluginsFromStepSummaries(raw.StepSummariesJSON)

		// Parse latest_results
		row.LatestResults = parseLatestResults(raw.LatestResultsJSON)

		// Apply plugin filter (OR semantics)
		if len(params.Plugins) > 0 {
			if !hasAnyPlugin(row.TestPlugins, params.Plugins) {
				continue
			}
		}

		rows = append(rows, row)
		suiteSet[raw.SuiteID] = raw.SuiteName
	}

	// Build suite options
	suiteOptions := make([]TestHealthSuiteOption, 0, len(suiteSet))
	for id, name := range suiteSet {
		suiteOptions = append(suiteOptions, TestHealthSuiteOption{ID: id, Name: name})
	}

	return rows, suiteOptions, nil
}

// parsePluginsFromStepSummaries extracts unique plugin names from step_summaries JSONB
func parsePluginsFromStepSummaries(json string) []string {
	// Simple parsing without unmarshalling entire struct
	// Format: [{"step_index":0,"plugin":"http","name":"..."},...]
	plugins := make(map[string]struct{})

	// Find all "plugin":"..." patterns
	idx := 0
	for {
		start := strings.Index(json[idx:], `"plugin":"`)
		if start == -1 {
			break
		}
		start += idx + len(`"plugin":"`)
		end := strings.Index(json[start:], `"`)
		if end == -1 {
			break
		}
		plugin := json[start : start+end]
		plugins[plugin] = struct{}{}
		idx = start + end
	}

	result := make([]string, 0, len(plugins))
	for p := range plugins {
		result = append(result, p)
	}
	return result
}

// parseLatestResults parses the JSON array of result statuses
func parseLatestResults(json string) []string {
	// Format: ["PASSED","FAILED","RUNNING",...]
	// Map to lowercase: success, failed, pending, running
	if json == "" || json == "[]" || json == "null" {
		return []string{}
	}

	// Simple parsing
	results := make([]string, 0, 20)
	idx := 0
	for {
		start := strings.Index(json[idx:], `"`)
		if start == -1 {
			break
		}
		start += idx + 1
		end := strings.Index(json[start:], `"`)
		if end == -1 {
			break
		}
		status := json[start : start+end]
		// Map to UI status
		switch strings.ToUpper(status) {
		case "PASSED":
			results = append(results, "success")
		case "FAILED", "CANCELLED", "TIMEOUT":
			results = append(results, "failed")
		case "RUNNING":
			results = append(results, "running")
		case "PENDING":
			results = append(results, "pending")
		default:
			results = append(results, "pending")
		}
		idx = start + end
	}
	return results
}

// hasAnyPlugin checks if testPlugins contains any of the requested plugins (case-insensitive)
func hasAnyPlugin(testPlugins, requestedPlugins []string) bool {
	for _, rp := range requestedPlugins {
		rpLower := strings.ToLower(rp)
		for _, tp := range testPlugins {
			if strings.ToLower(tp) == rpLower {
				return true
			}
		}
	}
	return false
}
