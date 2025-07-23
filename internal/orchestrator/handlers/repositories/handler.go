package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/utils"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Handler manages repository operations
type Handler struct {
	*handlers.BaseHandler
}

// NewHandler creates a new repository handler
func NewHandler(rbacRepo *rbac.Repository) *Handler {
	return &Handler{
		BaseHandler: handlers.NewBaseHandler(rbacRepo),
	}
}

// AddRepository adds a new repository to the system
func (h *Handler) AddRepository(ctx context.Context, req *generated.AddRepositoryRequest) (*generated.AddRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user has repository management permissions (global admin)
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate repository URL
	if err := utils.ValidateRepositoryURL(req.RepositoryUrl); err != nil {
		return nil, fmt.Errorf("invalid repository URL: %w", err)
	}

	// Check if repository already exists
	existing, err := h.RbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil && !strings.Contains(err.Error(), "no rows") {
		return nil, fmt.Errorf("failed to check existing repository: %w", err)
	}
	if existing != nil {
		return &generated.AddRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository already exists: %s", req.RepositoryUrl),
		}, nil
	}

	// Create new repository entity
	repoEntity := &rbac.RepositoryEntity{
		ID:                uuid.New().String(),
		URL:               req.RepositoryUrl,
		EnforceCodeowners: req.EnforceCodeowners,
		CreatedAt:         time.Now(),
	}

	// Create repository
	if err := h.RbacRepo.CreateRepository(ctx, repoEntity); err != nil {
		return nil, fmt.Errorf("failed to create repository: %w", err)
	}

	return &generated.AddRepositoryResponse{
		RepositoryId:  repoEntity.ID,
		RepositoryUrl: repoEntity.URL,
		Success:       true,
		Message:       fmt.Sprintf("Repository '%s' added successfully", req.RepositoryUrl),
	}, nil
}

// ListRepositories lists all repositories in the system
func (h *Handler) ListRepositories(ctx context.Context, req *generated.ListRepositoriesRequest) (*generated.ListRepositoriesResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user has permission to view repositories (org admin OR team admin with repositories:read)
	hasGlobalAccess := false
	
	// First check if user is org admin
	if authCtx.IsOrgAdmin() {
		hasGlobalAccess = true
	} else {
		// Check if user has repositories:read permission in any team
		for _, membership := range authCtx.TeamMemberships {
			for _, perm := range membership.Permissions {
				if perm == rbac.PermissionRepositoriesRead {
					hasGlobalAccess = true
					break
				}
			}
			if hasGlobalAccess {
				break
			}
		}
	}
	
	if !hasGlobalAccess {
		return nil, fmt.Errorf("permission denied: only organization admins or team admins can perform this action")
	}

	// List repositories
	repositories, err := h.RbacRepo.ListRepositories(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list repositories: %w", err)
	}

	// Convert to proto format
	var protoRepos []*generated.Repository
	for _, repo := range repositories {
		// Get teams for this repository
		teams, err := h.RbacRepo.GetRepositoryTeamsDetailed(ctx, repo.ID)
		if err != nil {
			// Log warning but continue
			teams = []*rbac.Team{}
		}

		// Extract team names
		var teamNames []string
		for _, team := range teams {
			teamNames = append(teamNames, team.Name)
		}

		protoRepos = append(protoRepos, &generated.Repository{
			Id:                repo.ID,
			Url:               repo.URL,
			EnforceCodeowners: repo.EnforceCodeowners,
			CreatedAt:         repo.CreatedAt.Format("2006-01-02T15:04:05Z"),
			TeamNames:         teamNames,
			TeamCount:         int32(len(teams)),
		})
	}

	return &generated.ListRepositoriesResponse{
		Repositories: protoRepos,
	}, nil
}

// GetRepository gets detailed information about a repository
func (h *Handler) GetRepository(ctx context.Context, req *generated.GetRepositoryRequest) (*generated.GetRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user has permission to view repositories
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesRead); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := h.RbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	// Get teams for this repository
	teams, err := h.RbacRepo.GetRepositoryTeamsDetailed(ctx, repository.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository teams: %w", err)
	}

	// Extract team names
	var teamNames []string
	var protoTeams []*generated.Team
	for _, team := range teams {
		teamNames = append(teamNames, team.Name)

		// Convert to proto team
		protoTeams = append(protoTeams, &generated.Team{
			Id:        team.ID,
			Name:      team.Name,
			CreatedAt: team.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	return &generated.GetRepositoryResponse{
		Repository: &generated.Repository{
			Id:                repository.ID,
			Url:               repository.URL,
			EnforceCodeowners: repository.EnforceCodeowners,
			CreatedAt:         repository.CreatedAt.Format("2006-01-02T15:04:05Z"),
			TeamNames:         teamNames,
			TeamCount:         int32(len(teams)),
		},
		Teams: protoTeams,
	}, nil
}

// RemoveRepository removes a repository from the system
func (h *Handler) RemoveRepository(ctx context.Context, req *generated.RemoveRepositoryRequest) (*generated.RemoveRepositoryResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user has repository management permissions
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionRepositoriesManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get repository
	repository, err := h.RbacRepo.GetRepository(ctx, req.RepositoryUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}
	if repository == nil {
		return &generated.RemoveRepositoryResponse{
			Success: false,
			Message: fmt.Sprintf("repository not found: %s", req.RepositoryUrl),
		}, nil
	}

	// Delete repository (cascade will remove team assignments)
	if err := h.RbacRepo.DeleteRepository(ctx, repository.ID); err != nil {
		return nil, fmt.Errorf("failed to delete repository: %w", err)
	}

	return &generated.RemoveRepositoryResponse{
		Success: true,
		Message: fmt.Sprintf("Repository '%s' removed successfully", req.RepositoryUrl),
	}, nil
}