package controlplane

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// dataStore defines the persistence interface for the auth broker
type dataStore interface {
	UpsertGitHubUser(ctx context.Context, input persistence.GitHubUserInput) (persistence.User, error)
	GetUserByID(ctx context.Context, userID uuid.UUID) (persistence.User, error)
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error
	UpdateUserName(ctx context.Context, userID uuid.UUID, name string) error
	RoleSummary(ctx context.Context, userID uuid.UUID) (persistence.RoleSummary, error)
	SaveRefreshToken(ctx context.Context, token string, rec persistence.RefreshTokenRecord) error
	GetRefreshToken(ctx context.Context, token string) (persistence.RefreshTokenRecord, error)
	DeleteRefreshToken(ctx context.Context, token string) error
	ListProjectMembers(ctx context.Context, projectID uuid.UUID) ([]persistence.ProjectMember, error)
	SetProjectMemberRole(ctx context.Context, projectID, userID uuid.UUID, role string) error
	RemoveProjectMember(ctx context.Context, projectID, userID uuid.UUID) error
	ProjectOrganizationID(ctx context.Context, projectID uuid.UUID) (uuid.UUID, error)
	IsOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) (bool, error)
	DeleteOrgRegistrationsForUser(ctx context.Context, userID uuid.UUID) error
	CreateOrgRegistration(ctx context.Context, rec persistence.OrganizationRegistration) (persistence.OrganizationRegistration, error)
	GetOrgRegistration(ctx context.Context, id uuid.UUID) (persistence.OrganizationRegistration, error)
	LatestOrgRegistrationForUser(ctx context.Context, userID uuid.UUID) (persistence.OrganizationRegistration, error)
	UpdateOrgRegistrationForResend(ctx context.Context, id uuid.UUID, hash, salt []byte, expiresAt, resendAt time.Time) (persistence.OrganizationRegistration, error)
	IncrementOrgRegistrationAttempts(ctx context.Context, id uuid.UUID) error
	DeleteOrgRegistration(ctx context.Context, id uuid.UUID) error
	CreateOrganization(ctx context.Context, userID uuid.UUID, name, slug string) (persistence.Organization, error)
	OrganizationSlugExists(ctx context.Context, slug string) (bool, error)
	AddOrganizationAdmin(ctx context.Context, orgID, userID uuid.UUID) error
	CreateOrgInvite(ctx context.Context, invite persistence.OrganizationInvite) (persistence.OrganizationInvite, error)
	FindPendingOrgInvites(ctx context.Context, email string) ([]persistence.OrganizationInvite, error)
	MarkOrgInviteAccepted(ctx context.Context, inviteID, userID uuid.UUID) error

	// GitHub App installation management
	UpsertGitHubAppInstallation(ctx context.Context, orgID uuid.UUID, installationID int64, installedBy uuid.UUID, accountLogin, accountType string) error
	GetGitHubAppInstallation(ctx context.Context, orgID uuid.UUID) (installationID int64, accountLogin, accountType string, err error)

	// GitHub webhook delivery tracking
	InsertWebhookDelivery(ctx context.Context, deliveryID, event, repoFullName, ref, action string) error

	// GitHub App installation lookup for multi-tenant safety
	ListOrgsByInstallationID(ctx context.Context, installationID int64) ([]uuid.UUID, error)

	// Scan attempt tracking
	InsertScanAttempt(ctx context.Context, attempt persistence.ScanAttempt) error

	// Project management for onboarding
	CreateProject(ctx context.Context, project persistence.Project) (persistence.Project, error)
	GetProject(ctx context.Context, projectID uuid.UUID) (persistence.Project, error)
	ListProjects(ctx context.Context, orgID uuid.UUID) ([]persistence.Project, error)
	ProjectNameExists(ctx context.Context, orgID uuid.UUID, name, sourceRef string) (bool, error)

	// Environment management
	CreateEnvironment(ctx context.Context, env persistence.ProjectEnvironment) (persistence.ProjectEnvironment, error)
	GetEnvironment(ctx context.Context, projectID, envID uuid.UUID) (persistence.ProjectEnvironment, error)
	GetEnvironmentBySlug(ctx context.Context, projectID uuid.UUID, slug string) (persistence.ProjectEnvironment, error)
	ListEnvironments(ctx context.Context, projectID uuid.UUID) ([]persistence.ProjectEnvironment, error)
	UpdateEnvironment(ctx context.Context, env persistence.ProjectEnvironment) (persistence.ProjectEnvironment, error)
	DeleteEnvironment(ctx context.Context, projectID, envID uuid.UUID) error
	// Environment selection (per-user, per-project)
	GetSelectedEnvironment(ctx context.Context, userID, projectID uuid.UUID) (persistence.ProjectEnvironment, bool, error)
	SetSelectedEnvironment(ctx context.Context, userID, projectID, environmentID uuid.UUID) error

	// Suite and test management
	UpsertSuite(ctx context.Context, suite persistence.Suite) (persistence.Suite, error)
	GetSuiteByName(ctx context.Context, projectID uuid.UUID, name, sourceRef string) (persistence.Suite, bool, error)
	ListSuites(ctx context.Context, projectID uuid.UUID) ([]persistence.Suite, error)
	ListSuitesForProjectCanonical(ctx context.Context, projectID uuid.UUID) ([]persistence.CanonicalSuiteRow, error)
	UpsertTest(ctx context.Context, test persistence.Test) (persistence.Test, error)

	// Schedule management
	ListEnabledSchedulesByProject(ctx context.Context, projectID uuid.UUID) ([]persistence.SuiteSchedule, error)

	// CI Token management
	ListActiveCITokens(ctx context.Context, projectID uuid.UUID) ([]persistence.CITokenRecord, error)

	// Overview setup counts
	CountProjectsForOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	CountSuitesForOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	CountSuitesOnDefaultBranchForOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	CountEnvsWithVarsForOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	CountEnabledSchedulesForOrg(ctx context.Context, orgID uuid.UUID) (int, error)
	CountActiveCITokensForOrg(ctx context.Context, orgID uuid.UUID) (int, error)

	// Console hydration queries
	ListProjectSummariesForOrg(ctx context.Context, orgID uuid.UUID) ([]persistence.ProjectSummary, error)
	GetProjectWithOrgCheck(ctx context.Context, orgID, projectID uuid.UUID) (persistence.Project, error)
	GetLatestScanForProject(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef string) (*persistence.ScanSummary, error)
	ListSuitesForOrg(ctx context.Context, orgID uuid.UUID, limit int) ([]persistence.SuiteActivityRow, error)
	GetSuiteDetail(ctx context.Context, orgID, suiteID uuid.UUID) (persistence.SuiteDetail, error)
	ListTestsBySuite(ctx context.Context, suiteID uuid.UUID) ([]persistence.Test, error)

	// Profile hydration queries
	GetOrganizationByID(ctx context.Context, orgID uuid.UUID) (persistence.Organization, error)
	ListProjectPermissionsForUser(ctx context.Context, orgID, userID uuid.UUID) ([]persistence.ProjectPermissionRow, error)

	// Suite run activity queries
	ListProjectIDsByRepoAndPathScope(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) ([]uuid.UUID, error)
	ListRunsForSuiteGroup(ctx context.Context, orgID uuid.UUID, projectIDs []uuid.UUID, suiteName, defaultBranch string, runsPerBranch int) ([]persistence.SuiteRunRow, error)

	// Run detail queries
	GetRun(ctx context.Context, orgID uuid.UUID, runID string) (persistence.RunRecord, error)
	ListRunTests(ctx context.Context, runID string) ([]persistence.RunTest, error)
	ListRunLogs(ctx context.Context, runID string, limit int) ([]persistence.RunLog, error)
	GetRunTestWithRun(ctx context.Context, orgID uuid.UUID, runTestID uuid.UUID) (persistence.RunTestWithRun, error)
	ListRunLogsByTest(ctx context.Context, runTestID uuid.UUID, limit int) ([]persistence.RunLog, error)
	ListRunSteps(ctx context.Context, runTestID uuid.UUID) ([]persistence.RunStep, error)

	// Project lifecycle management (PR close/reopen)
	DeactivateProjectsForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef, reason string) (int, error)
	ReactivateProjectsForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef string) (int, error)

	// Suite lifecycle management (PR close)
	DeactivateSuitesForRepoAndSourceRef(ctx context.Context, orgID uuid.UUID, repoURL, sourceRef, reason string) (int, error)

	// Project lookup for PR delta scanning
	FindDefaultBranchProject(ctx context.Context, orgID uuid.UUID, repoURL string, pathScope []string) (persistence.Project, bool, error)

	// Suite lifecycle management for PR delta scanning
	DeactivateSuiteByProjectRefAndFilePath(ctx context.Context, projectID uuid.UUID, sourceRef, filePath, reason string) error

	// Reconciliation for full scans (deactivate missing suites/tests)
	DeactivateSuitesMissingFromDir(ctx context.Context, projectID uuid.UUID, sourceRef, rocketshipDir string, presentFilePaths []string, reason string) (int, error)
	DeactivateTestsMissingFromSuite(ctx context.Context, suiteID uuid.UUID, sourceRef string, presentTestNames []string, reason string) (int, error)
}

// githubProvider defines the interface for GitHub OAuth operations (identity only)
type githubProvider interface {
	RequestDeviceCode(ctx context.Context, scopes []string) (DeviceCodeResponse, error)
	ExchangeDeviceCode(ctx context.Context, deviceCode string) (TokenResponse, tokenError, error)
	ExchangeAuthorizationCode(ctx context.Context, code, redirectURI, codeVerifier string) (TokenResponse, error)
	FetchUser(ctx context.Context, accessToken string) (GitHubUser, error)
}

// Note: mailer interface is defined in postmark.go

// brokerPrincipal represents an authenticated user with their roles and metadata
type brokerPrincipal struct {
	UserID   uuid.UUID
	OrgID    uuid.UUID
	Roles    []string
	Email    string
	Name     string
	Username string
}

// HasRole checks if the principal has a specific role (case-insensitive)
func (p brokerPrincipal) HasRole(role string) bool {
	return containsRole(p.Roles, role)
}

// HasAnyRole checks if the principal has any of the specified roles
func (p brokerPrincipal) HasAnyRole(roles ...string) bool {
	for _, role := range roles {
		if p.HasRole(role) {
			return true
		}
	}
	return false
}

// IsPending returns true if the user has only the "pending" role (no org membership)
func (p brokerPrincipal) IsPending() bool {
	if len(p.Roles) == 1 && containsRole(p.Roles, "pending") {
		return true
	}
	return false
}

// RequiresOrgMembership returns true if the user needs an org to use console features
func (p brokerPrincipal) RequiresOrgMembership() bool {
	return p.IsPending() || p.OrgID == uuid.Nil
}

// deviceSession tracks a pending device flow authorization
type deviceSession struct {
	clientID  string
	scopes    []string
	expiresAt time.Time
}

// authSession tracks a pending web authorization flow with PKCE
type authSession struct {
	state         string
	codeChallenge string
	redirectURI   string
	scopes        []string
	expiresAt     time.Time
}

// oauthTokenResponse is the OAuth 2.0 token response format
type oauthTokenResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token,omitempty"`
	Scope        string `json:"scope,omitempty"`
	IDToken      string `json:"id_token,omitempty"`
}
