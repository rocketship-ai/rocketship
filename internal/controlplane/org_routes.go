package controlplane

import (
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleOrgRoutes dispatches organization-scoped API requests to appropriate handlers
func (s *Server) handleOrgRoutes(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/orgs/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/orgs/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "organization id required")
		return
	}

	orgID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid organization id")
		return
	}

	if len(segments) < 2 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	switch segments[1] {
	case "invites":
		s.handleOrgInvites(w, r, principal, orgID, segments[2:])
	case "owners":
		s.handleOrgOwners(w, r, principal, orgID, segments[2:])
	case "project-members":
		s.handleOrgProjectMembers(w, r, principal, orgID, segments[2:])
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// handleOrgOwners lists organization owners
func (s *Server) handleOrgOwners(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, orgID uuid.UUID, tail []string) {
	if len(tail) > 0 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// Verify the requesting user is an org owner
	isOwner, err := s.store.IsOrganizationOwner(ctx, orgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to authorize request")
		return
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "owner role required")
		return
	}

	owners, err := s.store.ListOrganizationOwners(ctx, orgID)
	if err != nil {
		log.Printf("failed to list org owners: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list owners")
		return
	}

	// Convert to JSON-friendly format
	payload := make([]map[string]interface{}, 0, len(owners))
	for _, o := range owners {
		payload = append(payload, map[string]interface{}{
			"user_id":  o.UserID.String(),
			"email":    o.Email,
			"name":     o.Name,
			"username": o.Username,
			"added_at": o.AddedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleOrgProjectMembers lists all project members across an organization
func (s *Server) handleOrgProjectMembers(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, orgID uuid.UUID, tail []string) {
	if len(tail) > 0 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()

	// Verify the requesting user is an org owner
	isOwner, err := s.store.IsOrganizationOwner(ctx, orgID, principal.UserID)
	if err != nil {
		log.Printf("failed to check org owner: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to authorize request")
		return
	}
	if !isOwner {
		writeError(w, http.StatusForbidden, "owner role required")
		return
	}

	members, err := s.store.ListAllProjectMembers(ctx, orgID)
	if err != nil {
		log.Printf("failed to list all project members: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	// Convert to JSON-friendly format
	payload := make([]map[string]interface{}, 0, len(members))
	for _, m := range members {
		payload = append(payload, map[string]interface{}{
			"project_id":   m.ProjectID.String(),
			"project_name": m.ProjectName,
			"user_id":      m.UserID.String(),
			"username":     m.Username,
			"email":        m.Email,
			"name":         m.Name,
			"role":         m.Role,
		})
	}

	writeJSON(w, http.StatusOK, payload)
}
