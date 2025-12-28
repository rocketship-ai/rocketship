package persistence

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// SaveRefreshToken stores or updates a refresh token
func (s *Store) SaveRefreshToken(ctx context.Context, token string, rec RefreshTokenRecord) error {
	if token == "" {
		return errors.New("token required")
	}

	hash := s.hashToken(token)
	if rec.TokenID == uuid.Nil {
		rec.TokenID = uuid.New()
	}

	const query = `
        INSERT INTO refresh_tokens (id, token_hash, user_id, organization_id, scopes, issued_at, expires_at, last_used_at, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, $6, $7, $6, NOW(), NOW())
        ON CONFLICT (token_hash) DO UPDATE SET
            user_id = EXCLUDED.user_id,
            organization_id = EXCLUDED.organization_id,
            scopes = EXCLUDED.scopes,
            issued_at = EXCLUDED.issued_at,
            expires_at = EXCLUDED.expires_at,
            last_used_at = EXCLUDED.issued_at,
            updated_at = NOW()
    `

	// Convert uuid.Nil to NULL for database storage
	var orgID interface{}
	if rec.OrganizationID == uuid.Nil {
		orgID = nil
	} else {
		orgID = rec.OrganizationID
	}

	scopes := pq.StringArray(rec.Scopes)
	if _, err := s.db.ExecContext(ctx, query, rec.TokenID, hash, rec.User.ID, orgID, scopes, rec.IssuedAt, rec.ExpiresAt); err != nil {
		return fmt.Errorf("failed to persist refresh token: %w", err)
	}
	return nil
}

// GetRefreshToken retrieves a refresh token record
func (s *Store) GetRefreshToken(ctx context.Context, token string) (RefreshTokenRecord, error) {
	if token == "" {
		return RefreshTokenRecord{}, ErrRefreshTokenNotFound
	}
	hash := s.hashToken(token)

	const query = `
        SELECT
            rt.id AS token_id,
            rt.user_id,
            rt.organization_id,
            rt.scopes,
            rt.issued_at,
            rt.expires_at,
            u.github_user_id,
            u.email,
            u.name,
            u.username,
            u.created_at,
            u.updated_at
        FROM refresh_tokens rt
        JOIN users u ON u.id = rt.user_id
        WHERE rt.token_hash = $1
    `

	dest := struct {
		TokenID        uuid.UUID      `db:"token_id"`
		UserID         uuid.UUID      `db:"user_id"`
		OrganizationID uuid.NullUUID  `db:"organization_id"`
		Scopes         pq.StringArray `db:"scopes"`
		IssuedAt       time.Time      `db:"issued_at"`
		ExpiresAt      time.Time      `db:"expires_at"`
		GitHubID       int64          `db:"github_user_id"`
		Email          string         `db:"email"`
		Name           sql.NullString `db:"name"`
		Username       sql.NullString `db:"username"`
		CreatedAt      time.Time      `db:"created_at"`
		UpdatedAt      time.Time      `db:"updated_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, hash); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return RefreshTokenRecord{}, ErrRefreshTokenNotFound
		}
		return RefreshTokenRecord{}, fmt.Errorf("failed to load refresh token: %w", err)
	}

	// Convert NULL back to uuid.Nil
	orgID := uuid.Nil
	if dest.OrganizationID.Valid {
		orgID = dest.OrganizationID.UUID
	}

	record := RefreshTokenRecord{
		TokenID:        dest.TokenID,
		OrganizationID: orgID,
		Scopes:         []string(dest.Scopes),
		IssuedAt:       dest.IssuedAt,
		ExpiresAt:      dest.ExpiresAt,
		User: User{
			ID:           dest.UserID,
			GitHubUserID: dest.GitHubID,
			Email:        dest.Email,
			Name:         dest.Name.String,
			Username:     dest.Username.String,
			CreatedAt:    dest.CreatedAt,
			UpdatedAt:    dest.UpdatedAt,
		},
	}
	return record, nil
}

// DeleteRefreshToken removes a refresh token from storage
func (s *Store) DeleteRefreshToken(ctx context.Context, token string) error {
	if token == "" {
		return ErrRefreshTokenNotFound
	}
	hash := s.hashToken(token)
	const query = `DELETE FROM refresh_tokens WHERE token_hash = $1`
	res, err := s.db.ExecContext(ctx, query, hash)
	if err != nil {
		return fmt.Errorf("failed to delete refresh token: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return ErrRefreshTokenNotFound
	}
	return nil
}

// hashToken creates an HMAC hash of the token for secure storage
func (s *Store) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.tokenKey)
	mac.Write([]byte(token))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
