package persistence

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

type User struct {
	ID           uuid.UUID  `db:"id"`
	GitHubUserID int64      `db:"github_user_id"`
	Email        string     `db:"email"`
	Name         string     `db:"name"`
	Username     string     `db:"username"`
	CreatedAt    time.Time  `db:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at"`
}

type GitHubUserInput struct {
	GitHubUserID int64
	Email        string
	Name         string
	Username     string
}

var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrOrganizationSlugUsed = errors.New("organization slug already in use")
)

type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	RepoURL        string
	DefaultBranch  string
	PathScope      []string
	CreatedAt      time.Time
}

type OrganizationMembership struct {
	OrganizationID uuid.UUID
	IsAdmin        bool
}

type ProjectMembership struct {
	ProjectID      uuid.UUID
	OrganizationID uuid.UUID
	Role           string
}

type OrganizationRegistration struct {
	ID                uuid.UUID
	UserID            uuid.UUID
	Email             string
	OrgName           string
	CodeHash          []byte
	CodeSalt          []byte
	Attempts          int
	MaxAttempts       int
	ExpiresAt         time.Time
	ResendAvailableAt time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type OrganizationInvite struct {
	ID               uuid.UUID
	OrganizationID   uuid.UUID
	Email            string
	Role             string
	CodeHash         []byte
	CodeSalt         []byte
	InvitedBy        uuid.UUID
	ExpiresAt        time.Time
	AcceptedAt       sql.NullTime
	AcceptedBy       uuid.NullUUID
	OrganizationName string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type RoleSummary struct {
	Organizations []OrganizationMembership
	Projects      []ProjectMembership
}

func (r RoleSummary) AggregatedRoles() []string {
	ordered := []string{}
	seen := make(map[string]struct{})

	add := func(role string) {
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		ordered = append(ordered, role)
	}

	if len(r.Organizations) > 0 {
		add("owner")
	}

	hasWrite := false
	hasRead := false
	for _, project := range r.Projects {
		switch strings.ToLower(project.Role) {
		case "write":
			hasWrite = true
		case "read":
			hasRead = true
		}
	}

	if hasWrite {
		add("editor")
	}
	if hasRead {
		add("viewer")
	}

	if len(ordered) == 0 {
		add("pending")
	}

	return ordered
}

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

func (s *Store) IsOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	const query = `SELECT COUNT(1) FROM organization_admins WHERE organization_id = $1 AND user_id = $2`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID, userID); err != nil {
		return false, fmt.Errorf("failed to check organization admin: %w", err)
	}
	return count > 0, nil
}

type RefreshTokenRecord struct {
	TokenID        uuid.UUID
	User           User
	OrganizationID uuid.UUID
	Scopes         []string
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

type CreateOrgInput struct {
	Name    string
	Slug    string
	Project ProjectInput
}

type ProjectInput struct {
	Name          string
	RepoURL       string
	DefaultBranch string
	PathScope     []string
}

type ProjectMember struct {
	UserID    uuid.UUID
	Email     string
	Name      string
	Username  string
	Role      string
	JoinedAt  time.Time
	UpdatedAt time.Time
}

func (s *Store) UpsertGitHubUser(ctx context.Context, input GitHubUserInput) (User, error) {
	if input.GitHubUserID == 0 {
		return User{}, errors.New("github user id required")
	}
	if strings.TrimSpace(input.Email) == "" {
		return User{}, errors.New("email required")
	}

	id := uuid.New()

	const query = `
        INSERT INTO users (id, github_user_id, email, name, username, created_at, updated_at)
        VALUES ($1, $2, $3, $4, $5, NOW(), NOW())
        ON CONFLICT (github_user_id)
        DO UPDATE SET
            email = EXCLUDED.email,
            name = EXCLUDED.name,
            username = EXCLUDED.username,
            updated_at = NOW()
        RETURNING id, github_user_id, email, name, username, created_at, updated_at
    `

	var user User
	if err := s.db.GetContext(ctx, &user, query, id, input.GitHubUserID, input.Email, input.Name, input.Username); err != nil {
		return User{}, fmt.Errorf("failed to upsert user: %w", err)
	}
	return user, nil
}

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

	scopes := pq.StringArray(rec.Scopes)
	if _, err := s.db.ExecContext(ctx, query, rec.TokenID, hash, rec.User.ID, rec.OrganizationID, scopes, rec.IssuedAt, rec.ExpiresAt); err != nil {
		return fmt.Errorf("failed to persist refresh token: %w", err)
	}
	return nil
}

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
		OrganizationID uuid.UUID      `db:"organization_id"`
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

	record := RefreshTokenRecord{
		TokenID:        dest.TokenID,
		OrganizationID: dest.OrganizationID,
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

func (s *Store) CreateOrganizationWithProject(ctx context.Context, userID uuid.UUID, input CreateOrgInput) (Organization, Project, error) {
	if strings.TrimSpace(input.Name) == "" {
		return Organization{}, Project{}, errors.New("organization name required")
	}
	if strings.TrimSpace(input.Slug) == "" {
		return Organization{}, Project{}, errors.New("organization slug required")
	}
	if strings.TrimSpace(input.Project.Name) == "" {
		return Organization{}, Project{}, errors.New("project name required")
	}
	if strings.TrimSpace(input.Project.RepoURL) == "" {
		return Organization{}, Project{}, errors.New("project repo url required")
	}
	if strings.TrimSpace(input.Project.DefaultBranch) == "" {
		return Organization{}, Project{}, errors.New("project default branch required")
	}

	orgID := uuid.New()
	projectID := uuid.New()

	pathJSON, err := json.Marshal(input.Project.PathScope)
	if err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to encode path scope: %w", err)
	}

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const insertOrg = `
        INSERT INTO organizations (id, name, slug, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING created_at
    `

	var orgCreatedAt time.Time
	if err = tx.GetContext(ctx, &orgCreatedAt, insertOrg, orgID, input.Name, strings.ToLower(input.Slug)); err != nil {
		if strings.Contains(err.Error(), "organizations_slug_key") {
			return Organization{}, Project{}, ErrOrganizationSlugUsed
		}
		return Organization{}, Project{}, fmt.Errorf("failed to create organization: %w", err)
	}

	const insertAdmin = `
        INSERT INTO organization_admins (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err = tx.ExecContext(ctx, insertAdmin, orgID, userID); err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to add organization admin: %w", err)
	}

	const insertProject = `
        INSERT INTO projects (id, organization_id, name, repo_url, default_branch, path_scope, created_at)
        VALUES ($1, $2, $3, $4, $5, $6::jsonb, NOW())
        RETURNING created_at
    `
	var projectCreatedAt time.Time
	if err = tx.GetContext(ctx, &projectCreatedAt, insertProject, projectID, orgID, input.Project.Name, input.Project.RepoURL, input.Project.DefaultBranch, string(pathJSON)); err != nil {
		if strings.Contains(err.Error(), "projects_org_name_idx") {
			return Organization{}, Project{}, fmt.Errorf("project name already exists in organization")
		}
		return Organization{}, Project{}, fmt.Errorf("failed to create project: %w", err)
	}

	const insertMember = `
        INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
        VALUES ($1, $2, 'write', NOW(), NOW())
        ON CONFLICT (project_id, user_id) DO UPDATE SET role = 'write', updated_at = NOW()
    `
	if _, err = tx.ExecContext(ctx, insertMember, projectID, userID); err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to add project member: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to commit: %w", err)
	}

	org := Organization{
		ID:        orgID,
		Name:      input.Name,
		Slug:      strings.ToLower(input.Slug),
		CreatedAt: orgCreatedAt,
	}
	project := Project{
		ID:             projectID,
		OrganizationID: orgID,
		Name:           input.Project.Name,
		RepoURL:        input.Project.RepoURL,
		DefaultBranch:  input.Project.DefaultBranch,
		PathScope:      append([]string(nil), input.Project.PathScope...),
		CreatedAt:      projectCreatedAt,
	}
	return org, project, nil
}

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

func (s *Store) DeleteOrgRegistrationsForUser(ctx context.Context, userID uuid.UUID) error {
	const query = `DELETE FROM organization_registrations WHERE user_id = $1`
	if _, err := s.db.ExecContext(ctx, query, userID); err != nil {
		return fmt.Errorf("failed to delete existing registrations: %w", err)
	}
	return nil
}

func (s *Store) CreateOrgRegistration(ctx context.Context, rec OrganizationRegistration) (OrganizationRegistration, error) {
	const query = `
        INSERT INTO organization_registrations (
            id, user_id, email, org_name, code_hash, code_salt, attempts, max_attempts,
            expires_at, resend_available_at, created_at, updated_at
        )
        VALUES ($1, $2, LOWER($3), $4, $5, $6, $7, $8, $9, $10, NOW(), NOW())
        RETURNING created_at, updated_at
    `

	dest := OrganizationRegistration{
		ID:                rec.ID,
		UserID:            rec.UserID,
		Email:             normalizeEmail(rec.Email),
		OrgName:           rec.OrgName,
		CodeHash:          append([]byte(nil), rec.CodeHash...),
		CodeSalt:          append([]byte(nil), rec.CodeSalt...),
		Attempts:          rec.Attempts,
		MaxAttempts:       rec.MaxAttempts,
		ExpiresAt:         rec.ExpiresAt,
		ResendAvailableAt: rec.ResendAvailableAt,
	}

	if dest.MaxAttempts == 0 {
		dest.MaxAttempts = 10
	}

	if err := s.db.QueryRowxContext(ctx, query,
		dest.ID, dest.UserID, dest.Email, dest.OrgName, dest.CodeHash, dest.CodeSalt,
		dest.Attempts, dest.MaxAttempts, dest.ExpiresAt, dest.ResendAvailableAt,
	).Scan(&dest.CreatedAt, &dest.UpdatedAt); err != nil {
		return OrganizationRegistration{}, fmt.Errorf("failed to insert organization registration: %w", err)
	}

	return dest, nil
}

func (s *Store) GetOrgRegistration(ctx context.Context, id uuid.UUID) (OrganizationRegistration, error) {
	const query = `
        SELECT id, user_id, email, org_name, code_hash, code_salt, attempts, max_attempts,
               expires_at, resend_available_at, created_at, updated_at
        FROM organization_registrations
        WHERE id = $1
        LIMIT 1
    `

	var rec OrganizationRegistration
	if err := s.db.GetContext(ctx, &rec, query, id); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrganizationRegistration{}, sql.ErrNoRows
		}
		return OrganizationRegistration{}, fmt.Errorf("failed to load organization registration: %w", err)
	}
	return rec, nil
}

func (s *Store) UpdateOrgRegistrationForResend(ctx context.Context, id uuid.UUID, hash, salt []byte, expiresAt, resend time.Time) (OrganizationRegistration, error) {
	const query = `
        UPDATE organization_registrations
        SET code_hash = $2,
            code_salt = $3,
            expires_at = $4,
            resend_available_at = $5,
            updated_at = NOW()
        WHERE id = $1
        RETURNING id, user_id, email, org_name, code_hash, code_salt, attempts, max_attempts,
                  expires_at, resend_available_at, created_at, updated_at
    `

	var rec OrganizationRegistration
	if err := s.db.GetContext(ctx, &rec, query, id, hash, salt, expiresAt, resend); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrganizationRegistration{}, sql.ErrNoRows
		}
		return OrganizationRegistration{}, fmt.Errorf("failed to update registration: %w", err)
	}
	return rec, nil
}

func (s *Store) IncrementOrgRegistrationAttempts(ctx context.Context, id uuid.UUID) error {
	const query = `
        UPDATE organization_registrations
        SET attempts = attempts + 1, updated_at = NOW()
        WHERE id = $1
    `
	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to increment attempt counter: %w", err)
	}
	return nil
}

func (s *Store) DeleteOrgRegistration(ctx context.Context, id uuid.UUID) error {
	const query = `DELETE FROM organization_registrations WHERE id = $1`
	if _, err := s.db.ExecContext(ctx, query, id); err != nil {
		return fmt.Errorf("failed to delete registration: %w", err)
	}
	return nil
}

func (s *Store) LatestOrgRegistrationForUser(ctx context.Context, userID uuid.UUID) (OrganizationRegistration, error) {
	const query = `
        SELECT id, user_id, email, org_name, code_hash, code_salt, attempts, max_attempts,
               expires_at, resend_available_at, created_at, updated_at
        FROM organization_registrations
        WHERE user_id = $1
        ORDER BY created_at DESC
        LIMIT 1
    `
	var rec OrganizationRegistration
	if err := s.db.GetContext(ctx, &rec, query, userID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return OrganizationRegistration{}, sql.ErrNoRows
		}
		return OrganizationRegistration{}, fmt.Errorf("failed to load registration: %w", err)
	}
	return rec, nil
}

func (s *Store) CreateOrganization(ctx context.Context, userID uuid.UUID, name, slug string) (Organization, error) {
	if strings.TrimSpace(name) == "" {
		return Organization{}, errors.New("organization name required")
	}
	if strings.TrimSpace(slug) == "" {
		return Organization{}, errors.New("organization slug required")
	}

	orgID := uuid.New()

	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return Organization{}, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	const insertOrg = `
        INSERT INTO organizations (id, name, slug, created_at)
        VALUES ($1, $2, $3, NOW())
        RETURNING created_at
    `
	var createdAt time.Time
	if err = tx.GetContext(ctx, &createdAt, insertOrg, orgID, name, slug); err != nil {
		if strings.Contains(err.Error(), "organizations_slug_key") {
			return Organization{}, ErrOrganizationSlugUsed
		}
		return Organization{}, fmt.Errorf("failed to create organization: %w", err)
	}

	const insertAdmin = `
        INSERT INTO organization_admins (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err = tx.ExecContext(ctx, insertAdmin, orgID, userID); err != nil {
		return Organization{}, fmt.Errorf("failed to add organization admin: %w", err)
	}

	if err = tx.Commit(); err != nil {
		return Organization{}, fmt.Errorf("failed to commit: %w", err)
	}

	return Organization{
		ID:        orgID,
		Name:      name,
		Slug:      slug,
		CreatedAt: createdAt,
	}, nil
}

func (s *Store) OrganizationSlugExists(ctx context.Context, slug string) (bool, error) {
	const query = `SELECT COUNT(1) FROM organizations WHERE slug = $1`
	var count int
	if err := s.db.GetContext(ctx, &count, query, strings.TrimSpace(slug)); err != nil {
		return false, fmt.Errorf("failed to check slug availability: %w", err)
	}
	return count > 0, nil
}

func (s *Store) AddOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) error {
	const query = `
        INSERT INTO organization_admins (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err := s.db.ExecContext(ctx, query, orgID, userID); err != nil {
		return fmt.Errorf("failed to add organization admin: %w", err)
	}
	return nil
}

func (s *Store) CreateOrgInvite(ctx context.Context, invite OrganizationInvite) (OrganizationInvite, error) {
	const query = `
        INSERT INTO organization_invites (
            id, organization_id, email, role, code_hash, code_salt, invited_by,
            expires_at, created_at, updated_at
        )
        VALUES ($1, $2, LOWER($3), $4, $5, $6, $7, $8, NOW(), NOW())
        RETURNING id, organization_id, email, role, code_hash, code_salt, invited_by,
                  expires_at, created_at, updated_at
    `

	var rec OrganizationInvite
	if err := s.db.GetContext(ctx, &rec, query,
		invite.ID, invite.OrganizationID, normalizeEmail(invite.Email), strings.ToLower(strings.TrimSpace(invite.Role)),
		invite.CodeHash, invite.CodeSalt, invite.InvitedBy, invite.ExpiresAt,
	); err != nil {
		return OrganizationInvite{}, fmt.Errorf("failed to create organization invite: %w", err)
	}

	const nameQuery = `SELECT name FROM organizations WHERE id = $1`
	if err := s.db.GetContext(ctx, &rec.OrganizationName, nameQuery, rec.OrganizationID); err != nil {
		return OrganizationInvite{}, fmt.Errorf("failed to fetch organization name: %w", err)
	}

	return rec, nil
}

func (s *Store) FindPendingOrgInvites(ctx context.Context, email string) ([]OrganizationInvite, error) {
	const query = `
        SELECT oi.id, oi.organization_id, oi.email, oi.role, oi.code_hash, oi.code_salt, oi.invited_by,
               oi.expires_at, oi.accepted_at, oi.accepted_by, oi.created_at, oi.updated_at,
               org.name AS organization_name
        FROM organization_invites oi
        JOIN organizations org ON org.id = oi.organization_id
        WHERE LOWER(oi.email) = LOWER($1) AND oi.accepted_at IS NULL AND oi.expires_at > NOW()
        ORDER BY oi.created_at DESC
    `

	rows := []OrganizationInvite{}
	if err := s.db.SelectContext(ctx, &rows, query, normalizeEmail(email)); err != nil {
		return nil, fmt.Errorf("failed to list organization invites: %w", err)
	}
	return rows, nil
}

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func (s *Store) MarkOrgInviteAccepted(ctx context.Context, inviteID, userID uuid.UUID) error {
	const query = `
        UPDATE organization_invites
        SET accepted_at = NOW(),
            accepted_by = $2,
            updated_at = NOW()
        WHERE id = $1 AND accepted_at IS NULL
    `

	res, err := s.db.ExecContext(ctx, query, inviteID, userID)
	if err != nil {
		return fmt.Errorf("failed to mark invite accepted: %w", err)
	}
	if rows, _ := res.RowsAffected(); rows == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (s *Store) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.tokenKey)
	mac.Write([]byte(token))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
