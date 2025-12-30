package controlplane

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
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

		// Reconcile: deactivate suites that were previously discovered but no longer exist
		// This ensures deleted suite files are marked inactive
		deactivatedCount, err := s.store.DeactivateSuitesMissingFromDir(
			ctx, project.ID, input.SourceRef.Ref, dir, suiteFiles, "missing_from_scan",
		)
		if err != nil {
			slog.Error("scanner: failed to reconcile deleted suites",
				"dir", dir,
				"ref", input.SourceRef.Ref,
				"error", err,
			)
		} else if deactivatedCount > 0 {
			slog.Info("scanner: deactivated missing suites",
				"dir", dir,
				"ref", input.SourceRef.Ref,
				"count", deactivatedCount,
			)
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

	// Upsert tests and collect present test names for reconciliation
	testsCreated := 0
	presentTestNames := make([]string, 0, len(config.Tests))
	for _, test := range config.Tests {
		presentTestNames = append(presentTestNames, test.Name)

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

	// Reconcile: deactivate tests that were previously discovered but no longer exist in the YAML
	deactivatedCount, err := s.store.DeactivateTestsMissingFromSuite(
		ctx, upsertedSuite.ID, input.SourceRef.Ref, presentTestNames, "missing_from_suite_yaml",
	)
	if err != nil {
		slog.Error("scanner: failed to reconcile deleted tests",
			"suite_id", upsertedSuite.ID,
			"ref", input.SourceRef.Ref,
			"error", err,
		)
	} else if deactivatedCount > 0 {
		slog.Info("scanner: deactivated missing tests",
			"suite_id", upsertedSuite.ID,
			"suite_name", upsertedSuite.Name,
			"ref", input.SourceRef.Ref,
			"count", deactivatedCount,
		)
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

// ScanPullRequestDelta performs a delta scan for a pull request.
// Instead of scanning the entire tree, it only scans the files changed in the PR.
// This prevents creating duplicate branch-specific discovery rows.
func (s *Scanner) ScanPullRequestDelta(ctx context.Context, input ScanInput, prNumber int) (*ScanResult, error) {
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

	// Get the list of files changed in the PR
	prFiles, err := s.github.ListPullRequestFiles(ctx, input.InstallationID, owner, repo, prNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to list PR files: %w", err)
	}

	// Group all files under .rocketship directories (not just YAML)
	// This ensures we detect new .rocketship dirs even without YAML files
	dirFiles := make(map[string][]PullRequestFile)
	for _, f := range prFiles {
		// Check if file is in a .rocketship directory
		if s.isUnderRocketshipDir(f.Filename) {
			rocketshipDir := s.extractRocketshipDir(f.Filename)
			dirFiles[rocketshipDir] = append(dirFiles[rocketshipDir], f)
		}
	}

	if len(dirFiles) == 0 {
		slog.Debug("scanner: PR does not change any .rocketship files",
			"repo", input.RepoFullName,
			"pr", prNumber,
			"total_files", len(prFiles),
		)
		return result, nil
	}

	slog.Info("scanner: processing PR delta scan",
		"repo", input.RepoFullName,
		"pr", prNumber,
		"ref", input.SourceRef.Ref,
		"rocketship_dirs", len(dirFiles),
	)

	// Determine which ref to use for fetching content
	fetchRef := input.HeadSHA
	if fetchRef == "" {
		fetchRef = input.SourceRef.Ref
	}

	// Process each .rocketship directory
	for rocketshipDir, files := range dirFiles {
		slog.Info("scanner: processing .rocketship directory (delta)",
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"dir", rocketshipDir,
			"files", len(files),
		)

		// Get or create project - but prefer using existing default-branch project
		project, err := s.upsertProjectForPR(ctx, input, repoInfo, rocketshipDir)
		if err != nil {
			errMsg := fmt.Sprintf("failed to upsert project for dir %s: %v", rocketshipDir, err)
			result.Errors = append(result.Errors, errMsg)
			slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
			continue
		}

		// Process each file based on its status and type
		for _, file := range files {
			// Only process YAML files for suite operations
			isYAML := strings.HasSuffix(file.Filename, ".yaml") || strings.HasSuffix(file.Filename, ".yml")

			switch file.Status {
			case "removed":
				// File was deleted - deactivate the suite for this branch
				if isYAML {
					slog.Info("scanner: deactivating removed suite",
						"file", file.Filename,
						"ref", input.SourceRef.Ref,
					)
					if err := s.store.DeactivateSuiteByProjectRefAndFilePath(
						ctx, project.ID, input.SourceRef.Ref, file.Filename, "removed_in_pr",
					); err != nil {
						slog.Error("scanner: failed to deactivate removed suite",
							"file", file.Filename,
							"error", err,
						)
					}
				}

			case "renamed":
				// File was renamed - deactivate old path, process new path
				if isYAML {
					// Deactivate the old file path
					if file.PreviousFilename != "" {
						slog.Info("scanner: deactivating renamed suite (old path)",
							"old_file", file.PreviousFilename,
							"new_file", file.Filename,
							"ref", input.SourceRef.Ref,
						)
						if err := s.store.DeactivateSuiteByProjectRefAndFilePath(
							ctx, project.ID, input.SourceRef.Ref, file.PreviousFilename, "renamed_in_pr",
						); err != nil {
							slog.Error("scanner: failed to deactivate old path for renamed suite",
								"file", file.PreviousFilename,
								"error", err,
							)
						}
					}

					// Process the new path (will create new suite)
					suitesCreated, testsCreated, err := s.processSuiteFile(ctx, input, project, file.Filename, fetchRef, owner, repo)
					if err != nil {
						errMsg := fmt.Sprintf("failed to process renamed suite file %s: %v", file.Filename, err)
						result.Errors = append(result.Errors, errMsg)
						slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
						continue
					}
					result.SuitesFound += suitesCreated
					result.TestsFound += testsCreated
				}

			case "added", "modified":
				// Process added/modified YAML files
				if isYAML {
					suitesCreated, testsCreated, err := s.processSuiteFile(ctx, input, project, file.Filename, fetchRef, owner, repo)
					if err != nil {
						errMsg := fmt.Sprintf("failed to process suite file %s: %v", file.Filename, err)
						result.Errors = append(result.Errors, errMsg)
						slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
						continue
					}
					result.SuitesFound += suitesCreated
					result.TestsFound += testsCreated
				}

			default:
				// Handle other statuses (copied, changed) as modified
				if isYAML {
					suitesCreated, testsCreated, err := s.processSuiteFile(ctx, input, project, file.Filename, fetchRef, owner, repo)
					if err != nil {
						errMsg := fmt.Sprintf("failed to process suite file %s: %v", file.Filename, err)
						result.Errors = append(result.Errors, errMsg)
						slog.Error("scanner: "+errMsg, "repo", input.RepoFullName)
						continue
					}
					result.SuitesFound += suitesCreated
					result.TestsFound += testsCreated
				}
			}
		}
	}

	return result, nil
}

// isUnderRocketshipDir checks if a file path is under a .rocketship directory
func (s *Scanner) isUnderRocketshipDir(filePath string) bool {
	return strings.Contains(filePath, "/.rocketship/") || strings.HasPrefix(filePath, ".rocketship/")
}

// extractRocketshipDir extracts the .rocketship directory path from a file path
func (s *Scanner) extractRocketshipDir(filePath string) string {
	// Handle root .rocketship/
	if strings.HasPrefix(filePath, ".rocketship/") {
		return ".rocketship"
	}

	// Find the .rocketship directory in the path
	parts := strings.Split(filePath, "/")
	for i, part := range parts {
		if part == ".rocketship" {
			return strings.Join(parts[:i+1], "/")
		}
	}

	// Fallback to parent directory
	return path.Dir(filePath)
}

// ScanPullRequestDeltaForWebhook performs a delta scan triggered by a webhook and records the attempt
func (s *Scanner) ScanPullRequestDeltaForWebhook(ctx context.Context, input ScanInput, prNumber int) {
	startTime := time.Now()

	result, err := s.ScanPullRequestDelta(ctx, input, prNumber)

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
		slog.Error("scanner: PR delta scan failed",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"pr", prNumber,
			"error", err,
			"duration", time.Since(startTime),
		)
	} else if len(result.Errors) > 0 {
		attempt.Status = persistence.ScanAttemptError
		attempt.ErrorMessage = strings.Join(result.Errors, "; ")
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
		slog.Warn("scanner: PR delta scan completed with errors",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"pr", prNumber,
			"suites_found", result.SuitesFound,
			"tests_found", result.TestsFound,
			"errors", len(result.Errors),
			"duration", time.Since(startTime),
		)
	} else {
		attempt.Status = persistence.ScanAttemptSuccess
		attempt.SuitesFound = result.SuitesFound
		attempt.TestsFound = result.TestsFound
		slog.Info("scanner: PR delta scan completed",
			"org_id", input.OrgID,
			"repo", input.RepoFullName,
			"ref", input.SourceRef.Ref,
			"pr", prNumber,
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

// upsertProjectForPR creates or retrieves a project for a PR scan.
// Unlike upsertProject, this prefers reusing an existing default-branch project
// rather than creating a new branch-specific project.
func (s *Scanner) upsertProjectForPR(ctx context.Context, input ScanInput, repoInfo *GitHubRepoInfo, rocketshipDir string) (persistence.Project, error) {
	// Build path_scope
	pathScope := []string{rocketshipDir + "/**"}
	repoURL := fmt.Sprintf("https://github.com/%s", input.RepoFullName)

	// First, check if a default-branch project exists for this repo+pathScope
	defaultProject, found, err := s.store.FindDefaultBranchProject(ctx, input.OrgID, repoURL, pathScope)
	if err != nil {
		return persistence.Project{}, fmt.Errorf("failed to check for default branch project: %w", err)
	}

	if found {
		// Use the existing default-branch project - don't create a branch variant
		slog.Debug("scanner: reusing default branch project for PR",
			"project_id", defaultProject.ID,
			"project_name", defaultProject.Name,
			"pr_ref", input.SourceRef.Ref,
			"default_branch", defaultProject.DefaultBranch,
		)
		return defaultProject, nil
	}

	// No default-branch project exists - this is a new .rocketship dir introduced by the PR
	// Create a new project for this PR branch
	slog.Info("scanner: creating new project for PR (new .rocketship dir)",
		"repo", input.RepoFullName,
		"ref", input.SourceRef.Ref,
		"dir", rocketshipDir,
	)

	return s.upsertProject(ctx, input, repoInfo, rocketshipDir)
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
