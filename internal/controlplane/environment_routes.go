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
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// EnvironmentCreateRequest is the request body for creating an environment
type EnvironmentCreateRequest struct {
	Name       string                 `json:"name"`
	Slug       string                 `json:"slug"`
	EnvSecrets map[string]string      `json:"env_secrets,omitempty"`
	ConfigVars map[string]interface{} `json:"config_vars,omitempty"`
}

// EnvironmentUpdateRequest is the request body for updating an environment
type EnvironmentUpdateRequest struct {
	Name       string                 `json:"name,omitempty"`
	Slug       string                 `json:"slug,omitempty"`
	EnvSecrets map[string]string      `json:"env_secrets,omitempty"`
	ConfigVars map[string]interface{} `json:"config_vars,omitempty"`
}

// handleProjectEnvironments handles all /api/projects/{projectId}/environments routes
func (s *Server) handleProjectEnvironments(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID, segments []string) {
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Check project access
	canAccess, err := s.store.UserCanAccessProject(r.Context(), principal.OrgID, principal.UserID, projectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "project not found")
		return
	}

	// /api/projects/{projectId}/environments
	if len(segments) == 0 || segments[0] == "" {
		switch r.Method {
		case http.MethodGet:
			s.handleListEnvironments(w, r, principal, projectID)
		case http.MethodPost:
			s.handleCreateEnvironment(w, r, principal, projectID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	envID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment id")
		return
	}

	// /api/projects/{projectId}/environments/{envId}
	if len(segments) == 1 {
		switch r.Method {
		case http.MethodGet:
			s.handleGetEnvironment(w, r, principal, projectID, envID)
		case http.MethodPut:
			s.handleUpdateEnvironment(w, r, principal, projectID, envID)
		case http.MethodDelete:
			s.handleDeleteEnvironment(w, r, principal, projectID, envID)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// handleListEnvironments handles GET /api/projects/{projectId}/environments
func (s *Server) handleListEnvironments(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID uuid.UUID) {
	envs, err := s.store.ListEnvironments(r.Context(), projectID)
	if err != nil {
		log.Printf("failed to list environments: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list environments")
		return
	}

	payload := make([]map[string]interface{}, 0, len(envs))
	for _, env := range envs {
		payload = append(payload, formatEnvironmentResponse(env))
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleCreateEnvironment handles POST /api/projects/{projectId}/environments
func (s *Server) handleCreateEnvironment(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID uuid.UUID) {
	var req EnvironmentCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return
	}
	if strings.TrimSpace(req.Slug) == "" {
		writeError(w, http.StatusBadRequest, "slug is required")
		return
	}

	env := persistence.ProjectEnvironment{
		ProjectID:  projectID,
		Name:       strings.TrimSpace(req.Name),
		Slug:       strings.ToLower(strings.TrimSpace(req.Slug)),
		EnvSecrets: req.EnvSecrets,
		ConfigVars: req.ConfigVars,
	}

	created, err := s.store.CreateEnvironment(r.Context(), env)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("failed to create environment: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create environment")
		return
	}

	writeJSON(w, http.StatusCreated, formatEnvironmentResponse(created))
}

// handleGetEnvironment handles GET /api/projects/{projectId}/environments/{envId}
func (s *Server) handleGetEnvironment(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID, envID uuid.UUID) {
	env, err := s.store.GetEnvironment(r.Context(), projectID, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		log.Printf("failed to get environment: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get environment")
		return
	}

	writeJSON(w, http.StatusOK, formatEnvironmentResponse(env))
}

// handleUpdateEnvironment handles PUT /api/projects/{projectId}/environments/{envId}
func (s *Server) handleUpdateEnvironment(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID, envID uuid.UUID) {
	// First get the existing environment
	existing, err := s.store.GetEnvironment(r.Context(), projectID, envID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		log.Printf("failed to get environment: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get environment")
		return
	}

	var req EnvironmentUpdateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	// Update fields if provided
	if req.Name != "" {
		existing.Name = strings.TrimSpace(req.Name)
	}
	if req.Slug != "" {
		existing.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	}

	// For env_secrets, we merge with existing (allow partial updates)
	// If a key is provided with empty string, we could remove it; otherwise merge
	if req.EnvSecrets != nil {
		if existing.EnvSecrets == nil {
			existing.EnvSecrets = make(map[string]string)
		}
		for k, v := range req.EnvSecrets {
			if v == "" {
				delete(existing.EnvSecrets, k)
			} else {
				existing.EnvSecrets[k] = v
			}
		}
	}

	// For config_vars, replace entirely if provided
	if req.ConfigVars != nil {
		existing.ConfigVars = req.ConfigVars
	}

	updated, err := s.store.UpdateEnvironment(r.Context(), existing)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		log.Printf("failed to update environment: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to update environment")
		return
	}

	writeJSON(w, http.StatusOK, formatEnvironmentResponse(updated))
}

// handleDeleteEnvironment handles DELETE /api/projects/{projectId}/environments/{envId}
func (s *Server) handleDeleteEnvironment(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, projectID, envID uuid.UUID) {
	if err := s.store.DeleteEnvironment(r.Context(), projectID, envID); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "environment not found")
			return
		}
		log.Printf("failed to delete environment: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to delete environment")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// formatEnvironmentResponse formats a ProjectEnvironment for API response
// Secret values are NOT returned; only keys are exposed
func formatEnvironmentResponse(env persistence.ProjectEnvironment) map[string]interface{} {
	resp := map[string]interface{}{
		"id":         env.ID.String(),
		"project_id": env.ProjectID.String(),
		"name":       env.Name,
		"slug":       env.Slug,
		"created_at": env.CreatedAt.Format(time.RFC3339),
		"updated_at": env.UpdatedAt.Format(time.RFC3339),
	}

	// Return only secret keys, not values
	secretKeys := make([]string, 0, len(env.EnvSecrets))
	for k := range env.EnvSecrets {
		secretKeys = append(secretKeys, k)
	}
	resp["env_secrets_keys"] = secretKeys

	// Return full config vars (values are visible)
	if env.ConfigVars == nil {
		resp["config_vars"] = map[string]interface{}{}
	} else {
		resp["config_vars"] = env.ConfigVars
	}

	return resp
}
