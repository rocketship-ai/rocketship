package teams

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/orchestrator/handlers"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// Handler manages team operations
type Handler struct {
	*handlers.BaseHandler
}

// NewHandler creates a new team handler
func NewHandler(rbacRepo *rbac.Repository) *Handler {
	return &Handler{
		BaseHandler: handlers.NewBaseHandler(rbacRepo),
	}
}

// CreateTeam creates a new team
func (h *Handler) CreateTeam(ctx context.Context, req *generated.CreateTeamRequest) (*generated.CreateTeamResponse, error) {
	// Get auth context (this uses the authentication interceptor)
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Check if user can create teams (only org admins)
	if err := enforcer.EnforceGlobalPermission(ctx, authCtx, rbac.PermissionTeamsWrite); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Create team
	team := &rbac.Team{
		ID:        uuid.New().String(),
		Name:      req.Name,
		CreatedAt: time.Now(),
	}

	if err := h.RbacRepo.CreateTeam(ctx, team); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	return &generated.CreateTeamResponse{
		TeamId: team.ID,
		Name:   team.Name,
	}, nil
}

// ListTeams lists all teams
func (h *Handler) ListTeams(ctx context.Context, req *generated.ListTeamsRequest) (*generated.ListTeamsResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Get all teams from database
	teams, err := h.RbacRepo.ListTeams(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	// Convert to proto format
	var protoTeams []*generated.Team
	for _, team := range teams {
		protoTeam := &generated.Team{
			Id:        team.ID,
			Name:      team.Name,
			CreatedAt: team.CreatedAt.Format(time.RFC3339),
		}

		// Get member count
		members, err := h.RbacRepo.GetTeamMembers(ctx, team.ID)
		if err == nil {
			protoTeam.MemberCount = int32(len(members))
		}

		// Get repository count
		repos, err := h.RbacRepo.GetTeamRepositories(ctx, team.ID)
		if err == nil {
			protoTeam.RepositoryCount = int32(len(repos))
		}

		// Check user's role in this team
		userTeams, err := h.RbacRepo.GetUserTeams(ctx, authCtx.UserID)
		if err == nil {
			for _, membership := range userTeams {
				if membership.TeamID == team.ID {
					protoTeam.UserRole = string(membership.Role)
					break
				}
			}
		}

		protoTeams = append(protoTeams, protoTeam)
	}

	return &generated.ListTeamsResponse{
		Teams: protoTeams,
	}, nil
}

// GetTeam retrieves detailed information about a team including members and repositories
func (h *Handler) GetTeam(ctx context.Context, req *generated.GetTeamRequest) (*generated.GetTeamResponse, error) {
	// Get auth context
	authCtx, err := h.RequireAuth(ctx)
	if err != nil {
		return nil, err
	}

	// Create RBAC enforcer
	enforcer := h.CreateEnforcer()

	// Get team by name
	team, err := h.GetTeamByName(ctx, req.TeamName)
	if err != nil {
		return nil, err
	}

	// Check if user can view this team (members can view, or org admins)
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersRead); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get team members
	members, err := h.RbacRepo.GetTeamMembers(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team members: %w", err)
	}

	// Get team repositories
	repositories, err := h.RbacRepo.GetTeamRepositories(ctx, team.ID)
	if err != nil {
		return nil, fmt.Errorf("failed to get team repositories: %w", err)
	}

	// Convert to proto format
	var protoMembers []*generated.TeamMember
	for _, member := range members {
		// Get user details for this member
		user, err := h.RbacRepo.GetUser(ctx, member.UserID)
		if err != nil {
			slog.Warn("failed to get user details for team member", "userID", member.UserID, "error", err)
			continue
		}

		// Convert permissions to strings
		var permissionStrs []string
		for _, perm := range member.Permissions {
			permissionStrs = append(permissionStrs, string(perm))
		}

		protoMembers = append(protoMembers, &generated.TeamMember{
			UserId:      member.UserID,
			Email:       user.Email,
			Role:        string(member.Role),
			Permissions: permissionStrs,
			JoinedAt:    member.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	var protoRepositories []*generated.TeamRepository
	for _, repo := range repositories {
		// Extract repository name from URL (e.g., "github.com/owner/repo" -> "owner/repo")
		repoName := repo.URL
		if idx := strings.LastIndex(repo.URL, "/"); idx != -1 {
			if idx2 := strings.LastIndex(repo.URL[:idx], "/"); idx2 != -1 {
				repoName = repo.URL[idx2+1:]
			}
		}

		protoRepositories = append(protoRepositories, &generated.TeamRepository{
			RepositoryUrl:  repo.URL,
			RepositoryName: repoName,
			AddedAt:        repo.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	// Get user's role in this team for the response
	userRole := ""
	for _, member := range members {
		if member.UserID == authCtx.UserID {
			userRole = string(member.Role)
			break
		}
	}

	return &generated.GetTeamResponse{
		Team: &generated.Team{
			Id:               team.ID,
			Name:             team.Name,
			CreatedAt:        team.CreatedAt.Format("2006-01-02T15:04:05Z"),
			MemberCount:      int32(len(members)),
			RepositoryCount:  int32(len(repositories)),
			UserRole:         userRole,
		},
		Members:      protoMembers,
		Repositories: protoRepositories,
	}, nil
}