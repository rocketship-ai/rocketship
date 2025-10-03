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
	ID           uuid.UUID
	GitHubUserID int64
	Email        string
	Name         string
	Username     string
	CreatedAt    time.Time
	UpdatedAt    time.Time
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

func (s *Store) hashToken(token string) string {
	mac := hmac.New(sha256.New, s.tokenKey)
	mac.Write([]byte(token))
	return fmt.Sprintf("%x", mac.Sum(nil))
}
