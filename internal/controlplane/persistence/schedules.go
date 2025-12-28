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
