package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// UpsertGitHubUser creates or updates a user based on GitHub user ID
func (s *Store) UpsertGitHubUser(ctx context.Context, input GitHubUserInput) (User, error) {
	if input.GitHubUserID == 0 {
		return User{}, errors.New("github user id required")
	}
	email := normalizeEmail(input.Email)
	if email == "" {
		return User{}, errors.New("email required")
	}

	id := uuid.New()

	const query = `
        INSERT INTO users (id, github_user_id, email, name, username, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
        ON CONFLICT (github_user_id)
        DO UPDATE SET
            name = EXCLUDED.name,
            username = EXCLUDED.username,
            updated_at = NOW()
        RETURNING id, github_user_id, email, name, username, created_at, updated_at
    `

	var user User
	if err := s.db.GetContext(ctx, &user, query, id, input.GitHubUserID, email, input.Name, input.Username); err != nil {
		if isUniqueViolation(err, "users_email_unique") {
			return User{}, ErrEmailInUse
		}
		return User{}, fmt.Errorf("failed to upsert user: %w", err)
	}
	return user, nil
}

// UpdateUserEmail changes the email address for a user
func (s *Store) UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error {
	email = normalizeEmail(email)
	if email == "" {
		return errors.New("email required")
	}

	const query = `
        UPDATE users
        SET email = $2,
            updated_at = NOW()
        WHERE id = $1
    `

	res, err := s.db.ExecContext(ctx, query, userID, email)
	if err != nil {
		if isUniqueViolation(err, "users_email_unique") {
			return ErrEmailInUse
		}
		return fmt.Errorf("failed to update user email: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RoleSummary returns aggregated user membership information
func (s *Store) RoleSummary(ctx context.Context, userID uuid.UUID) (RoleSummary, error) {
	summary := RoleSummary{}

	const adminQuery = `SELECT organization_id FROM organization_admins WHERE user_id = $1`
	var adminOrgIDs []uuid.UUID
	if err := s.db.SelectContext(ctx, &adminOrgIDs, adminQuery, userID); err != nil {
		return RoleSummary{}, fmt.Errorf("failed to load organization admins: %w", err)
	}
	summary.Organizations = make([]OrganizationMembership, 0, len(adminOrgIDs))
	for _, id := range adminOrgIDs {
		summary.Organizations = append(summary.Organizations, OrganizationMembership{
			OrganizationID: id,
			IsAdmin:        true,
		})
	}

	const projectQuery = `
        SELECT pm.project_id, p.organization_id, pm.role
        FROM project_members pm
        JOIN projects p ON p.id = pm.project_id
        WHERE pm.user_id = $1
    `
	rows := []struct {
		ProjectID      uuid.UUID `db:"project_id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Role           string    `db:"role"`
	}{}
	if err := s.db.SelectContext(ctx, &rows, projectQuery, userID); err != nil {
		return RoleSummary{}, fmt.Errorf("failed to load project memberships: %w", err)
	}
	summary.Projects = make([]ProjectMembership, 0, len(rows))
	for _, r := range rows {
		summary.Projects = append(summary.Projects, ProjectMembership{
			ProjectID:      r.ProjectID,
			OrganizationID: r.OrganizationID,
			Role:           r.Role,
		})
	}

	return summary, nil
}
