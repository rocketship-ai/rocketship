package persistence

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"github.com/google/uuid"
)

// ProjectPermissionRow represents a user's permission on a project for profile display
type ProjectPermissionRow struct {
	ProjectID   uuid.UUID `db:"project_id"`
	ProjectName string    `db:"project_name"`
	SourceRef   string    `db:"source_ref"`
	Permissions []string  // computed: ["read"] or ["read","write"]
}

// GetOrganizationByID retrieves an organization by its ID
func (s *Store) GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (Organization, error) {
	const query = `
		SELECT id, name, slug, created_at
		FROM organizations
		WHERE id = $1
	`

	var org Organization
	if err := s.db.QueryRowContext(ctx, query, orgID).Scan(&org.ID, &org.Name, &org.Slug, &org.CreatedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Organization{}, sql.ErrNoRows
		}
		return Organization{}, fmt.Errorf("failed to get organization: %w", err)
	}
	return org, nil
}

// ListProjectPermissionsForUser returns project permissions for a user within an organization.
// If the user is an org admin, they get read+write on all projects in the org.
// Otherwise, permissions come from project_members table.
// Results are deduped by (repo_url, path_scope), preferring the default branch version.
// Only returns active projects.
func (s *Store) ListProjectPermissionsForUser(ctx context.Context, orgID, userID uuid.UUID) ([]ProjectPermissionRow, error) {
	// First check if user is org owner
	isAdmin, err := s.IsOrganizationOwner(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check admin status: %w", err)
	}

	if isAdmin {
		// Org admins get read+write on all active projects in the org
		// Dedup by (repo_url, path_scope), preferring default branch version
		const adminQuery = `
			WITH ranked_projects AS (
				SELECT
					id,
					name,
					source_ref,
					ROW_NUMBER() OVER (
						PARTITION BY lower(repo_url), path_scope::text
						ORDER BY
							CASE WHEN source_ref = default_branch THEN 0 ELSE 1 END,
							updated_at DESC
					) AS rn
				FROM projects
				WHERE organization_id = $1 AND is_active = true
			)
			SELECT id, name, source_ref
			FROM ranked_projects
			WHERE rn = 1
			ORDER BY name ASC
		`

		rows, err := s.db.QueryContext(ctx, adminQuery, orgID)
		if err != nil {
			return nil, fmt.Errorf("failed to list projects for admin: %w", err)
		}
		defer func() { _ = rows.Close() }()

		var perms []ProjectPermissionRow
		for rows.Next() {
			var p ProjectPermissionRow
			if err := rows.Scan(&p.ProjectID, &p.ProjectName, &p.SourceRef); err != nil {
				return nil, fmt.Errorf("failed to scan project: %w", err)
			}
			p.Permissions = []string{"read", "write"}
			perms = append(perms, p)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("error iterating projects: %w", err)
		}
		return perms, nil
	}

	// Non-admin: get permissions from project_members, scoped to org
	// Dedup by (repo_url, path_scope), preferring default branch version
	const memberQuery = `
		WITH ranked_projects AS (
			SELECT
				p.id,
				p.name,
				p.source_ref,
				pm.role,
				ROW_NUMBER() OVER (
					PARTITION BY lower(p.repo_url), p.path_scope::text
					ORDER BY
						CASE WHEN p.source_ref = p.default_branch THEN 0 ELSE 1 END,
						p.updated_at DESC
				) AS rn
			FROM projects p
			JOIN project_members pm ON pm.project_id = p.id
			WHERE p.organization_id = $1 AND pm.user_id = $2 AND p.is_active = true
		)
		SELECT id, name, source_ref, role
		FROM ranked_projects
		WHERE rn = 1
		ORDER BY name ASC
	`

	rows, err := s.db.QueryContext(ctx, memberQuery, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list project permissions: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var perms []ProjectPermissionRow
	for rows.Next() {
		var p ProjectPermissionRow
		var role string
		if err := rows.Scan(&p.ProjectID, &p.ProjectName, &p.SourceRef, &role); err != nil {
			return nil, fmt.Errorf("failed to scan project permission: %w", err)
		}

		// Map role to permissions
		switch role {
		case "write":
			p.Permissions = []string{"read", "write"}
		case "read":
			p.Permissions = []string{"read"}
		default:
			p.Permissions = []string{"read"}
		}
		perms = append(perms, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating project permissions: %w", err)
	}
	return perms, nil
}
