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

	varsJSON, err := json.Marshal(env.Variables)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode variables: %w", err)
	}

	const query = `
        INSERT INTO project_environments (id, project_id, name, slug, description, is_default, variables, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7::jsonb, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	var desc interface{}
	if env.Description.Valid {
		desc = env.Description.String
	}

	dest := struct {
		CreatedAt time.Time `db:"created_at"`
		UpdatedAt time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query,
		env.ID, env.ProjectID, env.Name, strings.ToLower(env.Slug),
		desc, env.IsDefault, string(varsJSON)); err != nil {
		if isUniqueViolation(err, "project_environments_project_slug_idx") {
			return ProjectEnvironment{}, fmt.Errorf("environment slug already exists in project")
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to create environment: %w", err)
	}

	env.CreatedAt = dest.CreatedAt
	env.UpdatedAt = dest.UpdatedAt
	return env, nil
}

// GetEnvironment retrieves an environment by ID
func (s *Store) GetEnvironment(ctx context.Context, projectID, envID uuid.UUID) (ProjectEnvironment, error) {
	const query = `
        SELECT id, project_id, name, slug, description, is_default, variables, created_at, updated_at
        FROM project_environments
        WHERE project_id = $1 AND id = $2
    `

	dest := struct {
		ProjectEnvironment
		Variables string `db:"variables"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, projectID, envID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, sql.ErrNoRows
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to get environment: %w", err)
	}

	env := dest.ProjectEnvironment
	if dest.Variables != "" {
		if err := json.Unmarshal([]byte(dest.Variables), &env.Variables); err != nil {
			return ProjectEnvironment{}, fmt.Errorf("failed to parse variables: %w", err)
		}
	}
	if env.Variables == nil {
		env.Variables = make(map[string]string)
	}

	return env, nil
}

// ListEnvironments returns all environments for a project
func (s *Store) ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]ProjectEnvironment, error) {
	const query = `
        SELECT id, project_id, name, slug, description, is_default, variables, created_at, updated_at
        FROM project_environments
        WHERE project_id = $1
        ORDER BY is_default DESC, name ASC
    `

	rows := []struct {
		ProjectEnvironment
		Variables string `db:"variables"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}

	envs := make([]ProjectEnvironment, 0, len(rows))
	for _, r := range rows {
		env := r.ProjectEnvironment
		if r.Variables != "" {
			if err := json.Unmarshal([]byte(r.Variables), &env.Variables); err != nil {
				return nil, fmt.Errorf("failed to parse variables: %w", err)
			}
		}
		if env.Variables == nil {
			env.Variables = make(map[string]string)
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

	varsJSON, err := json.Marshal(env.Variables)
	if err != nil {
		return ProjectEnvironment{}, fmt.Errorf("failed to encode variables: %w", err)
	}

	const query = `
        UPDATE project_environments
        SET name = $3, slug = $4, description = $5, is_default = $6, variables = $7::jsonb, updated_at = NOW()
        WHERE id = $1 AND project_id = $2
        RETURNING updated_at
    `

	var desc interface{}
	if env.Description.Valid {
		desc = env.Description.String
	}

	var updatedAt time.Time
	if err := s.db.GetContext(ctx, &updatedAt, query,
		env.ID, env.ProjectID, env.Name, strings.ToLower(env.Slug),
		desc, env.IsDefault, string(varsJSON)); err != nil {
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

// GetDefaultEnvironment returns the default environment for a project, if any
func (s *Store) GetDefaultEnvironment(ctx context.Context, projectID uuid.UUID) (ProjectEnvironment, error) {
	const query = `
        SELECT id, project_id, name, slug, description, is_default, variables, created_at, updated_at
        FROM project_environments
        WHERE project_id = $1 AND is_default = TRUE
    `

	dest := struct {
		ProjectEnvironment
		Variables string `db:"variables"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectEnvironment{}, sql.ErrNoRows
		}
		return ProjectEnvironment{}, fmt.Errorf("failed to get default environment: %w", err)
	}

	env := dest.ProjectEnvironment
	if dest.Variables != "" {
		if err := json.Unmarshal([]byte(dest.Variables), &env.Variables); err != nil {
			return ProjectEnvironment{}, fmt.Errorf("failed to parse variables: %w", err)
		}
	}
	if env.Variables == nil {
		env.Variables = make(map[string]string)
	}

	return env, nil
}
