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

// CITokenCreateRequest is the request body for creating a CI token
type CITokenCreateRequest struct {
	Name         string                   `json:"name"`
	Description  string                   `json:"description,omitempty"`
	NeverExpires bool                     `json:"never_expires"`
	ExpiresAt    *time.Time               `json:"expires_at,omitempty"`
	Projects     []CITokenProjectRequest  `json:"projects"`
}

// CITokenProjectRequest represents a project-scope pair in the create request
type CITokenProjectRequest struct {
	ProjectID string `json:"project_id"`
	Scope     string `json:"scope"` // "read" or "write"
}

// handleCITokensDispatch handles /api/ci-tokens routes
func (s *Server) handleCITokensDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/ci-tokens")
	path = strings.Trim(path, "/")

	// /api/ci-tokens
	if path == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListCITokens(w, r, principal)
		case http.MethodPost:
			s.handleCreateCIToken(w, r, principal)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Parse token ID
	segments := strings.Split(path, "/")
	tokenID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid token id")
		return
	}

	// /api/ci-tokens/{tokenId}/revoke
	if len(segments) == 2 && segments[1] == "revoke" {
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		s.handleRevokeCIToken(w, r, principal, tokenID)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// handleListCITokens handles GET /api/ci-tokens
// Query params:
//   - include_revoked: "true" to include revoked tokens, default is false (only active tokens)
func (s *Server) handleListCITokens(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	includeRevoked := r.URL.Query().Get("include_revoked") == "true"

	tokens, err := s.store.ListCITokensForOrg(r.Context(), principal.OrgID, includeRevoked)
	if err != nil {
		log.Printf("failed to list CI tokens: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list CI tokens")
		return
	}

	payload := make([]map[string]interface{}, 0, len(tokens))
	for _, token := range tokens {
		payload = append(payload, formatCITokenResponse(token))
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleCreateCIToken handles POST /api/ci-tokens
func (s *Server) handleCreateCIToken(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	var req CITokenCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Validate request
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if len(req.Projects) == 0 {
		writeError(w, http.StatusBadRequest, "at least one project is required")
		return
	}

	// Parse and validate project scopes
	projectScopes := make([]persistence.CITokenProjectScope, 0, len(req.Projects))
	projectIDs := make([]uuid.UUID, 0, len(req.Projects))
	for _, p := range req.Projects {
		projectID, err := uuid.Parse(p.ProjectID)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid project_id: "+p.ProjectID)
			return
		}
		if p.Scope != "read" && p.Scope != "write" {
			writeError(w, http.StatusBadRequest, "scope must be 'read' or 'write'")
			return
		}
		projectScopes = append(projectScopes, persistence.CITokenProjectScope{
			ProjectID: projectID,
			Scope:     p.Scope,
		})
		projectIDs = append(projectIDs, projectID)
	}

	// Verify user has write permission on all projects
	if err := s.store.VerifyUserHasWriteOnProjects(r.Context(), principal.OrgID, principal.UserID, projectIDs); err != nil {
		log.Printf("permission check failed for CI token creation: %v", err)
		writeError(w, http.StatusForbidden, "you must have write permission on all selected projects")
		return
	}

	// Create the token
	input := persistence.CITokenCreateInput{
		Name:         strings.TrimSpace(req.Name),
		Description:  strings.TrimSpace(req.Description),
		NeverExpires: req.NeverExpires,
		ExpiresAt:    req.ExpiresAt,
		Projects:     projectScopes,
	}

	tokenPlaintext, tokenRecord, err := s.store.CreateCIToken(r.Context(), principal.OrgID, principal.UserID, input)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("failed to create CI token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create CI token")
		return
	}

	// Return the plaintext token (shown once) along with the token record
	response := map[string]interface{}{
		"token":        tokenPlaintext,
		"token_record": formatCITokenResponse(tokenRecord),
	}

	writeJSON(w, http.StatusCreated, response)
}

// handleRevokeCIToken handles POST /api/ci-tokens/{tokenId}/revoke
func (s *Server) handleRevokeCIToken(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, tokenID uuid.UUID) {
	// Get the token to check project permissions
	token, err := s.store.GetCIToken(r.Context(), principal.OrgID, tokenID)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			writeError(w, http.StatusNotFound, "token not found")
			return
		}
		log.Printf("failed to get CI token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get token")
		return
	}

	// Verify user has write permission on all projects in the token
	projectIDs := make([]uuid.UUID, len(token.Projects))
	for i, p := range token.Projects {
		projectIDs[i] = p.ProjectID
	}

	if err := s.store.VerifyUserHasWriteOnProjects(r.Context(), principal.OrgID, principal.UserID, projectIDs); err != nil {
		log.Printf("permission check failed for CI token revocation: %v", err)
		writeError(w, http.StatusForbidden, "you must have write permission on all projects in this token")
		return
	}

	// Revoke the token
	if err := s.store.RevokeCIToken(r.Context(), principal.OrgID, tokenID, principal.UserID); err != nil {
		if strings.Contains(err.Error(), "not found") || strings.Contains(err.Error(), "already revoked") {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		log.Printf("failed to revoke CI token: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to revoke token")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// formatCITokenResponse formats a CITokenRecord for API response
func formatCITokenResponse(token persistence.CITokenRecord) map[string]interface{} {
	// Determine status
	var status string
	if token.RevokedAt.Valid {
		status = "revoked"
	} else if !token.NeverExpires && token.ExpiresAt.Valid && token.ExpiresAt.Time.Before(time.Now().UTC()) {
		status = "expired"
	} else {
		status = "active"
	}

	resp := map[string]interface{}{
		"id":            token.ID.String(),
		"name":          token.Name,
		"status":        status,
		"never_expires": token.NeverExpires,
		"created_at":    token.CreatedAt.Format(time.RFC3339),
		"updated_at":    token.UpdatedAt.Format(time.RFC3339),
	}

	if token.Description.Valid {
		resp["description"] = token.Description.String
	}

	if token.ExpiresAt.Valid {
		resp["expires_at"] = token.ExpiresAt.Time.Format(time.RFC3339)
	}

	if token.LastUsedAt.Valid {
		resp["last_used_at"] = token.LastUsedAt.Time.Format(time.RFC3339)
	}

	if token.RevokedAt.Valid {
		resp["revoked_at"] = token.RevokedAt.Time.Format(time.RFC3339)
	}

	if token.CreatedBy.Valid {
		resp["created_by"] = token.CreatedBy.UUID.String()
	}

	if token.RevokedBy.Valid {
		resp["revoked_by"] = token.RevokedBy.UUID.String()
	}

	// Format projects
	projects := make([]map[string]interface{}, 0, len(token.Projects))
	for _, p := range token.Projects {
		projects = append(projects, map[string]interface{}{
			"project_id":   p.ProjectID.String(),
			"project_name": p.ProjectName,
			"scope":        p.Scope,
		})
	}
	resp["projects"] = projects

	return resp
}
