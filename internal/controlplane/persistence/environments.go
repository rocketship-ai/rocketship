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
)

// CreateEnvironment creates a new project environment
func (s *Store) CreateEnvironment(ctx context.Context, env ProjectEnvironment) (ProjectEnvironment, error) {
	if env.ProjectID == uuid.Nil {
		return ProjectEnvironment{}, errors.New("project id required")
	}
	if strings.TrimSpace(env.Name) == "" {
		return ProjectEnvironment{}, errors.New("environment name required")
	}
	if strings.TrimSpace(env.Slug) == "" {
		return ProjectEnvironment{}, errors.New("environment slug required")
	}

	if env.ID == uuid.Nil {
		env.ID = uuid.New()
	}

	// Normalize slug to lowercase
	env.Slug = strings.ToLower(strings.TrimSpace(env.Slug))

	// Encode env_secrets
	if env.EnvSecrets == nil {
		env.EnvSecrets = make(map[string]string)
	}
	secretsJSON, err := json.Marshal(env.EnvSecrets)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode env_secrets: %w", err)
	}

	// Encode config_vars
	if env.ConfigVars == nil {
		env.ConfigVars = make(map[string]interface{})
	}
	varsJSON, err := json.Marshal(env.ConfigVars)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode config_vars: %w", err)
	}

	var desc interface{}
	if env.Description.Valid {
		desc = env.Description.String
	}

	const query = `
		INSERT INTO project_environments (id, project_id, name, slug, description, env_secrets, config_vars, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7::jsonb, NOW(), NOW())
		RETURNING created_at, updated_at
	`

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		env.ID, env.ProjectID, env.Name, env.Slug,
		desc, string(secretsJSON), string(varsJSON)); err != nil {
		if isUniqueViolation(err, "project_environments_project_slug_idx") {
			return ProjectEnvironment{}, fmt.Errorf("environment slug already exists in project")
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to create environment: %w", err)
	}

	env.CreatedAt = dest.CreatedAt
	env.UpdatedAt = dest.UpdatedAt
	return env, nil
}

// envRow is a helper type for scanning environment rows with JSONB columns
type envRow struct {
	ID          uuid.UUID      `db:"id"`
	ProjectID   uuid.UUID      `db:"project_id"`
	Name        string         `db:"name"`
	Slug        string         `db:"slug"`
	Description sql.NullString `db:"description"`
	EnvSecrets  []byte         `db:"env_secrets"`
	ConfigVars  []byte         `db:"config_vars"`
	CreatedAt   time.Time      `db:"created_at"`
	UpdatedAt   time.Time      `db:"updated_at"`
}

func (r envRow) toEnvironment() (ProjectEnvironment, error) {
	env := ProjectEnvironment{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Name:        r.Name,
		Slug:        r.Slug,
		Description: r.Description,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}

	// Parse env_secrets
	if len(r.EnvSecrets) > 0 {
		var secrets map[string]string
		if err := json.Unmarshal(r.EnvSecrets, &secrets); err != nil {
			return ProjectEnvironment{}, fmt.Errorf("failed to parse env_secrets: %w", err)
		}
		env.EnvSecrets = secrets
	}
	if env.EnvSecrets == nil {
		env.EnvSecrets = make(map[string]string)
	}

	// Parse config_vars
	if len(r.ConfigVars) > 0 {
		var vars map[string]interface{}
		if err := json.Unmarshal(r.ConfigVars, &vars); err != nil {
			return ProjectEnvironment{}, fmt.Errorf("failed to parse config_vars: %w", err)
		}
		env.ConfigVars = vars
	}
	if env.ConfigVars == nil {
		env.ConfigVars = make(map[string]interface{})
	}

	return env, nil
}

// GetEnvironment retrieves an environment by ID
func (s *Store) GetEnvironment(ctx context.Context, projectID, envID uuid.UUID) (ProjectEnvironment, error) {
	const query = `
		SELECT id, project_id, name, slug, description, env_secrets, config_vars, created_at, updated_at
		FROM project_environments
		WHERE project_id = $1 AND id = $2
	`

	var row envRow
	if err := s.db.GetContext(ctx, &row, query, projectID, envID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, sql.ErrNoRows
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to get environment: %w", err)
	}

	return row.toEnvironment()
}

// ListEnvironments returns all environments for a project
func (s *Store) ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]ProjectEnvironment, error) {
	const query = `
		SELECT id, project_id, name, slug, description, env_secrets, config_vars, created_at, updated_at
		FROM project_environments
		WHERE project_id = $1
		ORDER BY name ASC
	`

	var rows []envRow
	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	envs := make([]ProjectEnvironment, 0, len(rows))
	for _, r := range rows {
		env, err := r.toEnvironment()
		if err != nil {
			return nil, err
		}
		envs = append(envs, env)
	}

	return envs, nil
}

// UpdateEnvironment updates an existing environment
func (s *Store) UpdateEnvironment(ctx context.Context, env ProjectEnvironment) (ProjectEnvironment, error) {
	if env.ID == uuid.Nil {
		return ProjectEnvironment{}, errors.New("environment id required")
	}
	if env.ProjectID == uuid.Nil {
		return ProjectEnvironment{}, errors.New("project id required")
	}

	// Normalize slug to lowercase
	env.Slug = strings.ToLower(strings.TrimSpace(env.Slug))

	// Encode env_secrets
	if env.EnvSecrets == nil {
		env.EnvSecrets = make(map[string]string)
	}
	secretsJSON, err := json.Marshal(env.EnvSecrets)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode env_secrets: %w", err)
	}

	// Encode config_vars
	if env.ConfigVars == nil {
		env.ConfigVars = make(map[string]interface{})
	}
	varsJSON, err := json.Marshal(env.ConfigVars)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode config_vars: %w", err)
	}

	var desc interface{}
	if env.Description.Valid {
		desc = env.Description.String
	}

	const query = `
		UPDATE project_environments
		SET name = $3, slug = $4, description = $5, env_secrets = $6::jsonb, config_vars = $7::jsonb, updated_at = NOW()
		WHERE id = $1 AND project_id = $2
		RETURNING updated_at
	`

	var updatedAt time.Time
	if err := s.db.GetContext(ctx, &updatedAt, query,
		env.ID, env.ProjectID, env.Name, env.Slug,
		desc, string(secretsJSON), string(varsJSON)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, sql.ErrNoRows
		}
		if isUniqueViolation(err, "project_environments_project_slug_idx") {
			return ProjectEnvironment{}, fmt.Errorf("environment slug already exists in project")
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to update environment: %w", err)
	}

	env.UpdatedAt = updatedAt
	return env, nil
}

// DeleteEnvironment removes an environment
func (s *Store) DeleteEnvironment(ctx context.Context, projectID, envID uuid.UUID) error {
	const query = `DELETE FROM project_environments WHERE project_id = $1 AND id = $2`

	res, err := s.db.ExecContext(ctx, query, projectID, envID)
	if err != nil {
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}

	return nil
}

// GetEnvironmentBySlug retrieves an environment by its slug within a project
func (s *Store) GetEnvironmentBySlug(ctx context.Context, projectID uuid.UUID, slug string) (ProjectEnvironment, error) {
	const query = `
		SELECT id, project_id, name, slug, description, env_secrets, config_vars, created_at, updated_at
		FROM project_environments
		WHERE project_id = $1 AND lower(slug) = lower($2)
	`

	var row envRow
	if err := s.db.GetContext(ctx, &row, query, projectID, strings.TrimSpace(slug)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, sql.ErrNoRows
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to get environment by slug: %w", err)
	}

	return row.toEnvironment()
}

// GetSelectedEnvironment retrieves the user's selected environment for a project
// Returns (env, true, nil) if a selection exists, (zero, false, nil) if no selection
func (s *Store) GetSelectedEnvironment(ctx context.Context, userID, projectID uuid.UUID) (ProjectEnvironment, bool, error) {
	const query = `
		SELECT e.id, e.project_id, e.name, e.slug, e.description, e.env_secrets, e.config_vars, e.created_at, e.updated_at
		FROM project_environment_selections s
		JOIN project_environments e ON e.id = s.environment_id
		WHERE s.user_id = $1 AND s.project_id = $2
	`

	var row envRow
	if err := s.db.GetContext(ctx, &row, query, userID, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, false, nil
		}
		return ProjectEnvironment{}, false, fmt.Errorf("failed to get selected environment: %w", err)
	}

	env, err := row.toEnvironment()
	if err != nil {
		return ProjectEnvironment{}, false, err
	}
	return env, true, nil
}

// SetSelectedEnvironment sets the user's selected environment for a project
// The environmentID must belong to the specified projectID
func (s *Store) SetSelectedEnvironment(ctx context.Context, userID, projectID, environmentID uuid.UUID) error {
	// Validate that the environment belongs to the project in a single query with upsert
	const query = `
		INSERT INTO project_environment_selections (user_id, project_id, environment_id, created_at, updated_at)
		SELECT $1, $2, $3, NOW(), NOW()
		WHERE EXISTS (
			SELECT 1 FROM project_environments WHERE id = $3 AND project_id = $2
		)
		ON CONFLICT (user_id, project_id) DO UPDATE
		SET environment_id = EXCLUDED.environment_id, updated_at = NOW()
	`

	result, err := s.db.ExecContext(ctx, query, userID, projectID, environmentID)
	if err != nil {
		return fmt.Errorf("failed to set selected environment: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to check rows affected: %w", err)
	}
	if rows == 0 {
		return fmt.Errorf("environment not found in project")
	}

	return nil
}
