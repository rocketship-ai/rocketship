package orchestrator

import (
	"context"
	"fmt"
	"strings"

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
	"/rocketship.v1.Engine/CreateRun":  permWrite,
	"/rocketship.v1.Engine/AddLog":     permWrite,
	"/rocketship.v1.Engine/CancelRun":  permWrite,
	"/rocketship.v1.Engine/ListRuns":   permRead,
	"/rocketship.v1.Engine/GetRun":     permRead,
	"/rocketship.v1.Engine/StreamLogs": permRead,
}

type principalContextKey struct{}

// Principal represents the authenticated caller derived from the bearer token.
type Principal struct {
	Subject  string
	Email    string
	Name     string
	Username string
	Roles    []string
	Scopes   []string
	TokenID  string
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

	principal, err := e.authConfig.Validate(ctx, token)
	if err != nil {
		return nil, err
	}
	if principal == nil {
		principal = &Principal{}
	}

	required := methodPermissions[fullMethod]
	if !principal.allows(required) {
		return nil, status.Error(codes.PermissionDenied, principal.denialMessage(required))
	}

	return contextWithPrincipal(ctx, principal), nil
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
