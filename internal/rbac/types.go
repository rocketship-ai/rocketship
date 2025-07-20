package rbac

import (
	"time"
)

// Permission represents a specific permission
type Permission string

const (
	// Test permissions (Buildkite-inspired)
	PermissionTestsRead    Permission = "tests:read"    // View test results
	PermissionTestsWrite   Permission = "tests:write"   // Run tests
	PermissionTestsManage  Permission = "tests:manage"  // Create/edit test suites
	
	// Workflow permissions
	PermissionWorkflowsRead    Permission = "workflows:read"    // View workflows
	PermissionWorkflowsWrite   Permission = "workflows:write"   // Run workflows
	PermissionWorkflowsManage  Permission = "workflows:manage"  // Create/edit workflows
	
	// Repository permissions
	PermissionRepositoriesRead   Permission = "repositories:read"   // View repository settings
	PermissionRepositoriesWrite  Permission = "repositories:write"  // Modify repository settings
	PermissionRepositoriesManage Permission = "repositories:manage" // Add/remove repositories
	
	// Team management permissions (for Team Admins)
	PermissionTeamMembersRead   Permission = "team:members:read"   // View team members
	PermissionTeamMembersWrite  Permission = "team:members:write"  // Add/remove team members
	PermissionTeamMembersManage Permission = "team:members:manage" // Manage member permissions
	
	// Global permissions (for Organization Admins)
	PermissionTeamsRead   Permission = "teams:read"   // View all teams
	PermissionTeamsWrite  Permission = "teams:write"  // Create/modify teams
	PermissionTeamsManage Permission = "teams:manage" // Full team management
	
	PermissionUsersRead   Permission = "users:read"   // View all users
	PermissionUsersWrite  Permission = "users:write"  // Modify user settings
	PermissionUsersManage Permission = "users:manage" // Full user management
	
	PermissionSystemRead   Permission = "system:read"   // View system settings
	PermissionSystemWrite  Permission = "system:write"  // Modify system settings
	PermissionSystemManage Permission = "system:manage" // Full system administration
)

// Role represents a role within a team
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// User represents a user in the system
type User struct {
	ID        string    `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	IsAdmin   bool      `json:"is_admin" db:"is_admin"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	LastLogin *time.Time `json:"last_login" db:"last_login"`
}

// Team represents a team in the system
type Team struct {
	ID        string    `json:"id" db:"id"`
	Name      string    `json:"name" db:"name"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	
	// Populated via joins
	Members []TeamMember `json:"members,omitempty"`
}

// TeamMember represents a team member with their role and permissions
type TeamMember struct {
	TeamID      string       `json:"team_id" db:"team_id"`
	UserID      string       `json:"user_id" db:"user_id"`
	Role        Role         `json:"role" db:"role"`
	Permissions []Permission `json:"permissions" db:"permissions"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	
	// Populated via joins
	User *User `json:"user,omitempty"`
}

// RepositoryEntity represents a repository in the system
type RepositoryEntity struct {
	ID                    string     `json:"id" db:"id"`
	URL                   string     `json:"url" db:"url"`
	GitHubInstallationID  *int64     `json:"github_installation_id" db:"github_installation_id"`
	EnforceCodeowners     bool       `json:"enforce_codeowners" db:"enforce_codeowners"`
	CodeownersCache       []byte     `json:"codeowners_cache" db:"codeowners_cache"`
	CodeownersCachedAt    *time.Time `json:"codeowners_cached_at" db:"codeowners_cached_at"`
	CreatedAt             time.Time  `json:"created_at" db:"created_at"`
	
	// Populated via joins
	Teams []Team `json:"teams,omitempty"`
}

// TeamRepository represents the many-to-many relationship between teams and repositories
type TeamRepository struct {
	TeamID       string    `json:"team_id" db:"team_id"`
	RepositoryID string    `json:"repository_id" db:"repository_id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}

// APIToken represents an API token
type APIToken struct {
	ID           string       `json:"id" db:"id"`
	TokenHash    string       `json:"token_hash" db:"token_hash"`
	TeamID       string       `json:"team_id" db:"team_id"`
	Name         string       `json:"name" db:"name"`
	Permissions  []Permission `json:"permissions" db:"permissions"`
	LastUsedAt   *time.Time   `json:"last_used_at" db:"last_used_at"`
	ExpiresAt    *time.Time   `json:"expires_at" db:"expires_at"`
	CreatedAt    time.Time    `json:"created_at" db:"created_at"`
	CreatedBy    string       `json:"created_by" db:"created_by"`
	
	// Populated via joins
	Team *Team `json:"team,omitempty"`
}

// AuthContext contains authentication information for a request
type AuthContext struct {
	// User information
	UserID  string `json:"user_id"`
	Email   string `json:"email"`
	Name    string `json:"name"`
	IsAdmin bool   `json:"is_admin"`
	
	// Token information (for API tokens)
	TokenID     *string      `json:"token_id,omitempty"`
	TokenTeamID *string      `json:"token_team_id,omitempty"`
	TokenPerms  []Permission `json:"token_permissions,omitempty"`
	
	// Team memberships (for users)
	TeamMemberships []TeamMember `json:"team_memberships,omitempty"`
}

// CodeownersRule represents a rule from a CODEOWNERS file
type CodeownersRule struct {
	Pattern string   `json:"pattern"`
	Owners  []string `json:"owners"`
}

// CodeownersData represents parsed CODEOWNERS data
type CodeownersData struct {
	Rules   []CodeownersRule `json:"rules"`
	Updated time.Time        `json:"updated"`
}

// TestRunRequest represents a request to run tests
type TestRunRequest struct {
	RepositoryURL string `json:"repository_url"`
	FilePath      string `json:"file_path"`
	Branch        string `json:"branch"`
	CommitSHA     string `json:"commit_sha"`
}