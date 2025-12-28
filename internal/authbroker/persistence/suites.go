package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
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

	if suite.ID == uuid.Nil {
		suite.ID = uuid.New()
	}

	const query = `
        INSERT INTO suites (id, project_id, name, description, file_path, source_ref, test_count, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc, filePath interface{}
	if suite.Description.Valid {
		desc = suite.Description.String
	}
	if suite.FilePath.Valid {
		filePath = suite.FilePath.String
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		suite.ID, suite.ProjectID, suite.Name, desc, filePath, suite.SourceRef, suite.TestCount); err != nil {
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
        SELECT id, project_id, name, description, file_path, source_ref, test_count,
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

// GetSuiteByName retrieves a suite by name and source_ref within a project
// Returns (suite, found, error) - found is true if the suite exists
func (s *Store) GetSuiteByName(ctx context.Context, projectID uuid.UUID, name, sourceRef string) (Suite, bool, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, test_count,
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

// UpsertSuite creates or updates a suite by name and source_ref
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

	// Check if suite exists (now keyed by project_id, name, source_ref)
	existing, found, err := s.GetSuiteByName(ctx, suite.ProjectID, suite.Name, suite.SourceRef)
	if err != nil {
		return Suite{}, err
	}

	if found {
		// Update existing suite
		const updateQuery = `
			UPDATE suites
			SET description = $2, file_path = $3, test_count = $4, updated_at = NOW()
			WHERE id = $1
			RETURNING updated_at
		`

		var desc, filePath interface{}
		if suite.Description.Valid {
			desc = suite.Description.String
		}
		if suite.FilePath.Valid {
			filePath = suite.FilePath.String
		}

		var updatedAt time.Time
		if err := s.db.GetContext(ctx, &updatedAt, updateQuery,
			existing.ID, desc, filePath, suite.TestCount); err != nil {
			return Suite{}, fmt.Errorf("failed to update suite: %w", err)
		}

		existing.Description = suite.Description
		existing.FilePath = suite.FilePath
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
			SET description = $2, step_count = $3, updated_at = NOW()
			WHERE id = $1
			RETURNING updated_at
		`

		var desc interface{}
		if test.Description.Valid {
			desc = test.Description.String
		}

		var updatedAt time.Time
		if err := s.db.GetContext(ctx, &updatedAt, updateQuery,
			existing.ID, desc, test.StepCount); err != nil {
			return Test{}, fmt.Errorf("failed to update test: %w", err)
		}

		existing.Description = test.Description
		existing.StepCount = test.StepCount
		existing.UpdatedAt = updatedAt
		return existing, nil
	}

	// Create new test
	return s.CreateTest(ctx, test)
}

// ListSuites returns all suites for a project
func (s *Store) ListSuites(ctx context.Context, projectID uuid.UUID) ([]Suite, error) {
	const query = `
        SELECT id, project_id, name, description, file_path, source_ref, test_count,
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
        SELECT id, project_id, name, description, file_path, source_ref, test_count,
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
        INSERT INTO tests (id, suite_id, project_id, name, description, source_ref, step_count, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc interface{}
	if test.Description.Valid {
		desc = test.Description.String
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		test.ID, test.SuiteID, test.ProjectID, test.Name, desc, test.SourceRef, test.StepCount); err != nil {
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

// ListTestsBySuite returns all tests for a suite
func (s *Store) ListTestsBySuite(ctx context.Context, suiteID uuid.UUID) ([]Test, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, source_ref, step_count,
               last_run_id, last_run_status, last_run_at, pass_rate, avg_duration_ms,
               created_at, updated_at
        FROM tests
        WHERE suite_id = $1
        ORDER BY name ASC, source_ref ASC
    `

	var tests []Test
	if err := s.db.SelectContext(ctx, &tests, query, suiteID); err != nil {
		return nil, fmt.Errorf("failed to list tests: %w", err)
	}
	if tests == nil {
		tests = []Test{}
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
