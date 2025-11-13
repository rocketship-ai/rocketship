package authbroker

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
)

// dataStore defines the persistence interface for the auth broker
type dataStore interface {
	UpsertGitHubUser(ctx context.Context, input persistence.GitHubUserInput) (persistence.User, error)
	UpdateUserEmail(ctx context.Context, userID uuid.UUID, email string) error
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
}

// githubProvider defines the interface for GitHub OAuth operations
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
