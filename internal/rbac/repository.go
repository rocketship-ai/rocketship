package rbac

import (
	"context"
	"fmt"
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