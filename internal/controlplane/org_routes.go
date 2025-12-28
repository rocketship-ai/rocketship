package controlplane

import (
	"net/http"
	"strings"

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
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}
