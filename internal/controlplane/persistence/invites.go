package persistence

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// CreateOrgInvite creates a new organization invitation
func (s *Store) CreateOrgInvite(ctx context.Context, invite OrganizationInvite) (OrganizationInvite, error) {
	if invite.ID == uuid.Nil {
		invite.ID = uuid.New()
	}

	const query = `
        INSERT INTO organization_invites (
            id, organization_id, email, role, code_hash, code_salt, invited_by, expires_at, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING (SELECT name FROM organizations WHERE id = $2) AS organization_name, created_at, updated_at
    `

	dest := struct {
		OrganizationName string    `db:"organization_name"`
		CreatedAt        time.Time `db:"created_at"`
		UpdatedAt        time.Time `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, invite.ID, invite.OrganizationID, invite.Email,
		invite.Role, invite.CodeHash, invite.CodeSalt, invite.InvitedBy, invite.ExpiresAt); err != nil {
		return OrganizationInvite{}, fmt.Errorf("failed to create invite: %w", err)
	}

	invite.OrganizationName = dest.OrganizationName
	invite.CreatedAt = dest.CreatedAt
	invite.UpdatedAt = dest.UpdatedAt
	return invite, nil
}

// FindPendingOrgInvites returns all pending invites for a given email
func (s *Store) FindPendingOrgInvites(ctx context.Context, email string) ([]OrganizationInvite, error) {
	email = normalizeEmail(email)
	const query = `
        SELECT i.id, i.organization_id, i.email, i.role, i.code_hash, i.code_salt,
               i.invited_by, i.expires_at, i.accepted_at, i.accepted_by, i.created_at, i.updated_at,
               o.name AS organization_name
        FROM organization_invites i
        JOIN organizations o ON o.id = i.organization_id
        WHERE LOWER(TRIM(i.email)) = $1
          AND i.accepted_at IS NULL
        ORDER BY i.created_at DESC
    `

	var invites []OrganizationInvite
	if err := s.db.SelectContext(ctx, &invites, query, email); err != nil {
		return nil, fmt.Errorf("failed to find invites: %w", err)
	}
	return invites, nil
}

// MarkOrgInviteAccepted marks an invite as accepted
func (s *Store) MarkOrgInviteAccepted(ctx context.Context, inviteID, userID uuid.UUID) error {
	const query = `
        UPDATE organization_invites
        SET accepted_at = NOW(), accepted_by = $2, updated_at = NOW()
        WHERE id = $1 AND accepted_at IS NULL
    `
	res, err := s.db.ExecContext(ctx, query, inviteID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark invite accepted: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return fmt.Errorf("invite not found or already accepted")
	}
	return nil
}
