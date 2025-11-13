package persistence

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lib/pq"
)

// Errors
var (
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrOrganizationSlugUsed = errors.New("organization slug already in use")
	ErrEmailInUse           = errors.New("email already in use")
)

// User represents a registered user
type User struct {
	ID           uuid.UUID `db:"id"`
	GitHubUserID int64     `db:"github_user_id"`
	Email        string    `db:"email"`
	Name         string    `db:"name"`
	Username     string    `db:"username"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`
}

type GitHubUserInput struct {
	GitHubUserID int64
	Email        string
	Name         string
	Username     string
}

// Organization represents a tenant organization
type Organization struct {
	ID        uuid.UUID
	Name      string
	Slug      string
	CreatedAt time.Time
}

type OrganizationMembership struct {
	OrganizationID uuid.UUID
	IsAdmin        bool
}

// Project represents a test project within an organization
type Project struct {
	ID             uuid.UUID
	OrganizationID uuid.UUID
	Name           string
	RepoURL        string
	DefaultBranch  string
	PathScope      []string
	CreatedAt      time.Time
}

type ProjectMembership struct {
	ProjectID      uuid.UUID
	OrganizationID uuid.UUID
	Role           string
}

type ProjectMember struct {
	UserID    uuid.UUID
	Email     string
	Name      string
	Username  string
	Role      string
	JoinedAt  time.Time
	UpdatedAt time.Time
}

type CreateOrgInput struct {
	Name    string
	Slug    string
	Project ProjectInput
}

type ProjectInput struct {
	Name          string
	RepoURL       string
	DefaultBranch string
	PathScope     []string
}

// RefreshTokenRecord represents a stored refresh token
type RefreshTokenRecord struct {
	TokenID        uuid.UUID
	User           User
	OrganizationID uuid.UUID
	Scopes         []string
	IssuedAt       time.Time
	ExpiresAt      time.Time
}

// OrganizationRegistration tracks new organization registration flow
type OrganizationRegistration struct {
	ID                uuid.UUID `db:"id"`
	UserID            uuid.UUID `db:"user_id"`
	Email             string    `db:"email"`
	OrgName           string    `db:"org_name"`
	CodeHash          []byte    `db:"code_hash"`
	CodeSalt          []byte    `db:"code_salt"`
	Attempts          int       `db:"attempts"`
	MaxAttempts       int       `db:"max_attempts"`
	ExpiresAt         time.Time `db:"expires_at"`
	ResendAvailableAt time.Time `db:"resend_available_at"`
	CreatedAt         time.Time `db:"created_at"`
	UpdatedAt         time.Time `db:"updated_at"`
}

// OrganizationInvite tracks pending organization invitations
type OrganizationInvite struct {
	ID               uuid.UUID     `db:"id"`
	OrganizationID   uuid.UUID     `db:"organization_id"`
	Email            string        `db:"email"`
	Role             string        `db:"role"`
	CodeHash         []byte        `db:"code_hash"`
	CodeSalt         []byte        `db:"code_salt"`
	InvitedBy        uuid.UUID     `db:"invited_by"`
	ExpiresAt        time.Time     `db:"expires_at"`
	AcceptedAt       sql.NullTime  `db:"accepted_at"`
	AcceptedBy       uuid.NullUUID `db:"accepted_by"`
	OrganizationName string        `db:"organization_name"`
	CreatedAt        time.Time     `db:"created_at"`
	UpdatedAt        time.Time     `db:"updated_at"`
}

// RunRecord represents a test suite execution
type RunRecord struct {
	ID             string         `db:"id"`
	OrganizationID uuid.UUID      `db:"organization_id"`
	ProjectID      uuid.NullUUID  `db:"project_id"`
	Status         string         `db:"status"`
	SuiteName      string         `db:"suite_name"`
	Initiator      string         `db:"initiator"`
	Trigger        string         `db:"trigger"`
	ScheduleName   string         `db:"schedule_name"`
	ConfigSource   string         `db:"config_source"`
	Source         string         `db:"source"`
	Branch         string         `db:"branch"`
	Environment    string         `db:"environment"`
	CommitSHA      sql.NullString `db:"commit_sha"`
	BundleSHA      sql.NullString `db:"bundle_sha"`
	TotalTests     int            `db:"total_tests"`
	PassedTests    int            `db:"passed_tests"`
	FailedTests    int            `db:"failed_tests"`
	TimeoutTests   int            `db:"timeout_tests"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	StartedAt      sql.NullTime   `db:"started_at"`
	EndedAt        sql.NullTime   `db:"ended_at"`
}

type RunTotals struct {
	Total   int
	Passed  int
	Failed  int
	Timeout int
}

type RunUpdate struct {
	RunID          string
	OrganizationID uuid.UUID
	Status         *string
	StartedAt      *time.Time
	EndedAt        *time.Time
	Totals         *RunTotals
	CommitSHA      *string
	BundleSHA      *string
}

// RoleSummary aggregates user memberships and roles
type RoleSummary struct {
	Organizations []OrganizationMembership
	Projects      []ProjectMembership
}

// AggregatedRoles computes the effective roles for a user based on memberships
func (r RoleSummary) AggregatedRoles() []string {
	ordered := []string{}
	seen := make(map[string]struct{})

	add := func(role string) {
		if _, ok := seen[role]; ok {
			return
		}
		seen[role] = struct{}{}
		ordered = append(ordered, role)
	}

	if len(r.Organizations) > 0 {
		add("owner")
	}

	hasWrite := false
	hasRead := false
	for _, project := range r.Projects {
		switch strings.ToLower(project.Role) {
		case "write":
			hasWrite = true
		case "read":
			hasRead = true
		}
	}

	if hasWrite {
		add("editor")
	}
	if hasRead {
		add("viewer")
	}

	if len(ordered) == 0 {
		add("pending")
	}

	return ordered
}

// Helper functions

func normalizeEmail(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func isUniqueViolation(err error, constraint string) bool {
	var pqErr *pq.Error
	if errors.As(err, &pqErr) && pqErr.Code == "23505" {
		if constraint == "" || pqErr.Constraint == constraint {
			return true
		}
	}
	return false
}
