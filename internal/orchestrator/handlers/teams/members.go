package teams

import (
	"context"
	"fmt"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/rbac"
)

// AddTeamMember adds a member to a team
func (h *Handler) AddTeamMember(ctx context.Context, req *generated.AddTeamMemberRequest) (*generated.AddTeamMemberResponse, error) {
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

	// Check if user can manage team members
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersWrite); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Validate role
	var role rbac.Role
	switch req.Role {
	case "admin":
		role = rbac.RoleAdmin
	case "member":
		role = rbac.RoleMember
	default:
		return nil, fmt.Errorf("invalid role: %s (must be 'admin' or 'member')", req.Role)
	}

	// Parse permissions
	permissions, err := parsePermissions(req.Permissions)
	if err != nil {
		return nil, err
	}

	// Get or create user by email
	user, err := h.RbacRepo.GetOrCreateUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get or create user: %w", err)
	}

	// Add team member
	if err := h.RbacRepo.AddTeamMember(ctx, team.ID, user.ID, role, permissions); err != nil {
		return nil, fmt.Errorf("failed to add team member: %w", err)
	}

	return &generated.AddTeamMemberResponse{
		Success: true,
		Message: fmt.Sprintf("Added %s to team '%s' as %s", req.Email, req.TeamName, req.Role),
	}, nil
}

// RemoveTeamMember removes a member from a team
func (h *Handler) RemoveTeamMember(ctx context.Context, req *generated.RemoveTeamMemberRequest) (*generated.RemoveTeamMemberResponse, error) {
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

	// Check if user can manage members for this team
	if err := enforcer.EnforceTeamPermission(ctx, authCtx, team.ID, rbac.PermissionTeamMembersManage); err != nil {
		return nil, fmt.Errorf("permission denied: %w", err)
	}

	// Get user by email
	user, err := h.RbacRepo.GetOrCreateUserByEmail(ctx, req.Email)
	if err != nil {
		return nil, fmt.Errorf("failed to get user: %w", err)
	}
	if user == nil {
		return nil, fmt.Errorf("user not found: %s", req.Email)
	}

	// Remove the member from the team
	if err := h.RbacRepo.RemoveTeamMember(ctx, team.ID, user.ID); err != nil {
		return nil, fmt.Errorf("failed to remove team member: %w", err)
	}

	return &generated.RemoveTeamMemberResponse{
		Success: true,
		Message: fmt.Sprintf("Removed %s from team '%s'", req.Email, req.TeamName),
	}, nil
}

// parsePermissions converts permission strings to rbac.Permission slice
func parsePermissions(permStrs []string) ([]rbac.Permission, error) {
	var permissions []rbac.Permission
	for _, permStr := range permStrs {
		switch permStr {
		case "tests:run":
			permissions = append(permissions, rbac.PermissionTestsRun)
		case "repositories:read":
			permissions = append(permissions, rbac.PermissionRepositoriesRead)
		case "repositories:write":
			permissions = append(permissions, rbac.PermissionRepositoriesWrite)
		case "repositories:manage":
			permissions = append(permissions, rbac.PermissionRepositoriesManage)
		case "team:members:read":
			permissions = append(permissions, rbac.PermissionTeamMembersRead)
		case "team:members:write":
			permissions = append(permissions, rbac.PermissionTeamMembersWrite)
		case "team:members:manage":
			permissions = append(permissions, rbac.PermissionTeamMembersManage)
		case "test:schedules:manage":
			permissions = append(permissions, rbac.PermissionTestSchedulesManage)
		default:
			return nil, fmt.Errorf("invalid permission: %s", permStr)
		}
	}
	return permissions, nil
}