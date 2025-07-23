package auth

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Handler manages authentication operations
type Handler struct {
	*handlers.BaseHandler
}

// NewHandler creates a new auth handler
func NewHandler(rbacRepo *rbac.Repository) *Handler {
	return &Handler{
		BaseHandler: handlers.NewBaseHandler(rbacRepo),
	}
}

// GetAuthConfig provides authentication configuration discovery for clients
func (h *Handler) GetAuthConfig(ctx context.Context, req *generated.GetAuthConfigRequest) (*generated.GetAuthConfigResponse, error) {
	// This endpoint is always accessible (no auth required) since clients need it to know HOW to authenticate

	// Check if authentication is configured on the server
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	dbHost := os.Getenv("ROCKETSHIP_DB_HOST")

	authEnabled := issuer != "" && clientID != "" && dbHost != ""

	response := &generated.GetAuthConfigResponse{
		AuthEnabled: authEnabled,
	}

	// Only include OIDC config if authentication is enabled
	if authEnabled {
		response.Oidc = &generated.OIDCConfig{
			Issuer:   issuer,
			ClientId: clientID,
			Scopes:   []string{"openid", "profile", "email"},
		}
	}

	slog.Debug("Auth config requested", "auth_enabled", authEnabled, "issuer", issuer, "client_id", clientID)

	return response, nil
}

// GetCurrentUser returns the current authenticated user's information with server-determined role
func (h *Handler) GetCurrentUser(ctx context.Context, req *generated.GetCurrentUserRequest) (*generated.GetCurrentUserResponse, error) {
	// This endpoint requires authentication - the auth context will be populated by the interceptor
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	slog.Debug("GetCurrentUser called", "user_id", authCtx.UserID, "email", authCtx.Email, "org_role", authCtx.OrgRole)

	// Return user info with SERVER-DETERMINED role
	return &generated.GetCurrentUserResponse{
		UserId:  authCtx.UserID,
		Email:   authCtx.Email,
		Name:    authCtx.Name,
		OrgRole: string(authCtx.OrgRole), // This is the SERVER-DETERMINED role
		Groups:  []string{},              // TODO: Add groups if needed
	}, nil
}