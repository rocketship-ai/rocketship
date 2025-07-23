package teams

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// AddTeamRepository adds a repository to a team
func (h *Handler) AddTeamRepository(ctx context.Context, req *generated.AddTeamRepositoryRequest) (*generated.AddTeamRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Get team
	team, err := h.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// For now, return a simplified implementation that doesn't validate GitHub repo
	// TODO: Implement full GitHub validation like in the CLI version

	// Parse repository URL to get standard format
	parts := strings.Split(strings.TrimSuffix(req.RepositoryUrl, ".git"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format")
	}

	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]
	standardURL := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)
	fullName := fmt.Sprintf("%s/%s", owner, repoName)

	// Create a simple repository entry (without full GitHub validation for now)
	repository := &rbac.RepositoryEntity{
		ID:                   uuid.New().String(),
		URL:                  standardURL,
		GitHubInstallationID: nil, // Not using GitHub app integration yet
		EnforceCodeowners:    req.EnforceCodeowners,
		CodeownersCache:      nil, // No cache yet
		CodeownersCachedAt:   nil, // No cache yet
		CreatedAt:            time.Now(),
	}

	// Create repository in database
	if err := h.RbacRepo.CreateRepository(ctx, repository); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	// Add team repository association
	if err := h.RbacRepo.AddTeamRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to add team repository: %w", err)
	}

	return &generated.AddTeamRepositoryResponse{
		Success:            true,
		Message:            fmt.Sprintf("Added repository '%s' to team '%s'", fullName, req.TeamName),
		RepositoryFullName: fullName,
	}, nil
}

// RemoveTeamRepository removes a repository from a team
func (h *Handler) RemoveTeamRepository(ctx context.Context, req *generated.RemoveTeamRepositoryRequest) (*generated.RemoveTeamRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Get team
	team, err := h.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}

	// Check if user can manage repositories for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Parse repository URL to get standard format
	parts := strings.Split(strings.TrimSuffix(req.RepositoryUrl, ".git"), "/")
	if len(parts) < 2 {
		return nil, fmt.Errorf("invalid repository URL format")
	}

	owner := parts[len(parts)-2]
	repoName := parts[len(parts)-1]
	standardURL := fmt.Sprintf("https://github.com/%s/%s", owner, repoName)

	// Get repository from database
	repository, err := h.RbacRepo.GetRepository(ctx, standardURL)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return nil, fmt.Errorf("repository not found in system: %s", standardURL)
	}

	// Remove team repository association
	if err := h.RbacRepo.RemoveTeamFromRepository(ctx, team.ID, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to remove repository from team: %w", err)
	}

	return &generated.RemoveTeamRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Removed repository '%s' from team '%s'", standardURL, req.TeamName),
	}, nil
}