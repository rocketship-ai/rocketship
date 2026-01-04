package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CreateSuite creates a new test suite
func (s *Store) CreateSuite(ctx context.Context, suite Suite) (Suite, error) {
	if suite.ProjectID == uuid.Nil {
		return Suite{}, errors.New("project id required")
	}
	if strings.TrimSpace(suite.Name) == "" {
		return Suite{}, errors.New("suite name required")
	}
	if strings.TrimSpace(suite.SourceRef) == "" {
		return Suite{}, errors.New("source ref required")
	}
	if !suite.FilePath.Valid || strings.TrimSpace(suite.FilePath.String) == "" {
		return Suite{}, errors.New("file_path required")
	}

	if suite.ID == uuid.Nil {
		suite.ID = uuid.New()
	}

	const query = `
        INSERT INTO suites (id, project_id, name, description, file_path, source_ref, yaml_payload, test_count, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc interface{}
	if suite.Description.Valid {
		desc = suite.Description.String
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		suite.ID, suite.ProjectID, suite.Name, desc, suite.FilePath.String, suite.SourceRef, suite.YamlPayload, suite.TestCount); err != nil {
		if isUniqueViolation(err, "suites_project_file_ref_idx") {
			return Suite{}, fmt.Errorf("suite file_path already exists in project for this ref")
		}
		if isUniqueViolation(err, "suites_project_name_ref_idx") {
			return Suite{}, fmt.Errorf("suite name already exists in project for this ref")
		}
		return Suite{}, fmt.Errorf("failed to create suite: %w", err)
	}

	suite.CreatedAt = dest.CreatedAt
	suite.UpdatedAt = dest.UpdatedAt
	return suite, nil
}

// GetSuite retrieves a suite by ID
func (s *Store) GetSuite(ctx context.Context, projectID, suiteID uuid.UUID) (Suite, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE project_id = $1 AND id = $2
    `

	var suite Suite
	if err := s.db.GetContext(ctx, &suite, query, projectID, suiteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Suite{}, sql.ErrNoRows
		}
		return Suite{}, fmt.Errorf("failed to get suite: %w", err)
	}

	return suite, nil
}

// GetSuiteByID retrieves a suite by ID only (without requiring project_id)
func (s *Store) GetSuiteByID(ctx context.Context, suiteID uuid.UUID) (Suite, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE id = $1
    `

	var suite Suite
	if err := s.db.GetContext(ctx, &suite, query, suiteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Suite{}, sql.ErrNoRows
		}
		return Suite{}, fmt.Errorf("failed to get suite: %w", err)
	}

	return suite, nil
}

// GetSuiteByName retrieves a suite by name and source_ref within a project
// Returns (suite, found, error) - found is true if the suite exists
func (s *Store) GetSuiteByName(ctx context.Context, projectID uuid.UUID, name, sourceRef string) (Suite, bool, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE project_id = $1 AND lower(name) = lower($2) AND lower(source_ref) = lower($3)
    `

	var suite Suite
	if err := s.db.GetContext(ctx, &suite, query, projectID, name, sourceRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Suite{}, false, nil
		}
		return Suite{}, false, fmt.Errorf("failed to get suite by name: %w", err)
	}

	return suite, true, nil
}

// GetSuiteByFilePath retrieves a suite by file_path and source_ref within a project
// Returns (suite, found, error) - found is true if the suite exists
func (s *Store) GetSuiteByFilePath(ctx context.Context, projectID uuid.UUID, filePath, sourceRef string) (Suite, bool, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE project_id = $1 AND lower(file_path) = lower($2) AND lower(source_ref) = lower($3)
    `

	var suite Suite
	if err := s.db.GetContext(ctx, &suite, query, projectID, filePath, sourceRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Suite{}, false, nil
		}
		return Suite{}, false, fmt.Errorf("failed to get suite by file_path: %w", err)
	}

	return suite, true, nil
}

// UpsertSuite creates or updates a suite by file_path and source_ref
// Suite identity is now based on file_path, not name - the YAML file location is the source of truth
func (s *Store) UpsertSuite(ctx context.Context, suite Suite) (Suite, error) {
	if suite.ProjectID == uuid.Nil {
		return Suite{}, errors.New("project id required")
	}
	if strings.TrimSpace(suite.Name) == "" {
		return Suite{}, errors.New("suite name required")
	}
	if strings.TrimSpace(suite.SourceRef) == "" {
		return Suite{}, errors.New("source ref required")
	}
	if !suite.FilePath.Valid || strings.TrimSpace(suite.FilePath.String) == "" {
		return Suite{}, errors.New("file_path required for suite upsert")
	}

	// Check if suite exists by file_path (primary identity)
	existing, found, err := s.GetSuiteByFilePath(ctx, suite.ProjectID, suite.FilePath.String, suite.SourceRef)
	if err != nil {
		return Suite{}, err
	}

	if found {
		// Update existing suite - allow name change since file_path is the identity
		const updateQuery = `
			UPDATE suites
			SET name = $2, description = $3, yaml_payload = $4, test_count = $5, is_active = true,
			    deactivated_at = NULL, deactivated_reason = NULL, updated_at = NOW()
			WHERE id = $1
			RETURNING updated_at
		`

		var desc interface{}
		if suite.Description.Valid {
			desc = suite.Description.String
		}

		var updatedAt time.Time
		if err := s.db.GetContext(ctx, &updatedAt, updateQuery,
			existing.ID, suite.Name, desc, suite.YamlPayload, suite.TestCount); err != nil {
			if isUniqueViolation(err, "suites_project_name_ref_idx") {
				return Suite{}, fmt.Errorf("suite name already exists in project for this ref")
			}
			return Suite{}, fmt.Errorf("failed to update suite: %w", err)
		}

		existing.Name = suite.Name
		existing.Description = suite.Description
		existing.YamlPayload = suite.YamlPayload
		existing.TestCount = suite.TestCount
		existing.UpdatedAt = updatedAt
		return existing, nil
	}

	// Create new suite
	return s.CreateSuite(ctx, suite)
}

// UpsertTest creates or updates a test by name and source_ref within a suite
func (s *Store) UpsertTest(ctx context.Context, test Test) (Test, error) {
	if test.SuiteID == uuid.Nil {
		return Test{}, errors.New("suite id required")
	}
	if test.ProjectID == uuid.Nil {
		return Test{}, errors.New("project id required")
	}
	if strings.TrimSpace(test.Name) == "" {
		return Test{}, errors.New("test name required")
	}
	if strings.TrimSpace(test.SourceRef) == "" {
		return Test{}, errors.New("source ref required")
	}

	// Check if test exists (now keyed by suite_id, name, source_ref)
	const selectQuery = `
		SELECT id, suite_id, project_id, name, description, source_ref, step_count,
		       last_run_id, last_run_status, last_run_at, pass_rate, avg_duration_ms,
		       created_at, updated_at
		FROM tests
		WHERE suite_id = $1 AND lower(name) = lower($2) AND lower(source_ref) = lower($3)
	`

	var existing Test
	err := s.db.GetContext(ctx, &existing, selectQuery, test.SuiteID, test.Name, test.SourceRef)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return Test{}, fmt.Errorf("failed to check existing test: %w", err)
	}

	if err == nil {
		// Update existing test
		const updateQuery = `
			UPDATE tests
			SET description = $2, step_count = $3, step_summaries = $4, updated_at = NOW()
			WHERE id = $1
			RETURNING updated_at
		`

		var desc interface{}
		if test.Description.Valid {
			desc = test.Description.String
		}

		// Marshal step summaries to JSON
		stepSummariesJSON, err := json.Marshal(test.StepSummaries)
		if err != nil {
			return Test{}, fmt.Errorf("failed to marshal step summaries: %w", err)
		}
		if test.StepSummaries == nil {
			stepSummariesJSON = []byte("[]")
		}

		var updatedAt time.Time
		if err := s.db.GetContext(ctx, &updatedAt, updateQuery,
			existing.ID, desc, test.StepCount, stepSummariesJSON); err != nil {
			return Test{}, fmt.Errorf("failed to update test: %w", err)
		}

		existing.Description = test.Description
		existing.StepCount = test.StepCount
		existing.StepSummaries = test.StepSummaries
		existing.UpdatedAt = updatedAt
		return existing, nil
	}

	// Create new test
	return s.CreateTest(ctx, test)
}

// ListSuites returns all suites for a project
func (s *Store) ListSuites(ctx context.Context, projectID uuid.UUID) ([]Suite, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE project_id = $1
        ORDER BY name ASC, source_ref ASC
    `

	var suites []Suite
	if err := s.db.SelectContext(ctx, &suites, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list suites: %w", err)
	}
	if suites == nil {
		suites = []Suite{}
	}

	return suites, nil
}

// ListRecentSuiteActivity returns suites with recent run activity
func (s *Store) ListRecentSuiteActivity(ctx context.Context, projectID uuid.UUID, limit int) ([]Suite, error) {
	if limit <= 0 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, yaml_payload, test_count,
               last_run_id, last_run_status, last_run_at, created_at, updated_at
        FROM suites
        WHERE project_id = $1 AND last_run_at IS NOT NULL
        ORDER BY last_run_at DESC
        LIMIT $2
    `

	var suites []Suite
	if err := s.db.SelectContext(ctx, &suites, query, projectID, limit); err != nil {
		return nil, fmt.Errorf("failed to list suite activity: %w", err)
	}
	if suites == nil {
		suites = []Suite{}
	}

	return suites, nil
}

// UpdateSuiteLastRun updates the last run info for a suite
func (s *Store) UpdateSuiteLastRun(ctx context.Context, suiteID uuid.UUID, runID, status string, runAt time.Time) error {
	const query = `
        UPDATE suites
        SET last_run_id = $2, last_run_status = $3, last_run_at = $4, updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, suiteID, runID, status, runAt)
	if err != nil {
		return fmt.Errorf("failed to update suite last run: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteSuite removes a suite and all its tests
func (s *Store) DeleteSuite(ctx context.Context, projectID, suiteID uuid.UUID) error {
	const query = `DELETE FROM suites WHERE project_id = $1 AND id = $2`

	res, err := s.db.ExecContext(ctx, query, projectID, suiteID)
	if err != nil {
		return fmt.Errorf("failed to delete suite: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// CreateTest creates a new test within a suite
func (s *Store) CreateTest(ctx context.Context, test Test) (Test, error) {
	if test.SuiteID == uuid.Nil {
		return Test{}, errors.New("suite id required")
	}
	if test.ProjectID == uuid.Nil {
		return Test{}, errors.New("project id required")
	}
	if strings.TrimSpace(test.Name) == "" {
		return Test{}, errors.New("test name required")
	}
	if strings.TrimSpace(test.SourceRef) == "" {
		return Test{}, errors.New("source ref required")
	}

	if test.ID == uuid.Nil {
		test.ID = uuid.New()
	}

	const query = `
        INSERT INTO tests (id, suite_id, project_id, name, description, source_ref, step_count, step_summaries, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc interface{}
	if test.Description.Valid {
		desc = test.Description.String
	}

	// Marshal step summaries to JSON
	stepSummariesJSON, err := json.Marshal(test.StepSummaries)
	if err != nil {
		return Test{}, fmt.Errorf("failed to marshal step summaries: %w", err)
	}
	if test.StepSummaries == nil {
		stepSummariesJSON = []byte("[]")
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		test.ID, test.SuiteID, test.ProjectID, test.Name, desc, test.SourceRef, test.StepCount, stepSummariesJSON); err != nil {
		if isUniqueViolation(err, "tests_suite_name_ref_idx") {
			return Test{}, fmt.Errorf("test name already exists in suite for this ref")
		}
		return Test{}, fmt.Errorf("failed to create test: %w", err)
	}

	test.CreatedAt = dest.CreatedAt
	test.UpdatedAt = dest.UpdatedAt
	return test, nil
}

// GetTest retrieves a test by ID
func (s *Store) GetTest(ctx context.Context, testID uuid.UUID) (Test, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, source_ref, step_count,
               last_run_id, last_run_status, last_run_at, pass_rate, avg_duration_ms,
               created_at, updated_at
        FROM tests
        WHERE id = $1
    `

	var test Test
	if err := s.db.GetContext(ctx, &test, query, testID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Test{}, sql.ErrNoRows
		}
		return Test{}, fmt.Errorf("failed to get test: %w", err)
	}

	return test, nil
}

// testRow is a helper struct for scanning tests with JSONB columns
type testRow struct {
	ID              uuid.UUID       `db:"id"`
	SuiteID         uuid.UUID       `db:"suite_id"`
	ProjectID       uuid.UUID       `db:"project_id"`
	Name            string          `db:"name"`
	Description     sql.NullString  `db:"description"`
	SourceRef       string          `db:"source_ref"`
	StepCount       int             `db:"step_count"`
	StepSummaries   []byte          `db:"step_summaries"`
	LastRunID       sql.NullString  `db:"last_run_id"`
	LastRunStatus   sql.NullString  `db:"last_run_status"`
	LastRunAt       sql.NullTime    `db:"last_run_at"`
	PassRate        sql.NullFloat64 `db:"pass_rate"`
	AvgDurationMs   sql.NullInt64   `db:"avg_duration_ms"`
	CreatedAt       time.Time       `db:"created_at"`
	UpdatedAt       time.Time       `db:"updated_at"`
}

func (r testRow) toTest() Test {
	t := Test{
		ID:            r.ID,
		SuiteID:       r.SuiteID,
		ProjectID:     r.ProjectID,
		Name:          r.Name,
		Description:   r.Description,
		SourceRef:     r.SourceRef,
		StepCount:     r.StepCount,
		LastRunID:     r.LastRunID,
		LastRunStatus: r.LastRunStatus,
		LastRunAt:     r.LastRunAt,
		PassRate:      r.PassRate,
		AvgDurationMs: r.AvgDurationMs,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
	// Parse step_summaries JSONB
	if len(r.StepSummaries) > 0 {
		var summaries []StepSummary
		if err := json.Unmarshal(r.StepSummaries, &summaries); err == nil {
			t.StepSummaries = summaries
		}
	}
	if t.StepSummaries == nil {
		t.StepSummaries = []StepSummary{}
	}
	return t
}

// ListTestsBySuite returns all tests for a suite
func (s *Store) ListTestsBySuite(ctx context.Context, suiteID uuid.UUID) ([]Test, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, source_ref, step_count, step_summaries,
               last_run_id, last_run_status, last_run_at, pass_rate, avg_duration_ms,
               created_at, updated_at
        FROM tests
        WHERE suite_id = $1
        ORDER BY name ASC, source_ref ASC
    `

	var rows []testRow
	if err := s.db.SelectContext(ctx, &rows, query, suiteID); err != nil {
		return nil, fmt.Errorf("failed to list tests: %w", err)
	}

	tests := make([]Test, 0, len(rows))
	for _, row := range rows {
		tests = append(tests, row.toTest())
	}

	return tests, nil
}

// ListTestsByProject returns all tests for a project
func (s *Store) ListTestsByProject(ctx context.Context, projectID uuid.UUID, limit int) ([]Test, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
        SELECT id, suite_id, project_id, name, description, source_ref, step_count,
               last_run_id, last_run_status, last_run_at, pass_rate, avg_duration_ms,
               created_at, updated_at
        FROM tests
        WHERE project_id = $1
        ORDER BY pass_rate ASC NULLS LAST, name ASC, source_ref ASC
        LIMIT $2
    `

	var tests []Test
	if err := s.db.SelectContext(ctx, &tests, query, projectID, limit); err != nil {
		return nil, fmt.Errorf("failed to list tests: %w", err)
	}
	if tests == nil {
		tests = []Test{}
	}

	return tests, nil
}

// UpdateTestLastRun updates the last run info for a test
func (s *Store) UpdateTestLastRun(ctx context.Context, testID uuid.UUID, runID, status string, runAt time.Time, durationMs int64) error {
	const query = `
        UPDATE tests
        SET last_run_id = $2, last_run_status = $3, last_run_at = $4,
            avg_duration_ms = CASE
                WHEN avg_duration_ms IS NULL THEN $5
                ELSE (avg_duration_ms + $5) / 2
            END,
            updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, testID, runID, status, runAt, durationMs)
	if err != nil {
		return fmt.Errorf("failed to update test last run: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// UpdateTestPassRate updates the pass rate for a test based on historical runs
func (s *Store) UpdateTestPassRate(ctx context.Context, testID uuid.UUID, passRate float64) error {
	const query = `
        UPDATE tests
        SET pass_rate = $2, updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, testID, passRate)
	if err != nil {
		return fmt.Errorf("failed to update test pass rate: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteTest removes a test
func (s *Store) DeleteTest(ctx context.Context, testID uuid.UUID) error {
	const query = `DELETE FROM tests WHERE id = $1`

	res, err := s.db.ExecContext(ctx, query, testID)
	if err != nil {
		return fmt.Errorf("failed to delete test: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeactivateSuitesMissingFromDir soft-deletes suites under a .rocketship directory that are no longer present.
// This is used during full scans to reconcile deleted suite files.
// Only affects suites for the specified source_ref.
func (s *Store) DeactivateSuitesMissingFromDir(ctx context.Context, projectID uuid.UUID, sourceRef, rocketshipDir string, presentFilePaths []string, reason string) (int, error) {
	if projectID == uuid.Nil {
		return 0, errors.New("project id required")
	}
	if strings.TrimSpace(sourceRef) == "" {
		return 0, errors.New("source ref required")
	}
	if strings.TrimSpace(rocketshipDir) == "" {
		return 0, errors.New("rocketship dir required")
	}

	// Build the directory prefix for matching (e.g., "api/.rocketship/")
	dirPrefix := rocketshipDir + "/"

	// First, find suites to deactivate
	var suiteIDs []uuid.UUID
	var findQuery string
	var args []interface{}

	if len(presentFilePaths) == 0 {
		// No files present - deactivate all suites under this directory
		findQuery = `
			SELECT id FROM suites
			WHERE project_id = $1
			  AND lower(source_ref) = lower($2)
			  AND file_path LIKE $3
			  AND is_active = true
		`
		args = []interface{}{projectID, sourceRef, dirPrefix + "%"}
	} else {
		// Build a list of lowercase present paths for comparison
		// We use a subquery with unnest to compare against the array
		findQuery = `
			SELECT id FROM suites
			WHERE project_id = $1
			  AND lower(source_ref) = lower($2)
			  AND file_path LIKE $3
			  AND is_active = true
			  AND lower(file_path) NOT IN (SELECT lower(unnest($4::text[])))
		`
		args = []interface{}{projectID, sourceRef, dirPrefix + "%", pq.Array(presentFilePaths)}
	}

	if err := s.db.SelectContext(ctx, &suiteIDs, findQuery, args...); err != nil {
		return 0, fmt.Errorf("failed to find suites to deactivate: %w", err)
	}

	if len(suiteIDs) == 0 {
		return 0, nil
	}

	// Deactivate the suites
	const deactivateSuitesQuery = `
		UPDATE suites
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $2, updated_at = NOW()
		WHERE id = ANY($1)
	`
	if _, err := s.db.ExecContext(ctx, deactivateSuitesQuery, pq.Array(suiteIDs), reason); err != nil {
		return 0, fmt.Errorf("failed to deactivate suites: %w", err)
	}

	// Deactivate all tests under these suites for the same source_ref
	const deactivateTestsQuery = `
		UPDATE tests
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $3, updated_at = NOW()
		WHERE suite_id = ANY($1) AND lower(source_ref) = lower($2)
	`
	if _, err := s.db.ExecContext(ctx, deactivateTestsQuery, pq.Array(suiteIDs), sourceRef, reason); err != nil {
		return 0, fmt.Errorf("failed to deactivate tests for removed suites: %w", err)
	}

	return len(suiteIDs), nil
}

// DeactivateTestsMissingFromSuite soft-deletes tests that are no longer present in a suite's YAML.
// This is used during scans to reconcile deleted/renamed tests.
// Only affects tests for the specified source_ref.
func (s *Store) DeactivateTestsMissingFromSuite(ctx context.Context, suiteID uuid.UUID, sourceRef string, presentTestNames []string, reason string) (int, error) {
	if suiteID == uuid.Nil {
		return 0, errors.New("suite id required")
	}
	if strings.TrimSpace(sourceRef) == "" {
		return 0, errors.New("source ref required")
	}

	var findQuery string
	var args []interface{}

	if len(presentTestNames) == 0 {
		// No tests present - deactivate all tests in this suite for this ref
		findQuery = `
			SELECT id FROM tests
			WHERE suite_id = $1
			  AND lower(source_ref) = lower($2)
			  AND is_active = true
		`
		args = []interface{}{suiteID, sourceRef}
	} else {
		// Find tests not in the present list
		findQuery = `
			SELECT id FROM tests
			WHERE suite_id = $1
			  AND lower(source_ref) = lower($2)
			  AND is_active = true
			  AND lower(name) NOT IN (SELECT lower(unnest($3::text[])))
		`
		args = []interface{}{suiteID, sourceRef, pq.Array(presentTestNames)}
	}

	var testIDs []uuid.UUID
	if err := s.db.SelectContext(ctx, &testIDs, findQuery, args...); err != nil {
		return 0, fmt.Errorf("failed to find tests to deactivate: %w", err)
	}

	if len(testIDs) == 0 {
		return 0, nil
	}

	// Deactivate the tests
	const deactivateQuery = `
		UPDATE tests
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $2, updated_at = NOW()
		WHERE id = ANY($1)
	`
	if _, err := s.db.ExecContext(ctx, deactivateQuery, pq.Array(testIDs), reason); err != nil {
		return 0, fmt.Errorf("failed to deactivate tests: %w", err)
	}

	return len(testIDs), nil
}

// DeactivateSuiteByProjectRefAndFilePath soft-deletes a suite and its tests by file_path.
// This is used when a PR removes or renames a suite file.
// Only affects the specified source_ref (branch), not the default branch.
func (s *Store) DeactivateSuiteByProjectRefAndFilePath(ctx context.Context, projectID uuid.UUID, sourceRef, filePath, reason string) error {
	// First, deactivate the suite
	const suiteQuery = `
		UPDATE suites
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $4, updated_at = NOW()
		WHERE project_id = $1 AND lower(source_ref) = lower($2) AND lower(file_path) = lower($3)
		RETURNING id
	`

	var suiteID uuid.UUID
	err := s.db.GetContext(ctx, &suiteID, suiteQuery, projectID, sourceRef, filePath, reason)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Suite doesn't exist for this ref - not an error
			return nil
		}
		return fmt.Errorf("failed to deactivate suite: %w", err)
	}

	// Deactivate all tests under this suite for the same source_ref
	const testQuery = `
		UPDATE tests
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $3, updated_at = NOW()
		WHERE suite_id = $1 AND lower(source_ref) = lower($2)
	`

	if _, err := s.db.ExecContext(ctx, testQuery, suiteID, sourceRef, reason); err != nil {
		return fmt.Errorf("failed to deactivate tests for suite: %w", err)
	}

	return nil
}

// DeactivateSuitesForRepoAndSourceRef soft-deletes all suites for a given repo and source_ref.
// This is used when a PR is closed to clean up feature-branch discovery.
// Also deactivates all tests under those suites.
func (s *Store) DeactivateSuitesForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef, reason string) (int, error) {
	if orgID == uuid.Nil {
		return 0, errors.New("org id required")
	}
	if strings.TrimSpace(repoURL) == "" {
		return 0, errors.New("repo url required")
	}
	if strings.TrimSpace(sourceRef) == "" {
		return 0, errors.New("source ref required")
	}

	// Find all suites for this org's projects with this repo and source_ref
	const findSuitesQuery = `
		SELECT s.id FROM suites s
		JOIN projects p ON s.project_id = p.id
		WHERE p.organization_id = $1
		  AND lower(p.repo_url) = lower($2)
		  AND lower(s.source_ref) = lower($3)
		  AND s.is_active = true
	`

	var suiteIDs []uuid.UUID
	if err := s.db.SelectContext(ctx, &suiteIDs, findSuitesQuery, orgID, repoURL, sourceRef); err != nil {
		return 0, fmt.Errorf("failed to find suites to deactivate: %w", err)
	}

	if len(suiteIDs) == 0 {
		return 0, nil
	}

	// Deactivate the suites
	const deactivateSuitesQuery = `
		UPDATE suites
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $2, updated_at = NOW()
		WHERE id = ANY($1)
	`
	if _, err := s.db.ExecContext(ctx, deactivateSuitesQuery, pq.Array(suiteIDs), reason); err != nil {
		return 0, fmt.Errorf("failed to deactivate suites: %w", err)
	}

	// Deactivate all tests under these suites for the same source_ref
	const deactivateTestsQuery = `
		UPDATE tests
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $3, updated_at = NOW()
		WHERE suite_id = ANY($1) AND lower(source_ref) = lower($2)
	`
	if _, err := s.db.ExecContext(ctx, deactivateTestsQuery, pq.Array(suiteIDs), sourceRef, reason); err != nil {
		return 0, fmt.Errorf("failed to deactivate tests for suites: %w", err)
	}

	return len(suiteIDs), nil
}
