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
	"github.com/lib/pq"
)

// RepoURLToFullName converts a full repo URL to owner/repo format.
// E.g., "https://github.com/owner/repo" -> "owner/repo"
func RepoURLToFullName(repoURL string) string {
	// Remove protocol
	s := strings.TrimPrefix(repoURL, "https://")
	s = strings.TrimPrefix(s, "http://")
	// Remove github.com/
	s = strings.TrimPrefix(s, "github.com/")
	// Remove trailing .git if present
	s = strings.TrimSuffix(s, ".git")
	// Remove trailing slash
	s = strings.TrimSuffix(s, "/")
	return s
}

// ProjectSummary represents a project with aggregated data for list views
type ProjectSummary struct {
	ID            uuid.UUID `db:"id"`
	Name          string    `db:"name"`
	RepoURL       string    `db:"repo_url"`
	DefaultBranch string    `db:"default_branch"`
	PathScope     []string  // Parsed from JSONB
	SourceRef     string    `db:"source_ref"`
	SuiteCount    int       `db:"suite_count"`
	TestCount     int       `db:"test_count"`
	CreatedAt     time.Time `db:"created_at"`
	LastScan      *ScanSummary
}

// ScanSummary represents the latest scan attempt for a project
type ScanSummary struct {
	Status       string    `db:"status"`
	CreatedAt    time.Time `db:"created_at"`
	HeadSHA      string    `db:"head_sha"`
	ErrorMessage string    `db:"error_message"`
	SuitesFound  int       `db:"suites_found"`
	TestsFound   int       `db:"tests_found"`
}

// ListProjectSummariesForOrg returns all active projects for an org with counts and last scan info
func (s *Store) ListProjectSummariesForOrg(ctx context.Context, orgID uuid.UUID) ([]ProjectSummary, error) {
	// Get active projects with counts (only count active suites/tests)
	const projectQuery = `
		SELECT
			p.id,
			p.name,
			p.repo_url,
			p.default_branch,
			p.path_scope,
			p.source_ref,
			p.created_at,
			COALESCE((SELECT COUNT(*) FROM suites WHERE project_id = p.id AND is_active = true), 0) AS suite_count,
			COALESCE((SELECT COUNT(*) FROM tests WHERE project_id = p.id AND is_active = true), 0) AS test_count
		FROM projects p
		WHERE p.organization_id = $1 AND p.is_active = true
		ORDER BY p.name ASC, p.source_ref ASC
	`

	rows := []struct {
		ID            uuid.UUID `db:"id"`
		Name          string    `db:"name"`
		RepoURL       string    `db:"repo_url"`
		DefaultBranch string    `db:"default_branch"`
		PathScope     string    `db:"path_scope"`
		SourceRef     string    `db:"source_ref"`
		CreatedAt     time.Time `db:"created_at"`
		SuiteCount    int       `db:"suite_count"`
		TestCount     int       `db:"test_count"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, projectQuery, orgID); err != nil {
		return nil, fmt.Errorf("failed to list project summaries: %w", err)
	}

	projects := make([]ProjectSummary, 0, len(rows))
	for _, r := range rows {
		p := ProjectSummary{
			ID:            r.ID,
			Name:          r.Name,
			RepoURL:       r.RepoURL,
			DefaultBranch: r.DefaultBranch,
			SourceRef:     r.SourceRef,
			SuiteCount:    r.SuiteCount,
			TestCount:     r.TestCount,
			CreatedAt:     r.CreatedAt,
		}
		if r.PathScope != "" {
			if err := json.Unmarshal([]byte(r.PathScope), &p.PathScope); err != nil {
				return nil, fmt.Errorf("failed to parse path_scope: %w", err)
			}
		}
		projects = append(projects, p)
	}

	// Fetch latest scan for each project (batched)
	if len(projects) > 0 {
		scans, err := s.getLatestScansForProjects(ctx, orgID, projects)
		if err != nil {
			return nil, err
		}
		for i := range projects {
			key := RepoURLToFullName(projects[i].RepoURL) + "|" + projects[i].SourceRef
			if scan, ok := scans[key]; ok {
				projects[i].LastScan = &scan
			}
		}
	}

	return projects, nil
}

// getLatestScansForProjects fetches the latest scan for each project by repo+sourceRef
func (s *Store) getLatestScansForProjects(ctx context.Context, orgID uuid.UUID, projects []ProjectSummary) (map[string]ScanSummary, error) {
	// Build a list of (repo_full_name, source_ref) pairs
	pairs := make([]struct {
		RepoFullName string
		SourceRef    string
	}, len(projects))
	for i, p := range projects {
		pairs[i].RepoFullName = RepoURLToFullName(p.RepoURL)
		pairs[i].SourceRef = p.SourceRef
	}

	// Use a lateral join to get the latest scan for each pair
	const query = `
		SELECT DISTINCT ON (repository_full_name, source_ref)
			repository_full_name,
			source_ref,
			status,
			created_at,
			COALESCE(head_sha, '') as head_sha,
			COALESCE(error_message, '') as error_message,
			suites_found,
			tests_found
		FROM github_scan_attempts
		WHERE organization_id = $1
		ORDER BY repository_full_name, source_ref, created_at DESC
	`

	rows := []struct {
		RepositoryFullName string    `db:"repository_full_name"`
		SourceRef          string    `db:"source_ref"`
		Status             string    `db:"status"`
		CreatedAt          time.Time `db:"created_at"`
		HeadSHA            string    `db:"head_sha"`
		ErrorMessage       string    `db:"error_message"`
		SuitesFound        int       `db:"suites_found"`
		TestsFound         int       `db:"tests_found"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, query, orgID); err != nil {
		return nil, fmt.Errorf("failed to fetch latest scans: %w", err)
	}

	result := make(map[string]ScanSummary)
	for _, r := range rows {
		key := r.RepositoryFullName + "|" + r.SourceRef
		result[key] = ScanSummary{
			Status:       r.Status,
			CreatedAt:    r.CreatedAt,
			HeadSHA:      r.HeadSHA,
			ErrorMessage: r.ErrorMessage,
			SuitesFound:  r.SuitesFound,
			TestsFound:   r.TestsFound,
		}
	}

	return result, nil
}

// ListProjectSummariesForUser returns active projects the user can access with counts and last scan info.
// Org owners see all projects; non-owners see only projects they're members of.
func (s *Store) ListProjectSummariesForUser(ctx context.Context, orgID, userID uuid.UUID) ([]ProjectSummary, error) {
	// Check if user is org owner
	isOwner, err := s.IsOrganizationOwner(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check org ownership: %w", err)
	}

	var projectQuery string
	var args []interface{}

	if isOwner {
		// Org owner: return all active projects
		projectQuery = `
			SELECT
				p.id,
				p.name,
				p.repo_url,
				p.default_branch,
				p.path_scope,
				p.source_ref,
				p.created_at,
				COALESCE((SELECT COUNT(*) FROM suites WHERE project_id = p.id AND is_active = true), 0) AS suite_count,
				COALESCE((SELECT COUNT(*) FROM tests WHERE project_id = p.id AND is_active = true), 0) AS test_count
			FROM projects p
			WHERE p.organization_id = $1 AND p.is_active = true
			ORDER BY p.name ASC, p.source_ref ASC
		`
		args = []interface{}{orgID}
	} else {
		// Non-owner: return only projects they're a member of
		projectQuery = `
			SELECT
				p.id,
				p.name,
				p.repo_url,
				p.default_branch,
				p.path_scope,
				p.source_ref,
				p.created_at,
				COALESCE((SELECT COUNT(*) FROM suites WHERE project_id = p.id AND is_active = true), 0) AS suite_count,
				COALESCE((SELECT COUNT(*) FROM tests WHERE project_id = p.id AND is_active = true), 0) AS test_count
			FROM projects p
			JOIN project_members pm ON pm.project_id = p.id
			WHERE p.organization_id = $1 AND pm.user_id = $2 AND p.is_active = true
			ORDER BY p.name ASC, p.source_ref ASC
		`
		args = []interface{}{orgID, userID}
	}

	rows := []struct {
		ID            uuid.UUID `db:"id"`
		Name          string    `db:"name"`
		RepoURL       string    `db:"repo_url"`
		DefaultBranch string    `db:"default_branch"`
		PathScope     string    `db:"path_scope"`
		SourceRef     string    `db:"source_ref"`
		CreatedAt     time.Time `db:"created_at"`
		SuiteCount    int       `db:"suite_count"`
		TestCount     int       `db:"test_count"`
	}{}

	if err := s.db.SelectContext(ctx, &rows, projectQuery, args...); err != nil {
		return nil, fmt.Errorf("failed to list project summaries for user: %w", err)
	}

	projects := make([]ProjectSummary, 0, len(rows))
	for _, r := range rows {
		p := ProjectSummary{
			ID:            r.ID,
			Name:          r.Name,
			RepoURL:       r.RepoURL,
			DefaultBranch: r.DefaultBranch,
			SourceRef:     r.SourceRef,
			SuiteCount:    r.SuiteCount,
			TestCount:     r.TestCount,
			CreatedAt:     r.CreatedAt,
		}
		if r.PathScope != "" {
			if err := json.Unmarshal([]byte(r.PathScope), &p.PathScope); err != nil {
				return nil, fmt.Errorf("failed to parse path_scope: %w", err)
			}
		}
		projects = append(projects, p)
	}

	// Fetch latest scan for each project (batched)
	if len(projects) > 0 {
		scans, err := s.getLatestScansForProjects(ctx, orgID, projects)
		if err != nil {
			return nil, err
		}
		for i := range projects {
			key := RepoURLToFullName(projects[i].RepoURL) + "|" + projects[i].SourceRef
			if scan, ok := scans[key]; ok {
				projects[i].LastScan = &scan
			}
		}
	}

	return projects, nil
}

// SuiteActivityRow represents a suite with project info for activity list
type SuiteActivityRow struct {
	SuiteID       uuid.UUID      `db:"suite_id"`
	SuiteName     string         `db:"suite_name"`
	Description   sql.NullString `db:"description"`
	FilePath      sql.NullString `db:"file_path"`
	SourceRef     string         `db:"source_ref"`
	TestCount     int            `db:"test_count"`
	LastRunStatus sql.NullString `db:"last_run_status"`
	LastRunAt     sql.NullTime   `db:"last_run_at"`
	ProjectID     uuid.UUID      `db:"project_id"`
	ProjectName   string         `db:"project_name"`
	RepoURL       string         `db:"repo_url"`
}

// ListSuitesForOrg returns active suites across the org with project info.
// Deduplicates suites by (project_id, file_path), preferring the default branch version.
// Suites that only exist on a PR branch (no default branch version) are also shown.
func (s *Store) ListSuitesForOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]SuiteActivityRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Use a CTE with ROW_NUMBER to deduplicate suites by (project_id, file_path).
	// Prefer suites where source_ref matches the project's default_branch.
	// This prevents showing duplicate suites when a PR modifies an existing suite.
	const query = `
		WITH ranked_suites AS (
			SELECT
				s.id AS suite_id,
				s.name AS suite_name,
				s.description,
				s.file_path,
				s.source_ref,
				s.test_count,
				s.last_run_status,
				s.last_run_at,
				p.id AS project_id,
				p.name AS project_name,
				p.repo_url,
				p.default_branch,
				ROW_NUMBER() OVER (
					PARTITION BY p.id, s.file_path
					ORDER BY
						-- Prefer default branch first
						CASE WHEN s.source_ref = p.default_branch THEN 0 ELSE 1 END,
						-- Then by most recently updated
						s.updated_at DESC
				) AS rn
			FROM suites s
			JOIN projects p ON p.id = s.project_id
			WHERE p.organization_id = $1 AND p.is_active = true AND s.is_active = true
		)
		SELECT
			suite_id,
			suite_name,
			description,
			file_path,
			source_ref,
			test_count,
			last_run_status,
			last_run_at,
			project_id,
			project_name,
			repo_url
		FROM ranked_suites
		WHERE rn = 1
		ORDER BY suite_name ASC, source_ref ASC
		LIMIT $2
	`

	var rows []SuiteActivityRow
	if err := s.db.SelectContext(ctx, &rows, query, orgID, limit); err != nil {
		return nil, fmt.Errorf("failed to list suites for org: %w", err)
	}
	if rows == nil {
		rows = []SuiteActivityRow{}
	}

	return rows, nil
}

// ListSuitesForUserProjects returns active suites across projects the user can access.
// Deduplicates suites by (project_id, file_path), preferring the default branch version.
// Suites that only exist on a PR branch (no default branch version) are also shown.
func (s *Store) ListSuitesForUserProjects(ctx context.Context, orgID, userID uuid.UUID, limit int) ([]SuiteActivityRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	// Get accessible project IDs first
	accessibleIDs, err := s.ListAccessibleProjectIDs(ctx, orgID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get accessible project IDs: %w", err)
	}

	if len(accessibleIDs) == 0 {
		return []SuiteActivityRow{}, nil
	}

	// Use a CTE with ROW_NUMBER to deduplicate suites by (project_id, file_path).
	// Prefer suites where source_ref matches the project's default_branch.
	const query = `
		WITH ranked_suites AS (
			SELECT
				s.id AS suite_id,
				s.name AS suite_name,
				s.description,
				s.file_path,
				s.source_ref,
				s.test_count,
				s.last_run_status,
				s.last_run_at,
				p.id AS project_id,
				p.name AS project_name,
				p.repo_url,
				p.default_branch,
				ROW_NUMBER() OVER (
					PARTITION BY p.id, s.file_path
					ORDER BY
						-- Prefer default branch first
						CASE WHEN s.source_ref = p.default_branch THEN 0 ELSE 1 END,
						-- Then by most recently updated
						s.updated_at DESC
				) AS rn
			FROM suites s
			JOIN projects p ON p.id = s.project_id
			WHERE p.id = ANY($1) AND p.is_active = true AND s.is_active = true
		)
		SELECT
			suite_id,
			suite_name,
			description,
			file_path,
			source_ref,
			test_count,
			last_run_status,
			last_run_at,
			project_id,
			project_name,
			repo_url
		FROM ranked_suites
		WHERE rn = 1
		ORDER BY suite_name ASC, source_ref ASC
		LIMIT $2
	`

	var rows []SuiteActivityRow
	if err := s.db.SelectContext(ctx, &rows, query, pq.Array(accessibleIDs), limit); err != nil {
		return nil, fmt.Errorf("failed to list suites for user projects: %w", err)
	}
	if rows == nil {
		rows = []SuiteActivityRow{}
	}

	return rows, nil
}

// SuiteDetail represents a suite with all info needed for detail view
type SuiteDetail struct {
	ID            uuid.UUID      `db:"id"`
	ProjectID     uuid.UUID      `db:"project_id"`
	Name          string         `db:"name"`
	Description   sql.NullString `db:"description"`
	FilePath      sql.NullString `db:"file_path"`
	SourceRef     string         `db:"source_ref"`
	TestCount     int            `db:"test_count"`
	LastRunID     sql.NullString `db:"last_run_id"`
	LastRunStatus sql.NullString `db:"last_run_status"`
	LastRunAt     sql.NullTime   `db:"last_run_at"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
	ProjectName   string         `db:"project_name"`
	RepoURL       string         `db:"repo_url"`
}

// GetSuiteDetail retrieves a suite by ID with org verification
func (s *Store) GetSuiteDetail(ctx context.Context, orgID, suiteID uuid.UUID) (SuiteDetail, error) {
	const query = `
		SELECT
			s.id,
			s.project_id,
			s.name,
			s.description,
			s.file_path,
			s.source_ref,
			s.test_count,
			s.last_run_id,
			s.last_run_status,
			s.last_run_at,
			s.created_at,
			s.updated_at,
			p.name AS project_name,
			p.repo_url
		FROM suites s
		JOIN projects p ON p.id = s.project_id
		WHERE s.id = $1 AND p.organization_id = $2
	`

	var suite SuiteDetail
	if err := s.db.GetContext(ctx, &suite, query, suiteID, orgID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return SuiteDetail{}, sql.ErrNoRows
		}
		return SuiteDetail{}, fmt.Errorf("failed to get suite detail: %w", err)
	}

	return suite, nil
}

// GetProjectWithOrgCheck retrieves a project with org verification
func (s *Store) GetProjectWithOrgCheck(ctx context.Context, orgID, projectID uuid.UUID) (Project, error) {
	const query = `
		SELECT id, organization_id, name, repo_url, default_branch, path_scope, source_ref, created_at
		FROM projects
		WHERE id = $1 AND organization_id = $2
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

	if err := s.db.GetContext(ctx, &dest, query, projectID, orgID); err != nil {
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

// CanonicalSuiteRow represents a suite for project canonical list views
type CanonicalSuiteRow struct {
	ID            uuid.UUID      `db:"id"`
	Name          string         `db:"name"`
	Description   sql.NullString `db:"description"`
	FilePath      sql.NullString `db:"file_path"`
	SourceRef     string         `db:"source_ref"`
	TestCount     int            `db:"test_count"`
	LastRunStatus sql.NullString `db:"last_run_status"`
	LastRunAt     sql.NullTime   `db:"last_run_at"`
}

// ListSuitesForProjectCanonical returns active suites for a project with deduplication.
// Deduplicates suites by (project_id, file_path), preferring the default branch version.
// Suites that only exist on a PR branch (no default branch version) are also shown.
// Only returns active suites from active projects.
func (s *Store) ListSuitesForProjectCanonical(ctx context.Context, projectID uuid.UUID) ([]CanonicalSuiteRow, error) {
	// Use a CTE with ROW_NUMBER to deduplicate suites by (project_id, file_path).
	// Prefer suites where source_ref matches the project's default_branch.
	const query = `
		WITH ranked_suites AS (
			SELECT
				s.id,
				s.name,
				s.description,
				s.file_path,
				s.source_ref,
				s.test_count,
				s.last_run_status,
				s.last_run_at,
				ROW_NUMBER() OVER (
					PARTITION BY s.file_path
					ORDER BY
						-- Prefer default branch first
						CASE WHEN s.source_ref = p.default_branch THEN 0 ELSE 1 END,
						-- Then by most recently updated
						s.updated_at DESC
				) AS rn
			FROM suites s
			JOIN projects p ON p.id = s.project_id
			WHERE s.project_id = $1 AND p.is_active = true AND s.is_active = true
		)
		SELECT
			id,
			name,
			description,
			file_path,
			source_ref,
			test_count,
			last_run_status,
			last_run_at
		FROM ranked_suites
		WHERE rn = 1
		ORDER BY name ASC
	`

	var rows []CanonicalSuiteRow
	if err := s.db.SelectContext(ctx, &rows, query, projectID); err != nil {
		return nil, fmt.Errorf("failed to list canonical suites for project: %w", err)
	}
	if rows == nil {
		rows = []CanonicalSuiteRow{}
	}

	return rows, nil
}

// GetLatestScanForProject returns the most recent scan for a project
func (s *Store) GetLatestScanForProject(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef string) (*ScanSummary, error) {
	repoFullName := RepoURLToFullName(repoURL)

	const query = `
		SELECT status, created_at, COALESCE(head_sha, '') as head_sha,
		       COALESCE(error_message, '') as error_message, suites_found, tests_found
		FROM github_scan_attempts
		WHERE organization_id = $1 AND repository_full_name = $2 AND source_ref = $3
		ORDER BY created_at DESC
		LIMIT 1
	`

	var scan ScanSummary
	if err := s.db.GetContext(ctx, &scan, query, orgID, repoFullName, sourceRef); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get latest scan: %w", err)
	}

	return &scan, nil
}
