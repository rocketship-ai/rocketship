package controlplane

import (
	"errors"
	"log"
	"net/http"

	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleProfile returns the current user's profile with organization, GitHub, and project permissions.
// GET /api/profile
func (s *Server) handleProfile(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// Require org membership for profile endpoint
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Get organization details
	org, err := s.store.GetOrganizationByID(ctx, principal.OrgID)
	if err != nil {
		log.Printf("failed to get organization: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load organization")
		return
	}

	// Determine user's role in the organization
	isAdmin, err := s.store.IsOrganizationAdmin(ctx, principal.OrgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check admin status: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to determine role")
		return
	}
	orgRole := "member"
	if isAdmin {
		orgRole = "admin"
	}

	// Get GitHub App installation status
	installationID, accountLogin, _, ghErr := s.store.GetGitHubAppInstallation(ctx, principal.OrgID)
	appInstalled := ghErr == nil
	if ghErr != nil && !errors.Is(ghErr, persistence.ErrGitHubAppNotInstalled) {
		log.Printf("failed to check GitHub App installation: %v", ghErr)
		// Non-fatal: continue with app_installed=false
	}

	// Build GitHub info from principal
	githubInfo := map[string]interface{}{
		"username":          principal.Username,
		"avatar_url":        "https://github.com/" + principal.Username + ".png",
		"app_installed":     appInstalled,
		"app_account_login": accountLogin,
		"installation_id":   installationID,
	}

	// Get project permissions
	perms, err := s.store.ListProjectPermissionsForUser(ctx, principal.OrgID, principal.UserID)
	if err != nil {
		log.Printf("failed to list project permissions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load project permissions")
		return
	}

	// Build project permissions response
	projectPerms := make([]map[string]interface{}, 0, len(perms))
	for _, p := range perms {
		projectPerms = append(projectPerms, map[string]interface{}{
			"project_id":   p.ProjectID.String(),
			"project_name": p.ProjectName,
			"source_ref":   p.SourceRef,
			"permissions":  p.Permissions,
		})
	}

	resp := map[string]interface{}{
		"user": map[string]string{
			"id":       principal.UserID.String(),
			"email":    principal.Email,
			"name":     principal.Name,
			"username": principal.Username,
		},
		"organization": map[string]string{
			"id":   org.ID.String(),
			"name": org.Name,
			"slug": org.Slug,
			"role": orgRole,
		},
		"github":              githubInfo,
		"project_permissions": projectPerms,
	}

	writeJSON(w, http.StatusOK, resp)
}
