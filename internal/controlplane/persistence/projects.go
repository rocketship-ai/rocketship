package persistence

import (
	"context"
	"database/sql"
	"encoding/json"
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

// AddProjectMember adds a user to a project with the specified role
// Uses upsert semantics - if the user already exists, updates their role
func (s *Store) AddProjectMember(ctx context.Context, projectID, userID uuid.UUID, role string) (ProjectMember, error) {
	role = strings.ToLower(strings.TrimSpace(role))
	if role != "read" && role != "write" {
		return ProjectMember{}, errors.New("role must be read or write")
	}

	const query = `
		INSERT INTO project_members (project_id, user_id, role, created_at, updated_at)
		VALUES ($1, $2, $3, NOW(), NOW())
		ON CONFLICT (project_id, user_id)
		DO UPDATE SET role = EXCLUDED.role, updated_at = NOW()
		RETURNING created_at, updated_at
	`
	var joinedAt, updatedAt time.Time
	if err := s.db.QueryRowxContext(ctx, query, projectID, userID, role).Scan(&joinedAt, &updatedAt); err != nil {
		return ProjectMember{}, fmt.Errorf("failed to add project member: %w", err)
	}

	// Fetch user details to return complete ProjectMember
	const userQuery = `SELECT email, COALESCE(name, '') as name, COALESCE(username, '') as username FROM users WHERE id = $1`
	var email, name, username string
	if err := s.db.QueryRowxContext(ctx, userQuery, userID).Scan(&email, &name, &username); err != nil {
		return ProjectMember{}, fmt.Errorf("failed to fetch user details: %w", err)
	}

	return ProjectMember{
		UserID:    userID,
		Email:     email,
		Name:      name,
		Username:  username,
		Role:      role,
		JoinedAt:  joinedAt,
		UpdatedAt: updatedAt,
	}, nil
}

// OrgProjectMember represents a project member with project context for org-wide views
type OrgProjectMember struct {
	ProjectID   uuid.UUID `db:"project_id" json:"project_id"`
	ProjectName string    `db:"project_name" json:"project_name"`
	UserID      uuid.UUID `db:"user_id" json:"user_id"`
	Username    string    `db:"username" json:"username"`
	Email       string    `db:"email" json:"email"`
	Name        string    `db:"name" json:"name"`
	Role        string    `db:"role" json:"role"`
}

// ListAllProjectMembers returns all project members across an organization
// Results are sorted by project name, then username/email
func (s *Store) ListAllProjectMembers(ctx context.Context, orgID uuid.UUID) ([]OrgProjectMember, error) {
	const query = `
		SELECT
			pm.project_id,
			p.name as project_name,
			pm.user_id,
			COALESCE(u.username, '') as username,
			u.email,
			COALESCE(u.name, '') as name,
			pm.role
		FROM project_members pm
		JOIN projects p ON p.id = pm.project_id
		JOIN users u ON u.id = pm.user_id
		WHERE p.organization_id = $1 AND p.is_active = true
		ORDER BY p.name ASC, COALESCE(u.username, u.email) ASC
	`
	var members []OrgProjectMember
	if err := s.db.SelectContext(ctx, &members, query, orgID); err != nil {
		return nil, fmt.Errorf("failed to list all project members: %w", err)
	}
	return members, nil
}

// CreateProject creates a new project
func (s *Store) CreateProject(ctx context.Context, project Project) (Project, error) {
	if project.OrganizationID == uuid.Nil {
		return Project{}, errors.New("organization id required")
	}
	if strings.TrimSpace(project.Name) == "" {
		return Project{}, errors.New("project name required")
	}
	if strings.TrimSpace(project.RepoURL) == "" {
		return Project{}, errors.New("repo url required")
	}

	if project.ID == uuid.Nil {
		project.ID = uuid.New()
	}
	if project.DefaultBranch == "" {
		project.DefaultBranch = "main"
	}
	// Default source_ref to default_branch if not specified
	if project.SourceRef == "" {
		project.SourceRef = project.DefaultBranch
	}

	// Ensure PathScope is never nil to avoid jsonb null
	if project.PathScope == nil {
		project.PathScope = []string{}
	}

	pathScopeJSON, err := json.Marshal(project.PathScope)
	if err != nil {
		return Project{}, fmt.Errorf("failed to encode path_scope: %w", err)
	}

	const query = `
		INSERT INTO projects (id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at, updated_at, last_synced_at)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7, NOW(), NOW(), NOW())
		RETURNING created_at
	`

	var createdAt time.Time
	if err := s.db.GetContext(ctx, &createdAt, query,
		project.ID, project.OrganizationID, project.Name, project.RepoURL,
		project.DefaultBranch, string(pathScopeJSON), project.SourceRef); err != nil {
		if isUniqueViolation(err, "projects_org_name_ref_idx") {
			return Project{}, fmt.Errorf("project name already exists in organization for this ref")
		}
		return Project{}, fmt.Errorf("failed to create project: %w", err)
	}

	project.CreatedAt = createdAt
	return project, nil
}

// GetProject retrieves a project by ID
func (s *Store) GetProject(ctx context.Context, projectID uuid.UUID) (Project, error) {
	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE id = $1
	`

	dest := struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Name           string    `db:"name"`
		RepoURL        string    `db:"repo_url"`
		DefaultBranch  string    `db:"default_branch"`
		PathScope      string    `db:"path_scope"`
		SourceRef      string    `db:"source_ref"`
		CreatedAt      time.Time `db:"created_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, projectID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, sql.ErrNoRows
		}
		return Project{}, fmt.Errorf("failed to get project: %w", err)
	}

	project := Project{
		ID:             dest.ID,
		OrganizationID: dest.OrganizationID,
		Name:           dest.Name,
		RepoURL:        dest.RepoURL,
		DefaultBranch:  dest.DefaultBranch,
		SourceRef:      dest.SourceRef,
		CreatedAt:      dest.CreatedAt,
	}

	if dest.PathScope != "" {
		if err := json.Unmarshal([]byte(dest.PathScope), &project.PathScope); err != nil {
			return Project{}, fmt.Errorf("failed to parse path_scope: %w", err)
		}
	}

	return project, nil
}

// ListProjects returns all projects for an organization
func (s *Store) ListProjects(ctx context.Context, orgID uuid.UUID) ([]Project, error) {
	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE organization_id = $1
		ORDER BY name ASC, source_ref ASC
	`

	rows := []struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Name           string    `db:"name"`
		RepoURL        string    `db:"repo_url"`
		DefaultBranch  string    `db:"default_branch"`
		PathScope      string    `db:"path_scope"`
		SourceRef      string    `db:"source_ref"`
		CreatedAt      time.Time `db:"created_at"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, orgID); err != nil {
		return nil, fmt.Errorf("failed to list projects: %w", err)
	}

	projects := make([]Project, 0, len(rows))
	for _, r := range rows {
		p := Project{
			ID:             r.ID,
			OrganizationID: r.OrganizationID,
			Name:           r.Name,
			RepoURL:        r.RepoURL,
			DefaultBranch:  r.DefaultBranch,
			SourceRef:      r.SourceRef,
			CreatedAt:      r.CreatedAt,
		}
		if r.PathScope != "" {
			if err := json.Unmarshal([]byte(r.PathScope), &p.PathScope); err != nil {
				return nil, fmt.Errorf("failed to parse path_scope: %w", err)
			}
		}
		projects = append(projects, p)
	}

	return projects, nil
}

// ProjectNameExists checks if a project name already exists in an organization for a given source_ref
func (s *Store) ProjectNameExists(ctx context.Context, orgID uuid.UUID, name, sourceRef string) (bool, error) {
	const query = `SELECT EXISTS(SELECT 1 FROM projects WHERE organization_id = $1 AND lower(name) = lower($2) AND lower(source_ref) = lower($3))`
	var exists bool
	if err := s.db.GetContext(ctx, &exists, query, orgID, name, sourceRef); err != nil {
		return false, fmt.Errorf("failed to check project name: %w", err)
	}
	return exists, nil
}

// DeactivateProjectsForRepoAndSourceRef marks all projects for a repo+sourceRef as inactive.
// This is called when a PR is closed/merged to hide the feature-branch discovery rows.
// Cascades to suites and tests for those project_ids.
// Returns the count of projects deactivated.
func (s *Store) DeactivateProjectsForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef, reason string) (int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find all matching projects
	const findQuery = `
		SELECT id FROM projects
		WHERE organization_id = $1 AND repo_url = $2 AND source_ref = $3 AND is_active = true
	`
	var projectIDs []uuid.UUID
	if err := tx.SelectContext(ctx, &projectIDs, findQuery, orgID, repoURL, sourceRef); err != nil {
		return 0, fmt.Errorf("failed to find projects: %w", err)
	}

	if len(projectIDs) == 0 {
		return 0, nil
	}

	// Deactivate projects
	const projectUpdate = `
		UPDATE projects
		SET is_active = false, deactivated_at = NOW(), deactivated_reason = $4
		WHERE organization_id = $1 AND repo_url = $2 AND source_ref = $3 AND is_active = true
	`
	result, err := tx.ExecContext(ctx, projectUpdate, orgID, repoURL, sourceRef, reason)
	if err != nil {
		return 0, fmt.Errorf("failed to deactivate projects: %w", err)
	}
	affected, _ := result.RowsAffected()

	// Deactivate suites for these projects
	for _, pid := range projectIDs {
		const suiteUpdate = `
			UPDATE suites
			SET is_active = false, deactivated_at = NOW(), deactivated_reason = $2
			WHERE project_id = $1 AND is_active = true
		`
		if _, err := tx.ExecContext(ctx, suiteUpdate, pid, reason); err != nil {
			return 0, fmt.Errorf("failed to deactivate suites: %w", err)
		}

		// Deactivate tests for these projects
		const testUpdate = `
			UPDATE tests
			SET is_active = false, deactivated_at = NOW(), deactivated_reason = $2
			WHERE project_id = $1 AND is_active = true
		`
		if _, err := tx.ExecContext(ctx, testUpdate, pid, reason); err != nil {
			return 0, fmt.Errorf("failed to deactivate tests: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return int(affected), nil
}

// ReactivateProjectsForRepoAndSourceRef marks all projects for a repo+sourceRef as active.
// This is called when a PR is reopened to restore the feature-branch discovery rows.
// Cascades to suites and tests for those project_ids.
// Returns the count of projects reactivated.
func (s *Store) ReactivateProjectsForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef string) (int, error) {
	tx, err := s.db.BeginTxx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// Find all matching inactive projects
	const findQuery = `
		SELECT id FROM projects
		WHERE organization_id = $1 AND repo_url = $2 AND source_ref = $3 AND is_active = false
	`
	var projectIDs []uuid.UUID
	if err := tx.SelectContext(ctx, &projectIDs, findQuery, orgID, repoURL, sourceRef); err != nil {
		return 0, fmt.Errorf("failed to find projects: %w", err)
	}

	if len(projectIDs) == 0 {
		return 0, nil
	}

	// Reactivate projects
	const projectUpdate = `
		UPDATE projects
		SET is_active = true, deactivated_at = NULL, deactivated_reason = NULL
		WHERE organization_id = $1 AND repo_url = $2 AND source_ref = $3 AND is_active = false
	`
	result, err := tx.ExecContext(ctx, projectUpdate, orgID, repoURL, sourceRef)
	if err != nil {
		return 0, fmt.Errorf("failed to reactivate projects: %w", err)
	}
	affected, _ := result.RowsAffected()

	// Reactivate suites for these projects
	for _, pid := range projectIDs {
		const suiteUpdate = `
			UPDATE suites
			SET is_active = true, deactivated_at = NULL, deactivated_reason = NULL
			WHERE project_id = $1 AND is_active = false
		`
		if _, err := tx.ExecContext(ctx, suiteUpdate, pid); err != nil {
			return 0, fmt.Errorf("failed to reactivate suites: %w", err)
		}

		// Reactivate tests for these projects
		const testUpdate = `
			UPDATE tests
			SET is_active = true, deactivated_at = NULL, deactivated_reason = NULL
			WHERE project_id = $1 AND is_active = false
		`
		if _, err := tx.ExecContext(ctx, testUpdate, pid); err != nil {
			return 0, fmt.Errorf("failed to reactivate tests: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return int(affected), nil
}

// FindProjectByRepoAndPathScope looks up a project by organization, repo URL, and path scope.
// This is used by the engine to resolve project_id from CLI metadata when project_id is not a UUID.
// Returns (project, found, error) - found is true if a matching project exists.
func (s *Store) FindProjectByRepoAndPathScope(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) (Project, bool, error) {
	// Encode path_scope as JSON for comparison
	pathScopeJSON, err := json.Marshal(pathScope)
	if err != nil {
		return Project{}, false, fmt.Errorf("failed to encode path_scope: %w", err)
	}

	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE organization_id = $1 AND repo_url = $2 AND path_scope = $3::jsonb
		ORDER BY created_at DESC
		LIMIT 1
	`

	dest := struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Name           string    `db:"name"`
		RepoURL        string    `db:"repo_url"`
		DefaultBranch  string    `db:"default_branch"`
		PathScope      string    `db:"path_scope"`
		SourceRef      string    `db:"source_ref"`
		CreatedAt      time.Time `db:"created_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, orgID, repoURL, string(pathScopeJSON)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, false, nil
		}
		return Project{}, false, fmt.Errorf("failed to find project: %w", err)
	}

	project := Project{
		ID:             dest.ID,
		OrganizationID: dest.OrganizationID,
		Name:           dest.Name,
		RepoURL:        dest.RepoURL,
		DefaultBranch:  dest.DefaultBranch,
		SourceRef:      dest.SourceRef,
		CreatedAt:      dest.CreatedAt,
	}

	if dest.PathScope != "" {
		if err := json.Unmarshal([]byte(dest.PathScope), &project.PathScope); err != nil {
			return Project{}, false, fmt.Errorf("failed to parse path_scope: %w", err)
		}
	}

	return project, true, nil
}

// ListAccessibleProjectIDs returns the project IDs that a user can access within an organization.
// Org owners can access all active projects; non-owners can only access projects they're members of.
func (s *Store) ListAccessibleProjectIDs(ctx context.Context, orgID, userID uuid.UUID) ([]uuid.UUID, error) {
	// Check if user is org owner
	isOwner, err := s.IsOrganizationOwner(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check org ownership: %w", err)
	}

	if isOwner {
		// Org owner: return all active projects
		const query = `
			SELECT id FROM projects
			WHERE organization_id = $1 AND is_active = true
		`
		var ids []uuid.UUID
		if err := s.db.SelectContext(ctx, &ids, query, orgID); err != nil {
			return nil, fmt.Errorf("failed to list all project IDs: %w", err)
		}
		if ids == nil {
			ids = []uuid.UUID{}
		}
		return ids, nil
	}

	// Non-owner: return only projects they're a member of
	const query = `
		SELECT p.id FROM projects p
		JOIN project_members pm ON pm.project_id = p.id
		WHERE p.organization_id = $1 AND pm.user_id = $2 AND p.is_active = true
	`
	var ids []uuid.UUID
	if err := s.db.SelectContext(ctx, &ids, query, orgID, userID); err != nil {
		return nil, fmt.Errorf("failed to list accessible project IDs: %w", err)
	}
	if ids == nil {
		ids = []uuid.UUID{}
	}
	return ids, nil
}

// UserCanAccessProject checks if a user can access a specific project.
// Org owners can access all active projects; non-owners must be project members.
func (s *Store) UserCanAccessProject(ctx context.Context, orgID, userID, projectID uuid.UUID) (bool, error) {
	// Check if user is org owner
	isOwner, err := s.IsOrganizationOwner(ctx, orgID, userID)
	if err != nil {
		return false, fmt.Errorf("failed to check org ownership: %w", err)
	}

	if isOwner {
		// Verify project exists and belongs to org
		const query = `
			SELECT EXISTS(SELECT 1 FROM projects WHERE id = $1 AND organization_id = $2 AND is_active = true)
		`
		var exists bool
		if err := s.db.GetContext(ctx, &exists, query, projectID, orgID); err != nil {
			return false, fmt.Errorf("failed to check project existence: %w", err)
		}
		return exists, nil
	}

	// Non-owner: check if they're a member of the project
	const query = `
		SELECT EXISTS(
			SELECT 1 FROM project_members pm
			JOIN projects p ON p.id = pm.project_id
			WHERE pm.project_id = $1 AND pm.user_id = $2 AND p.organization_id = $3 AND p.is_active = true
		)
	`
	var exists bool
	if err := s.db.GetContext(ctx, &exists, query, projectID, userID, orgID); err != nil {
		return false, fmt.Errorf("failed to check project membership: %w", err)
	}
	return exists, nil
}

// FindDefaultBranchProject looks up a project where source_ref matches default_branch.
// This is used during PR scans to reuse existing default-branch projects instead of creating branch variants.
// Returns (project, found, error) - found is true if a matching default-branch project exists.
func (s *Store) FindDefaultBranchProject(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) (Project, bool, error) {
	// Encode path_scope as JSON for comparison
	pathScopeJSON, err := json.Marshal(pathScope)
	if err != nil {
		return Project{}, false, fmt.Errorf("failed to encode path_scope: %w", err)
	}

	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE organization_id = $1 AND repo_url = $2 AND path_scope = $3::jsonb
		  AND source_ref = default_branch AND is_active = true
		LIMIT 1
	`

	dest := struct {
		ID             uuid.UUID `db:"id"`
		OrganizationID uuid.UUID `db:"organization_id"`
		Name           string    `db:"name"`
		RepoURL        string    `db:"repo_url"`
		DefaultBranch  string    `db:"default_branch"`
		PathScope      string    `db:"path_scope"`
		SourceRef      string    `db:"source_ref"`
		CreatedAt      time.Time `db:"created_at"`
	}{}

	if err := s.db.GetContext(ctx, &dest, query, orgID, repoURL, string(pathScopeJSON)); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Project{}, false, nil
		}
		return Project{}, false, fmt.Errorf("failed to find default branch project: %w", err)
	}

	project := Project{
		ID:             dest.ID,
		OrganizationID: dest.OrganizationID,
		Name:           dest.Name,
		RepoURL:        dest.RepoURL,
		DefaultBranch:  dest.DefaultBranch,
		SourceRef:      dest.SourceRef,
		CreatedAt:      dest.CreatedAt,
	}

	if dest.PathScope != "" {
		if err := json.Unmarshal([]byte(dest.PathScope), &project.PathScope); err != nil {
			return Project{}, false, fmt.Errorf("failed to parse path_scope: %w", err)
		}
	}

	return project, true, nil
}
