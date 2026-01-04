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

// CreateSchedule creates a new suite schedule
func (s *Store) CreateSchedule(ctx context.Context, schedule SuiteSchedule) (SuiteSchedule, error) {
	if schedule.SuiteID == uuid.Nil {
		return SuiteSchedule{}, errors.New("suite id required")
	}
	if schedule.ProjectID == uuid.Nil {
		return SuiteSchedule{}, errors.New("project id required")
	}
	if strings.TrimSpace(schedule.Name) == "" {
		return SuiteSchedule{}, errors.New("schedule name required")
	}
	if strings.TrimSpace(schedule.CronExpression) == "" {
		return SuiteSchedule{}, errors.New("cron expression required")
	}

	if schedule.ID == uuid.Nil {
		schedule.ID = uuid.New()
	}
	if schedule.Timezone == "" {
		schedule.Timezone = "UTC"
	}

	const query = `
        INSERT INTO suite_schedules (
            id, suite_id, project_id, name, description, cron_expression, timezone,
            enabled, environment_id, next_run_at, created_by, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc, envID, createdBy, nextRunAt interface{}
	if schedule.Description.Valid {
		desc = schedule.Description.String
	}
	if schedule.EnvironmentID.Valid {
		envID = schedule.EnvironmentID.UUID
	}
	if schedule.CreatedBy.Valid {
		createdBy = schedule.CreatedBy.UUID
	}
	if schedule.NextRunAt.Valid {
		nextRunAt = schedule.NextRunAt.Time
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		schedule.ID, schedule.SuiteID, schedule.ProjectID, schedule.Name, desc,
		schedule.CronExpression, schedule.Timezone, schedule.Enabled,
		envID, nextRunAt, createdBy); err != nil {
		if isUniqueViolation(err, "suite_schedules_suite_name_idx") {
			return SuiteSchedule{}, fmt.Errorf("schedule name already exists for this suite")
		}
		return SuiteSchedule{}, fmt.Errorf("failed to create schedule: %w", err)
	}

	schedule.CreatedAt = dest.CreatedAt
	schedule.UpdatedAt = dest.UpdatedAt
	return schedule, nil
}

// GetSchedule retrieves a schedule by ID
func (s *Store) GetSchedule(ctx context.Context, scheduleID uuid.UUID) (SuiteSchedule, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, cron_expression, timezone,
               enabled, environment_id, next_run_at, last_run_at, last_run_id, last_run_status,
               created_by, created_at, updated_at
        FROM suite_schedules
        WHERE id = $1
    `

	var schedule SuiteSchedule
	if err := s.db.GetContext(ctx, &schedule, query, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteSchedule{}, sql.ErrNoRows
		}
		return SuiteSchedule{}, fmt.Errorf("failed to get schedule: %w", err)
	}

	return schedule, nil
}

// ListSchedulesBySuite returns all schedules for a suite
func (s *Store) ListSchedulesBySuite(ctx context.Context, suiteID uuid.UUID) ([]SuiteSchedule, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, cron_expression, timezone,
               enabled, environment_id, next_run_at, last_run_at, last_run_id, last_run_status,
               created_by, created_at, updated_at
        FROM suite_schedules
        WHERE suite_id = $1
        ORDER BY name ASC
    `

	var schedules []SuiteSchedule
	if err := s.db.SelectContext(ctx, &schedules, query, suiteID); err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	if schedules == nil {
		schedules = []SuiteSchedule{}
	}

	return schedules, nil
}

// ListSchedulesByProject returns all schedules for a project
func (s *Store) ListSchedulesByProject(ctx context.Context, projectID uuid.UUID) ([]SuiteSchedule, error) {
	const query = `
        SELECT id, suite_id, project_id, name, description, cron_expression, timezone,
               enabled, environment_id, next_run_at, last_run_at, last_run_id, last_run_status,
               created_by, created_at, updated_at
        FROM suite_schedules
        WHERE project_id = $1
        ORDER BY name ASC
    `

	var schedules []SuiteSchedule
	if err := s.db.SelectContext(ctx, &schedules, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list schedules: %w", err)
	}
	if schedules == nil {
		schedules = []SuiteSchedule{}
	}

	return schedules, nil
}

// ListDueSchedules returns all enabled schedules that are due to run
func (s *Store) ListDueSchedules(ctx context.Context, before time.Time, limit int) ([]SuiteSchedule, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
        SELECT id, suite_id, project_id, name, description, cron_expression, timezone,
               enabled, environment_id, next_run_at, last_run_at, last_run_id, last_run_status,
               created_by, created_at, updated_at
        FROM suite_schedules
        WHERE enabled = TRUE AND next_run_at IS NOT NULL AND next_run_at <= $1
        ORDER BY next_run_at ASC
        LIMIT $2
    `

	var schedules []SuiteSchedule
	if err := s.db.SelectContext(ctx, &schedules, query, before, limit); err != nil {
		return nil, fmt.Errorf("failed to list due schedules: %w", err)
	}
	if schedules == nil {
		schedules = []SuiteSchedule{}
	}

	return schedules, nil
}

// UpdateSchedule updates an existing schedule
func (s *Store) UpdateSchedule(ctx context.Context, schedule SuiteSchedule) (SuiteSchedule, error) {
	if schedule.ID == uuid.Nil {
		return SuiteSchedule{}, errors.New("schedule id required")
	}

	const query = `
        UPDATE suite_schedules
        SET name = $2, description = $3, cron_expression = $4, timezone = $5,
            enabled = $6, environment_id = $7, next_run_at = $8, updated_at = NOW()
        WHERE id = $1
        RETURNING updated_at
    `

	var desc, envID, nextRunAt interface{}
	if schedule.Description.Valid {
		desc = schedule.Description.String
	}
	if schedule.EnvironmentID.Valid {
		envID = schedule.EnvironmentID.UUID
	}
	if schedule.NextRunAt.Valid {
		nextRunAt = schedule.NextRunAt.Time
	}

	var updatedAt time.Time
	if err := s.db.GetContext(ctx, &updatedAt, query,
		schedule.ID, schedule.Name, desc, schedule.CronExpression, schedule.Timezone,
		schedule.Enabled, envID, nextRunAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteSchedule{}, sql.ErrNoRows
		}
		if isUniqueViolation(err, "suite_schedules_suite_name_idx") {
			return SuiteSchedule{}, fmt.Errorf("schedule name already exists for this suite")
		}
		return SuiteSchedule{}, fmt.Errorf("failed to update schedule: %w", err)
	}

	schedule.UpdatedAt = updatedAt
	return schedule, nil
}

// UpdateScheduleLastRun updates the last run info and next run time for a schedule
func (s *Store) UpdateScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt, nextRunAt time.Time) error {
	const query = `
        UPDATE suite_schedules
        SET last_run_id = $2, last_run_status = $3, last_run_at = $4, next_run_at = $5, updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, scheduleID, runID, status, runAt, nextRunAt)
	if err != nil {
		return fmt.Errorf("failed to update schedule last run: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// SetScheduleEnabled enables or disables a schedule
func (s *Store) SetScheduleEnabled(ctx context.Context, scheduleID uuid.UUID, enabled bool) error {
	const query = `
        UPDATE suite_schedules
        SET enabled = $2, updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, scheduleID, enabled)
	if err != nil {
		return fmt.Errorf("failed to set schedule enabled: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// DeleteSchedule removes a schedule
func (s *Store) DeleteSchedule(ctx context.Context, scheduleID uuid.UUID) error {
	const query = `DELETE FROM suite_schedules WHERE id = $1`

	res, err := s.db.ExecContext(ctx, query, scheduleID)
	if err != nil {
		return fmt.Errorf("failed to delete schedule: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListEnabledSchedulesByProject returns all enabled schedules for a project
func (s *Store) ListEnabledSchedulesByProject(ctx context.Context, projectID uuid.UUID) ([]SuiteSchedule, error) {
	const query = `
		SELECT id, suite_id, project_id, name, description, cron_expression, timezone,
		       enabled, environment_id, next_run_at, last_run_at, last_run_id, last_run_status,
		       created_by, created_at, updated_at
		FROM suite_schedules
		WHERE project_id = $1 AND enabled = TRUE
		ORDER BY name ASC
	`

	var schedules []SuiteSchedule
	if err := s.db.SelectContext(ctx, &schedules, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list enabled schedules: %w", err)
	}
	if schedules == nil {
		schedules = []SuiteSchedule{}
	}

	return schedules, nil
}

// ==================== Suite Schedule Override Functions ====================
// These functions support suite-level schedule overrides with environment scoping.

// SuiteScheduleWithEnv extends SuiteSchedule with joined environment fields
type SuiteScheduleWithEnv struct {
	SuiteSchedule
	EnvironmentName string `db:"env_name"`
	EnvironmentSlug string `db:"env_slug"`
}

// CreateSuiteScheduleInput contains fields for creating a suite schedule override
type CreateSuiteScheduleInput struct {
	SuiteID        uuid.UUID
	ProjectID      uuid.UUID
	EnvironmentID  uuid.UUID
	Name           string
	CronExpression string
	Timezone       string
	Enabled        bool
	CreatedBy      uuid.UUID
}

// UpdateSuiteScheduleInput contains fields for updating a suite schedule override
type UpdateSuiteScheduleInput struct {
	Name           *string
	CronExpression *string
	Timezone       *string
	Enabled        *bool
}

// ListSuiteSchedulesBySuiteWithEnv returns all schedule overrides for a suite with environment details
func (s *Store) ListSuiteSchedulesBySuiteWithEnv(ctx context.Context, suiteID uuid.UUID) ([]SuiteScheduleWithEnv, error) {
	const query = `
		SELECT ss.id, ss.suite_id, ss.project_id, ss.name, ss.description, ss.cron_expression, ss.timezone,
		       ss.enabled, ss.environment_id, ss.next_run_at, ss.last_run_at, ss.last_run_id, ss.last_run_status,
		       ss.created_by, ss.created_at, ss.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM suite_schedules ss
		JOIN project_environments pe ON ss.environment_id = pe.id
		WHERE ss.suite_id = $1 AND ss.environment_id IS NOT NULL
		ORDER BY pe.name ASC
	`

	var schedules []SuiteScheduleWithEnv
	if err := s.db.SelectContext(ctx, &schedules, query, suiteID); err != nil {
		return nil, fmt.Errorf("failed to list suite schedules with env: %w", err)
	}
	if schedules == nil {
		schedules = []SuiteScheduleWithEnv{}
	}

	return schedules, nil
}

// GetSuiteScheduleWithEnv retrieves a suite schedule by ID with environment details
func (s *Store) GetSuiteScheduleWithEnv(ctx context.Context, scheduleID uuid.UUID) (SuiteScheduleWithEnv, error) {
	const query = `
		SELECT ss.id, ss.suite_id, ss.project_id, ss.name, ss.description, ss.cron_expression, ss.timezone,
		       ss.enabled, ss.environment_id, ss.next_run_at, ss.last_run_at, ss.last_run_id, ss.last_run_status,
		       ss.created_by, ss.created_at, ss.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM suite_schedules ss
		JOIN project_environments pe ON ss.environment_id = pe.id
		WHERE ss.id = $1
	`

	var schedule SuiteScheduleWithEnv
	if err := s.db.GetContext(ctx, &schedule, query, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteScheduleWithEnv{}, sql.ErrNoRows
		}
		return SuiteScheduleWithEnv{}, fmt.Errorf("failed to get suite schedule: %w", err)
	}

	return schedule, nil
}

// GetSuiteScheduleBySuiteEnv retrieves a suite schedule by suite and environment IDs
func (s *Store) GetSuiteScheduleBySuiteEnv(ctx context.Context, suiteID, environmentID uuid.UUID) (SuiteScheduleWithEnv, bool, error) {
	const query = `
		SELECT ss.id, ss.suite_id, ss.project_id, ss.name, ss.description, ss.cron_expression, ss.timezone,
		       ss.enabled, ss.environment_id, ss.next_run_at, ss.last_run_at, ss.last_run_id, ss.last_run_status,
		       ss.created_by, ss.created_at, ss.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM suite_schedules ss
		JOIN project_environments pe ON ss.environment_id = pe.id
		WHERE ss.suite_id = $1 AND ss.environment_id = $2
	`

	var schedule SuiteScheduleWithEnv
	if err := s.db.GetContext(ctx, &schedule, query, suiteID, environmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteScheduleWithEnv{}, false, nil
		}
		return SuiteScheduleWithEnv{}, false, fmt.Errorf("failed to get suite schedule by suite/env: %w", err)
	}

	return schedule, true, nil
}

// CreateSuiteScheduleOverride creates a new suite schedule override for an environment
func (s *Store) CreateSuiteScheduleOverride(ctx context.Context, input CreateSuiteScheduleInput) (SuiteScheduleWithEnv, error) {
	if input.SuiteID == uuid.Nil {
		return SuiteScheduleWithEnv{}, errors.New("suite_id required")
	}
	if input.ProjectID == uuid.Nil {
		return SuiteScheduleWithEnv{}, errors.New("project_id required")
	}
	if input.EnvironmentID == uuid.Nil {
		return SuiteScheduleWithEnv{}, errors.New("environment_id required for suite override")
	}
	if strings.TrimSpace(input.Name) == "" {
		return SuiteScheduleWithEnv{}, errors.New("name required")
	}
	if strings.TrimSpace(input.CronExpression) == "" {
		return SuiteScheduleWithEnv{}, errors.New("cron_expression required")
	}
	if strings.TrimSpace(input.Timezone) == "" {
		input.Timezone = "UTC"
	}

	// Compute next_run_at using the shared function
	nextRunAt, err := ComputeNextRunAt(input.CronExpression, input.Timezone)
	if err != nil {
		return SuiteScheduleWithEnv{}, err
	}

	id := uuid.New()
	const query = `
		INSERT INTO suite_schedules (id, suite_id, project_id, name, cron_expression, timezone, enabled, environment_id, next_run_at, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	var createdBy interface{}
	if input.CreatedBy != uuid.Nil {
		createdBy = input.CreatedBy
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		id, input.SuiteID, input.ProjectID, input.Name, input.CronExpression,
		input.Timezone, input.Enabled, input.EnvironmentID, nextRunAt, createdBy); err != nil {
		if isUniqueViolation(err, "suite_schedules_suite_env_idx") {
			return SuiteScheduleWithEnv{}, fmt.Errorf("a schedule override already exists for this suite and environment")
		}
		if isUniqueViolation(err, "suite_schedules_suite_name_idx") {
			return SuiteScheduleWithEnv{}, fmt.Errorf("schedule name already exists for this suite")
		}
		return SuiteScheduleWithEnv{}, fmt.Errorf("failed to create suite schedule: %w", err)
	}

	// Return the created schedule with env details
	return s.GetSuiteScheduleWithEnv(ctx, id)
}

// UpsertSuiteScheduleOverride creates or updates a suite schedule override for an environment
func (s *Store) UpsertSuiteScheduleOverride(ctx context.Context, input CreateSuiteScheduleInput) (SuiteScheduleWithEnv, bool, error) {
	// Check if an override already exists for this suite+env
	existing, exists, err := s.GetSuiteScheduleBySuiteEnv(ctx, input.SuiteID, input.EnvironmentID)
	if err != nil {
		return SuiteScheduleWithEnv{}, false, err
	}

	if exists {
		// Update the existing override
		updateInput := UpdateSuiteScheduleInput{
			Name:           &input.Name,
			CronExpression: &input.CronExpression,
			Timezone:       &input.Timezone,
			Enabled:        &input.Enabled,
		}
		updated, err := s.UpdateSuiteScheduleOverride(ctx, existing.ID, updateInput)
		if err != nil {
			return SuiteScheduleWithEnv{}, false, err
		}
		return updated, false, nil // false = not created (updated)
	}

	// Create new override
	created, err := s.CreateSuiteScheduleOverride(ctx, input)
	if err != nil {
		return SuiteScheduleWithEnv{}, false, err
	}
	return created, true, nil // true = created (not updated)
}

// UpdateSuiteScheduleOverride updates a suite schedule override
func (s *Store) UpdateSuiteScheduleOverride(ctx context.Context, scheduleID uuid.UUID, input UpdateSuiteScheduleInput) (SuiteScheduleWithEnv, error) {
	// First fetch current schedule to compute next_run_at if cron/tz changed
	current, err := s.GetSuiteScheduleWithEnv(ctx, scheduleID)
	if err != nil {
		return SuiteScheduleWithEnv{}, err
	}

	// Apply updates
	if input.Name != nil {
		current.Name = *input.Name
	}
	if input.CronExpression != nil {
		current.CronExpression = *input.CronExpression
	}
	if input.Timezone != nil {
		current.Timezone = *input.Timezone
	}
	if input.Enabled != nil {
		current.Enabled = *input.Enabled
	}

	// Recompute next_run_at if cron or timezone changed
	if input.CronExpression != nil || input.Timezone != nil {
		nextRunAt, err := ComputeNextRunAt(current.CronExpression, current.Timezone)
		if err != nil {
			return SuiteScheduleWithEnv{}, err
		}
		current.NextRunAt = sql.NullTime{Time: nextRunAt, Valid: true}
	}

	const query = `
		UPDATE suite_schedules
		SET name = $2, cron_expression = $3, timezone = $4, enabled = $5, next_run_at = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	if err := s.db.GetContext(ctx, &current.UpdatedAt, query,
		scheduleID, current.Name, current.CronExpression, current.Timezone, current.Enabled, current.NextRunAt.Time); err != nil {
		if isUniqueViolation(err, "suite_schedules_suite_name_idx") {
			return SuiteScheduleWithEnv{}, fmt.Errorf("schedule name already exists for this suite")
		}
		return SuiteScheduleWithEnv{}, fmt.Errorf("failed to update suite schedule: %w", err)
	}

	return current, nil
}

// DeleteSuiteScheduleOverride removes a suite schedule override
func (s *Store) DeleteSuiteScheduleOverride(ctx context.Context, scheduleID uuid.UUID) error {
	const query = `DELETE FROM suite_schedules WHERE id = $1`

	res, err := s.db.ExecContext(ctx, query, scheduleID)
	if err != nil {
		return fmt.Errorf("failed to delete suite schedule: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ==================== Suite Schedule Scheduler Functions ====================
// These functions support the scheduler for firing suite schedule overrides.

// ListDueSuiteScheduleIDs returns IDs of enabled suite schedules that are due to run.
// This is a lightweight query intended for the scheduler's discovery phase.
func (s *Store) ListDueSuiteScheduleIDs(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 100
	}

	const query = `
		SELECT id
		FROM suite_schedules
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= $1 AND environment_id IS NOT NULL
		ORDER BY next_run_at ASC
		LIMIT $2
	`

	var ids []uuid.UUID
	if err := s.db.SelectContext(ctx, &ids, query, before, limit); err != nil {
		return nil, fmt.Errorf("failed to list due suite schedule IDs: %w", err)
	}

	return ids, nil
}

// ClaimDueSuiteSchedule atomically claims a suite schedule if it's due to run.
// This is safe in HA deployments - only one instance can successfully claim a schedule.
// Returns (true, schedule) if claimed, (false, empty) if schedule was not due or already claimed.
func (s *Store) ClaimDueSuiteSchedule(ctx context.Context, scheduleID uuid.UUID, now time.Time) (bool, SuiteScheduleWithEnv, error) {
	// First get the schedule to compute next_run_at
	schedule, err := s.GetSuiteScheduleWithEnv(ctx, scheduleID)
	if err != nil {
		return false, SuiteScheduleWithEnv{}, err
	}

	// Compute next occurrence
	nextRunAt, err := ComputeNextRunAt(schedule.CronExpression, schedule.Timezone)
	if err != nil {
		return false, SuiteScheduleWithEnv{}, err
	}

	// Atomically claim: only succeeds if schedule is still enabled and due
	const query = `
		UPDATE suite_schedules
		SET next_run_at = $2, updated_at = NOW()
		WHERE id = $1
		  AND enabled = true
		  AND next_run_at IS NOT NULL
		  AND next_run_at <= $3
		RETURNING id
	`

	var claimedID uuid.UUID
	err = s.db.GetContext(ctx, &claimedID, query, scheduleID, nextRunAt, now)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Schedule was not due or already claimed by another instance
			return false, SuiteScheduleWithEnv{}, nil
		}
		return false, SuiteScheduleWithEnv{}, fmt.Errorf("failed to claim suite schedule: %w", err)
	}

	// Update the schedule's next_run_at in the returned object
	schedule.NextRunAt = sql.NullTime{Time: nextRunAt, Valid: true}

	return true, schedule, nil
}

// UpdateSuiteScheduleLastRun updates the last run info for a suite schedule
func (s *Store) UpdateSuiteScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt time.Time) error {
	const query = `
		UPDATE suite_schedules
		SET last_run_at = $2, last_run_id = $3, last_run_status = $4, updated_at = NOW()
		WHERE id = $1
	`

	res, err := s.db.ExecContext(ctx, query, scheduleID, runAt, runID, status)
	if err != nil {
		return fmt.Errorf("failed to update suite schedule last run: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetSuiteWithProjectAndEnv retrieves a suite with its project and environment info for the scheduler
type SuiteWithProjectAndEnv struct {
	Suite
	ProjectOrganizationID        uuid.UUID `db:"project_org_id"`
	ProjectDefaultBranch         string    `db:"project_default_branch"`
	EnvironmentSlug              string    `db:"env_slug"`
	EnvironmentName              string    `db:"env_name"`
	ProjectDefaultBranchHeadSHA  string    `db:"project_head_sha"`
	ProjectDefaultBranchHeadMsg  string    `db:"project_head_message"`
}

func (s *Store) GetSuiteWithProjectAndEnv(ctx context.Context, suiteID, environmentID uuid.UUID) (SuiteWithProjectAndEnv, error) {
	const query = `
		SELECT s.id, s.project_id, s.name, s.description, s.file_path, s.source_ref, s.yaml_payload, s.test_count,
		       s.last_run_id, s.last_run_status, s.last_run_at, s.created_at, s.updated_at,
		       p.organization_id as project_org_id, p.default_branch as project_default_branch,
		       pe.slug as env_slug, pe.name as env_name,
		       COALESCE(p.default_branch_head_sha, '') as project_head_sha,
		       COALESCE(p.default_branch_head_message, '') as project_head_message
		FROM suites s
		JOIN projects p ON s.project_id = p.id
		JOIN project_environments pe ON pe.id = $2
		WHERE s.id = $1
	`

	var result SuiteWithProjectAndEnv
	if err := s.db.GetContext(ctx, &result, query, suiteID, environmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteWithProjectAndEnv{}, sql.ErrNoRows
		}
		return SuiteWithProjectAndEnv{}, fmt.Errorf("failed to get suite with project and env: %w", err)
	}

	return result, nil
}
