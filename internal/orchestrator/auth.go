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

// NewAuthUnaryInterceptor returns a unary interceptor that enforces token authentication
// on all RPCs except those listed in authExemptMethods.

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

	if !e.authConfig.Validate(token) {
		return status.Error(codes.PermissionDenied, "invalid token")
	}

	return nil
}

// authConfig encapsulates token authentication state.
type authConfig struct {
	enabled bool
	token   string
}

func (a authConfig) Disabled() bool { return !a.enabled }

// Validate returns true if the provided token matches the configured secret.
func (a authConfig) Validate(token string) bool {
	return a.enabled && token == a.token
}

// configureServerInfo mutates the GetServerInfo response to reflect auth configuration.
func (a authConfig) configureServerInfo(resp *generated.GetServerInfoResponse) {
	if resp == nil {
		return
	}
	resp.AuthEnabled = a.enabled
	if !a.enabled {
		if resp.AuthType == "" {
			resp.AuthType = "none"
		}
		return
	}
	resp.AuthType = "token"
	if !containsString(resp.Capabilities, "auth.token") {
		resp.Capabilities = append(resp.Capabilities, "auth.token")
	}
}

// ConfigureToken enables token authentication for the engine.
func (e *Engine) ConfigureToken(token string) {
	trimmed := strings.TrimSpace(token)
	e.authConfig = authConfig{
		enabled: trimmed != "",
		token:   trimmed,
	}
}

// MustConfigureToken reads configuration and panics if the supplied token is invalid.
// Exported for tests and bootstrap wiring.
func (e *Engine) MustConfigureToken(token string) {
	trimmed := strings.TrimSpace(token)
	if token != "" && trimmed == "" {
		panic(fmt.Sprintf("invalid token (blank): %q", token))
	}
	e.ConfigureToken(trimmed)
}

// TokenAuthEnabled reports whether token authentication is currently enabled.
func (e *Engine) TokenAuthEnabled() bool {
	return !e.authConfig.Disabled()
}

func containsString(values []string, needle string) bool {
	for _, v := range values {
		if v == needle {
			return true
		}
	}
	return false
}
