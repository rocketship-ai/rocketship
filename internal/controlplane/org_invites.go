package controlplane

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleOrgInviteAccept accepts an organization invite by verifying the invite code
func (s *Server) handleOrgInviteAccept(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if strings.TrimSpace(principal.Email) == "" {
		writeError(w, http.StatusBadRequest, "email address required to accept invite")
		return
	}

	var req struct {
		Code string `json:"code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	code := strings.TrimSpace(req.Code)
	if code == "" {
		writeError(w, http.StatusBadRequest, "invite code is required")
		return
	}

	ctx := r.Context()
	invites, err := s.store.FindPendingOrgInvites(ctx, principal.Email)
	if err != nil {
		log.Printf("failed to list invites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to look up invites")
		return
	}
	if len(invites) == 0 {
		writeError(w, http.StatusNotFound, "no invites found for this account")
		return
	}

	now := s.nowUTC()
	var matched *persistence.OrganizationInvite
	for _, inv := range invites {
		if inv.ExpiresAt.Before(now) {
			continue
		}
		if verifyCode(code, inv.CodeSalt, inv.CodeHash) {
			matched = &inv
			break
		}
	}
	if matched == nil {
		writeError(w, http.StatusUnauthorized, "invite code invalid")
		return
	}

	if err := s.store.AddOrganizationOwner(ctx, matched.OrganizationID, principal.UserID); err != nil {
		log.Printf("failed to add organization owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to apply invite")
		return
	}
	if err := s.store.MarkOrgInviteAccepted(ctx, matched.ID, principal.UserID); err != nil {
		log.Printf("failed to mark invite accepted: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update invite")
		return
	}

	// Rotate tokens immediately to update roles and org_id in access token
	if refreshCookie, err := r.Cookie("refresh_token"); err == nil && refreshCookie.Value != "" {
		if tokens, err := s.validateAndRotateRefreshToken(ctx, refreshCookie.Value); err == nil {
			s.setAuthCookies(w, r, tokens)
		} else {
			log.Printf("failed to rotate tokens after invite acceptance: %v", err)
		}
	}

	response := map[string]interface{}{
		"organization": map[string]interface{}{
			"id":   matched.OrganizationID.String(),
			"name": matched.OrganizationName,
		},
		"needs_claim_refresh": true,
	}
	writeJSON(w, http.StatusOK, response)
}

// handleOrgInvites creates a new organization invite and sends it via email
func (s *Server) handleOrgInvites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, orgID uuid.UUID, tail []string) {
	if len(tail) > 0 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !principal.HasRole("owner") {
		writeError(w, http.StatusForbidden, "owner role required")
		return
	}

	ctx := r.Context()
	isAdmin, err := s.store.IsOrganizationOwner(ctx, orgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to authorize request")
		return
	}
	if !isAdmin {
		writeError(w, http.StatusForbidden, "owner role required for target organization")
		return
	}

	var req struct {
		Email string `json:"email"`
		Role  string `json:"role"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}
	email := strings.TrimSpace(req.Email)
	if email == "" {
		writeError(w, http.StatusBadRequest, "invite email is required")
		return
	}
	if len(email) > maxInviteEmailLength || !strings.Contains(email, "@") {
		writeError(w, http.StatusBadRequest, "invite email appears invalid")
		return
	}
	role := strings.ToLower(strings.TrimSpace(req.Role))
	if role == "" {
		role = "owner"
	}
	if role != "owner" {
		writeError(w, http.StatusBadRequest, "only owner role is supported for invites")
		return
	}

	code, salt, hash, err := newVerificationSecret(verificationCodeLength)
	if err != nil {
		log.Printf("failed to generate invite code: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to generate invite code")
		return
	}

	invite := persistence.OrganizationInvite{
		ID:             uuid.New(),
		OrganizationID: orgID,
		Email:          email,
		Role:           role,
		CodeHash:       hash,
		CodeSalt:       salt,
		InvitedBy:      principal.UserID,
		ExpiresAt:      s.nowUTC().Add(orgRegistrationTTL),
	}
	record, err := s.store.CreateOrgInvite(ctx, invite)
	if err != nil {
		log.Printf("failed to create invite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create invite")
		return
	}

	displayName := principal.Name
	if displayName == "" {
		displayName = principal.Email
	}

	if err := s.mailer.SendOrgInvite(ctx, email, record.OrganizationName, code, record.ExpiresAt, displayName); err != nil {
		log.Printf("failed to send invite email: %v", err)
		writeError(w, http.StatusBadGateway, "failed to send invite email")
		return
	}

	response := map[string]interface{}{
		"invite_id":         record.ID.String(),
		"organization_id":   record.OrganizationID.String(),
		"organization_name": record.OrganizationName,
		"role":              record.Role,
		"expires_at":        record.ExpiresAt.Format(time.RFC3339),
		"invite_code":       code,
	}
	writeJSON(w, http.StatusCreated, response)
}
