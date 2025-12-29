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

// ListProjectSummariesForOrg returns all projects for an org with counts and last scan info
func (s *Store) ListProjectSummariesForOrg(ctx context.Context, orgID uuid.UUID) ([]ProjectSummary, error) {
	// Get projects with counts
	const projectQuery = `
		SELECT
			p.id,
			p.name,
			p.repo_url,
			p.default_branch,
			p.path_scope,
			p.source_ref,
			p.created_at,
			COALESCE((SELECT COUNT(*) FROM suites WHERE project_id = p.id), 0) AS suite_count,
			COALESCE((SELECT COUNT(*) FROM tests WHERE project_id = p.id), 0) AS test_count
		FROM projects p
		WHERE p.organization_id = $1
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

// ListSuitesForOrg returns suites across the org with project info
func (s *Store) ListSuitesForOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]SuiteActivityRow, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	const query = `
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
			p.repo_url
		FROM suites s
		JOIN projects p ON p.id = s.project_id
		WHERE p.organization_id = $1
		ORDER BY s.name ASC, s.source_ref ASC
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
