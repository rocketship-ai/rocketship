package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ListActiveCITokens returns all active (non-revoked) CI tokens for a project
func (s *Store) ListActiveCITokens(ctx context.Context, projectID uuid.UUID) ([]CITokenRecord, error) {
	const query = `
		SELECT id, project_id, name, token_hash, scopes, never_expires,
		       expires_at, revoked_at, created_by, last_used_at, revoked_by,
		       description, created_at, updated_at
		FROM ci_tokens
		WHERE project_id = $1 AND revoked_at IS NULL
		ORDER BY name ASC
	`

	rows := []struct {
		CITokenRecord
		Scopes pq.StringArray `db:"scopes"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list active ci tokens: %w", err)
	}

	tokens := make([]CITokenRecord, 0, len(rows))
	for _, r := range rows {
		t := r.CITokenRecord
		t.Scopes = []string(r.Scopes)
		tokens = append(tokens, t)
	}

	return tokens, nil
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

// CountActiveCITokensForOrg returns the number of active (non-revoked) CI tokens across all projects in an organization
func (s *Store) CountActiveCITokensForOrg(ctx context.Context, orgID uuid.UUID) (int, error) {
	const query = `
		SELECT COUNT(*)
		FROM ci_tokens t
		JOIN projects p ON p.id = t.project_id
		WHERE p.organization_id = $1 AND t.revoked_at IS NULL
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID); err != nil {
		return 0, fmt.Errorf("failed to count active ci tokens: %w", err)
	}
	return count, nil
}
