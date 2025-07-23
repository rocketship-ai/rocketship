package tokens

import (
	"context"
	"fmt"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers"
	"github.com/rocketship-ai/rocketship/internal/rbac"
	"github.com/rocketship-ai/rocketship/internal/tokens"
)

// Handler manages API token operations
type Handler struct {
	*handlers.BaseHandler
}

// NewHandler creates a new token handler
func NewHandler(rbacRepo *rbac.Repository) *Handler {
	return &Handler{
		BaseHandler: handlers.NewBaseHandler(rbacRepo),
	}
}

// CreateToken creates a new API token for a team
func (h *Handler) CreateToken(ctx context.Context, req *generated.CreateTokenRequest) (*generated.CreateTokenResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Get team
	team, err := h.RbacRepo.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return &generated.CreateTokenResponse{
			Success: false,
			Message: fmt.Sprintf("team not found: %s", req.TeamName),
		}, nil
	}

	// Check if user can manage API tokens for this team
	// For now, allow team admins or org admins to create tokens
	canCreateToken := false
	if authCtx.IsOrgAdmin() {
		canCreateToken = true
	} else {
		// Check if user is admin of this specific team
		for _, membership := range authCtx.TeamMemberships {
			if membership.TeamID == team.ID && membership.Role == rbac.RoleAdmin {
				canCreateToken = true
				break
			}
		}
	}
	
	if !canCreateToken {
		return nil, fmt.Errorf("permission denied: only organization admins or team admins can create API tokens")
	}

	// Parse permissions
	var permissions []rbac.Permission
	for _, permStr := range req.Permissions {
		switch permStr {
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		default:
			return &generated.CreateTokenResponse{
				Success: false,
				Message: fmt.Sprintf("invalid permission: %s. Valid permissions: tests:run, repositories:read, repositories:write, repositories:manage", permStr),
			}, nil
		}
	}

	// Parse expiration date
	var expiresAt *time.Time
	if req.ExpiresAt != "" {
		expires, err := time.Parse("2006-01-02", req.ExpiresAt)
		if err != nil {
			return &generated.CreateTokenResponse{
				Success: false,
				Message: fmt.Sprintf("invalid expires date format (use YYYY-MM-DD): %v", err),
			}, nil
		}
		expiresAt = &expires
	}

	// Create token manager
	tokenManager := tokens.NewManager(h.RbacRepo)

	// Create token
	createReq := &tokens.CreateTokenRequest{
		TeamID:      team.ID,
		Name:        req.Name,
		Permissions: permissions,
		ExpiresAt:   expiresAt,
		CreatedBy:   authCtx.UserID,
	}

	// Create token
	resp, err := tokenManager.CreateToken(ctx, createReq)
	if err != nil {
		return nil, fmt.Errorf("failed to create token: %w", err)
	}

	// Format expiration date for response
	expiresStr := ""
	if resp.ExpiresAt != nil {
		expiresStr = resp.ExpiresAt.Format("2006-01-02")
	}

	return &generated.CreateTokenResponse{
		TokenId:     resp.TokenID,
		Token:       resp.Token,
		TeamName:    req.TeamName,
		Permissions: req.Permissions,
		ExpiresAt:   expiresStr,
		Success:     true,
		Message:     fmt.Sprintf("API token '%s' created successfully for team '%s'", req.Name, req.TeamName),
	}, nil
}

// ListTokens lists API tokens for teams the user has access to
func (h *Handler) ListTokens(ctx context.Context, req *generated.ListTokensRequest) (*generated.ListTokensResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create token manager
	tokenManager := tokens.NewManager(h.RbacRepo)

	var allTokens []*generated.ApiToken

	if req.TeamName != "" {
		// List tokens for specific team
		team, err := h.RbacRepo.GetTeamByName(ctx, req.TeamName)
		if err != nil {
			return nil, fmt.Errorf("failed to get team: %w", err)
		}
		if team == nil {
			return nil, fmt.Errorf("team not found: %s", req.TeamName)
		}

		// Check if user has access to this team
		hasAccess := false
		if authCtx.IsOrgAdmin() {
			hasAccess = true
		} else {
			for _, membership := range authCtx.TeamMemberships {
				if membership.TeamID == team.ID {
					hasAccess = true
					break
				}
			}
		}

		if !hasAccess {
			return nil, fmt.Errorf("permission denied: no access to team %s", req.TeamName)
		}

		// Get tokens for this team
		teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to list tokens for team %s: %w", req.TeamName, err)
		}

		// Convert to protobuf format
		for _, token := range teamTokens {
			// Convert permissions to string slice
			permStrs := make([]string, len(token.Permissions))
			for i, perm := range token.Permissions {
				permStrs[i] = string(perm)
			}

			// Format timestamps
			lastUsedStr := ""
			if token.LastUsedAt != nil {
				lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
			}

			expiresStr := ""
			if token.ExpiresAt != nil {
				expiresStr = token.ExpiresAt.Format("2006-01-02")
			}

			allTokens = append(allTokens, &generated.ApiToken{
				Id:          token.ID,
				Name:        token.Name,
				TeamId:      token.TeamID,
				TeamName:    team.Name,
				Permissions: permStrs,
				CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
				LastUsedAt:  lastUsedStr,
				ExpiresAt:   expiresStr,
				CreatedBy:   token.CreatedBy,
			})
		}
	} else {
		// List tokens for all teams the user has access to
		if authCtx.IsOrgAdmin() {
			// Org admins can see all tokens
			teams, err := h.RbacRepo.ListTeams(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list teams: %w", err)
			}

			for _, team := range teams {
				teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
				if err != nil {
					continue // Skip teams with errors
				}

				// Convert to protobuf format
				for _, token := range teamTokens {
					permStrs := make([]string, len(token.Permissions))
					for i, perm := range token.Permissions {
						permStrs[i] = string(perm)
					}

					lastUsedStr := ""
					if token.LastUsedAt != nil {
						lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
					}

					expiresStr := ""
					if token.ExpiresAt != nil {
						expiresStr = token.ExpiresAt.Format("2006-01-02")
					}

					allTokens = append(allTokens, &generated.ApiToken{
						Id:          token.ID,
						Name:        token.Name,
						TeamId:      token.TeamID,
						TeamName:    team.Name,
						Permissions: permStrs,
						CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
						LastUsedAt:  lastUsedStr,
						ExpiresAt:   expiresStr,
						CreatedBy:   token.CreatedBy,
					})
				}
			}
		} else {
			// Regular users can only see tokens for their teams
			for _, membership := range authCtx.TeamMemberships {
				team, err := h.RbacRepo.GetTeam(ctx, membership.TeamID)
				if err != nil {
					continue
				}
				if team == nil {
					continue
				}

				teamTokens, err := tokenManager.ListTokens(ctx, team.ID)
				if err != nil {
					continue
				}

				// Convert to protobuf format
				for _, token := range teamTokens {
					permStrs := make([]string, len(token.Permissions))
					for i, perm := range token.Permissions {
						permStrs[i] = string(perm)
					}

					lastUsedStr := ""
					if token.LastUsedAt != nil {
						lastUsedStr = token.LastUsedAt.Format("2006-01-02 15:04:05")
					}

					expiresStr := ""
					if token.ExpiresAt != nil {
						expiresStr = token.ExpiresAt.Format("2006-01-02")
					}

					allTokens = append(allTokens, &generated.ApiToken{
						Id:          token.ID,
						Name:        token.Name,
						TeamId:      token.TeamID,
						TeamName:    team.Name,
						Permissions: permStrs,
						CreatedAt:   token.CreatedAt.Format("2006-01-02 15:04:05"),
						LastUsedAt:  lastUsedStr,
						ExpiresAt:   expiresStr,
						CreatedBy:   token.CreatedBy,
					})
				}
			}
		}
	}

	return &generated.ListTokensResponse{
		Tokens: allTokens,
	}, nil
}

// RevokeToken revokes an API token
func (h *Handler) RevokeToken(ctx context.Context, req *generated.RevokeTokenRequest) (*generated.RevokeTokenResponse, error) {
	// Get auth context
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}

	// Create token manager
	tokenManager := tokens.NewManager(h.RbacRepo)

	// Check if user can revoke tokens
	canRevoke := false
	
	// Check if user is org admin (can revoke any token)
	if authCtx.IsOrgAdmin() {
		canRevoke = true
	} else {
		// Check if user is admin of any team (simplified check)
		for _, teamMember := range authCtx.TeamMemberships {
			if teamMember.Role == rbac.RoleAdmin {
				canRevoke = true
				break
			}
		}
	}

	if !canRevoke {
		return &generated.RevokeTokenResponse{
			Success: false,
			Message: "Permission denied: only organization admins or team admins can revoke API tokens",
		}, nil
	}

	// Revoke the token
	if err := tokenManager.RevokeToken(ctx, req.TokenId); err != nil {
		return &generated.RevokeTokenResponse{
			Success: false,
			Message: fmt.Sprintf("Failed to revoke token: %v", err),
		}, nil
	}

	return &generated.RevokeTokenResponse{
		Success: true,
		Message: fmt.Sprintf("API token '%s' revoked successfully", req.TokenId),
	}, nil
}