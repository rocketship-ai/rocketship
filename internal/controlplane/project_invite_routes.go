package controlplane

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleProjectInvites handles all /api/project-invites routes
func (s *Server) handleProjectInvites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/project-invites") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/project-invites")
	trimmed = strings.Trim(trimmed, "/")

	// POST /api/project-invites - Create invite
	// GET /api/project-invites - List invites
	if trimmed == "" {
		switch r.Method {
		case http.MethodPost:
			s.handleCreateProjectInvite(w, r, principal)
		case http.MethodGet:
			s.handleListProjectInvites(w, r, principal)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// POST /api/project-invites/accept - Accept an invite
	if trimmed == "accept" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleAcceptProjectInvite(w, r, principal)
		return
	}

	// GET /api/project-invites/preview - Preview invite details (requires code)
	if trimmed == "preview" {
		if r.Method != http.MethodGet {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleProjectInvitePreview(w, r, principal)
		return
	}

	// POST /api/project-invites/:id/revoke - Revoke an invite
	segments := strings.Split(trimmed, "/")
	if len(segments) == 2 && segments[1] == "revoke" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		inviteID, err := uuid.Parse(segments[0])
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid invite id")
			return
		}
		s.handleRevokeProjectInvite(w, r, principal, inviteID)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// handleCreateProjectInvite creates a new project invite
func (s *Server) handleCreateProjectInvite(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if principal.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	var req struct {
		Email    string `json:"email"`
		Projects []struct {
			ProjectID string `json:"project_id"`
			Role      string `json:"role"`
		} `json:"projects"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	email := strings.TrimSpace(req.Email)
	if email == "" || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "valid email address is required")
		return
	}
	if len(email) > maxInviteEmailLength {
		writeError(w, http.StatusBadRequest, "email address too long")
		return
	}

	if len(req.Projects) == 0 {
		writeError(w, http.StatusBadRequest, "at least one project is required")
		return
	}

	// Parse and validate projects
	projectIDs := make([]uuid.UUID, 0, len(req.Projects))
	projectInputs := make([]persistence.ProjectInviteProjectInput, 0, len(req.Projects))
	for _, p := range req.Projects {
		id, err := uuid.Parse(p.ProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project id: "+p.ProjectID)
			return
		}
		role := strings.ToLower(strings.TrimSpace(p.Role))
		if role != "read" && role != "write" {
			writeError(w, http.StatusBadRequest, "role must be read or write")
			return
		}
		projectIDs = append(projectIDs, id)
		projectInputs = append(projectInputs, persistence.ProjectInviteProjectInput{
			ProjectID: id,
			Role:      role,
		})
	}

	ctx := r.Context()

	// Check if user can invite to all specified projects
	canInvite, err := s.store.CanUserInviteToProjects(ctx, principal.OrgID, principal.UserID, projectIDs)
	if err != nil {
		log.Printf("failed to check invite permissions: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify permissions")
		return
	}
	if !canInvite {
		writeError(w, http.StatusForbidden, "you must have write access to all selected projects to invite members")
		return
	}

	// Generate invite code
	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate invite code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate invite")
		return
	}

	// Create invite
	input := persistence.ProjectInviteInput{
		OrganizationID: principal.OrgID,
		Email:          email,
		InvitedBy:      principal.UserID,
		CodeHash:       hash,
		CodeSalt:       salt,
		ExpiresAt:      s.nowUTC().Add(orgRegistrationTTL),
		Projects:       projectInputs,
	}

	invite, err := s.store.CreateProjectInvite(ctx, input)
	if err != nil {
		if errors.Is(err, persistence.ErrPendingInviteExists) {
			writeError(w, http.StatusConflict, "a pending invite already exists for this email")
			return
		}
		log.Printf("failed to create project invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	// Prepare project list for email
	emailProjects := make([]ProjectInviteProject, len(invite.Projects))
	for i, p := range invite.Projects {
		emailProjects[i] = ProjectInviteProject{
			ProjectName: p.ProjectName,
			Role:        p.Role,
		}
	}

	// Build accept URL
	acceptURL := strings.TrimRight(s.cfg.Issuer, "/") +
		"/invites/accept?invite=" + url.QueryEscape(invite.ID.String()) +
		"&code=" + url.QueryEscape(code)

	// Send invite email
	displayName := principal.Name
	if displayName == "" {
		displayName = principal.Email
	}
	if err := s.mailer.SendProjectInvite(ctx, email, invite.OrganizationName, emailProjects, code, invite.ExpiresAt, displayName, acceptURL); err != nil {
		log.Printf("failed to send project invite email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send invite email")
		return
	}

	// Return response (don't include code in production)
	response := map[string]interface{}{
		"invite_id":  invite.ID.String(),
		"email":      invite.Email,
		"expires_at": invite.ExpiresAt.Format(time.RFC3339),
		"projects":   invite.Projects,
	}
	writeJSON(w, http.StatusCreated, response)
}

// handleListProjectInvites lists project invites for the org
func (s *Server) handleListProjectInvites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if principal.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Check if org owner - if so, return all invites; otherwise just invites they created
	isOwner, err := s.store.IsOrganizationOwner(ctx, principal.OrgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify permissions")
		return
	}

	var invites []persistence.ProjectInvite
	if isOwner {
		invites, err = s.store.ListProjectInvitesForOrg(ctx, principal.OrgID)
	} else {
		invites, err = s.store.ListProjectInvitesByCreator(ctx, principal.OrgID, principal.UserID)
	}
	if err != nil {
		log.Printf("failed to list project invites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list invites")
		return
	}

	// Convert to JSON-friendly format
	payload := make([]map[string]interface{}, 0, len(invites))
	for _, inv := range invites {
		status := "pending"
		if inv.AcceptedAt.Valid {
			status = "accepted"
		} else if inv.RevokedAt.Valid {
			status = "revoked"
		} else if time.Now().After(inv.ExpiresAt) {
			status = "expired"
		}

		projects := make([]map[string]interface{}, len(inv.Projects))
		for j, p := range inv.Projects {
			projects[j] = map[string]interface{}{
				"project_id":   p.ProjectID.String(),
				"project_name": p.ProjectName,
				"role":         p.Role,
			}
		}

		payload = append(payload, map[string]interface{}{
			"id":           inv.ID.String(),
			"email":        inv.Email,
			"invited_by":   inv.InvitedBy.String(),
			"inviter_name": inv.InviterName,
			"status":       status,
			"expires_at":   inv.ExpiresAt.Format(time.RFC3339),
			"created_at":   inv.CreatedAt.Format(time.RFC3339),
			"projects":     projects,
		})
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleAcceptProjectInvite accepts a project invite
func (s *Server) handleAcceptProjectInvite(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	var req struct {
		InviteID string `json:"invite_id"`
		Code     string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	inviteID, err := uuid.Parse(strings.TrimSpace(req.InviteID))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite id")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}

	ctx := r.Context()
	invite, err := s.store.GetProjectInvite(ctx, inviteID)
	if err != nil {
		log.Printf("failed to get project invite: %v", err)
		writeError(w, http.StatusNotFound, "invite not found")
		return
	}

	if invite.AcceptedAt.Valid {
		writeError(w, http.StatusBadRequest, "invite already accepted")
		return
	}
	if invite.RevokedAt.Valid {
		writeError(w, http.StatusBadRequest, "invite has been revoked")
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invite has expired")
		return
	}
	if !verifyCode(code, invite.CodeSalt, invite.CodeHash) {
		writeError(w, http.StatusUnauthorized, "invite code invalid")
		return
	}

	if err := s.store.AcceptProjectInvite(ctx, invite.ID, principal.UserID); err != nil {
		log.Printf("failed to accept project invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to accept invite")
		return
	}

	// Rotate tokens immediately to update roles in access token
	if refreshCookie, err := r.Cookie("refresh_token"); err == nil && refreshCookie.Value != "" {
		if tokens, err := s.validateAndRotateRefreshToken(ctx, refreshCookie.Value); err == nil {
			s.setAuthCookies(w, r, tokens)
		} else {
			log.Printf("failed to rotate tokens after project invite acceptance: %v", err)
		}
	}

	// Build response
	projects := make([]map[string]interface{}, len(invite.Projects))
	for i, p := range invite.Projects {
		projects[i] = map[string]interface{}{
			"project_id":   p.ProjectID.String(),
			"project_name": p.ProjectName,
			"role":         p.Role,
		}
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":   invite.OrganizationID.String(),
			"name": invite.OrganizationName,
		},
		"projects":            projects,
		"needs_claim_refresh": true,
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) handleProjectInvitePreview(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	inviteIDParam := strings.TrimSpace(r.URL.Query().Get("invite"))
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	if inviteIDParam == "" || code == "" {
		writeError(w, http.StatusBadRequest, "invite and code are required")
		return
	}

	inviteID, err := uuid.Parse(inviteIDParam)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid invite id")
		return
	}

	ctx := r.Context()
	invite, err := s.store.GetProjectInvite(ctx, inviteID)
	if err != nil {
		log.Printf("failed to get project invite: %v", err)
		writeError(w, http.StatusNotFound, "invite not found")
		return
	}

	if invite.AcceptedAt.Valid {
		writeError(w, http.StatusBadRequest, "invite already accepted")
		return
	}
	if invite.RevokedAt.Valid {
		writeError(w, http.StatusBadRequest, "invite has been revoked")
		return
	}
	if time.Now().After(invite.ExpiresAt) {
		writeError(w, http.StatusBadRequest, "invite has expired")
		return
	}
	if !verifyCode(code, invite.CodeSalt, invite.CodeHash) {
		writeError(w, http.StatusUnauthorized, "invite code invalid")
		return
	}

	projects := make([]map[string]interface{}, len(invite.Projects))
	for i, p := range invite.Projects {
		projects[i] = map[string]interface{}{
			"project_id":   p.ProjectID.String(),
			"project_name": p.ProjectName,
			"role":         p.Role,
		}
	}

	response := map[string]interface{}{
		"id":                invite.ID.String(),
		"organization_id":   invite.OrganizationID.String(),
		"organization_name": invite.OrganizationName,
		"inviter_name":      invite.InviterName,
		"expires_at":        invite.ExpiresAt.Format(time.RFC3339),
		"created_at":        invite.CreatedAt.Format(time.RFC3339),
		"projects":          projects,
	}
	writeJSON(w, http.StatusOK, response)
}

// handleRevokeProjectInvite revokes a project invite
func (s *Server) handleRevokeProjectInvite(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, inviteID uuid.UUID) {
	if principal.OrgID == uuid.Nil {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Get the invite to check permissions
	invite, err := s.store.GetProjectInvite(ctx, inviteID)
	if err != nil {
		log.Printf("failed to get project invite: %v", err)
		writeError(w, http.StatusNotFound, "invite not found")
		return
	}

	// Verify the invite belongs to the user's org
	if invite.OrganizationID != principal.OrgID {
		writeError(w, http.StatusForbidden, "invite not found in your organization")
		return
	}

	// Check permissions: org owner, inviter, or has write access to all projects
	isOwner, err := s.store.IsOrganizationOwner(ctx, principal.OrgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify permissions")
		return
	}

	canRevoke := isOwner || invite.InvitedBy == principal.UserID
	if !canRevoke {
		// Check if user has write access to all projects in the invite
		projectIDs := make([]uuid.UUID, len(invite.Projects))
		for i, p := range invite.Projects {
			projectIDs[i] = p.ProjectID
		}
		canRevoke, err = s.store.CanUserInviteToProjects(ctx, principal.OrgID, principal.UserID, projectIDs)
		if err != nil {
			log.Printf("failed to check project permissions: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to verify permissions")
			return
		}
	}

	if !canRevoke {
		writeError(w, http.StatusForbidden, "you don't have permission to revoke this invite")
		return
	}

	if err := s.store.RevokeProjectInvite(ctx, inviteID, principal.UserID); err != nil {
		log.Printf("failed to revoke project invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to revoke invite")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handlePendingProjectInvites returns pending invites for the authenticated user
func (s *Server) handlePendingProjectInvites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if strings.TrimSpace(principal.Email) == "" {
		writeError(w, http.StatusBadRequest, "email address required")
		return
	}

	ctx := r.Context()

	invites, err := s.store.FindPendingProjectInvitesByEmail(ctx, principal.Email)
	if err != nil {
		log.Printf("failed to find pending project invites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to look up invites")
		return
	}

	payload := make([]map[string]interface{}, 0, len(invites))
	for _, inv := range invites {
		projects := make([]map[string]interface{}, len(inv.Projects))
		for j, p := range inv.Projects {
			projects[j] = map[string]interface{}{
				"project_id":   p.ProjectID.String(),
				"project_name": p.ProjectName,
				"role":         p.Role,
			}
		}

		payload = append(payload, map[string]interface{}{
			"id":                inv.ID.String(),
			"organization_id":   inv.OrganizationID.String(),
			"organization_name": inv.OrganizationName,
			"inviter_name":      inv.InviterName,
			"expires_at":        inv.ExpiresAt.Format(time.RFC3339),
			"created_at":        inv.CreatedAt.Format(time.RFC3339),
			"projects":          projects,
		})
	}

	writeJSON(w, http.StatusOK, payload)
}
