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
	"github.com/robfig/cron/v3"
)

// CreateProjectScheduleInput contains fields for creating a project schedule
type CreateProjectScheduleInput struct {
	ProjectID      uuid.UUID
	EnvironmentID  uuid.UUID
	Name           string
	CronExpression string
	Timezone       string
	Enabled        bool
	CreatedBy      uuid.UUID
}

// UpdateProjectScheduleInput contains fields for updating a project schedule
type UpdateProjectScheduleInput struct {
	Name           *string
	CronExpression *string
	Timezone       *string
	Enabled        *bool
}

// ComputeNextRunAt calculates the next run time for a cron expression in the given timezone
func ComputeNextRunAt(cronExpression, timezone string) (time.Time, error) {
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timezone %q: %w", timezone, err)
	}

	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(cronExpression)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid cron expression %q: %w", cronExpression, err)
	}

	now := time.Now().In(loc)
	next := schedule.Next(now)
	return next.UTC(), nil
}

// CreateProjectSchedule creates a new project schedule
func (s *Store) CreateProjectSchedule(ctx context.Context, input CreateProjectScheduleInput) (ProjectSchedule, error) {
	if input.ProjectID == uuid.Nil {
		return ProjectSchedule{}, errors.New("project_id required")
	}
	if input.EnvironmentID == uuid.Nil {
		return ProjectSchedule{}, errors.New("environment_id required")
	}
	if strings.TrimSpace(input.Name) == "" {
		return ProjectSchedule{}, errors.New("name required")
	}
	if strings.TrimSpace(input.CronExpression) == "" {
		return ProjectSchedule{}, errors.New("cron_expression required")
	}
	if strings.TrimSpace(input.Timezone) == "" {
		input.Timezone = "UTC"
	}

	// Compute next_run_at
	nextRunAt, err := ComputeNextRunAt(input.CronExpression, input.Timezone)
	if err != nil {
		return ProjectSchedule{}, err
	}

	id := uuid.New()
	const query = `
		INSERT INTO project_schedules (id, project_id, environment_id, name, cron_expression, timezone, enabled, next_run_at, created_by, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, NOW(), NOW())
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
		id, input.ProjectID, input.EnvironmentID, input.Name, input.CronExpression,
		input.Timezone, input.Enabled, nextRunAt, createdBy); err != nil {
		if isUniqueViolation(err, "project_schedules_project_env_idx") {
			return ProjectSchedule{}, fmt.Errorf("a schedule already exists for this project and environment")
		}
		return ProjectSchedule{}, fmt.Errorf("failed to create project schedule: %w", err)
	}

	schedule := ProjectSchedule{
		ID:             id,
		ProjectID:      input.ProjectID,
		EnvironmentID:  input.EnvironmentID,
		Name:           input.Name,
		CronExpression: input.CronExpression,
		Timezone:       input.Timezone,
		Enabled:        input.Enabled,
		NextRunAt:      sql.NullTime{Time: nextRunAt, Valid: true},
		CreatedAt:      dest.CreatedAt,
		UpdatedAt:      dest.UpdatedAt,
	}
	if input.CreatedBy != uuid.Nil {
		schedule.CreatedBy = uuid.NullUUID{UUID: input.CreatedBy, Valid: true}
	}

	return schedule, nil
}

// GetProjectSchedule retrieves a project schedule by ID
func (s *Store) GetProjectSchedule(ctx context.Context, scheduleID uuid.UUID) (ProjectSchedule, error) {
	const query = `
		SELECT ps.id, ps.project_id, ps.environment_id, ps.name, ps.cron_expression, ps.timezone, ps.enabled,
		       ps.next_run_at, ps.last_run_at, ps.last_run_id, ps.last_run_status, ps.created_by,
		       ps.created_at, ps.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM project_schedules ps
		JOIN project_environments pe ON ps.environment_id = pe.id
		WHERE ps.id = $1
	`

	var row struct {
		ProjectSchedule
		EnvName string `db:"env_name"`
		EnvSlug string `db:"env_slug"`
	}
	if err := s.db.GetContext(ctx, &row, query, scheduleID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectSchedule{}, sql.ErrNoRows
		}
		return ProjectSchedule{}, fmt.Errorf("failed to get project schedule: %w", err)
	}

	row.EnvironmentName = row.EnvName
	row.EnvironmentSlug = row.EnvSlug
	return row.ProjectSchedule, nil
}

// GetProjectScheduleByProjectEnv retrieves a schedule by project and environment
func (s *Store) GetProjectScheduleByProjectEnv(ctx context.Context, projectID, environmentID uuid.UUID) (ProjectSchedule, bool, error) {
	const query = `
		SELECT ps.id, ps.project_id, ps.environment_id, ps.name, ps.cron_expression, ps.timezone, ps.enabled,
		       ps.next_run_at, ps.last_run_at, ps.last_run_id, ps.last_run_status, ps.created_by,
		       ps.created_at, ps.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM project_schedules ps
		JOIN project_environments pe ON ps.environment_id = pe.id
		WHERE ps.project_id = $1 AND ps.environment_id = $2
	`

	var row struct {
		ProjectSchedule
		EnvName string `db:"env_name"`
		EnvSlug string `db:"env_slug"`
	}
	if err := s.db.GetContext(ctx, &row, query, projectID, environmentID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectSchedule{}, false, nil
		}
		return ProjectSchedule{}, false, fmt.Errorf("failed to get project schedule: %w", err)
	}

	row.EnvironmentName = row.EnvName
	row.EnvironmentSlug = row.EnvSlug
	return row.ProjectSchedule, true, nil
}

// ListProjectSchedulesByProject lists all schedules for a project
func (s *Store) ListProjectSchedulesByProject(ctx context.Context, projectID uuid.UUID) ([]ProjectSchedule, error) {
	const query = `
		SELECT ps.id, ps.project_id, ps.environment_id, ps.name, ps.cron_expression, ps.timezone, ps.enabled,
		       ps.next_run_at, ps.last_run_at, ps.last_run_id, ps.last_run_status, ps.created_by,
		       ps.created_at, ps.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM project_schedules ps
		JOIN project_environments pe ON ps.environment_id = pe.id
		WHERE ps.project_id = $1
		ORDER BY pe.name ASC
	`

	type row struct {
		ProjectSchedule
		EnvName string `db:"env_name"`
		EnvSlug string `db:"env_slug"`
	}

	var rows []row
	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list project schedules: %w", err)
	}

	schedules := make([]ProjectSchedule, 0, len(rows))
	for _, r := range rows {
		r.EnvironmentName = r.EnvName
		r.EnvironmentSlug = r.EnvSlug
		schedules = append(schedules, r.ProjectSchedule)
	}

	return schedules, nil
}

// UpdateProjectSchedule updates a project schedule
func (s *Store) UpdateProjectSchedule(ctx context.Context, scheduleID uuid.UUID, input UpdateProjectScheduleInput) (ProjectSchedule, error) {
	// First fetch current schedule to compute next_run_at if cron/tz changed
	current, err := s.GetProjectSchedule(ctx, scheduleID)
	if err != nil {
		return ProjectSchedule{}, err
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
			return ProjectSchedule{}, err
		}
		current.NextRunAt = sql.NullTime{Time: nextRunAt, Valid: true}
	}

	const query = `
		UPDATE project_schedules
		SET name = $2, cron_expression = $3, timezone = $4, enabled = $5, next_run_at = $6, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	if err := s.db.GetContext(ctx, &current.UpdatedAt, query,
		scheduleID, current.Name, current.CronExpression, current.Timezone, current.Enabled, current.NextRunAt.Time); err != nil {
		if isUniqueViolation(err, "project_schedules_project_env_idx") {
			return ProjectSchedule{}, fmt.Errorf("a schedule already exists for this project and environment")
		}
		return ProjectSchedule{}, fmt.Errorf("failed to update project schedule: %w", err)
	}

	return current, nil
}

// DeleteProjectSchedule deletes a project schedule
func (s *Store) DeleteProjectSchedule(ctx context.Context, scheduleID uuid.UUID) error {
	const query = `DELETE FROM project_schedules WHERE id = $1`

	res, err := s.db.ExecContext(ctx, query, scheduleID)
	if err != nil {
		return fmt.Errorf("failed to delete project schedule: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListDueProjectScheduleIDs returns IDs of enabled project schedules that are due to run.
// This is a lightweight query intended for the scheduler's discovery phase.
// Actual claiming is done atomically via ClaimDueProjectSchedule.
func (s *Store) ListDueProjectScheduleIDs(ctx context.Context, before time.Time, limit int) ([]uuid.UUID, error) {
	if limit <= 0 {
		limit = 100
	}

	const query = `
		SELECT id
		FROM project_schedules
		WHERE enabled = true AND next_run_at IS NOT NULL AND next_run_at <= $1
		ORDER BY next_run_at ASC
		LIMIT $2
	`

	var ids []uuid.UUID
	if err := s.db.SelectContext(ctx, &ids, query, before, limit); err != nil {
		return nil, fmt.Errorf("failed to list due project schedule IDs: %w", err)
	}

	return ids, nil
}

// ListDueProjectSchedules returns enabled project schedules that are due to run.
// Note: This no longer uses row locking since ClaimDueProjectSchedule provides
// atomic claiming that's safe in HA deployments.
func (s *Store) ListDueProjectSchedules(ctx context.Context, before time.Time, limit int) ([]ProjectSchedule, error) {
	if limit <= 0 {
		limit = 100
	}

	const query = `
		SELECT ps.id, ps.project_id, ps.environment_id, ps.name, ps.cron_expression, ps.timezone, ps.enabled,
		       ps.next_run_at, ps.last_run_at, ps.last_run_id, ps.last_run_status, ps.created_by,
		       ps.created_at, ps.updated_at,
		       pe.name as env_name, pe.slug as env_slug
		FROM project_schedules ps
		JOIN project_environments pe ON ps.environment_id = pe.id
		WHERE ps.enabled = true AND ps.next_run_at IS NOT NULL AND ps.next_run_at <= $1
		ORDER BY ps.next_run_at ASC
		LIMIT $2
	`

	type row struct {
		ProjectSchedule
		EnvName string `db:"env_name"`
		EnvSlug string `db:"env_slug"`
	}

	var rows []row
	if err := s.db.SelectContext(ctx, &rows, query, before, limit); err != nil {
		return nil, fmt.Errorf("failed to list due project schedules: %w", err)
	}

	schedules := make([]ProjectSchedule, 0, len(rows))
	for _, r := range rows {
		r.EnvironmentName = r.EnvName
		r.EnvironmentSlug = r.EnvSlug
		schedules = append(schedules, r.ProjectSchedule)
	}

	return schedules, nil
}

// ClaimAndAdvanceProjectSchedule atomically advances a schedule's next_run_at to the next occurrence
// Returns the claimed schedule with updated next_run_at
//
// DEPRECATED: Use ClaimDueProjectSchedule for atomic claiming that's safe in HA deployments.
func (s *Store) ClaimAndAdvanceProjectSchedule(ctx context.Context, scheduleID uuid.UUID) (ProjectSchedule, error) {
	schedule, err := s.GetProjectSchedule(ctx, scheduleID)
	if err != nil {
		return ProjectSchedule{}, err
	}

	// Compute next occurrence
	nextRunAt, err := ComputeNextRunAt(schedule.CronExpression, schedule.Timezone)
	if err != nil {
		return ProjectSchedule{}, err
	}

	const query = `
		UPDATE project_schedules
		SET next_run_at = $2, updated_at = NOW()
		WHERE id = $1
		RETURNING updated_at
	`

	if err := s.db.GetContext(ctx, &schedule.UpdatedAt, query, scheduleID, nextRunAt); err != nil {
		return ProjectSchedule{}, fmt.Errorf("failed to advance project schedule: %w", err)
	}

	schedule.NextRunAt = sql.NullTime{Time: nextRunAt, Valid: true}
	return schedule, nil
}

// ClaimDueProjectSchedule atomically claims a schedule if it's due to run.
// This is safe in HA deployments - only one instance can successfully claim a schedule.
// Returns (true, schedule) if claimed, (false, empty) if schedule was not due or already claimed.
func (s *Store) ClaimDueProjectSchedule(ctx context.Context, scheduleID uuid.UUID, now time.Time) (bool, ProjectSchedule, error) {
	// First get the schedule to compute next_run_at
	schedule, err := s.GetProjectSchedule(ctx, scheduleID)
	if err != nil {
		return false, ProjectSchedule{}, err
	}

	// Compute next occurrence
	nextRunAt, err := ComputeNextRunAt(schedule.CronExpression, schedule.Timezone)
	if err != nil {
		return false, ProjectSchedule{}, err
	}

	// Atomically claim: only succeeds if schedule is still enabled and due
	const query = `
		UPDATE project_schedules
		SET next_run_at = $2, updated_at = NOW()
		WHERE id = $1
		  AND enabled = true
		  AND next_run_at IS NOT NULL
		  AND next_run_at <= $3
		RETURNING id, project_id, environment_id, name, cron_expression, timezone, enabled,
		          next_run_at, last_run_at, last_run_id, last_run_status, created_by,
		          created_at, updated_at
	`

	var result ProjectSchedule
	err = s.db.GetContext(ctx, &result, query, scheduleID, nextRunAt, now)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Schedule was not due or already claimed by another instance
			return false, ProjectSchedule{}, nil
		}
		return false, ProjectSchedule{}, fmt.Errorf("failed to claim project schedule: %w", err)
	}

	// Preserve joined fields from the original schedule
	result.EnvironmentName = schedule.EnvironmentName
	result.EnvironmentSlug = schedule.EnvironmentSlug

	return true, result, nil
}

// UpdateProjectScheduleLastRun updates the last run info for a schedule
func (s *Store) UpdateProjectScheduleLastRun(ctx context.Context, scheduleID uuid.UUID, runID, status string, runAt time.Time) error {
	const query = `
		UPDATE project_schedules
		SET last_run_at = $2, last_run_id = $3, last_run_status = $4, updated_at = NOW()
		WHERE id = $1
	`

	res, err := s.db.ExecContext(ctx, query, scheduleID, runAt, runID, status)
	if err != nil {
		return fmt.Errorf("failed to update project schedule last run: %w", err)
	}

	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// ListActiveSuitesForProjectSchedule returns active suites for a project's default branch
// that don't have their own override schedule for the given environment
func (s *Store) ListActiveSuitesForProjectSchedule(ctx context.Context, projectID, environmentID uuid.UUID) ([]Suite, error) {
	// Get project's default branch
	var defaultBranch string
	const getBranchQuery = `SELECT default_branch FROM projects WHERE id = $1`
	if err := s.db.GetContext(ctx, &defaultBranch, getBranchQuery, projectID); err != nil {
		return nil, fmt.Errorf("failed to get project default branch: %w", err)
	}

	// Get active suites for default branch that don't have suite-level overrides for this environment
	const query = `
		SELECT s.id, s.project_id, s.name, s.description, s.file_path, s.source_ref, s.yaml_payload, s.test_count,
		       s.last_run_id, s.last_run_status, s.last_run_at, s.created_at, s.updated_at
		FROM suites s
		WHERE s.project_id = $1
		  AND lower(s.source_ref) = lower($2)
		  AND s.is_active = true
		  AND s.yaml_payload != ''
		  AND NOT EXISTS (
		      SELECT 1 FROM suite_schedules ss
		      WHERE ss.suite_id = s.id
		        AND ss.enabled = true
		        AND ss.environment_id = $3
		  )
		ORDER BY s.name ASC
	`

	var suites []Suite
	if err := s.db.SelectContext(ctx, &suites, query, projectID, defaultBranch, environmentID); err != nil {
		return nil, fmt.Errorf("failed to list active suites for schedule: %w", err)
	}
	if suites == nil {
		suites = []Suite{}
	}

	return suites, nil
}

// ProjectWithOrg returns a project with its organization ID for scheduler use
type ProjectWithOrg struct {
	ID             uuid.UUID `db:"id"`
	OrganizationID uuid.UUID `db:"organization_id"`
	Name           string    `db:"name"`
	RepoURL        string    `db:"repo_url"`
	DefaultBranch  string    `db:"default_branch"`
	PathScope      []string  // Unmarshaled from JSONB
	SourceRef      string    `db:"source_ref"`
	CreatedAt      time.Time `db:"created_at"`
}

func (s *Store) GetProjectWithOrg(ctx context.Context, projectID uuid.UUID) (ProjectWithOrg, error) {
	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE id = $1
	`

	var dest struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Name           string    `db:"name"`
		RepoURL        string    `db:"repo_url"`
		DefaultBranch  string    `db:"default_branch"`
		PathScope      string    `db:"path_scope"`
		SourceRef      string    `db:"source_ref"`
		CreatedAt      time.Time `db:"created_at"`
	}

	if err := s.db.GetContext(ctx, &dest, query, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectWithOrg{}, sql.ErrNoRows
		}
		return ProjectWithOrg{}, fmt.Errorf("failed to get project: %w", err)
	}

	result := ProjectWithOrg{
		ID:             dest.ID,
		OrganizationID: dest.OrganizationID,
		Name:           dest.Name,
		RepoURL:        dest.RepoURL,
		DefaultBranch:  dest.DefaultBranch,
		SourceRef:      dest.SourceRef,
		CreatedAt:      dest.CreatedAt,
	}

	// Unmarshal JSONB path_scope
	if dest.PathScope != "" {
		if err := json.Unmarshal([]byte(dest.PathScope), &result.PathScope); err != nil {
			return ProjectWithOrg{}, fmt.Errorf("failed to parse path_scope: %w", err)
		}
	}

	return result, nil
}
