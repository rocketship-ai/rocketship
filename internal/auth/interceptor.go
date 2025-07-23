package auth

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

	"github.com/rocketship-ai/rocketship/internal/rbac"
	"github.com/rocketship-ai/rocketship/internal/tokens"
)

// contextKey is a custom type for context keys to avoid collisions
type contextKey string

const authContextKey contextKey = "auth_context"

// AuthInterceptor handles authentication for gRPC requests
type AuthInterceptor struct {
	authManager   *Manager
	tokenManager  *tokens.Manager
	rbacRepo      *rbac.Repository
	authRequired  bool // Whether authentication is required
}

// NewAuthInterceptor creates a new authentication interceptor
func NewAuthInterceptor(authManager *Manager, tokenManager *tokens.Manager, rbacRepo *rbac.Repository) *AuthInterceptor {
	// Check if authentication is configured
	authRequired := isAuthConfigured()
	
	return &AuthInterceptor{
		authManager:  authManager,
		tokenManager: tokenManager,
		rbacRepo:     rbacRepo,
		authRequired: authRequired,
	}
}

// UnaryInterceptor returns a gRPC unary interceptor for authentication
func (a *AuthInterceptor) UnaryInterceptor() grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
		// Skip authentication for health checks and other public endpoints
		if isPublicEndpoint(info.FullMethod) {
			return handler(ctx, req)
		}

		// If authentication is not required (not configured), allow all requests
		if !a.authRequired {
			return handler(ctx, req)
		}

		// Extract and validate authentication
		authCtx, err := a.extractAuthContext(ctx)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, err.Error())
		}

		// If no auth context but auth is required, reject
		if authCtx == nil {
			return nil, status.Error(codes.Unauthenticated, "authentication required")
		}

		// Add auth context to request context
		ctx = context.WithValue(ctx, authContextKey, authCtx)

		return handler(ctx, req)
	}
}

// StreamInterceptor returns a gRPC stream interceptor for authentication
func (a *AuthInterceptor) StreamInterceptor() grpc.StreamServerInterceptor {
	return func(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {
		// Skip authentication for health checks and other public endpoints
		if isPublicEndpoint(info.FullMethod) {
			return handler(srv, stream)
		}

		// If authentication is not required (not configured), allow all requests
		if !a.authRequired {
			return handler(srv, stream)
		}

		// Extract and validate authentication
		authCtx, err := a.extractAuthContext(stream.Context())
		if err != nil {
			return status.Error(codes.Unauthenticated, err.Error())
		}

		// If no auth context but auth is required, reject
		if authCtx == nil {
			return status.Error(codes.Unauthenticated, "authentication required")
		}

		// Create wrapped stream with auth context
		wrappedStream := &authContextStream{
			ServerStream: stream,
			authCtx:      authCtx,
		}

		return handler(srv, wrappedStream)
	}
}

// extractAuthContext extracts authentication context from gRPC metadata
func (a *AuthInterceptor) extractAuthContext(ctx context.Context) (*rbac.AuthContext, error) {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok {
		return nil, nil // No metadata, no auth
	}

	// Check for Authorization header
	authHeaders := md.Get("authorization")
	if len(authHeaders) == 0 {
		return nil, nil // No authorization header
	}

	authHeader := authHeaders[0]
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return nil, fmt.Errorf("invalid authorization header format")
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")

	// Try to validate as API token first
	if a.tokenManager != nil {
		authCtx, err := a.tokenManager.ValidateToken(ctx, token)
		if err == nil {
			return authCtx, nil
		}
	}

	// Try to validate as OIDC token
	if a.authManager != nil {
		userInfo, err := a.authManager.client.ValidateToken(ctx, token)
		if err != nil {
			return nil, fmt.Errorf("invalid token: %w", err)
		}

		// Ensure user exists in database and handle initial admin setup
		var dbUser *rbac.User
		if a.rbacRepo != nil {
			if err := a.ensureUserExists(ctx, userInfo); err != nil {
				return nil, fmt.Errorf("failed to ensure user exists: %w", err)
			}
			
			// Get user from database to get authoritative role information
			var err error
			dbUser, err = a.rbacRepo.GetUser(ctx, userInfo.Subject)
			if err != nil {
				return nil, fmt.Errorf("failed to get user from database: %w", err)
			}
		}

		// Get user teams
		var teamMemberships []rbac.TeamMember
		if a.rbacRepo != nil {
			memberships, err := a.rbacRepo.GetUserTeams(ctx, userInfo.Subject)
			if err != nil {
				return nil, fmt.Errorf("failed to get user teams: %w", err)
			}
			teamMemberships = memberships
		}

		// Use database user's OrgRole as source of truth, fall back to userInfo if no database
		orgRole := userInfo.OrgRole
		if dbUser != nil {
			orgRole = dbUser.OrgRole
		}

		// Create auth context
		authCtx := &rbac.AuthContext{
			UserID:          userInfo.Subject,
			Email:           userInfo.Email,
			Name:            userInfo.Name,
			OrgRole:         orgRole,
			TeamMemberships: teamMemberships,
		}

		return authCtx, nil
	}

	return nil, fmt.Errorf("token validation not available")
}

// isPublicEndpoint checks if an endpoint is public (doesn't require authentication)
func isPublicEndpoint(method string) bool {
	publicEndpoints := []string{
		"/grpc.health.v1.Health/Check",
		"/grpc.health.v1.Health/Watch",
		"/rocketship.v1.Engine/GetAuthConfig", // Allow clients to discover auth configuration
	}

	for _, endpoint := range publicEndpoints {
		if method == endpoint {
			return true
		}
	}

	return false
}

// isAuthConfigured checks if authentication is configured
func isAuthConfigured() bool {
	issuer := os.Getenv("ROCKETSHIP_OIDC_ISSUER")
	clientID := os.Getenv("ROCKETSHIP_OIDC_CLIENT_ID")
	dbHost := os.Getenv("ROCKETSHIP_DB_HOST")
	
	// Auth is considered configured if we have OIDC settings and database settings
	return issuer != "" && clientID != "" && dbHost != ""
}

// authContextStream wraps a grpc.ServerStream with authentication context
type authContextStream struct {
	grpc.ServerStream
	authCtx *rbac.AuthContext
}

// Context returns the context with authentication information
func (s *authContextStream) Context() context.Context {
	return context.WithValue(s.ServerStream.Context(), authContextKey, s.authCtx)
}

// GetAuthContext extracts the authentication context from a gRPC context
func GetAuthContext(ctx context.Context) *rbac.AuthContext {
	if authCtx, ok := ctx.Value(authContextKey).(*rbac.AuthContext); ok {
		return authCtx
	}
	return nil
}

// IsAuthenticated checks if the current request is authenticated
func IsAuthenticated(ctx context.Context) bool {
	return GetAuthContext(ctx) != nil
}

// RequireAuth returns an error if the request is not authenticated
func RequireAuth(ctx context.Context) (*rbac.AuthContext, error) {
	authCtx := GetAuthContext(ctx)
	if authCtx == nil {
		return nil, status.Error(codes.Unauthenticated, "authentication required")
	}
	return authCtx, nil
}

// RequireAdmin returns an error if the request is not from an admin user
func RequireAdmin(ctx context.Context) (*rbac.AuthContext, error) {
	authCtx, err := RequireAuth(ctx)
	if err != nil {
		return nil, err
	}
	
	if !authCtx.IsOrgAdmin() {
		return nil, status.Error(codes.PermissionDenied, "admin access required")
	}
	
	return authCtx, nil
}

// ensureUserExists ensures the user exists in the database and handles initial admin setup
func (a *AuthInterceptor) ensureUserExists(ctx context.Context, userInfo *UserInfo) error {
	log.Printf("INTERCEPTOR DEBUG: ensureUserExists called for user %s", userInfo.Subject)
	
	// SECURITY: Determine role on SERVER-SIDE based on server configuration, not client data
	serverDeterminedRole := a.determineUserRole(userInfo.Email)
	log.Printf("INTERCEPTOR DEBUG: Server-side role determination for %s: %s", userInfo.Email, serverDeterminedRole)
	
	// Check if user already exists
	existingUser, err := a.rbacRepo.GetUser(ctx, userInfo.Subject)
	if err == nil && existingUser != nil {
		// User exists, check if role needs updating based on SERVER determination
		if existingUser.OrgRole != serverDeterminedRole {
			log.Printf("INTERCEPTOR DEBUG: User %s exists with role %s, updating to %s", userInfo.Subject, existingUser.OrgRole, serverDeterminedRole)
			existingUser.OrgRole = serverDeterminedRole
			existingUser.Email = userInfo.Email // Update email in case it changed
			existingUser.Name = userInfo.Name   // Update name in case it changed
			if err := a.rbacRepo.UpdateUser(ctx, existingUser); err != nil {
				log.Printf("INTERCEPTOR DEBUG: Failed to update user %s role: %v", userInfo.Subject, err)
				return fmt.Errorf("failed to update user role: %w", err)
			}
			log.Printf("INTERCEPTOR DEBUG: Successfully updated user %s role to %s", userInfo.Subject, serverDeterminedRole)
		} else {
			log.Printf("INTERCEPTOR DEBUG: User %s already exists with correct role %s", userInfo.Subject, existingUser.OrgRole)
		}
		return nil
	}

	log.Printf("INTERCEPTOR DEBUG: User %s doesn't exist, creating with role %s", userInfo.Subject, serverDeterminedRole)

	// User doesn't exist, create them with SERVER-DETERMINED role
	user := &rbac.User{
		ID:      userInfo.Subject,
		Email:   userInfo.Email,
		Name:    userInfo.Name,
		OrgRole: serverDeterminedRole, // Use server-determined role for security
	}

	if err := a.rbacRepo.CreateUser(ctx, user); err != nil {
		log.Printf("INTERCEPTOR DEBUG: Failed to create user %s: %v", userInfo.Subject, err)
		return fmt.Errorf("failed to create user: %w", err)
	}

	log.Printf("INTERCEPTOR DEBUG: Successfully created user %s with role %s", userInfo.Subject, serverDeterminedRole)
	return nil
}

// determineUserRole determines user role based on SERVER-SIDE configuration (SECURITY)
func (a *AuthInterceptor) determineUserRole(userEmail string) rbac.OrganizationRole {
	// Get admin emails from server-side environment variable
	adminEmailsEnv := os.Getenv("ROCKETSHIP_ADMIN_EMAILS")
	if adminEmailsEnv == "" {
		log.Printf("SECURITY DEBUG: No ROCKETSHIP_ADMIN_EMAILS configured, defaulting user %s to org_member", userEmail)
		return rbac.OrgRoleMember
	}
	
	adminEmails := strings.Split(adminEmailsEnv, ",")
	userEmailLower := strings.TrimSpace(strings.ToLower(userEmail))
	
	log.Printf("SECURITY DEBUG: Checking admin emails server-side - AdminEmails=%q, UserEmail=%q", adminEmailsEnv, userEmailLower)
	
	for _, adminEmail := range adminEmails {
		cleanAdminEmail := strings.TrimSpace(strings.ToLower(adminEmail))
		log.Printf("SECURITY DEBUG: Comparing %q == %q", cleanAdminEmail, userEmailLower)
		if cleanAdminEmail == userEmailLower {
			log.Printf("SECURITY DEBUG: User %s determined to be org_admin by SERVER", userEmail)
			return rbac.OrgRoleAdmin
		}
	}
	
	log.Printf("SECURITY DEBUG: User %s determined to be org_member by SERVER", userEmail)
	return rbac.OrgRoleMember
}