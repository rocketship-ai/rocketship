package rbac

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
)

// Enforcer handles permission enforcement
type Enforcer struct {
	repo *Repository
}

// NewEnforcer creates a new permission enforcer
func NewEnforcer(repo *Repository) *Enforcer {
	return &Enforcer{repo: repo}
}

// CanRunTest checks if a user can run tests for a specific repository/path
func (e *Enforcer) CanRunTest(ctx context.Context, authCtx *AuthContext, req *TestRunRequest) (bool, error) {
	// Organization admins can always run tests
	if authCtx.IsAdmin {
		return true, nil
	}

	// Get repository
	repo, err := e.repo.GetRepository(ctx, req.RepositoryURL)
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}
	if repo == nil {
		return false, fmt.Errorf("repository not found: %s", req.RepositoryURL)
	}

	// Get repository teams
	teams, err := e.repo.GetRepositoryTeams(ctx, repo.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get repository teams: %w", err)
	}

	// Check if user is in any team with access to this repository
	hasTeamAccess := false
	userTeamIDs := make(map[string]bool)
	
	// For API tokens, check token team
	if authCtx.TokenTeamID != nil {
		userTeamIDs[*authCtx.TokenTeamID] = true
		
		// Check if token has test_runs permission
		hasTestRunsPermission := false
		for _, perm := range authCtx.TokenPerms {
			if perm == PermissionTestRuns {
				hasTestRunsPermission = true
				break
			}
		}
		if !hasTestRunsPermission {
			return false, nil
		}
	} else {
		// For users, check team memberships
		for _, membership := range authCtx.TeamMemberships {
			userTeamIDs[membership.TeamID] = true
		}
	}

	// Check if user has access to repository via any team
	for _, team := range teams {
		if userTeamIDs[team.ID] {
			hasTeamAccess = true
			break
		}
	}

	if !hasTeamAccess {
		return false, nil
	}

	// If CODEOWNERS enforcement is disabled, allow access
	if !repo.EnforceCodeowners {
		return true, nil
	}

	// Check CODEOWNERS if enforcement is enabled
	return e.checkCodeownersAccess(ctx, authCtx, repo, req.FilePath)
}

// checkCodeownersAccess checks if user has access based on CODEOWNERS
func (e *Enforcer) checkCodeownersAccess(ctx context.Context, authCtx *AuthContext, repo *RepositoryEntity, filePath string) (bool, error) {
	// Parse CODEOWNERS data
	if repo.CodeownersCache == nil {
		return false, fmt.Errorf("CODEOWNERS cache not available")
	}

	var codeownersData CodeownersData
	if err := parseCodeownersCache(repo.CodeownersCache, &codeownersData); err != nil {
		return false, fmt.Errorf("failed to parse CODEOWNERS cache: %w", err)
	}

	// Find matching rule for file path
	var matchingRule *CodeownersRule
	for i := len(codeownersData.Rules) - 1; i >= 0; i-- {
		rule := &codeownersData.Rules[i]
		if matchesPattern(rule.Pattern, filePath) {
			matchingRule = rule
			break
		}
	}

	if matchingRule == nil {
		// No matching rule, allow access
		return true, nil
	}

	// Check if user or their teams are in the owners list
	return e.isUserInOwners(ctx, authCtx, matchingRule.Owners)
}

// isUserInOwners checks if user is in the owners list
func (e *Enforcer) isUserInOwners(ctx context.Context, authCtx *AuthContext, owners []string) (bool, error) {
	// Check direct user mention
	userHandle := "@" + authCtx.Email
	for _, owner := range owners {
		if owner == userHandle {
			return true, nil
		}
	}

	// Check team mentions
	userTeamNames := make(map[string]bool)
	
	if authCtx.TokenTeamID != nil {
		// For API tokens, get team name
		team, err := e.repo.GetTeam(ctx, *authCtx.TokenTeamID)
		if err != nil {
			return false, fmt.Errorf("failed to get token team: %w", err)
		}
		if team != nil {
			userTeamNames[team.Name] = true
		}
	} else {
		// For users, get team names from memberships
		for _, membership := range authCtx.TeamMemberships {
			team, err := e.repo.GetTeam(ctx, membership.TeamID)
			if err != nil {
				return false, fmt.Errorf("failed to get team: %w", err)
			}
			if team != nil {
				userTeamNames[team.Name] = true
			}
		}
	}

	// Check team mentions in owners
	for _, owner := range owners {
		if strings.HasPrefix(owner, "@") {
			teamName := strings.TrimPrefix(owner, "@")
			if userTeamNames[teamName] {
				return true, nil
			}
		}
	}

	return false, nil
}

// CanManageTeam checks if a user can manage a team
func (e *Enforcer) CanManageTeam(ctx context.Context, authCtx *AuthContext, teamID string) (bool, error) {
	// Organization admins can manage any team
	if authCtx.IsAdmin {
		return true, nil
	}

	// API tokens cannot manage teams
	if authCtx.TokenTeamID != nil {
		return false, nil
	}

	// Check if user is team admin
	for _, membership := range authCtx.TeamMemberships {
		if membership.TeamID == teamID && membership.Role == RoleAdmin {
			return true, nil
		}
	}

	return false, nil
}

// CanManageRepository checks if a user can manage a repository
func (e *Enforcer) CanManageRepository(ctx context.Context, authCtx *AuthContext, repoURL string) (bool, error) {
	// Organization admins can manage any repository
	if authCtx.IsAdmin {
		return true, nil
	}

	// Get repository
	repo, err := e.repo.GetRepository(ctx, repoURL)
	if err != nil {
		return false, fmt.Errorf("failed to get repository: %w", err)
	}
	if repo == nil {
		return false, nil
	}

	// Get repository teams
	teams, err := e.repo.GetRepositoryTeams(ctx, repo.ID)
	if err != nil {
		return false, fmt.Errorf("failed to get repository teams: %w", err)
	}

	// Check if user is admin of any team that has access to this repository
	userTeamIDs := make(map[string]bool)
	
	if authCtx.TokenTeamID != nil {
		// API tokens with repository_mgmt permission can manage repositories
		for _, perm := range authCtx.TokenPerms {
			if perm == PermissionRepositoryMgmt {
				userTeamIDs[*authCtx.TokenTeamID] = true
				break
			}
		}
	} else {
		// For users, check team admin memberships with repository_mgmt permission
		for _, membership := range authCtx.TeamMemberships {
			if membership.Role == RoleAdmin {
				for _, perm := range membership.Permissions {
					if perm == PermissionRepositoryMgmt {
						userTeamIDs[membership.TeamID] = true
						break
					}
				}
			}
		}
	}

	// Check if user has management access via any team
	for _, team := range teams {
		if userTeamIDs[team.ID] {
			return true, nil
		}
	}

	return false, nil
}

// matchesPattern checks if a file path matches a CODEOWNERS pattern
func matchesPattern(pattern, path string) bool {
	// Simple pattern matching - could be enhanced with proper glob matching
	// For now, handle basic patterns
	
	// Exact match
	if pattern == path {
		return true
	}
	
	// Directory pattern (ends with /)
	if strings.HasSuffix(pattern, "/") {
		return strings.HasPrefix(path, pattern)
	}
	
	// Wildcard pattern
	if strings.Contains(pattern, "*") {
		matched, _ := filepath.Match(pattern, path)
		return matched
	}
	
	// Prefix match
	return strings.HasPrefix(path, pattern)
}

// parseCodeownersCache parses CODEOWNERS cache data
func parseCodeownersCache(data []byte, result *CodeownersData) error {
	// This would typically use JSON unmarshaling
	// For now, return a simple implementation
	*result = CodeownersData{
		Rules: []CodeownersRule{},
	}
	return nil
}