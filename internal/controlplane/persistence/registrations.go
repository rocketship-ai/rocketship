package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DeleteOrgRegistrationsForUser removes all registrations for a user
func (s *Store) DeleteOrgRegistrationsForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `DELETE FROM organization_registrations WHERE user_id = $1`
	if _, err := s.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete previous registrations: %w", err)
	}
	return nil
}

// CreateOrgRegistration creates a new organization registration
func (s *Store) CreateOrgRegistration(ctx context.Context, rec OrganizationRegistration) (OrganizationRegistration, error) {
	if rec.ID == uuid.Nil {
		rec.ID = uuid.New()
	}

	const query = `
        INSERT INTO organization_registrations (
            id, user_id, email, org_name, code_hash, code_salt,
            attempts, max_attempts, expires_at, resend_available_at, created_at, updated_at
        )
        VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	if err := s.db.GetContext(ctx, &rec, query, rec.ID, rec.UserID, rec.Email, rec.OrgName,
		rec.CodeHash, rec.CodeSalt, rec.Attempts, rec.MaxAttempts, rec.ExpiresAt, rec.ResendAvailableAt); err != nil {
		return OrganizationRegistration{}, fmt.Errorf("failed to create registration: %w", err)
	}
	return rec, nil
}

// GetOrgRegistration retrieves a registration by ID
func (s *Store) GetOrgRegistration(ctx context.Context, id uuid.UUID) (OrganizationRegistration, error) {
	const query = `
        SELECT id, user_id, email, org_name, code_hash, code_salt,
               attempts, max_attempts, expires_at, resend_available_at, created_at, updated_at
        FROM organization_registrations
        WHERE id = $1
    `
	var reg OrganizationRegistration
	if err := s.db.GetContext(ctx, &reg, query, id); err != nil {
		return OrganizationRegistration{}, err
	}
	return reg, nil
}

// UpdateOrgRegistrationForResend updates registration with new code and expiry
func (s *Store) UpdateOrgRegistrationForResend(ctx context.Context, id uuid.UUID, hash, salt []byte, expiresAt, resend time.Time) (OrganizationRegistration, error) {
	const query = `
        UPDATE organization_registrations
        SET code_hash = $2, code_salt = $3, expires_at = $4, resend_available_at = $5, updated_at = NOW()
        WHERE id = $1
        RETURNING id, user_id, email, org_name, code_hash, code_salt,
                  attempts, max_attempts, expires_at, resend_available_at, created_at, updated_at
    `
	var reg OrganizationRegistration
	if err := s.db.GetContext(ctx, &reg, query, id, hash, salt, expiresAt, resend); err != nil {
		return OrganizationRegistration{}, err
	}
	return reg, nil
}

// IncrementOrgRegistrationAttempts increments the failed attempt counter
func (s *Store) IncrementOrgRegistrationAttempts(ctx context.Context, id uuid.UUID) error {
	const query = `UPDATE organization_registrations SET attempts = attempts + 1, updated_at = NOW() WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to increment attempts: %w", err)
	}
	return nil
}

// DeleteOrgRegistration removes a registration
func (s *Store) DeleteOrgRegistration(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM organization_registrations WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to delete registration: %w", err)
	}
	return nil
}

// LatestOrgRegistrationForUser returns the most recent registration for a user
func (s *Store) LatestOrgRegistrationForUser(ctx context.Context, userID uuid.UUID) (OrganizationRegistration, error) {
	const query = `
        SELECT id, user_id, email, org_name, code_hash, code_salt,
               attempts, max_attempts, expires_at, resend_available_at, created_at, updated_at
        FROM organization_registrations
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 1
    `
	var reg OrganizationRegistration
	if err := s.db.GetContext(ctx, &reg, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrganizationRegistration{}, sql.ErrNoRows
		}
		return OrganizationRegistration{}, fmt.Errorf("failed to load registration: %w", err)
	}
	return reg, nil
}
