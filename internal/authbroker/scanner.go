package authbroker

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
	"github.com/rocketship-ai/rocketship/internal/dsl"
)

// ScanInput contains all the information needed to scan a repository
type ScanInput struct {
	OrgID          uuid.UUID
	InstallationID int64
	RepoFullName   string // owner/repo format
	SourceRef      NormalizedRef
	HeadSHA        string // Commit SHA if available
	DeliveryID     string // Webhook delivery ID for tracking
}

// ScanResult contains the results of a scan
type ScanResult struct {
	SuitesFound int
	TestsFound  int
	Errors      []string
}

// Scanner handles repository scanning for .rocketship configurations
type Scanner struct {
	store  dataStore
	github *GitHubAppClient
}

// NewScanner creates a new scanner
func NewScanner(store dataStore, github *GitHubAppClient) *Scanner {
	return &Scanner{
		store:  store,
		github: github,
	}
}

// Scan scans a repository for .rocketship configurations and upserts them to the database
func (s *Scanner) Scan(ctx context.Context, input ScanInput) (*ScanResult, error) {
	if s.github == nil || !s.github.Configured() {
		return nil, fmt.Errorf("github app client not configured")
	}

	result := &ScanResult{}

	// Parse owner/repo from full name
	parts := strings.SplitN(input.RepoFullName, "/", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid repo full name: %s", input.RepoFullName)
	}
	owner, repo := parts[0], parts[1]

	// Get repository metadata for default_branch
	repoInfo, err := s.github.GetRepository(ctx, input.InstallationID, owner, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository info: %w", err)
	}

	// Determine which ref to use for fetching content
	fetchRef := input.HeadSHA
	if fetchRef == "" {
		fetchRef = input.SourceRef.Ref
	}

	// Get the tree to find .rocketship directories
	tree, err := s.github.GetTree(ctx, input.InstallationID, owner, repo, fetchRef, true)
	if err != nil {
		return nil, fmt.Errorf("failed to get tree: %w", err)
	}

	// Find all .rocketship directories
	rocketshipDirs := s.findRocketshipDirs(tree)
	if len(rocketshipDirs) == 0 {
		slog.Debug("scanner: no .rocketship directories found",
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
		)
		return result, nil
	}

	// Process each .rocketship directory
	for _, dir := range rocketshipDirs {
		slog.Info("scanner: processing .rocketship directory",
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"dir", dir,
		)

		// Create or update project
		project, err := s.upsertProject(ctx, input, repoInfo, dir)
		if err != nil {
			errMsg := fmt.Sprintf("failed to upsert project for dir %s: %v", dir, err)
			result.Errors = append(result.Errors, errMsg)
			slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
			continue
		}

		// Find and process suite files
		suiteFiles := s.findSuiteFiles(tree, dir)
		for _, suiteFile := range suiteFiles {
			suitesCreated, testsCreated, err := s.processSuiteFile(ctx, input, project, suiteFile, fetchRef, owner, repo)
			if err != nil {
				errMsg := fmt.Sprintf("failed to process suite file %s: %v", suiteFile, err)
				result.Errors = append(result.Errors, errMsg)
				slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
				continue
			}
			result.SuitesFound += suitesCreated
			result.TestsFound += testsCreated
		}
	}

	return result, nil
}

// findRocketshipDirs finds all .rocketship directories in the tree
func (s *Scanner) findRocketshipDirs(tree *GitHubTree) []string {
	seen := make(map[string]bool)
	var dirs []string

	for _, entry := range tree.Tree {
		if entry.Type != "tree" {
			continue
		}

		// Check if this is a .rocketship directory or contains one
		if entry.Path == ".rocketship" || strings.HasSuffix(entry.Path, "/.rocketship") {
			if !seen[entry.Path] {
				seen[entry.Path] = true
				dirs = append(dirs, entry.Path)
			}
		}
	}

	// Also check for files in .rocketship directories to catch cases where the tree
	// entry for the directory itself isn't present
	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}

		// Check if this file is in a .rocketship directory
		dir := path.Dir(entry.Path)
		if path.Base(dir) == ".rocketship" {
			if !seen[dir] {
				seen[dir] = true
				dirs = append(dirs, dir)
			}
		}
	}

	return dirs
}

// findSuiteFiles finds all YAML files in a .rocketship directory
func (s *Scanner) findSuiteFiles(tree *GitHubTree, rocketshipDir string) []string {
	var files []string

	for _, entry := range tree.Tree {
		if entry.Type != "blob" {
			continue
		}

		// Check if file is in the .rocketship directory
		if !strings.HasPrefix(entry.Path, rocketshipDir+"/") {
			continue
		}

		// Check if it's a YAML file
		if strings.HasSuffix(entry.Path, ".yaml") || strings.HasSuffix(entry.Path, ".yml") {
			files = append(files, entry.Path)
		}
	}

	return files
}

// upsertProject creates or updates a project for a .rocketship directory
func (s *Scanner) upsertProject(ctx context.Context, input ScanInput, repoInfo *GitHubRepoInfo, rocketshipDir string) (persistence.Project, error) {
	// Generate a stable project name
	projectName := s.generateProjectName(input.RepoFullName, rocketshipDir)

	// Build path_scope
	pathScope := []string{rocketshipDir + "/**"}

	// Check if project exists for this org+name+ref
	exists, err := s.store.ProjectNameExists(ctx, input.OrgID, projectName, input.SourceRef.Ref)
	if err != nil {
		return persistence.Project{}, fmt.Errorf("failed to check project existence: %w", err)
	}

	if exists {
		// Get existing project to return it
		projects, err := s.store.ListProjects(ctx, input.OrgID)
		if err != nil {
			return persistence.Project{}, fmt.Errorf("failed to list projects: %w", err)
		}
		for _, p := range projects {
			if strings.EqualFold(p.Name, projectName) && strings.EqualFold(p.SourceRef, input.SourceRef.Ref) {
				// TODO: could update pathScope here if needed
				return p, nil
			}
		}
	}

	// Create new project
	project := persistence.Project{
		ID:             uuid.New(),
		OrganizationID: input.OrgID,
		Name:           projectName,
		RepoURL:        fmt.Sprintf("https://github.com/%s", input.RepoFullName),
		DefaultBranch:  repoInfo.DefaultBranch,
		PathScope:      pathScope,
		SourceRef:      input.SourceRef.Ref,
	}

	created, err := s.store.CreateProject(ctx, project)
	if err != nil {
		// If it's a unique violation, the project was created concurrently - try to fetch it
		if strings.Contains(err.Error(), "already exists") {
			projects, listErr := s.store.ListProjects(ctx, input.OrgID)
			if listErr == nil {
				for _, p := range projects {
					if strings.EqualFold(p.Name, projectName) && strings.EqualFold(p.SourceRef, input.SourceRef.Ref) {
						return p, nil
					}
				}
			}
		}
		return persistence.Project{}, fmt.Errorf("failed to create project: %w", err)
	}

	slog.Info("scanner: created project",
		"project_id", created.ID,
		"project_name", created.Name,
		"source_ref", created.SourceRef,
	)

	return created, nil
}

// generateProjectName generates a stable project name from repo and directory
func (s *Scanner) generateProjectName(repoFullName, rocketshipDir string) string {
	// Extract repo name
	parts := strings.SplitN(repoFullName, "/", 2)
	repoName := parts[len(parts)-1]

	// If .rocketship is at root, use repo name
	if rocketshipDir == ".rocketship" {
		return repoName
	}

	// Otherwise, add directory suffix
	// e.g., "myrepo" for root, "myrepo-subdir" for subdir/.rocketship
	parentDir := path.Dir(rocketshipDir)
	if parentDir == "." {
		return repoName
	}

	// Sanitize directory name for use in project name
	suffix := strings.ReplaceAll(parentDir, "/", "-")
	return fmt.Sprintf("%s-%s", repoName, suffix)
}

// processSuiteFile processes a single suite YAML file
func (s *Scanner) processSuiteFile(ctx context.Context, input ScanInput, project persistence.Project, filePath, fetchRef, owner, repo string) (int, int, error) {
	// Fetch file content
	content, err := s.github.GetFileContent(ctx, input.InstallationID, owner, repo, filePath, fetchRef)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to fetch file: %w", err)
	}

	// Parse YAML
	config, err := dsl.ParseYAML(content)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Build suite metadata config JSONB
	// Note: We're not using a separate config column for suites/tests currently,
	// but if needed we could add it later

	// Upsert suite
	suite := persistence.Suite{
		ProjectID:   project.ID,
		Name:        config.Name,
		Description: sql.NullString{String: config.Description, Valid: config.Description != ""},
		FilePath:    sql.NullString{String: filePath, Valid: true},
		SourceRef:   input.SourceRef.Ref,
		TestCount:   len(config.Tests),
	}

	upsertedSuite, err := s.store.UpsertSuite(ctx, suite)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to upsert suite: %w", err)
	}

	slog.Debug("scanner: upserted suite",
		"suite_id", upsertedSuite.ID,
		"suite_name", upsertedSuite.Name,
		"test_count", len(config.Tests),
	)

	// Upsert tests
	testsCreated := 0
	for _, test := range config.Tests {
		testRecord := persistence.Test{
			SuiteID:   upsertedSuite.ID,
			ProjectID: project.ID,
			Name:      test.Name,
			SourceRef: input.SourceRef.Ref,
			StepCount: len(test.Steps),
		}

		_, err := s.store.UpsertTest(ctx, testRecord)
		if err != nil {
			slog.Error("scanner: failed to upsert test",
				"test_name", test.Name,
				"error", err,
			)
			continue
		}
		testsCreated++
	}

	return 1, testsCreated, nil
}

// ScanForWebhook performs a scan triggered by a webhook and records the attempt
func (s *Scanner) ScanForWebhook(ctx context.Context, input ScanInput) {
	startTime := time.Now()

	result, err := s.Scan(ctx, input)

	// Record the scan attempt
	attempt := persistence.ScanAttempt{
		DeliveryID:         input.DeliveryID,
		OrganizationID:     input.OrgID,
		RepositoryFullName: input.RepoFullName,
		SourceRef:          input.SourceRef.Ref,
		HeadSHA:            input.HeadSHA,
	}

	if err != nil {
		attempt.Status = persistence.ScanAttemptError
		attempt.ErrorMessage = err.Error()
		slog.Error("scanner: scan failed",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"error", err,
			"duration", time.Since(startTime),
		)
	} else if len(result.Errors) > 0 {
		attempt.Status = persistence.ScanAttemptError
		attempt.ErrorMessage = strings.Join(result.Errors, "; ")
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
		slog.Warn("scanner: scan completed with errors",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"suites_found", result.SuitesFound,
			"tests_found", result.TestsFound,
			"errors", len(result.Errors),
			"duration", time.Since(startTime),
		)
	} else {
		attempt.Status = persistence.ScanAttemptSuccess
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
		slog.Info("scanner: scan completed",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"suites_found", result.SuitesFound,
			"tests_found", result.TestsFound,
			"duration", time.Since(startTime),
		)
	}

	// Insert scan attempt record
	if insertErr := s.store.InsertScanAttempt(ctx, attempt); insertErr != nil {
		slog.Error("scanner: failed to record scan attempt",
			"delivery_id", input.DeliveryID,
			"error", insertErr,
		)
	}
}

// recordScanAttempt records a scan attempt without performing the scan
// Used by bootstrap scans where the scan is already performed
func (s *Scanner) recordScanAttempt(ctx context.Context, input ScanInput, result *ScanResult, scanErr error) {
	attempt := persistence.ScanAttempt{
		DeliveryID:         input.DeliveryID,
		OrganizationID:     input.OrgID,
		RepositoryFullName: input.RepoFullName,
		SourceRef:          input.SourceRef.Ref,
		HeadSHA:            input.HeadSHA,
	}

	if scanErr != nil {
		attempt.Status = persistence.ScanAttemptError
		attempt.ErrorMessage = scanErr.Error()
	} else if result != nil && len(result.Errors) > 0 {
		attempt.Status = persistence.ScanAttemptError
		attempt.ErrorMessage = strings.Join(result.Errors, "; ")
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
	} else if result != nil {
		attempt.Status = persistence.ScanAttemptSuccess
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
	} else {
		attempt.Status = persistence.ScanAttemptSkipped
	}

	if insertErr := s.store.InsertScanAttempt(ctx, attempt); insertErr != nil {
		slog.Error("scanner: failed to record scan attempt",
			"delivery_id", input.DeliveryID,
			"error", insertErr,
		)
	}
}
