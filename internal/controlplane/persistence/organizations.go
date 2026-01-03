package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// IsOrganizationOwner checks if a user is an owner of an organization
func (s *Store) IsOrganizationOwner(ctx context.Context, orgID, userID uuid.UUID) (bool, error) {
	const query = `SELECT COUNT(1) FROM organization_owners WHERE organization_id = $1 AND user_id = $2`
	var count int
	if err := s.db.GetContext(ctx, &count, query, orgID, userID); err != nil {
		return false, fmt.Errorf("failed to check organization owner: %w", err)
	}
	return count > 0, nil
}

// CreateOrganizationWithProject creates an organization and initial project atomically
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

	const insertOwner = `
        INSERT INTO organization_owners (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err = tx.ExecContext(ctx, insertOwner, orgID, userID); err != nil {
		return Organization{}, Project{}, fmt.Errorf("failed to add organization owner: %w", err)
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

// CreateOrganization creates an organization (without initial project)
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

	const insertOwner = `
        INSERT INTO organization_owners (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err = tx.ExecContext(ctx, insertOwner, orgID, userID); err != nil {
		return Organization{}, fmt.Errorf("failed to add organization owner: %w", err)
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

// OrganizationSlugExists checks if a slug is already taken
func (s *Store) OrganizationSlugExists(ctx context.Context, slug string) (bool, error) {
	const query = `SELECT COUNT(1) FROM organizations WHERE slug = $1`
	var count int
	if err := s.db.GetContext(ctx, &count, query, strings.TrimSpace(slug)); err != nil {
		return false, fmt.Errorf("failed to check slug availability: %w", err)
	}
	return count > 0, nil
}

// AddOrganizationOwner adds a user as an organization owner
func (s *Store) AddOrganizationOwner(ctx context.Context, orgID, userID uuid.UUID) error {
	const query = `
        INSERT INTO organization_owners (organization_id, user_id, created_at)
        VALUES ($1, $2, NOW())
        ON CONFLICT DO NOTHING
    `
	if _, err := s.db.ExecContext(ctx, query, orgID, userID); err != nil {
		return fmt.Errorf("failed to add organization owner: %w", err)
	}
	return nil
}

// OrganizationOwner represents an owner of an organization with user details
type OrganizationOwner struct {
	UserID   uuid.UUID `db:"user_id" json:"user_id"`
	Email    string    `db:"email" json:"email"`
	Name     string    `db:"name" json:"name"`
	Username string    `db:"username" json:"username"`
	AddedAt  time.Time `db:"added_at" json:"added_at"`
}

// ListOrganizationOwners returns all owners of an organization with user details
func (s *Store) ListOrganizationOwners(ctx context.Context, orgID uuid.UUID) ([]OrganizationOwner, error) {
	const query = `
		SELECT oo.user_id, u.email, COALESCE(u.name, '') as name, COALESCE(u.username, '') as username, oo.created_at as added_at
		FROM organization_owners oo
		JOIN users u ON u.id = oo.user_id
		WHERE oo.organization_id = $1
		ORDER BY oo.created_at ASC
	`
	var owners []OrganizationOwner
	if err := s.db.SelectContext(ctx, &owners, query, orgID); err != nil {
		return nil, fmt.Errorf("failed to list organization owners: %w", err)
	}
	return owners, nil
}
