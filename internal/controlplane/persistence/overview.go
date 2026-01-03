package persistence

import (
	"context"
	"fmt"

	"github.com/google/uuid"
)

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
