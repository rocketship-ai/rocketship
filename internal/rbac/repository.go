package rbac

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repository handles database operations for RBAC
type Repository struct {
	db *pgxpool.Pool
}

// NewRepository creates a new RBAC repository
func NewRepository(db *pgxpool.Pool) *Repository {
	return &Repository{db: db}
}

// User operations

// GetUser retrieves a user by ID
func (r *Repository) GetUser(ctx context.Context, userID string) (*User, error) {
	query := `SELECT id, email, name, is_admin, created_at, last_login FROM users WHERE id = $1`
	
	var user User
	err := r.db.QueryRow(ctx, query, userID).Scan(
		&user.ID, &user.Email, &user.Name, &user.IsAdmin,
		&user.CreatedAt, &user.LastLogin,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	
	return &user, nil
}

// CreateUser creates a new user
func (r *Repository) CreateUser(ctx context.Context, user *User) error {
	query := `
		INSERT INTO users (id, email, name, is_admin, created_at, last_login)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	
	_, err := r.db.Exec(ctx, query,
		user.ID, user.Email, user.Name, user.IsAdmin,
		user.CreatedAt, user.LastLogin,
	)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	
	return nil
}

// UpdateUser updates an existing user
func (r *Repository) UpdateUser(ctx context.Context, user *User) error {
	query := `
		UPDATE users 
		SET email = $2, name = $3, is_admin = $4, last_login = $5
		WHERE id = $1
	`
	
	_, err := r.db.Exec(ctx, query,
		user.ID, user.Email, user.Name, user.IsAdmin, user.LastLogin,
	)
	if err != nil {
		return fmt.Errorf("failed to update user: %w", err)
	}
	
	return nil
}

// GetOrCreateUserByEmail gets a user by email or creates them if they don't exist
func (r *Repository) GetOrCreateUserByEmail(ctx context.Context, email string) (*User, error) {
	// First try to get existing user
	query := `SELECT id, email, name, is_admin, created_at, last_login FROM users WHERE email = $1`
	
	var user User
	err := r.db.QueryRow(ctx, query, email).Scan(
		&user.ID, &user.Email, &user.Name, &user.IsAdmin, &user.CreatedAt, &user.LastLogin,
	)
	
	if err == nil {
		// User exists, return it
		return &user, nil
	}
	
	if err != pgx.ErrNoRows {
		return nil, fmt.Errorf("failed to query user: %w", err)
	}
	
	// User doesn't exist, create them
	newUser := &User{
		ID:        "external|" + email, // Use email-based ID for external users
		Email:     email,
		Name:      email, // Use email as name by default
		IsAdmin:   false, // External users are not admin by default
		CreatedAt: time.Now(),
		LastLogin: nil,
	}
	
	if err := r.CreateUser(ctx, newUser); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}
	
	return newUser, nil
}

// Team operations

// GetTeam retrieves a team by ID
func (r *Repository) GetTeam(ctx context.Context, teamID string) (*Team, error) {
	query := `SELECT id, name, created_at FROM teams WHERE id = $1`
	
	var team Team
	err := r.db.QueryRow(ctx, query, teamID).Scan(
		&team.ID, &team.Name, &team.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	
	return &team, nil
}

// GetTeamByName retrieves a team by name
func (r *Repository) GetTeamByName(ctx context.Context, name string) (*Team, error) {
	query := `SELECT id, name, created_at FROM teams WHERE name = $1`
	
	var team Team
	err := r.db.QueryRow(ctx, query, name).Scan(
		&team.ID, &team.Name, &team.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	
	return &team, nil
}

// CreateTeam creates a new team
func (r *Repository) CreateTeam(ctx context.Context, team *Team) error {
	query := `
		INSERT INTO teams (id, name, created_at)
		VALUES ($1, $2, $3)
	`
	
	_, err := r.db.Exec(ctx, query, team.ID, team.Name, team.CreatedAt)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}
	
	return nil
}

// ListTeams retrieves all teams
func (r *Repository) ListTeams(ctx context.Context) ([]*Team, error) {
	query := `SELECT id, name, created_at FROM teams ORDER BY name`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	defer rows.Close()
	
	var teams []*Team
	for rows.Next() {
		var team Team
		err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, &team)
	}
	
	return teams, nil
}

// GetTeamMembers retrieves all members of a team
func (r *Repository) GetTeamMembers(ctx context.Context, teamID string) ([]*TeamMember, error) {
	query := `
		SELECT tm.team_id, tm.user_id, tm.role, tm.permissions, tm.created_at
		FROM team_members tm
		WHERE tm.team_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}
	defer rows.Close()
	
	var members []*TeamMember
	for rows.Next() {
		var member TeamMember
		err := rows.Scan(
			&member.TeamID, &member.UserID, &member.Role,
			&member.Permissions, &member.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team member: %w", err)
		}
		members = append(members, &member)
	}
	
	return members, nil
}

// GetUserTeams retrieves all teams for a user
func (r *Repository) GetUserTeams(ctx context.Context, userID string) ([]TeamMember, error) {
	query := `
		SELECT tm.team_id, tm.user_id, tm.role, tm.permissions, tm.created_at,
			   t.name
		FROM team_members tm
		JOIN teams t ON tm.team_id = t.id
		WHERE tm.user_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get user teams: %w", err)
	}
	defer rows.Close()
	
	var memberships []TeamMember
	for rows.Next() {
		var membership TeamMember
		var teamName string
		
		err := rows.Scan(
			&membership.TeamID, &membership.UserID, &membership.Role,
			&membership.Permissions, &membership.CreatedAt, &teamName,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team membership: %w", err)
		}
		
		memberships = append(memberships, membership)
	}
	
	return memberships, nil
}

// AddTeamMember adds a member to a team
func (r *Repository) AddTeamMember(ctx context.Context, teamID, userID string, role Role, permissions []Permission) error {
	query := `
		INSERT INTO team_members (team_id, user_id, role, permissions, created_at)
		VALUES ($1, $2, $3, $4, NOW())
		ON CONFLICT (team_id, user_id) 
		DO UPDATE SET role = $3, permissions = $4
	`
	
	_, err := r.db.Exec(ctx, query, teamID, userID, role, permissions)
	if err != nil {
		return fmt.Errorf("failed to add team member: %w", err)
	}
	
	return nil
}

// RemoveTeamMember removes a member from a team
func (r *Repository) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	query := `DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`
	
	_, err := r.db.Exec(ctx, query, teamID, userID)
	if err != nil {
		return fmt.Errorf("failed to remove team member: %w", err)
	}
	
	return nil
}

// Repository operations

// GetRepository retrieves a repository by URL
func (r *Repository) GetRepository(ctx context.Context, url string) (*RepositoryEntity, error) {
	query := `
		SELECT id, url, github_installation_id, enforce_codeowners, 
			   codeowners_cache, codeowners_cached_at, created_at
		FROM repositories WHERE url = $1
	`
	
	var repo RepositoryEntity
	err := r.db.QueryRow(ctx, query, url).Scan(
		&repo.ID, &repo.URL, &repo.GitHubInstallationID, &repo.EnforceCodeowners,
		&repo.CodeownersCache, &repo.CodeownersCachedAt, &repo.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	
	return &repo, nil
}

// CreateRepository creates a new repository
func (r *Repository) CreateRepository(ctx context.Context, repo *RepositoryEntity) error {
	query := `
		INSERT INTO repositories (id, url, github_installation_id, enforce_codeowners, 
								  codeowners_cache, codeowners_cached_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	
	_, err := r.db.Exec(ctx, query,
		repo.ID, repo.URL, repo.GitHubInstallationID, repo.EnforceCodeowners,
		repo.CodeownersCache, repo.CodeownersCachedAt, repo.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}
	
	return nil
}

// UpdateRepositoryCodeowners updates the CODEOWNERS cache for a repository
func (r *Repository) UpdateRepositoryCodeowners(ctx context.Context, repoID string, codeowners []byte) error {
	query := `
		UPDATE repositories 
		SET codeowners_cache = $2, codeowners_cached_at = NOW()
		WHERE id = $1
	`
	
	_, err := r.db.Exec(ctx, query, repoID, codeowners)
	if err != nil {
		return fmt.Errorf("failed to update repository codeowners: %w", err)
	}
	
	return nil
}

// GetRepositoryTeams retrieves all teams that have access to a repository
func (r *Repository) GetRepositoryTeams(ctx context.Context, repoID string) ([]Team, error) {
	query := `
		SELECT t.id, t.name, t.created_at
		FROM teams t
		JOIN team_repositories tr ON t.id = tr.team_id
		WHERE tr.repository_id = $1
	`
	
	rows, err := r.db.Query(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository teams: %w", err)
	}
	defer rows.Close()
	
	var teams []Team
	for rows.Next() {
		var team Team
		err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, team)
	}
	
	return teams, nil
}

// AddTeamRepository adds a repository to a team
func (r *Repository) AddTeamRepository(ctx context.Context, teamID, repoID string) error {
	query := `
		INSERT INTO team_repositories (team_id, repository_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (team_id, repository_id) DO NOTHING
	`
	
	_, err := r.db.Exec(ctx, query, teamID, repoID)
	if err != nil {
		return fmt.Errorf("failed to add team repository: %w", err)
	}
	
	return nil
}

// API Token operations

// GetAPIToken retrieves an API token by hash
func (r *Repository) GetAPIToken(ctx context.Context, tokenHash string) (*APIToken, error) {
	query := `
		SELECT id, token_hash, team_id, name, permissions, last_used_at, 
			   expires_at, created_at, created_by
		FROM api_tokens WHERE token_hash = $1
	`
	
	var token APIToken
	err := r.db.QueryRow(ctx, query, tokenHash).Scan(
		&token.ID, &token.TokenHash, &token.TeamID, &token.Name,
		&token.Permissions, &token.LastUsedAt, &token.ExpiresAt,
		&token.CreatedAt, &token.CreatedBy,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get API token: %w", err)
	}
	
	return &token, nil
}

// CreateAPIToken creates a new API token
func (r *Repository) CreateAPIToken(ctx context.Context, token *APIToken) error {
	query := `
		INSERT INTO api_tokens (id, token_hash, team_id, name, permissions, 
								expires_at, created_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
	`
	
	_, err := r.db.Exec(ctx, query,
		token.ID, token.TokenHash, token.TeamID, token.Name,
		token.Permissions, token.ExpiresAt, token.CreatedAt, token.CreatedBy,
	)
	if err != nil {
		return fmt.Errorf("failed to create API token: %w", err)
	}
	
	return nil
}

// UpdateAPITokenLastUsed updates the last used timestamp for an API token
func (r *Repository) UpdateAPITokenLastUsed(ctx context.Context, tokenID string) error {
	query := `UPDATE api_tokens SET last_used_at = NOW() WHERE id = $1`
	
	_, err := r.db.Exec(ctx, query, tokenID)
	if err != nil {
		return fmt.Errorf("failed to update API token last used: %w", err)
	}
	
	return nil
}

// Repository management operations

// GetRepositoryByID retrieves a repository by ID
func (r *Repository) GetRepositoryByID(ctx context.Context, repoID string) (*RepositoryEntity, error) {
	query := `
		SELECT id, url, github_installation_id, enforce_codeowners, 
		       codeowners_cache, codeowners_cached_at, created_at
		FROM repositories WHERE id = $1
	`
	
	var repo RepositoryEntity
	err := r.db.QueryRow(ctx, query, repoID).Scan(
		&repo.ID, &repo.URL, &repo.GitHubInstallationID, &repo.EnforceCodeowners,
		&repo.CodeownersCache, &repo.CodeownersCachedAt, &repo.CreatedAt,
	)
	
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	
	return &repo, nil
}

// GetRepositoryByURL retrieves a repository by URL (alias for existing GetRepository method)
func (r *Repository) GetRepositoryByURL(ctx context.Context, url string) (*RepositoryEntity, error) {
	return r.GetRepository(ctx, url)
}

// ListRepositories lists all repositories
func (r *Repository) ListRepositories(ctx context.Context) ([]*RepositoryEntity, error) {
	query := `
		SELECT id, url, github_installation_id, enforce_codeowners, 
		       codeowners_cache, codeowners_cached_at, created_at
		FROM repositories
		ORDER BY created_at DESC
	`
	
	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}
	defer rows.Close()
	
	var repositories []*RepositoryEntity
	for rows.Next() {
		var repo RepositoryEntity
		err := rows.Scan(
			&repo.ID, &repo.URL, &repo.GitHubInstallationID, &repo.EnforceCodeowners,
			&repo.CodeownersCache, &repo.CodeownersCachedAt, &repo.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repositories = append(repositories, &repo)
	}
	
	return repositories, nil
}

// UpdateRepository updates a repository
func (r *Repository) UpdateRepository(ctx context.Context, repo *RepositoryEntity) error {
	query := `
		UPDATE repositories 
		SET url = $2, github_installation_id = $3, enforce_codeowners = $4,
		    codeowners_cache = $5, codeowners_cached_at = $6
		WHERE id = $1
	`
	
	_, err := r.db.Exec(ctx, query,
		repo.ID, repo.URL, repo.GitHubInstallationID, repo.EnforceCodeowners,
		repo.CodeownersCache, repo.CodeownersCachedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to update repository: %w", err)
	}
	
	return nil
}

// DeleteRepository deletes a repository
func (r *Repository) DeleteRepository(ctx context.Context, repoID string) error {
	query := `DELETE FROM repositories WHERE id = $1`
	
	_, err := r.db.Exec(ctx, query, repoID)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}
	
	return nil
}

// AssignTeamToRepository assigns a team to a repository
func (r *Repository) AssignTeamToRepository(ctx context.Context, teamID, repoID string) error {
	query := `
		INSERT INTO team_repositories (team_id, repository_id, created_at)
		VALUES ($1, $2, NOW())
		ON CONFLICT (team_id, repository_id) DO NOTHING
	`
	
	_, err := r.db.Exec(ctx, query, teamID, repoID)
	if err != nil {
		return fmt.Errorf("failed to assign team to repository: %w", err)
	}
	
	return nil
}

// RemoveTeamFromRepository removes a team from a repository
func (r *Repository) RemoveTeamFromRepository(ctx context.Context, teamID, repoID string) error {
	query := `DELETE FROM team_repositories WHERE team_id = $1 AND repository_id = $2`
	
	_, err := r.db.Exec(ctx, query, teamID, repoID)
	if err != nil {
		return fmt.Errorf("failed to remove team from repository: %w", err)
	}
	
	return nil
}

// GetRepositoryTeamsDetailed gets all teams assigned to a repository (enhanced version)
func (r *Repository) GetRepositoryTeamsDetailed(ctx context.Context, repoID string) ([]*Team, error) {
	query := `
		SELECT t.id, t.name, t.created_at
		FROM teams t
		JOIN team_repositories tr ON t.id = tr.team_id
		WHERE tr.repository_id = $1
		ORDER BY t.name
	`
	
	rows, err := r.db.Query(ctx, query, repoID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository teams: %w", err)
	}
	defer rows.Close()
	
	var teams []*Team
	for rows.Next() {
		var team Team
		err := rows.Scan(&team.ID, &team.Name, &team.CreatedAt)
		if err != nil {
			return nil, fmt.Errorf("failed to scan team: %w", err)
		}
		teams = append(teams, &team)
	}
	
	return teams, nil
}

// GetTeamRepositories gets all repositories assigned to a team
func (r *Repository) GetTeamRepositories(ctx context.Context, teamID string) ([]*RepositoryEntity, error) {
	query := `
		SELECT r.id, r.url, r.github_installation_id, r.enforce_codeowners, 
		       r.codeowners_cache, r.codeowners_cached_at, r.created_at
		FROM repositories r
		JOIN team_repositories tr ON r.id = tr.repository_id
		WHERE tr.team_id = $1
		ORDER BY r.url
	`
	
	rows, err := r.db.Query(ctx, query, teamID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team repositories: %w", err)
	}
	defer rows.Close()
	
	var repositories []*RepositoryEntity
	for rows.Next() {
		var repo RepositoryEntity
		err := rows.Scan(
			&repo.ID, &repo.URL, &repo.GitHubInstallationID, &repo.EnforceCodeowners,
			&repo.CodeownersCache, &repo.CodeownersCachedAt, &repo.CreatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan repository: %w", err)
		}
		repositories = append(repositories, &repo)
	}
	
	return repositories, nil
}

// Permissions resolution operations

// CheckTestRunPermission implements the complete permissions resolution flow
// This is the central method that determines if a user can run tests on a specific file path
func (r *Repository) CheckTestRunPermission(ctx context.Context, userID, repositoryURL, filePath string) (bool, error) {
	// Step a: Check if user is an Organization Admin
	user, err := r.GetOrCreateUserByEmail(ctx, userID)
	if err != nil {
		return false, fmt.Errorf("failed to get user: %w", err)
	}
	
	if user.IsAdmin {
		// Organization Admins can run tests anywhere
		return true, nil
	}
	
	// Step b: Get the repository from database
	repo, err := r.GetRepository(ctx, repositoryURL)
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}
	
	if repo == nil {
		// Repository not registered in system - deny access
		return false, nil
	}
	
	// Step c: Get user's team memberships
	teamMemberships, err := r.GetUserTeams(ctx, user.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get user teams: %w", err)
	}
	
	// Step d: Check which teams have access to this repository
	var authorizedTeams []string
	
	for _, membership := range teamMemberships {
		// Get team details
		team, err := r.GetTeam(ctx, membership.TeamID)
		if err != nil {
			continue // Skip this team if we can't get details
		}
		
		// Check if this team has access to the repository
		teamRepos, err := r.GetTeamRepositories(ctx, membership.TeamID)
		if err != nil {
			continue // Skip this team if we can't get repositories
		}
		
		for _, teamRepo := range teamRepos {
			if teamRepo.URL == repositoryURL {
				authorizedTeams = append(authorizedTeams, team.Name)
				break
			}
		}
	}
	
	if len(authorizedTeams) == 0 {
		// User has no teams with access to this repository
		return false, nil
	}
	
	// Step e: If repository has CODEOWNERS enforcement, check path ownership
	if repo.EnforceCodeowners {
		// Parse cached CODEOWNERS data
		if repo.CodeownersCache != nil {
			var codeownersData CodeownersData
			if err := json.Unmarshal(repo.CodeownersCache, &codeownersData); err != nil {
				return false, fmt.Errorf("failed to parse CODEOWNERS cache: %w", err)
			}
			
			// Check if the user's teams own this path according to CODEOWNERS
			if !checkPathOwnership(&codeownersData, filePath, authorizedTeams) {
				// User's teams don't own this path
				return false, nil
			}
		} else {
			// CODEOWNERS enforcement is enabled but no cache exists
			// This means we need to fetch CODEOWNERS from GitHub
			// For now, we'll deny access until the cache is populated
			return false, fmt.Errorf("CODEOWNERS enforcement enabled but cache not available")
		}
	}
	
	// If we get here, user has team access to the repository
	// and either CODEOWNERS is not enforced or the path check passed
	return true, nil
}

// CreateTestRun creates a new test run record
func (r *Repository) CreateTestRun(ctx context.Context, testRun *TestRun) error {
	query := `
		INSERT INTO test_runs (id, suite_name, status, repository_id, file_path, 
		                      branch, commit_sha, triggered_by, authorized_teams, 
		                      metadata, started_at, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
	`
	
	_, err := r.db.Exec(ctx, query,
		testRun.ID, testRun.SuiteName, testRun.Status, testRun.RepositoryID,
		testRun.FilePath, testRun.Branch, testRun.CommitSHA, testRun.TriggeredBy,
		testRun.AuthorizedTeams, testRun.Metadata, testRun.StartedAt, testRun.CreatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create test run: %w", err)
	}
	
	return nil
}

// GetTestRun retrieves a test run by ID
func (r *Repository) GetTestRun(ctx context.Context, testRunID string) (*TestRun, error) {
	query := `
		SELECT id, suite_name, status, repository_id, file_path, branch, 
		       commit_sha, triggered_by, authorized_teams, metadata, 
		       started_at, ended_at, created_at
		FROM test_runs WHERE id = $1
	`
	
	var testRun TestRun
	err := r.db.QueryRow(ctx, query, testRunID).Scan(
		&testRun.ID, &testRun.SuiteName, &testRun.Status, &testRun.RepositoryID,
		&testRun.FilePath, &testRun.Branch, &testRun.CommitSHA, &testRun.TriggeredBy,
		&testRun.AuthorizedTeams, &testRun.Metadata, &testRun.StartedAt,
		&testRun.EndedAt, &testRun.CreatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get test run: %w", err)
	}
	
	return &testRun, nil
}

// checkPathOwnership checks if a team owns a specific path according to CODEOWNERS
// This is a copy of the logic from internal/github to avoid circular imports
func checkPathOwnership(codeownersData *CodeownersData, filePath string, teamNames []string) bool {
	if codeownersData == nil || len(codeownersData.Rules) == 0 {
		// No CODEOWNERS file means no restrictions
		return true
	}
	
	// Convert team names to GitHub team format for comparison
	githubTeams := make(map[string]bool)
	for _, teamName := range teamNames {
		// Convert "Backend Team" to "@rocketship-ai/backend-team"
		githubTeam := convertTeamNameToGitHub(teamName)
		githubTeams[githubTeam] = true
	}
	
	// Find the most specific rule that matches the path
	var matchingRule *CodeownersRule
	var longestMatch int
	
	for _, rule := range codeownersData.Rules {
		if matchesCodeownersPattern(rule.Pattern, filePath) {
			// For CODEOWNERS, later rules override earlier ones
			// and more specific patterns take precedence
			patternSpecificity := getCodeownersPatternSpecificity(rule.Pattern, filePath)
			if patternSpecificity > longestMatch {
				longestMatch = patternSpecificity
				matchingRule = &rule
			}
		}
	}
	
	if matchingRule == nil {
		// No matching rule means no restrictions
		return true
	}
	
	// Check if any of the user's teams own this path
	for _, owner := range matchingRule.Owners {
		if githubTeams[owner] {
			return true
		}
	}
	
	return false
}

// Helper functions for CODEOWNERS path matching

// convertTeamNameToGitHub converts internal team name to GitHub team format
func convertTeamNameToGitHub(teamName string) string {
	// Convert "Backend Team" to "@rocketship-ai/backend-team"
	// Convert "QA Team" to "@rocketship-ai/qa-team"
	// Convert "Frontend Team" to "@rocketship-ai/frontend-team"
	
	teamName = strings.ToLower(teamName)
	teamName = strings.ReplaceAll(teamName, " ", "-")
	return fmt.Sprintf("@rocketship-ai/%s", teamName)
}

// matchesCodeownersPattern checks if a file path matches a CODEOWNERS pattern
func matchesCodeownersPattern(pattern, filePath string) bool {
	// Normalize paths
	pattern = strings.TrimPrefix(pattern, "/")
	filePath = strings.TrimPrefix(filePath, "/")
	
	// Handle different pattern types
	if pattern == "*" {
		return true
	}
	
	if strings.HasSuffix(pattern, "/") {
		// Directory pattern - matches if path starts with pattern
		return strings.HasPrefix(filePath, pattern) || strings.HasPrefix(filePath+"/", pattern)
	}
	
	if strings.HasPrefix(pattern, "*.") {
		// Extension pattern - check file extension
		extension := pattern[1:] // Remove the *
		return strings.HasSuffix(filePath, extension)
	}
	
	if strings.Contains(pattern, "*") {
		// Glob pattern - use simplified glob matching
		return matchCodeownersGlob(pattern, filePath)
	}
	
	// Exact match or prefix match
	if pattern == filePath {
		return true
	}
	
	// Check if it's a directory prefix match
	if strings.HasPrefix(filePath, pattern+"/") {
		return true
	}
	
	return false
}

// matchCodeownersGlob provides simplified glob pattern matching
func matchCodeownersGlob(pattern, str string) bool {
	// Convert glob pattern to regex-like matching
	// This is a simplified implementation for common CODEOWNERS patterns
	
	// Handle ** patterns (match any directories)
	if strings.Contains(pattern, "**") {
		parts := strings.Split(pattern, "**")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			
			// Remove leading/trailing slashes for comparison
			prefix = strings.Trim(prefix, "/")
			suffix = strings.Trim(suffix, "/")
			
			if prefix != "" && !strings.HasPrefix(str, prefix) {
				return false
			}
			
			if suffix != "" && !strings.HasSuffix(str, suffix) {
				return false
			}
			
			return true
		}
	}
	
	// Handle single * patterns
	if strings.Contains(pattern, "*") {
		// Simple implementation - check prefix and suffix
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			prefix := parts[0]
			suffix := parts[1]
			
			return strings.HasPrefix(str, prefix) && strings.HasSuffix(str, suffix)
		}
	}
	
	return false
}

// getCodeownersPatternSpecificity returns a score for how specific a pattern is
// Higher scores indicate more specific patterns
func getCodeownersPatternSpecificity(pattern, filePath string) int {
	specificity := 0
	
	// Exact matches are most specific
	if pattern == filePath {
		return 1000
	}
	
	// Extension patterns are less specific than directory patterns
	if strings.HasPrefix(pattern, "*.") {
		specificity = 10
	} else if strings.Contains(pattern, "*") {
		// Glob patterns are less specific
		specificity = 50
	} else {
		// Directory/path patterns
		specificity = 100 + len(strings.Split(pattern, "/"))
	}
	
	return specificity
}