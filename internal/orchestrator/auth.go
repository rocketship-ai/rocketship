package orchestrator

import (
	"context"
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

func (e *Engine) NewAuthUnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		if err := e.authorize(ctx, info.FullMethod); err != nil {
			return nil, err
		}
		return handler(ctx, req)
	}
}

func (e *Engine) NewAuthStreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		if err := e.authorize(stream.Context(), info.FullMethod); err != nil {
			return err
		}
		return handler(srv, stream)
	}
}

func (e *Engine) authorize(ctx context.Context, fullMethod string) error {
	if e.authConfig.Disabled() {
		return nil
	}
	if _, exempt := authExemptMethods[fullMethod]; exempt {
		return nil
	}

	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return status.Error(codes.Unauthenticated, "missing metadata")
	}

	tokens := md.Get(authorizationHeader)
	if len(tokens) == 0 {
		return status.Error(codes.Unauthenticated, "missing authorization token")
	}

	header := strings.TrimSpace(tokens[0])
	if header == "" {
		return status.Error(codes.Unauthenticated, "empty authorization header")
	}

	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return status.Error(codes.Unauthenticated, "invalid authorization header")
	}

	token := strings.TrimSpace(header[len(bearerPrefix):])
	if token == "" {
		return status.Error(codes.Unauthenticated, "empty bearer token")
	}

	return e.authConfig.Validate(ctx, token)
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

func (a authConfig) Validate(ctx context.Context, bearer string) error {
	switch a.mode {
	case authModeToken:
		if bearer == a.token {
			return nil
		}
		return status.Error(codes.PermissionDenied, "invalid token")
	case authModeOIDC:
		if a.oidc == nil {
			return status.Error(codes.Internal, "oidc verifier misconfigured")
		}
		return a.oidc.Validate(ctx, bearer)
	default:
		return nil
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
