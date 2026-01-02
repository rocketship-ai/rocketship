package persistence

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// CITokenCreateInput contains the parameters for creating a CI token
type CITokenCreateInput struct {
	Name         string
	Description  string
	NeverExpires bool
	ExpiresAt    *time.Time
	Projects     []CITokenProjectScope // ProjectID and Scope must be set; ProjectName is optional
}

// ListCITokensForOrg returns CI tokens for an organization with their project scopes.
// When includeRevoked is false (default), only non-revoked tokens are returned.
// When includeRevoked is true, all tokens including revoked ones are returned.
func (s *Store) ListCITokensForOrg(ctx context.Context, orgID uuid.UUID, includeRevoked bool) ([]CITokenRecord, error) {
	const tokensQuery = `
		SELECT id, organization_id, name, token_hash, never_expires,
		       expires_at, revoked_at, created_by, last_used_at, revoked_by,
		       description, created_at, updated_at
		FROM ci_tokens
		WHERE organization_id = $1
		  AND ($2::bool = true OR revoked_at IS NULL)
		ORDER BY name ASC
	`

	var tokens []CITokenRecord
	if err := s.db.SelectContext(ctx, &tokens, tokensQuery, orgID, includeRevoked); err != nil {
		return nil, fmt.Errorf("failed to list ci tokens: %w", err)
	}

	if len(tokens) == 0 {
		return tokens, nil
	}

	// Collect token IDs to batch-load projects
	tokenIDs := make([]uuid.UUID, len(tokens))
	tokenMap := make(map[uuid.UUID]*CITokenRecord, len(tokens))
	for i := range tokens {
		tokenIDs[i] = tokens[i].ID
		tokenMap[tokens[i].ID] = &tokens[i]
		tokens[i].Projects = []CITokenProjectScope{} // Initialize empty slice
	}

	// Load all project scopes for these tokens
	const projectsQuery = `
		SELECT ctp.token_id, ctp.project_id, ctp.scope, p.name as project_name
		FROM ci_token_projects ctp
		JOIN projects p ON p.id = ctp.project_id
		WHERE ctp.token_id = ANY($1)
		ORDER BY p.name ASC
	`

	type projectRow struct {
		TokenID     uuid.UUID `db:"token_id"`
		ProjectID   uuid.UUID `db:"project_id"`
		Scope       string    `db:"scope"`
		ProjectName string    `db:"project_name"`
	}
	var projectRows []projectRow
	if err := s.db.SelectContext(ctx, &projectRows, projectsQuery, pq.Array(tokenIDs)); err != nil {
		return nil, fmt.Errorf("failed to list ci token projects: %w", err)
	}

	// Assign projects to their tokens
	for _, pr := range projectRows {
		if token, ok := tokenMap[pr.TokenID]; ok {
			token.Projects = append(token.Projects, CITokenProjectScope{
				ProjectID:   pr.ProjectID,
				ProjectName: pr.ProjectName,
				Scope:       pr.Scope,
			})
		}
	}

	return tokens, nil
}

// CreateCIToken creates a new CI token with multi-project scopes
// Returns the plaintext token (shown once), the token record, and any error
func (s *Store) CreateCIToken(ctx context.Context, orgID, createdBy uuid.UUID, input CITokenCreateInput) (string, CITokenRecord, error) {
	if input.Name == "" {
		return "", CITokenRecord{}, fmt.Errorf("token name is required")
	}
	if len(input.Projects) == 0 {
		return "", CITokenRecord{}, fmt.Errorf("at least one project scope is required")
	}

	// Generate random token: rs_ci_<32 bytes random in base64url>
	randomBytes := make([]byte, 32)
	if _, err := rand.Read(randomBytes); err != nil {
		return "", CITokenRecord{}, fmt.Errorf("failed to generate random token: %w", err)
	}
	tokenPlaintext := "rs_ci_" + base64.RawURLEncoding.EncodeToString(randomBytes)

	// Hash the token for storage
	hash := sha256.Sum256([]byte(tokenPlaintext))
	tokenHash := hex.EncodeToString(hash[:])

	tokenID := uuid.New()
	now := time.Now().UTC()

	// Build the token record
	record := CITokenRecord{
		ID:             tokenID,
		OrganizationID: orgID,
		Name:           input.Name,
		TokenHash:      tokenHash,
		NeverExpires:   input.NeverExpires,
		CreatedBy:      uuid.NullUUID{UUID: createdBy, Valid: true},
		CreatedAt:      now,
		UpdatedAt:      now,
		Projects:       make([]CITokenProjectScope, len(input.Projects)),
	}

	if input.Description != "" {
		record.Description = sql.NullString{String: input.Description, Valid: true}
	}

	if !input.NeverExpires && input.ExpiresAt != nil {
		record.ExpiresAt = sql.NullTime{Time: *input.ExpiresAt, Valid: true}
	}

	// Start transaction
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return "", CITokenRecord{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Insert token
	const insertTokenQuery = `
		INSERT INTO ci_tokens (id, organization_id, name, token_hash, never_expires, expires_at, created_by, description, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`
	_, err = tx.ExecContext(ctx, insertTokenQuery,
		record.ID, record.OrganizationID, record.Name, record.TokenHash,
		record.NeverExpires, record.ExpiresAt, record.CreatedBy,
		record.Description, record.CreatedAt, record.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err, "ci_tokens_org_active_name_idx") {
			return "", CITokenRecord{}, fmt.Errorf("an active token with this name already exists; revoke it first to reuse the name")
		}
		return "", CITokenRecord{}, fmt.Errorf("failed to insert ci token: %w", err)
	}

	// Insert project scopes
	const insertProjectQuery = `
		INSERT INTO ci_token_projects (token_id, project_id, scope)
		VALUES ($1, $2, $3)
	`
	for i, ps := range input.Projects {
		scope := ps.Scope
		if scope != "read" && scope != "write" {
			return "", CITokenRecord{}, fmt.Errorf("invalid scope %q for project %s", scope, ps.ProjectID)
		}
		_, err = tx.ExecContext(ctx, insertProjectQuery, tokenID, ps.ProjectID, scope)
		if err != nil {
			return "", CITokenRecord{}, fmt.Errorf("failed to insert project scope: %w", err)
		}
		record.Projects[i] = CITokenProjectScope{
			ProjectID:   ps.ProjectID,
			ProjectName: ps.ProjectName,
			Scope:       scope,
		}
	}

	if err := tx.Commit(); err != nil {
		return "", CITokenRecord{}, fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Don't include token_hash in returned record
	record.TokenHash = ""

	return tokenPlaintext, record, nil
}

// RevokeCIToken revokes a CI token
func (s *Store) RevokeCIToken(ctx context.Context, orgID, tokenID, revokedBy uuid.UUID) error {
	const query = `
		UPDATE ci_tokens
		SET revoked_at = NOW(), revoked_by = $1, updated_at = NOW()
		WHERE id = $2 AND organization_id = $3 AND revoked_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, revokedBy, tokenID, orgID)
	if err != nil {
		return fmt.Errorf("failed to revoke ci token: %w", err)
	}

	rows, _ := result.RowsAffected()
	if rows == 0 {
		return fmt.Errorf("token not found or already revoked")
	}

	return nil
}

// CITokenLookupResult contains the result of finding a CI token by plaintext
type CITokenLookupResult struct {
	TokenID      uuid.UUID
	OrgID        uuid.UUID
	IsRevoked    bool
	IsExpired    bool
	NeverExpires bool
	ExpiresAt    sql.NullTime
	Projects     []CITokenProjectScope
}

// FindCITokenByPlaintext looks up a CI token by its plaintext value
// Used by engine auth to validate CI tokens
func (s *Store) FindCITokenByPlaintext(ctx context.Context, tokenPlaintext string) (*CITokenLookupResult, error) {
	// Hash the input to compare
	hash := sha256.Sum256([]byte(tokenPlaintext))
	tokenHash := hex.EncodeToString(hash[:])

	const tokenQuery = `
		SELECT id, organization_id, never_expires, expires_at, revoked_at
		FROM ci_tokens
		WHERE token_hash = $1
	`

	type tokenRow struct {
		ID           uuid.UUID    `db:"id"`
		OrgID        uuid.UUID    `db:"organization_id"`
		NeverExpires bool         `db:"never_expires"`
		ExpiresAt    sql.NullTime `db:"expires_at"`
		RevokedAt    sql.NullTime `db:"revoked_at"`
	}

	var row tokenRow
	if err := s.db.GetContext(ctx, &row, tokenQuery, tokenHash); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Token not found
		}
		return nil, fmt.Errorf("failed to find ci token: %w", err)
	}

	result := &CITokenLookupResult{
		TokenID:      row.ID,
		OrgID:        row.OrgID,
		IsRevoked:    row.RevokedAt.Valid,
		NeverExpires: row.NeverExpires,
		ExpiresAt:    row.ExpiresAt,
	}

	// Check expiration
	if !row.NeverExpires && row.ExpiresAt.Valid && row.ExpiresAt.Time.Before(time.Now().UTC()) {
		result.IsExpired = true
	}

	// Load project scopes
	const projectsQuery = `
		SELECT ctp.project_id, ctp.scope, p.name as project_name
		FROM ci_token_projects ctp
		JOIN projects p ON p.id = ctp.project_id
		WHERE ctp.token_id = $1
	`

	var projects []CITokenProjectScope
	if err := s.db.SelectContext(ctx, &projects, projectsQuery, row.ID); err != nil {
		return nil, fmt.Errorf("failed to load token projects: %w", err)
	}
	result.Projects = projects

	return result, nil
}

// UpdateCITokenLastUsed updates the last_used_at timestamp for a CI token
func (s *Store) UpdateCITokenLastUsed(ctx context.Context, tokenID uuid.UUID) error {
	const query = `
		UPDATE ci_tokens
		SET last_used_at = NOW(), updated_at = NOW()
		WHERE id = $1
	`

	_, err := s.db.ExecContext(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("failed to update token last_used_at: %w", err)
	}

	return nil
}

// GetCIToken retrieves a single CI token by ID with project scopes
func (s *Store) GetCIToken(ctx context.Context, orgID, tokenID uuid.UUID) (CITokenRecord, error) {
	const tokenQuery = `
		SELECT id, organization_id, name, token_hash, never_expires,
		       expires_at, revoked_at, created_by, last_used_at, revoked_by,
		       description, created_at, updated_at
		FROM ci_tokens
		WHERE id = $1 AND organization_id = $2
	`

	var token CITokenRecord
	if err := s.db.GetContext(ctx, &token, tokenQuery, tokenID, orgID); err != nil {
		return CITokenRecord{}, fmt.Errorf("failed to get ci token: %w", err)
	}

	// Load project scopes
	const projectsQuery = `
		SELECT ctp.project_id, ctp.scope, p.name as project_name
		FROM ci_token_projects ctp
		JOIN projects p ON p.id = ctp.project_id
		WHERE ctp.token_id = $1
		ORDER BY p.name ASC
	`

	if err := s.db.SelectContext(ctx, &token.Projects, projectsQuery, tokenID); err != nil {
		return CITokenRecord{}, fmt.Errorf("failed to load token projects: %w", err)
	}

	return token, nil
}

// VerifyUserHasWriteOnProjects checks if a user has write permission on all specified projects
// Returns error if user lacks write permission on any project
func (s *Store) VerifyUserHasWriteOnProjects(ctx context.Context, orgID, userID uuid.UUID, projectIDs []uuid.UUID) error {
	// First check if user is org admin (org admins have implicit write on all projects)
	isAdmin, err := s.IsOrganizationAdmin(ctx, orgID, userID)
	if err != nil {
		return fmt.Errorf("failed to check org admin status: %w", err)
	}
	if isAdmin {
		return nil
	}

	// Check that all projects belong to the org
	const orgCheckQuery = `
		SELECT COUNT(*) FROM projects WHERE id = ANY($1) AND organization_id = $2
	`
	var orgCount int
	if err := s.db.GetContext(ctx, &orgCount, orgCheckQuery, projectIDs, orgID); err != nil {
		return fmt.Errorf("failed to verify projects belong to org: %w", err)
	}
	if orgCount != len(projectIDs) {
		return fmt.Errorf("one or more projects do not belong to this organization")
	}

	// Check user has write role on all projects
	const writeCheckQuery = `
		SELECT COUNT(DISTINCT project_id)
		FROM project_members
		WHERE project_id = ANY($1) AND user_id = $2 AND role = 'write'
	`
	var writeCount int
	if err := s.db.GetContext(ctx, &writeCount, writeCheckQuery, projectIDs, userID); err != nil {
		return fmt.Errorf("failed to verify user write permissions: %w", err)
	}
	if writeCount != len(projectIDs) {
		return fmt.Errorf("user does not have write permission on all specified projects")
	}

	return nil
}
