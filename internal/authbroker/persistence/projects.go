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

// ProjectOrganizationID returns the organization ID for a project
func (s *Store) ProjectOrganizationID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error) {
	var orgID uuid.UUID
	if err := s.db.GetContext(ctx, &orgID, `SELECT organization_id FROM projects WHERE id = $1`, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return uuid.Nil, sql.ErrNoRows
		}
		return uuid.Nil, fmt.Errorf("failed to load project organization: %w", err)
	}
	return orgID, nil
}

// ListProjectMembers returns all members of a project
func (s *Store) ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]ProjectMember, error) {
	const query = `
        SELECT pm.user_id, u.email, u.name, u.username, pm.role, pm.created_at, pm.updated_at
        FROM project_members pm
        JOIN users u ON u.id = pm.user_id
        WHERE pm.project_id = $1
        ORDER BY u.email
    `

	rows := []struct {
		UserID    uuid.UUID      `db:"user_id"`
		Email     string         `db:"email"`
		Name      sql.NullString `db:"name"`
		Username  sql.NullString `db:"username"`
		Role      string         `db:"role"`
		CreatedAt time.Time      `db:"created_at"`
		UpdatedAt time.Time      `db:"updated_at"`
	}{}
	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list project members: %w", err)
	}

	members := make([]ProjectMember, 0, len(rows))
	for _, r := range rows {
		members = append(members, ProjectMember{
			UserID:    r.UserID,
			Email:     r.Email,
			Name:      r.Name.String,
			Username:  r.Username.String,
			Role:      r.Role,
			JoinedAt:  r.CreatedAt,
			UpdatedAt: r.UpdatedAt,
		})
	}
	return members, nil
}

// SetProjectMemberRole updates the role of a project member
func (s *Store) SetProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role string) error {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "read" && role != "write" {
		return errors.New("role must be read or write")
	}

	const query = `
        UPDATE project_members
        SET role = $3, updated_at = NOW()
        WHERE project_id = $1 AND user_id = $2
    `

	res, err := s.db.ExecContext(ctx, query, projectID, userID, role)
	if err != nil {
		return fmt.Errorf("failed to update project member: %w", err)
	}
	rows, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to determine rows affected: %w", err)
	}
	if rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// RemoveProjectMember removes a user from a project
func (s *Store) RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error {
	const query = `DELETE FROM project_members WHERE project_id = $1 AND user_id = $2`
	res, err := s.db.ExecContext(ctx, query, projectID, userID)
	if err != nil {
		return fmt.Errorf("failed to delete project member: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}
