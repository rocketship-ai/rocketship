package repositories

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// AssignTeamToRepository assigns a team to a repository
func (h *Handler) AssignTeamToRepository(ctx context.Context, req *generated.AssignTeamToRepositoryRequest) (*generated.AssignTeamToRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user has permission to manage team-repository assignments
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := h.RbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.AssignTeamToRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Get team
	team, err := h.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}

	// Add team repository association
	if err := h.RbacRepo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to assign team to repository: %w", err)
	}

	return &generated.AssignTeamToRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Team '%s' assigned to repository '%s'", req.TeamName, req.RepositoryUrl),
	}, nil
}

// UnassignTeamFromRepository removes a team assignment from a repository
func (h *Handler) UnassignTeamFromRepository(ctx context.Context, req *generated.UnassignTeamFromRepositoryRequest) (*generated.UnassignTeamFromRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user has permission to manage team-repository assignments
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := h.RbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.UnassignTeamFromRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Get team
	team, err := h.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}

	// Remove team repository association
	if err := h.RbacRepo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to unassign team from repository: %w", err)
	}

	return &generated.UnassignTeamFromRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Team '%s' unassigned from repository '%s'", req.TeamName, req.RepositoryUrl),
	}, nil
}