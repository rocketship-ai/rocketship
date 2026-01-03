package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// ProjectInviteInput is the input for creating a project invite
type ProjectInviteInput struct {
	OrganizationID uuid.UUID
	Email          string
	InvitedBy      uuid.UUID
	CodeHash       []byte
	CodeSalt       []byte
	ExpiresAt      time.Time
	Projects       []ProjectInviteProjectInput
}

// ProjectInviteProjectInput is a project+role for invite creation
type ProjectInviteProjectInput struct {
	ProjectID uuid.UUID
	Role      string // "read" or "write"
}

var ErrPendingInviteExists = errors.New("a pending invite already exists for this email in this organization")

// CreateProjectInvite creates a new project invite with associated projects
func (s *Store) CreateProjectInvite(ctx context.Context, input ProjectInviteInput) (ProjectInvite, error) {
	email := normalizeEmail(input.Email)
	if email == "" {
		return ProjectInvite{}, errors.New("email required")
	}
	if len(input.Projects) == 0 {
		return ProjectInvite{}, errors.New("at least one project required")
	}

	// Validate project roles
	for _, p := range input.Projects {
		role := strings.ToLower(strings.TrimSpace(p.Role))
		if role != "read" && role != "write" {
			return ProjectInvite{}, fmt.Errorf("invalid role %q for project %s", p.Role, p.ProjectID)
		}
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return ProjectInvite{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Check for existing pending invite
	var existingCount int
	checkQuery := `
		SELECT COUNT(*) FROM project_invites
		WHERE organization_id = $1
		  AND lower(email) = $2
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
		  AND expires_at > NOW()
	`
	if err := tx.GetContext(ctx, &existingCount, checkQuery, input.OrganizationID, email); err != nil {
		return ProjectInvite{}, fmt.Errorf("failed to check existing invites: %w", err)
	}
	if existingCount > 0 {
		return ProjectInvite{}, ErrPendingInviteExists
	}

	// Insert the invite
	inviteID := uuid.New()
	insertInviteQuery := `
		INSERT INTO project_invites (
			id, organization_id, email, invited_by, code_hash, code_salt, expires_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING created_at, updated_at
	`
	var createdAt, updatedAt time.Time
	if err := tx.QueryRowxContext(ctx, insertInviteQuery,
		inviteID, input.OrganizationID, email, input.InvitedBy,
		input.CodeHash, input.CodeSalt, input.ExpiresAt,
	).Scan(&createdAt, &updatedAt); err != nil {
		return ProjectInvite{}, fmt.Errorf("failed to insert invite: %w", err)
	}

	// Insert project associations
	insertProjectQuery := `
		INSERT INTO project_invite_projects (invite_id, project_id, role)
		VALUES ($1, $2, $3)
	`
	for _, p := range input.Projects {
		role := strings.ToLower(strings.TrimSpace(p.Role))
		if _, err := tx.ExecContext(ctx, insertProjectQuery, inviteID, p.ProjectID, role); err != nil {
			return ProjectInvite{}, fmt.Errorf("failed to insert project association: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return ProjectInvite{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Fetch the complete invite with org name and inviter name
	return s.GetProjectInvite(ctx, inviteID)
}

// GetProjectInvite retrieves a single project invite by ID with all details
func (s *Store) GetProjectInvite(ctx context.Context, inviteID uuid.UUID) (ProjectInvite, error) {
	query := `
		SELECT
			pi.id, pi.organization_id, pi.email, pi.invited_by,
			pi.code_hash, pi.code_salt, pi.expires_at,
			pi.accepted_at, pi.accepted_by, pi.revoked_at, pi.revoked_by,
			pi.created_at, pi.updated_at,
			o.name as organization_name,
			COALESCE(u.name, u.username, u.email) as inviter_name
		FROM project_invites pi
		JOIN organizations o ON o.id = pi.organization_id
		JOIN users u ON u.id = pi.invited_by
		WHERE pi.id = $1
	`
	var invite ProjectInvite
	if err := s.db.GetContext(ctx, &invite, query, inviteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ProjectInvite{}, fmt.Errorf("invite not found")
		}
		return ProjectInvite{}, fmt.Errorf("failed to get invite: %w", err)
	}

	// Fetch associated projects
	projectsQuery := `
		SELECT pip.project_id, pip.role, p.name as project_name
		FROM project_invite_projects pip
		JOIN projects p ON p.id = pip.project_id
		WHERE pip.invite_id = $1
		ORDER BY p.name
	`
	if err := s.db.SelectContext(ctx, &invite.Projects, projectsQuery, inviteID); err != nil {
		return ProjectInvite{}, fmt.Errorf("failed to get invite projects: %w", err)
	}

	return invite, nil
}

// ListProjectInvitesForOrg returns all invites for an organization (for org owners)
func (s *Store) ListProjectInvitesForOrg(ctx context.Context, orgID uuid.UUID) ([]ProjectInvite, error) {
	query := `
		SELECT
			pi.id, pi.organization_id, pi.email, pi.invited_by,
			pi.code_hash, pi.code_salt, pi.expires_at,
			pi.accepted_at, pi.accepted_by, pi.revoked_at, pi.revoked_by,
			pi.created_at, pi.updated_at,
			o.name as organization_name,
			COALESCE(u.name, u.username, u.email) as inviter_name
		FROM project_invites pi
		JOIN organizations o ON o.id = pi.organization_id
		JOIN users u ON u.id = pi.invited_by
		WHERE pi.organization_id = $1
		ORDER BY pi.created_at DESC
	`
	var invites []ProjectInvite
	if err := s.db.SelectContext(ctx, &invites, query, orgID); err != nil {
		return nil, fmt.Errorf("failed to list invites: %w", err)
	}

	// Fetch projects for each invite
	inviteIDs := make([]uuid.UUID, len(invites))
	for i, inv := range invites {
		inviteIDs[i] = inv.ID
	}
	if len(inviteIDs) > 0 {
		if err := s.populateInviteProjects(ctx, invites); err != nil {
			return nil, err
		}
	}

	return invites, nil
}

// ListProjectInvitesByCreator returns invites created by a specific user
func (s *Store) ListProjectInvitesByCreator(ctx context.Context, orgID, userID uuid.UUID) ([]ProjectInvite, error) {
	query := `
		SELECT
			pi.id, pi.organization_id, pi.email, pi.invited_by,
			pi.code_hash, pi.code_salt, pi.expires_at,
			pi.accepted_at, pi.accepted_by, pi.revoked_at, pi.revoked_by,
			pi.created_at, pi.updated_at,
			o.name as organization_name,
			COALESCE(u.name, u.username, u.email) as inviter_name
		FROM project_invites pi
		JOIN organizations o ON o.id = pi.organization_id
		JOIN users u ON u.id = pi.invited_by
		WHERE pi.organization_id = $1 AND pi.invited_by = $2
		ORDER BY pi.created_at DESC
	`
	var invites []ProjectInvite
	if err := s.db.SelectContext(ctx, &invites, query, orgID, userID); err != nil {
		return nil, fmt.Errorf("failed to list invites by creator: %w", err)
	}

	if len(invites) > 0 {
		if err := s.populateInviteProjects(ctx, invites); err != nil {
			return nil, err
		}
	}

	return invites, nil
}

// FindPendingProjectInvitesByEmail returns pending invites for a given email
func (s *Store) FindPendingProjectInvitesByEmail(ctx context.Context, email string) ([]ProjectInvite, error) {
	email = normalizeEmail(email)
	query := `
		SELECT
			pi.id, pi.organization_id, pi.email, pi.invited_by,
			pi.code_hash, pi.code_salt, pi.expires_at,
			pi.accepted_at, pi.accepted_by, pi.revoked_at, pi.revoked_by,
			pi.created_at, pi.updated_at,
			o.name as organization_name,
			COALESCE(u.name, u.username, u.email) as inviter_name
		FROM project_invites pi
		JOIN organizations o ON o.id = pi.organization_id
		JOIN users u ON u.id = pi.invited_by
		WHERE lower(pi.email) = $1
		  AND pi.accepted_at IS NULL
		  AND pi.revoked_at IS NULL
		  AND pi.expires_at > NOW()
		ORDER BY pi.created_at DESC
	`
	var invites []ProjectInvite
	if err := s.db.SelectContext(ctx, &invites, query, email); err != nil {
		return nil, fmt.Errorf("failed to find pending invites: %w", err)
	}

	if len(invites) > 0 {
		if err := s.populateInviteProjects(ctx, invites); err != nil {
			return nil, err
		}
	}

	return invites, nil
}

// AcceptProjectInvite marks an invite as accepted and creates project memberships
func (s *Store) AcceptProjectInvite(ctx context.Context, inviteID, userID uuid.UUID) error {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Verify invite is valid
	var invite struct {
		AcceptedAt sql.NullTime  `db:"accepted_at"`
		RevokedAt  sql.NullTime  `db:"revoked_at"`
		ExpiresAt  time.Time     `db:"expires_at"`
	}
	checkQuery := `SELECT accepted_at, revoked_at, expires_at FROM project_invites WHERE id = $1`
	if err := tx.GetContext(ctx, &invite, checkQuery, inviteID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("invite not found")
		}
		return fmt.Errorf("failed to check invite: %w", err)
	}
	if invite.AcceptedAt.Valid {
		return fmt.Errorf("invite already accepted")
	}
	if invite.RevokedAt.Valid {
		return fmt.Errorf("invite has been revoked")
	}
	if time.Now().After(invite.ExpiresAt) {
		return fmt.Errorf("invite has expired")
	}

	// Mark invite as accepted
	updateQuery := `
		UPDATE project_invites
		SET accepted_at = NOW(), accepted_by = $2, updated_at = NOW()
		WHERE id = $1
	`
	if _, err := tx.ExecContext(ctx, updateQuery, inviteID, userID); err != nil {
		return fmt.Errorf("failed to mark invite accepted: %w", err)
	}

	// Get associated projects
	projectsQuery := `SELECT project_id, role FROM project_invite_projects WHERE invite_id = $1`
	var projects []struct {
		ProjectID uuid.UUID `db:"project_id"`
		Role      string    `db:"role"`
	}
	if err := tx.SelectContext(ctx, &projects, projectsQuery, inviteID); err != nil {
		return fmt.Errorf("failed to get invite projects: %w", err)
	}

	// Upsert project memberships
	upsertMemberQuery := `
		INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (project_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
	`
	for _, p := range projects {
		if _, err := tx.ExecContext(ctx, upsertMemberQuery, p.ProjectID, userID, p.Role); err != nil {
			return fmt.Errorf("failed to create project membership: %w", err)
		}
	}

	return tx.Commit()
}

// RevokeProjectInvite marks an invite as revoked
func (s *Store) RevokeProjectInvite(ctx context.Context, inviteID, revokedBy uuid.UUID) error {
	query := `
		UPDATE project_invites
		SET revoked_at = NOW(), revoked_by = $2, updated_at = NOW()
		WHERE id = $1
		  AND accepted_at IS NULL
		  AND revoked_at IS NULL
	`
	res, err := s.db.ExecContext(ctx, query, inviteID, revokedBy)
	if err != nil {
		return fmt.Errorf("failed to revoke invite: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("invite not found or already accepted/revoked")
	}
	return nil
}

// CanUserInviteToProjects checks if a user can invite to the given projects
// Returns true if user is org owner OR has write access to ALL specified projects
func (s *Store) CanUserInviteToProjects(ctx context.Context, orgID, userID uuid.UUID, projectIDs []uuid.UUID) (bool, error) {
	if len(projectIDs) == 0 {
		return false, nil
	}

	// Check if org owner
	isOwner, err := s.IsOrganizationOwner(ctx, orgID, userID)
	if err != nil {
		return false, err
	}
	if isOwner {
		return true, nil
	}

	// Check if user has write access to ALL projects
	query := `
		SELECT COUNT(*) FROM project_members
		WHERE user_id = $1 AND role = 'write' AND project_id = ANY($2)
	`
	var count int
	if err := s.db.GetContext(ctx, &count, query, userID, pq.Array(projectIDs)); err != nil {
		return false, fmt.Errorf("failed to check project permissions: %w", err)
	}

	return count == len(projectIDs), nil
}

// populateInviteProjects fetches and attaches project details to invites
func (s *Store) populateInviteProjects(ctx context.Context, invites []ProjectInvite) error {
	if len(invites) == 0 {
		return nil
	}

	inviteIDs := make([]uuid.UUID, len(invites))
	inviteMap := make(map[uuid.UUID]*ProjectInvite)
	for i := range invites {
		inviteIDs[i] = invites[i].ID
		inviteMap[invites[i].ID] = &invites[i]
		invites[i].Projects = []ProjectInviteProject{} // Initialize to empty slice
	}

	query := `
		SELECT pip.invite_id, pip.project_id, pip.role, p.name as project_name
		FROM project_invite_projects pip
		JOIN projects p ON p.id = pip.project_id
		WHERE pip.invite_id = ANY($1)
		ORDER BY p.name
	`
	var rows []struct {
		InviteID    uuid.UUID `db:"invite_id"`
		ProjectID   uuid.UUID `db:"project_id"`
		Role        string    `db:"role"`
		ProjectName string    `db:"project_name"`
	}
	if err := s.db.SelectContext(ctx, &rows, query, pq.Array(inviteIDs)); err != nil {
		return fmt.Errorf("failed to fetch invite projects: %w", err)
	}

	for _, row := range rows {
		if inv, ok := inviteMap[row.InviteID]; ok {
			inv.Projects = append(inv.Projects, ProjectInviteProject{
				ProjectID:   row.ProjectID,
				ProjectName: row.ProjectName,
				Role:        row.Role,
			})
		}
	}

	return nil
}
