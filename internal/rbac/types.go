package rbac

import (
	"time"
)

// Permission represents a specific permission
type Permission string

const (
	// Test permissions (simplified - tests are IaC managed)
	PermissionTestsRun Permission = "tests:run" // Run tests
	
	// Repository permissions
	PermissionRepositoriesRead   Permission = "repositories:read"   // View repository settings
	PermissionRepositoriesWrite  Permission = "repositories:write"  // Modify repository settings
	PermissionRepositoriesManage Permission = "repositories:manage" // Add/remove repositories
	
	// Team management permissions (for Team Admins)
	PermissionTeamMembersRead   Permission = "team:members:read"   // View team members
	PermissionTeamMembersWrite  Permission = "team:members:write"  // Add/remove team members
	PermissionTeamMembersManage Permission = "team:members:manage" // Manage member permissions
	
	// Test suite schedules (smoke tests)
	PermissionTestSchedulesManage Permission = "test:schedules:manage" // Manage smoke test schedules
	
	// Global permissions (for Organization Admins only)
	PermissionTeamsRead   Permission = "teams:read"   // View all teams
	PermissionTeamsWrite  Permission = "teams:write"  // Create/modify teams
	PermissionTeamsManage Permission = "teams:manage" // Full team management
	
	PermissionUsersManage  Permission = "users:manage"  // Full user management
	PermissionSystemManage Permission = "system:manage" // Full system administration
)

// Role represents a role within a team
type Role string

const (
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

// OrganizationRole represents organization-level roles
type OrganizationRole string

const (
	OrgRoleAdmin  OrganizationRole = "org_admin"
	OrgRoleMember OrganizationRole = "org_member"
)

// User represents a user in the system
type User struct {
	ID        string            `json:"id" db:"id"`
	Email     string            `json:"email" db:"email"`
	Name      string            `json:"name" db:"name"`
	OrgRole   OrganizationRole  `json:"org_role" db:"org_role"`
	CreatedAt time.Time         `json:"created_at" db:"created_at"`
	LastLogin *time.Time        `json:"last_login" db:"last_login"`
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

// HasPermission checks if the team member has a specific permission
func (tm *TeamMember) HasPermission(perm Permission) bool {
	for _, p := range tm.Permissions {
		if p == perm {
			return true
		}
	}
	return false
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
	UserID  string           `json:"user_id"`
	Email   string           `json:"email"`
	Name    string           `json:"name"`
	OrgRole OrganizationRole `json:"org_role"`
	
	// Token information (for API tokens)
	TokenID     *string      `json:"token_id,omitempty"`
	TokenTeamID *string      `json:"token_team_id,omitempty"`
	TokenPerms  []Permission `json:"token_permissions,omitempty"`
	
	// Team memberships (for users)
	TeamMemberships []TeamMember `json:"team_memberships,omitempty"`
}

// IsOrgAdmin returns true if the user is an organization admin
func (a *AuthContext) IsOrgAdmin() bool {
	return a.OrgRole == OrgRoleAdmin
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

// TestRun represents a test run in the database
type TestRun struct {
	ID               string     `json:"id" db:"id"`
	SuiteName        string     `json:"suite_name" db:"suite_name"`
	Status           string     `json:"status" db:"status"`
	RepositoryID     *string    `json:"repository_id" db:"repository_id"`
	FilePath         string     `json:"file_path" db:"file_path"`
	Branch           *string    `json:"branch" db:"branch"`
	CommitSHA        *string    `json:"commit_sha" db:"commit_sha"`
	TriggeredBy      *string    `json:"triggered_by" db:"triggered_by"`
	AuthorizedTeams  []string   `json:"authorized_teams" db:"authorized_teams"`
	Metadata         []byte     `json:"metadata" db:"metadata"`
	StartedAt        time.Time  `json:"started_at" db:"started_at"`
	EndedAt          *time.Time `json:"ended_at" db:"ended_at"`
	CreatedAt        time.Time  `json:"created_at" db:"created_at"`
	
	// Populated via joins
	Repository *RepositoryEntity `json:"repository,omitempty"`
	User       *User             `json:"user,omitempty"`
}