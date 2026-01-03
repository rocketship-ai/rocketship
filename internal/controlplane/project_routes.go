package controlplane

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleProjectRoutes dispatches project-scoped API requests to appropriate handlers
// For member listing (GET), requires project access; for management (PUT/DELETE), requires org owner
func (s *Server) handleProjectRoutes(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/projects/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "project id required")
		return
	}

	projectID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	if len(segments) < 2 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	// Verify user has org membership
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	switch segments[1] {
	case "members":
		s.handleProjectMembers(w, r, principal, projectID, segments[2:])
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// handleProjectMembers manages project membership (list, update role, remove members)
// GET: Requires project access (read or write)
// PUT/DELETE: Requires org owner
// Note: Adding members is done via the invite flow (POST /api/project-invites)
func (s *Server) handleProjectMembers(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID, remainder []string) {
	ctx := r.Context()

	// For GET requests, verify project access
	// For management requests, verify org owner
	if r.Method == http.MethodGet {
		canAccess, err := s.store.UserCanAccessProject(ctx, principal.OrgID, principal.UserID, projectID)
		if err != nil {
			log.Printf("failed to check project access: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to check access")
			return
		}
		if !canAccess {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
	} else {
		// PUT/DELETE require org owner
		orgID, err := s.store.ProjectOrganizationID(ctx, projectID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "project not found")
				return
			}
			log.Printf("failed to resolve project organization: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to resolve project")
			return
		}

		isOwner, err := s.store.IsOrganizationOwner(ctx, orgID, principal.UserID)
		if err != nil {
			log.Printf("failed to verify organization owner: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to authorize request")
			return
		}
		if !isOwner {
			writeError(w, http.StatusForbidden, "owner role required")
			return
		}
	}

	if len(remainder) == 0 {
		switch r.Method {
		case http.MethodGet:
			s.handleListProjectMembers(w, r, principal, projectID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if len(remainder) != 1 {
		writeError(w, http.StatusNotFound, "resource not found")
		return
	}

	userID, err := uuid.Parse(remainder[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid user id")
		return
	}

	switch r.Method {
	case http.MethodPut:
		var body struct {
			Role string `json:"role"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}
		if err := s.store.SetProjectMemberRole(ctx, projectID, userID, body.Role); err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "membership not found")
			default:
				log.Printf("failed to update member role: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to update member")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	case http.MethodDelete:
		if err := s.store.RemoveProjectMember(ctx, projectID, userID); err != nil {
			switch {
			case errors.Is(err, sql.ErrNoRows):
				writeError(w, http.StatusNotFound, "membership not found")
			default:
				log.Printf("failed to remove project member: %v", err)
				writeError(w, http.StatusInternalServerError, "failed to remove member")
			}
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleListProjectMembers lists all members of a project
// Note: Access is verified in handleProjectMembers; any user with project access can view members
func (s *Server) handleListProjectMembers(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID uuid.UUID) {
	members, err := s.store.ListProjectMembers(r.Context(), projectID)
	if err != nil {
		log.Printf("failed to list project members: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list members")
		return
	}

	payload := make([]map[string]interface{}, 0, len(members))
	for _, m := range members {
		payload = append(payload, map[string]interface{}{
			"user_id":    m.UserID.String(),
			"email":      m.Email,
			"name":       m.Name,
			"username":   m.Username,
			"role":       m.Role,
			"joined_at":  m.JoinedAt.Format(time.RFC3339),
			"updated_at": m.UpdatedAt.Format(time.RFC3339),
		})
	}

	writeJSON(w, http.StatusOK, payload)
}

