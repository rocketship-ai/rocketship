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
	SourceRef      string // Branch/ref this project was discovered from
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
	SkippedTests   int            `db:"skipped_tests"`
	EnvironmentID  uuid.NullUUID  `db:"environment_id"`
	ScheduleID     uuid.NullUUID  `db:"schedule_id"`
	CommitMessage  sql.NullString `db:"commit_message"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
	StartedAt      sql.NullTime   `db:"started_at"`
	EndedAt        sql.NullTime   `db:"ended_at"`
}

// ProjectEnvironment represents a deployment environment for a project
type ProjectEnvironment struct {
	ID          uuid.UUID              `db:"id"`
	ProjectID   uuid.UUID              `db:"project_id"`
	Name        string                 `db:"name"`
	Slug        string                 `db:"slug"`
	Description sql.NullString         `db:"description"`
	EnvSecrets  map[string]string      `db:"-"` // Parsed from JSONB - accessed via {{ .env.* }}
	ConfigVars  map[string]interface{} `db:"-"` // Parsed from JSONB - accessed via {{ .vars.* }}
	CreatedAt   time.Time              `db:"created_at"`
	UpdatedAt   time.Time              `db:"updated_at"`
}

// ProjectEnvironmentSelection tracks which environment a user has selected for a project
// This is used by the UI to remember the user's preferred environment
type ProjectEnvironmentSelection struct {
	UserID        uuid.UUID `db:"user_id"`
	ProjectID     uuid.UUID `db:"project_id"`
	EnvironmentID uuid.UUID `db:"environment_id"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

// Suite represents a test suite definition
type Suite struct {
	ID            uuid.UUID      `db:"id"`
	ProjectID     uuid.UUID      `db:"project_id"`
	Name          string         `db:"name"`
	Description   sql.NullString `db:"description"`
	FilePath      sql.NullString `db:"file_path"`
	SourceRef     string         `db:"source_ref"` // Branch/ref this suite was discovered from
	TestCount     int            `db:"test_count"`
	LastRunID     sql.NullString `db:"last_run_id"`
	LastRunStatus sql.NullString `db:"last_run_status"`
	LastRunAt     sql.NullTime   `db:"last_run_at"`
	CreatedAt     time.Time      `db:"created_at"`
	UpdatedAt     time.Time      `db:"updated_at"`
}

// StepSummary represents a step descriptor for UI display
// This is stored in the tests.step_summaries JSONB column
type StepSummary struct {
	StepIndex int    `json:"step_index"`
	Plugin    string `json:"plugin"`
	Name      string `json:"name"`
}

// Test represents an individual test definition
type Test struct {
	ID            uuid.UUID       `db:"id"`
	SuiteID       uuid.UUID       `db:"suite_id"`
	ProjectID     uuid.UUID       `db:"project_id"`
	Name          string          `db:"name"`
	Description   sql.NullString  `db:"description"`
	SourceRef     string          `db:"source_ref"` // Branch/ref this test was discovered from
	StepCount     int             `db:"step_count"`
	StepSummaries []StepSummary   `db:"-"` // Parsed from JSONB
	LastRunID     sql.NullString  `db:"last_run_id"`
	LastRunStatus sql.NullString  `db:"last_run_status"`
	LastRunAt     sql.NullTime    `db:"last_run_at"`
	PassRate      sql.NullFloat64 `db:"pass_rate"`
	AvgDurationMs sql.NullInt64   `db:"avg_duration_ms"`
	CreatedAt     time.Time       `db:"created_at"`
	UpdatedAt     time.Time       `db:"updated_at"`
}

// SuiteSchedule represents a scheduled test run
type SuiteSchedule struct {
	ID             uuid.UUID      `db:"id"`
	SuiteID        uuid.UUID      `db:"suite_id"`
	ProjectID      uuid.UUID      `db:"project_id"`
	Name           string         `db:"name"`
	Description    sql.NullString `db:"description"`
	CronExpression string         `db:"cron_expression"`
	Timezone       string         `db:"timezone"`
	Enabled        bool           `db:"enabled"`
	EnvironmentID  uuid.NullUUID  `db:"environment_id"`
	NextRunAt      sql.NullTime   `db:"next_run_at"`
	LastRunAt      sql.NullTime   `db:"last_run_at"`
	LastRunID      sql.NullString `db:"last_run_id"`
	LastRunStatus  sql.NullString `db:"last_run_status"`
	CreatedBy      uuid.NullUUID  `db:"created_by"`
	CreatedAt      time.Time      `db:"created_at"`
	UpdatedAt      time.Time      `db:"updated_at"`
}

// RunTest represents an individual test result within a run
type RunTest struct {
	ID           uuid.UUID      `db:"id"`
	RunID        string         `db:"run_id"`
	TestID       uuid.NullUUID  `db:"test_id"`
	WorkflowID   string         `db:"workflow_id"`
	Name         string         `db:"name"`
	Status       string         `db:"status"`
	ErrorMessage sql.NullString `db:"error_message"`
	StartedAt    sql.NullTime   `db:"started_at"`
	EndedAt      sql.NullTime   `db:"ended_at"`
	DurationMs   sql.NullInt64  `db:"duration_ms"`
	StepCount    int            `db:"step_count"`
	PassedSteps  int            `db:"passed_steps"`
	FailedSteps  int            `db:"failed_steps"`
	CreatedAt    time.Time      `db:"created_at"`
}

// AssertionResult represents a single assertion result for UI display
type AssertionResult struct {
	Type     string      `json:"type"`               // status_code, json_path, header
	Name     string      `json:"name,omitempty"`     // Header name for header assertions
	Path     string      `json:"path,omitempty"`     // JSONPath/jq expression for json_path assertions
	Expected interface{} `json:"expected,omitempty"` // Expected value
	Actual   interface{} `json:"actual,omitempty"`   // Actual value received
	Passed   bool        `json:"passed"`             // Whether the assertion passed
	Message  string      `json:"message,omitempty"`  // Error message if failed
}

// SavedVariable represents a variable saved from a step
type SavedVariable struct {
	Name       string `json:"name"`                  // Variable name (the "as" field)
	Value      string `json:"value"`                 // Saved value
	SourceType string `json:"source_type,omitempty"` // "json_path", "header", or "auto"
	Source     string `json:"source,omitempty"`      // The expression used (json_path or header name)
}

// RunStep represents a step result within a test run
type RunStep struct {
	ID               uuid.UUID              `db:"id"`
	RunTestID        uuid.UUID              `db:"run_test_id"`
	StepIndex        int                    `db:"step_index"`
	Name             string                 `db:"name"`
	Plugin           string                 `db:"plugin"`
	Status           string                 `db:"status"`
	ErrorMessage     sql.NullString         `db:"error_message"`
	RequestData      map[string]interface{} `db:"-"` // Parsed from JSONB
	ResponseData     map[string]interface{} `db:"-"` // Parsed from JSONB
	AssertionsData   []AssertionResult      `db:"-"` // Parsed from JSONB
	VariablesData    []SavedVariable        `db:"-"` // Parsed from JSONB
	StepConfig       map[string]interface{} `db:"-"` // Parsed from JSONB
	AssertionsPassed int                    `db:"assertions_passed"`
	AssertionsFailed int                    `db:"assertions_failed"`
	StartedAt        sql.NullTime           `db:"started_at"`
	EndedAt          sql.NullTime           `db:"ended_at"`
	DurationMs       sql.NullInt64          `db:"duration_ms"`
	CreatedAt        time.Time              `db:"created_at"`
}

// RunLog represents an execution log entry
type RunLog struct {
	ID        uuid.UUID              `db:"id"`
	RunID     string                 `db:"run_id"`
	RunTestID uuid.NullUUID          `db:"run_test_id"`
	RunStepID uuid.NullUUID          `db:"run_step_id"`
	Level     string                 `db:"level"`
	Message   string                 `db:"message"`
	Metadata  map[string]interface{} `db:"-"` // Parsed from JSONB
	LoggedAt  time.Time              `db:"logged_at"`
}

// RunArtifact represents a test artifact (screenshot, file, etc.)
type RunArtifact struct {
	ID           uuid.UUID      `db:"id"`
	RunID        string         `db:"run_id"`
	RunTestID    uuid.NullUUID  `db:"run_test_id"`
	RunStepID    uuid.NullUUID  `db:"run_step_id"`
	Name         string         `db:"name"`
	ArtifactType string         `db:"artifact_type"`
	MimeType     sql.NullString `db:"mime_type"`
	SizeBytes    sql.NullInt64  `db:"size_bytes"`
	StoragePath  string         `db:"storage_path"`
	CreatedAt    time.Time      `db:"created_at"`
}

// CITokenRecord represents a CI token with audit fields
type CITokenRecord struct {
	ID           uuid.UUID      `db:"id"`
	ProjectID    uuid.UUID      `db:"project_id"`
	Name         string         `db:"name"`
	TokenHash    string         `db:"token_hash"`
	Scopes       []string       `db:"-"`
	NeverExpires bool           `db:"never_expires"`
	ExpiresAt    sql.NullTime   `db:"expires_at"`
	RevokedAt    sql.NullTime   `db:"revoked_at"`
	CreatedBy    uuid.NullUUID  `db:"created_by"`
	LastUsedAt   sql.NullTime   `db:"last_used_at"`
	RevokedBy    uuid.NullUUID  `db:"revoked_by"`
	Description  sql.NullString `db:"description"`
	CreatedAt    time.Time      `db:"created_at"`
	UpdatedAt    time.Time      `db:"updated_at"`
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
