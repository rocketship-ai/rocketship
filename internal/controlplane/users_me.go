package controlplane

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
)

// handleCurrentUser returns the current user's profile, roles, and pending registrations/invites
func (s *Server) handleCurrentUser(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	ctx := r.Context()
	summary, err := s.store.RoleSummary(ctx, principal.UserID)
	if err != nil {
		log.Printf("failed to load role summary: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load user state")
		return
	}

	roles := summary.AggregatedRoles()
	status := "ready"
	if len(roles) == 0 || (len(roles) == 1 && strings.EqualFold(roles[0], "pending")) {
		status = "pending"
	}

	resp := map[string]interface{}{
		"user": map[string]string{
			"id":       principal.UserID.String(),
			"email":    principal.Email,
			"name":     principal.Name,
			"username": principal.Username,
		},
		"roles":  roles,
		"status": status,
	}

	// Include organization info if user has one
	if principal.OrgID != uuid.Nil {
		org, err := s.store.GetOrganizationByID(ctx, principal.OrgID)
		if err == nil {
			resp["organization"] = map[string]string{
				"id":   org.ID.String(),
				"name": org.Name,
				"slug": org.Slug,
			}
		} else {
			log.Printf("failed to load organization for /api/users/me: %v", err)
			// Non-fatal: continue without org info
		}
	}

	if reg, err := s.store.LatestOrgRegistrationForUser(ctx, principal.UserID); err == nil {
		if reg.ExpiresAt.After(s.nowUTC()) {
			resp["pending_registration"] = map[string]interface{}{
				"registration_id":     reg.ID.String(),
				"org_name":            reg.OrgName,
				"email":               reg.Email,
				"expires_at":          reg.ExpiresAt.Format(time.RFC3339),
				"resend_available_at": reg.ResendAvailableAt.Format(time.RFC3339),
				"attempts":            reg.Attempts,
				"max_attempts":        reg.MaxAttempts,
			}
		} else {
			_ = s.store.DeleteOrgRegistration(ctx, reg.ID)
		}
	} else if err != nil && !errors.Is(err, sql.ErrNoRows) {
		log.Printf("failed to inspect pending registration: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to load registration state")
		return
	}

	if principal.Email != "" {
		invites, err := s.store.FindPendingOrgInvites(ctx, principal.Email)
		if err != nil {
			log.Printf("failed to list invites: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to load invites")
			return
		}
		if len(invites) > 0 {
			list := make([]map[string]interface{}, 0, len(invites))
			now := s.nowUTC()
			for _, inv := range invites {
				if inv.ExpiresAt.Before(now) {
					continue
				}
				list = append(list, map[string]interface{}{
					"invite_id":         inv.ID.String(),
					"organization_id":   inv.OrganizationID.String(),
					"organization_name": inv.OrganizationName,
					"role":              inv.Role,
					"expires_at":        inv.ExpiresAt.Format(time.RFC3339),
				})
			}
			if len(list) > 0 {
				resp["pending_invites"] = list
			}
		}
	}

	writeJSON(w, http.StatusOK, resp)
}
