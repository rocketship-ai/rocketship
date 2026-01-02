package orchestrator

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const (
	authorizationHeader = "authorization"
	bearerPrefix        = "Bearer "
)

// authExemptMethods lists RPCs that should always be accessible without authentication.
var authExemptMethods = map[string]struct{}{
	"/rocketship.v1.Engine/Health":        {},
	"/rocketship.v1.Engine/GetServerInfo": {},
}

type permission int

const (
	permNone permission = iota
	permRead
	permWrite
)

func (p permission) String() string {
	switch p {
	case permRead:
		return "read"
	case permWrite:
		return "write"
	default:
		return "unknown"
	}
}

var methodPermissions = map[string]permission{
	"/rocketship.v1.Engine/CreateRun":     permWrite,
	"/rocketship.v1.Engine/AddLog":        permWrite,
	"/rocketship.v1.Engine/CancelRun":     permWrite,
	"/rocketship.v1.Engine/UpsertRunStep": permWrite,
	"/rocketship.v1.Engine/ListRuns":      permRead,
	"/rocketship.v1.Engine/GetRun":        permRead,
	"/rocketship.v1.Engine/StreamLogs":    permRead,
}

type principalContextKey struct{}

// CITokenProjectScope represents a project-scope pair for CI token authorization
type CITokenProjectScope struct {
	ProjectID uuid.UUID
	Scope     string // "read" or "write"
}

// Principal represents the authenticated caller derived from the bearer token.
type Principal struct {
	Subject  string
	Email    string
	Name     string
	Username string
	Roles    []string
	Scopes   []string
	TokenID  string
	OrgID    string
	// CI Token specific fields
	IsCIToken       bool
	CITokenID       uuid.UUID
	AllowedProjects []CITokenProjectScope // Projects this CI token has access to
}

func (p *Principal) allows(perm permission) bool {
	if len(p.Roles) == 0 {
		return perm == permNone
	}
	for _, role := range p.Roles {
		switch strings.TrimSpace(strings.ToLower(role)) {
		case "owner", "admin", "editor", "service_account":
			return true
		case "viewer":
			if perm == permRead {
				return true
			}
		}
	}
	return perm == permNone
}

func (p *Principal) denialMessage(required permission) string {
	roles := strings.Join(p.Roles, ", ")
	if roles == "" {
		roles = "none"
	}
	return fmt.Sprintf("requires %s access (roles: %s)", required.String(), roles)
}

// isServiceAccount returns true if the principal represents a service account.
// Service accounts have either "service_account" role or a subject starting with "service:".
func (p *Principal) isServiceAccount() bool {
	if p == nil {
		return false
	}
	if strings.HasPrefix(p.Subject, "service:") {
		return true
	}
	for _, role := range p.Roles {
		if strings.TrimSpace(strings.ToLower(role)) == "service_account" {
			return true
		}
	}
	return false
}

// HasProjectAccess checks if a CI token principal has access to a specific project with the required permission
func (p *Principal) HasProjectAccess(projectID uuid.UUID, required permission) bool {
	if p == nil || !p.IsCIToken {
		return true // Non-CI token principals are not restricted by project
	}
	for _, ps := range p.AllowedProjects {
		if ps.ProjectID == projectID {
			if required == permWrite {
				return ps.Scope == "write"
			}
			return true // read access: any scope is sufficient
		}
	}
	return false
}

// GetAllowedProjectIDs returns a list of project IDs this CI token can access
func (p *Principal) GetAllowedProjectIDs() []uuid.UUID {
	if p == nil || !p.IsCIToken {
		return nil
	}
	ids := make([]uuid.UUID, len(p.AllowedProjects))
	for i, ps := range p.AllowedProjects {
		ids[i] = ps.ProjectID
	}
	return ids
}

func contextWithPrincipal(ctx context.Context, p *Principal) context.Context {
	if p == nil {
		return ctx
	}
	return context.WithValue(ctx, principalContextKey{}, p)
}

// PrincipalFromContext extracts the authenticated principal from context when available.
func PrincipalFromContext(ctx context.Context) (*Principal, bool) {
	if ctx == nil {
		return nil, false
	}
	p, ok := ctx.Value(principalContextKey{}).(*Principal)
	return p, ok
}

func (e *Engine) NewAuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		authCtx, err := e.authorize(ctx, info.FullMethod)
		if err != nil {
			return nil, err
		}
		return handler(authCtx, req)
	}
}

func (e *Engine) NewAuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		authCtx, err := e.authorize(stream.Context(), info.FullMethod)
		if err != nil {
			return err
		}
		wrapped := &contextServerStream{ServerStream: stream, ctx: authCtx}
		return handler(srv, wrapped)
	}
}

type contextServerStream struct {
	grpc.ServerStream
	ctx context.Context
}

func (s *contextServerStream) Context() context.Context {
	return s.ctx
}

func (e *Engine) authorize(ctx context.Context, fullMethod string) (context.Context, error) {
	if e.authConfig.Disabled() {
		return ctx, nil
	}
	if _, exempt := authExemptMethods[fullMethod]; exempt {
		return ctx, nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get(authorizationHeader)
	if len(tokens) == 0 {
		return nil, status.Error(codes.Unauthenticated, "missing authorization token")
	}

	header := strings.TrimSpace(tokens[0])
	if header == "" {
		return nil, status.Error(codes.Unauthenticated, "empty authorization header")
	}

	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return nil, status.Error(codes.Unauthenticated, "invalid authorization header")
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return nil, status.Error(codes.Unauthenticated, "empty bearer token")
	}

	var principal *Principal
	var err error

	// Check if this is a CI token (starts with "rs_ci_")
	if strings.HasPrefix(token, "rs_ci_") {
		principal, err = e.validateCIToken(ctx, token)
	} else {
		// Standard auth (JWT/OIDC/Token)
		principal, err = e.authConfig.Validate(ctx, token)
	}

	if err != nil {
		return nil, err
	}
	if principal == nil {
		principal = &Principal{}
	}

	required, ok := methodPermissions[fullMethod]
	if !ok {
		return nil, status.Error(codes.PermissionDenied, "method not authorised")
	}
	if !principal.allows(required) {
		return nil, status.Error(codes.PermissionDenied, principal.denialMessage(required))
	}

	return contextWithPrincipal(ctx, principal), nil
}

// validateCIToken validates a CI token against the database
func (e *Engine) validateCIToken(ctx context.Context, token string) (*Principal, error) {
	if e.runStore == nil {
		return nil, status.Error(codes.Unauthenticated, "CI token authentication not available")
	}

	result, err := e.runStore.FindCITokenByPlaintext(ctx, token)
	if err != nil {
		return nil, status.Error(codes.Internal, "failed to validate CI token")
	}
	if result == nil {
		return nil, status.Error(codes.Unauthenticated, "invalid CI token")
	}

	// Check if token is revoked
	if result.IsRevoked {
		return nil, status.Error(codes.Unauthenticated, "CI token has been revoked")
	}

	// Check if token is expired
	if result.IsExpired {
		return nil, status.Error(codes.Unauthenticated, "CI token has expired")
	}

	// Update last_used_at (fire and forget - don't fail auth if this fails)
	go func() {
		_ = e.runStore.UpdateCITokenLastUsed(context.Background(), result.TokenID)
	}()

	// Build allowed projects list
	allowedProjects := make([]CITokenProjectScope, len(result.Projects))
	for i, p := range result.Projects {
		allowedProjects[i] = CITokenProjectScope{
			ProjectID: p.ProjectID,
			Scope:     p.Scope,
		}
	}

	// Determine roles based on scopes
	// If any project has write scope, grant editor role; otherwise viewer
	hasWrite := false
	for _, p := range result.Projects {
		if p.Scope == "write" {
			hasWrite = true
			break
		}
	}
	var roles []string
	if hasWrite {
		roles = []string{"editor"}
	} else {
		roles = []string{"viewer"}
	}

	return &Principal{
		Subject:         fmt.Sprintf("ci_token:%s", result.TokenID.String()),
		OrgID:           result.OrgID.String(),
		Roles:           roles,
		IsCIToken:       true,
		CITokenID:       result.TokenID,
		AllowedProjects: allowedProjects,
	}, nil
}

type authMode int

const (
	authModeNone authMode = iota
	authModeToken
	authModeOIDC
)

// authConfig encapsulates authentication state for the engine.
type authConfig struct {
	mode authMode

	// token mode
	token string

	// oidc mode
	oidc *oidcProvider
}

func (a authConfig) Disabled() bool {
	return a.mode == authModeNone
}

func (a authConfig) Type() string {
	switch a.mode {
	case authModeToken:
		return "token"
	case authModeOIDC:
		return "oidc"
	default:
		return "none"
	}
}

func (a authConfig) Validate(ctx context.Context, bearer string) (*Principal, error) {
	switch a.mode {
	case authModeToken:
		if bearer == a.token {
			return &Principal{
				Subject: "token",
				Roles:   []string{"owner"},
			}, nil
		}
		return nil, status.Error(codes.PermissionDenied, "invalid token")
	case authModeOIDC:
		if a.oidc == nil {
			return nil, status.Error(codes.Internal, "oidc verifier misconfigured")
		}
		return a.oidc.Validate(ctx, bearer)
	default:
		return &Principal{Roles: []string{"owner"}}, nil
	}
}

func (a authConfig) configureServerInfo(resp *generated.GetServerInfoResponse) {
	if resp == nil {
		return
	}
	if a.mode == authModeNone {
		if resp.AuthType == "" {
			resp.AuthType = "none"
		}
		resp.AuthEnabled = false
		return
	}

	resp.AuthEnabled = true
	resp.AuthType = a.Type()

	switch a.mode {
	case authModeToken:
		if !containsString(resp.Capabilities, "auth.token") {
			resp.Capabilities = append(resp.Capabilities, "auth.token")
		}
	case authModeOIDC:
		if !containsString(resp.Capabilities, "auth.oidc") {
			resp.Capabilities = append(resp.Capabilities, "auth.oidc")
		}
		if a.oidc != nil {
			resp.DeviceAuthorizationEndpoint = a.oidc.DeviceEndpoint
			resp.TokenEndpoint = a.oidc.TokenEndpoint
			resp.Issuer = a.oidc.Issuer
			resp.Audience = a.oidc.Audience
			resp.ClientId = a.oidc.ClientID
			resp.Scopes = append([]string(nil), a.oidc.Scopes...)
			if resp.AuthEndpoint == "" {
				resp.AuthEndpoint = a.oidc.DeviceEndpoint
			}
		}
	}
}

func (e *Engine) ConfigureToken(token string) {
	trimmed := strings.TrimSpace(token)
	if trimmed == "" {
		e.authConfig = authConfig{mode: authModeNone}
		return
	}
	e.authConfig = authConfig{
		mode:  authModeToken,
		token: trimmed,
	}
}

func (e *Engine) ConfigureOIDC(ctx context.Context, settings OIDCSettings) error {
	provider, err := newOIDCProvider(ctx, settings)
	if err != nil {
		return err
	}
	e.authConfig = authConfig{
		mode: authModeOIDC,
		oidc: provider,
	}
	return nil
}

func (e *Engine) AuthMode() string {
	return e.authConfig.Type()
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}

// resolvePrincipalAndOrg extracts the principal and organization ID from context
// This is used by multiple service handlers (runs, streams, health) for consistent auth handling
func (e *Engine) resolvePrincipalAndOrg(ctx context.Context) (*Principal, uuid.UUID, error) {
	principal, ok := PrincipalFromContext(ctx)
	if e.authConfig.mode == authModeNone {
		return principal, uuid.Nil, nil
	}
	if !ok {
		return nil, uuid.Nil, fmt.Errorf("missing authentication context")
	}

	orgIDStr := strings.TrimSpace(principal.OrgID)
	if orgIDStr == "" {
		if !e.requireOrgScope {
			return principal, uuid.Nil, nil
		}
		return nil, uuid.Nil, fmt.Errorf("token missing organization scope")
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid organization identifier: %w", err)
	}
	return principal, orgID, nil
}

// resolvePrincipalAndOrgForInternalCallbacks is similar to resolvePrincipalAndOrg but allows
// service account tokens to omit org scope. This is used for internal worker callbacks (AddLog,
// UpsertRunStep) where the worker has a service token but no org_id claim.
//
// - If principal has OrgID: parse and return it (same as resolvePrincipalAndOrg).
// - If principal has no OrgID AND is a service account: return (principal, uuid.Nil, nil).
// - If principal has no OrgID AND is NOT a service account: enforce requireOrgScope as usual.
func (e *Engine) resolvePrincipalAndOrgForInternalCallbacks(ctx context.Context) (*Principal, uuid.UUID, error) {
	principal, ok := PrincipalFromContext(ctx)
	if e.authConfig.mode == authModeNone {
		return principal, uuid.Nil, nil
	}
	if !ok {
		return nil, uuid.Nil, fmt.Errorf("missing authentication context")
	}

	orgIDStr := strings.TrimSpace(principal.OrgID)
	if orgIDStr == "" {
		// Service accounts may omit org scope for internal callbacks
		if principal.isServiceAccount() {
			return principal, uuid.Nil, nil
		}
		if !e.requireOrgScope {
			return principal, uuid.Nil, nil
		}
		return nil, uuid.Nil, fmt.Errorf("token missing organization scope")
	}

	orgID, err := uuid.Parse(orgIDStr)
	if err != nil {
		return nil, uuid.Nil, fmt.Errorf("invalid organization identifier: %w", err)
	}
	return principal, orgID, nil
}
