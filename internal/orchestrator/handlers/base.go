package handlers

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/auth"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// BaseHandler provides common functionality for all handlers
type BaseHandler struct {
	RbacRepo *rbac.Repository
}

// NewBaseHandler creates a new base handler
func NewBaseHandler(rbacRepo *rbac.Repository) *BaseHandler {
	return &BaseHandler{
		RbacRepo: rbacRepo,
	}
}

// RequireAuth ensures the request is authenticated and returns the auth context
func (h *BaseHandler) RequireAuth(ctx context.Context) (*rbac.AuthContext, error) {
	authCtx := auth.GetAuthContext(ctx)
	if authCtx == nil {
		return nil, fmt.Errorf("authentication required")
	}
	return authCtx, nil
}

// CreateEnforcer creates an RBAC enforcer
func (h *BaseHandler) CreateEnforcer() *rbac.Enforcer {
	return rbac.NewEnforcer(h.RbacRepo)
}

// GetTeamByName gets a team by name with proper error handling
func (h *BaseHandler) GetTeamByName(ctx context.Context, teamName string) (*rbac.Team, error) {
	team, err := h.RbacRepo.GetTeamByName(ctx, teamName)
	if err != nil {
		return nil, fmt.Errorf("failed to get team: %w", err)
	}
	if team == nil {
		return nil, fmt.Errorf("team not found: %s", teamName)
	}
	return team, nil
}

// GetRepository gets a repository by URL with proper error handling
func (h *BaseHandler) GetRepository(ctx context.Context, repoURL string) (*rbac.RepositoryEntity, error) {
	repository, err := h.RbacRepo.GetRepository(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return nil, fmt.Errorf("repository not found: %s", repoURL)
	}
	return repository, nil
}