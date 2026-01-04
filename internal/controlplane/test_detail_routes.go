package controlplane

import (
	"database/sql"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleTestDetail handles GET /api/tests/{testId}
func (s *Server) handleTestDetail(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, testID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	test, err := s.store.GetTestDetail(r.Context(), principal.OrgID, testID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "test not found")
			return
		}
		log.Printf("failed to get test detail: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get test")
		return
	}

	// Check project access
	canAccess, err := s.store.UserCanAccessProject(r.Context(), principal.OrgID, principal.UserID, test.ProjectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "test not found")
		return
	}

	// Build response payload
	payload := map[string]interface{}{
		"id":          test.ID.String(),
		"name":        test.Name,
		"source_ref":  test.SourceRef,
		"step_count":  test.StepCount,
		"suite_id":    test.SuiteID.String(),
		"suite_name":  test.SuiteName,
		"project_id":  test.ProjectID.String(),
		"project_name": test.ProjectName,
		"created_at":  test.CreatedAt.Format(time.RFC3339),
		"updated_at":  test.UpdatedAt.Format(time.RFC3339),
	}

	if test.Description.Valid {
		payload["description"] = test.Description.String
	}

	// Include enriched step summaries
	// Recompute step_index from array order (0-based) to match run_steps convention
	steps := make([]map[string]interface{}, 0, len(test.StepSummaries))
	for i, step := range test.StepSummaries {
		stepPayload := map[string]interface{}{
			"step_index": i, // 0-based, matching run_steps.step_index
			"plugin":     step.Plugin,
			"name":       step.Name,
		}
		if step.Config != nil {
			stepPayload["config"] = step.Config
		}
		if step.Assertions != nil {
			stepPayload["assertions"] = step.Assertions
		}
		if step.Save != nil {
			stepPayload["save"] = step.Save
		}
		if step.Retry != nil {
			stepPayload["retry"] = step.Retry
		}
		steps = append(steps, stepPayload)
	}
	payload["steps"] = steps

	writeJSON(w, http.StatusOK, payload)
}

// handleTestRuns handles GET /api/tests/{testId}/runs
func (s *Server) handleTestRuns(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, testID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// First verify the test exists and user has access
	test, err := s.store.GetTestDetail(r.Context(), principal.OrgID, testID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "test not found")
			return
		}
		log.Printf("failed to verify test: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify test")
		return
	}

	// Check project access
	canAccess, err := s.store.UserCanAccessProject(r.Context(), principal.OrgID, principal.UserID, test.ProjectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "test not found")
		return
	}

	// Parse query parameters
	params := persistence.TestRunsParams{}

	// Parse triggers filter
	if triggersStr := r.URL.Query().Get("triggers"); triggersStr != "" {
		params.Triggers = strings.Split(triggersStr, ",")
	}

	// Parse environment filter
	if envIDStr := r.URL.Query().Get("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err == nil {
			params.EnvironmentID = uuid.NullUUID{UUID: envID, Valid: true}
		}
	}

	// Parse limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil && limit > 0 {
			params.Limit = limit
		}
	}

	// Parse offset
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil && offset >= 0 {
			params.Offset = offset
		}
	}

	runs, err := s.store.ListTestRuns(r.Context(), principal.OrgID, testID, params)
	if err != nil {
		log.Printf("failed to list test runs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list runs")
		return
	}

	// Build response payload
	payload := make([]map[string]interface{}, 0, len(runs))
	for _, run := range runs {
		item := map[string]interface{}{
			"id":          run.ID.String(),
			"run_id":      run.RunID,
			"status":      run.Status,
			"trigger":     run.Trigger,
			"environment": run.Environment,
			"branch":      run.Branch,
			"initiator":   run.Initiator,
			"created_at":  run.CreatedAt.Format(time.RFC3339),
		}
		if run.DurationMs.Valid {
			item["duration_ms"] = run.DurationMs.Int64
		}
		if run.StartedAt.Valid {
			item["started_at"] = run.StartedAt.Time.Format(time.RFC3339)
		}
		if run.EndedAt.Valid {
			item["ended_at"] = run.EndedAt.Time.Format(time.RFC3339)
		}
		if run.CommitSHA.Valid {
			item["commit_sha"] = run.CommitSHA.String
		}

		// Parse initiator_name from initiator field
		// Format: "user:<github_username>" for manual runs
		if run.Trigger == "manual" && run.Initiator != "" {
			item["initiator_name"] = strings.TrimPrefix(run.Initiator, "user:")
		}

		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleTestRoutesDispatch dispatches /api/tests/* routes
func (s *Server) handleTestRoutesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/tests/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/tests/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "test ID required")
		return
	}

	testID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid test ID")
		return
	}

	// If just test ID, return test detail
	if len(segments) == 1 {
		s.handleTestDetail(w, r, principal, testID)
		return
	}

	// Handle sub-resources
	switch segments[1] {
	case "runs":
		s.handleTestRuns(w, r, principal, testID)
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}
