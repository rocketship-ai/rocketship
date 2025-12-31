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

// handleRunDetail handles GET /api/runs/{runId}
func (s *Server) handleRunDetail(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	run, err := s.store.GetRun(r.Context(), principal.OrgID, runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		log.Printf("failed to get run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get run")
		return
	}

	payload := buildRunPayload(run)
	writeJSON(w, http.StatusOK, payload)
}

// handleRunTests handles GET /api/runs/{runId}/tests
func (s *Server) handleRunTests(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Verify run belongs to org
	_, err := s.store.GetRun(r.Context(), principal.OrgID, runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		log.Printf("failed to verify run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify run")
		return
	}

	tests, err := s.store.ListRunTests(r.Context(), runID)
	if err != nil {
		log.Printf("failed to list run tests: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list tests")
		return
	}

	payload := make([]map[string]interface{}, 0, len(tests))
	for _, test := range tests {
		item := map[string]interface{}{
			"id":           test.ID.String(),
			"run_id":       test.RunID,
			"workflow_id":  test.WorkflowID,
			"name":         test.Name,
			"status":       test.Status,
			"step_count":   test.StepCount,
			"passed_steps": test.PassedSteps,
			"failed_steps": test.FailedSteps,
			"created_at":   test.CreatedAt.Format(time.RFC3339),
		}
		if test.TestID.Valid {
			item["test_id"] = test.TestID.UUID.String()
		}
		if test.ErrorMessage.Valid {
			item["error_message"] = test.ErrorMessage.String
		}
		if test.StartedAt.Valid {
			item["started_at"] = test.StartedAt.Time.Format(time.RFC3339)
		}
		if test.EndedAt.Valid {
			item["ended_at"] = test.EndedAt.Time.Format(time.RFC3339)
		}
		if test.DurationMs.Valid {
			item["duration_ms"] = test.DurationMs.Int64
		}

		// Fetch step summaries for this test to enable step chips display
		steps, stepsErr := s.store.ListRunSteps(r.Context(), test.ID)
		if stepsErr == nil && len(steps) > 0 {
			stepSummaries := make([]map[string]interface{}, 0, len(steps))
			for _, step := range steps {
				stepSummary := map[string]interface{}{
					"step_index": step.StepIndex,
					"name":       step.Name,
					"plugin":     step.Plugin,
					"status":     step.Status,
				}
				stepSummaries = append(stepSummaries, stepSummary)
			}
			item["steps"] = stepSummaries
		} else {
			item["steps"] = []map[string]interface{}{}
		}

		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleRunLogs handles GET /api/runs/{runId}/logs?limit=500
func (s *Server) handleRunLogs(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runID string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Verify run belongs to org
	_, err := s.store.GetRun(r.Context(), principal.OrgID, runID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "run not found")
			return
		}
		log.Printf("failed to verify run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify run")
		return
	}

	limit := 500
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := s.store.ListRunLogs(r.Context(), runID, limit)
	if err != nil {
		log.Printf("failed to list run logs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list logs")
		return
	}

	payload := make([]map[string]interface{}, 0, len(logs))
	for _, logEntry := range logs {
		item := map[string]interface{}{
			"id":        logEntry.ID.String(),
			"run_id":    logEntry.RunID,
			"level":     logEntry.Level,
			"message":   logEntry.Message,
			"logged_at": logEntry.LoggedAt.Format(time.RFC3339),
		}
		if logEntry.RunTestID.Valid {
			item["run_test_id"] = logEntry.RunTestID.UUID.String()
		}
		if logEntry.RunStepID.Valid {
			item["run_step_id"] = logEntry.RunStepID.UUID.String()
		}
		if logEntry.Metadata != nil {
			item["metadata"] = logEntry.Metadata
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleTestRunDetail handles GET /api/test-runs/{runTestId}
func (s *Server) handleTestRunDetail(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runTestID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	result, err := s.store.GetRunTestWithRun(r.Context(), principal.OrgID, runTestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "test run not found")
			return
		}
		log.Printf("failed to get test run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get test run")
		return
	}

	test := result.RunTest
	run := result.Run

	testPayload := map[string]interface{}{
		"id":           test.ID.String(),
		"run_id":       test.RunID,
		"workflow_id":  test.WorkflowID,
		"name":         test.Name,
		"status":       test.Status,
		"step_count":   test.StepCount,
		"passed_steps": test.PassedSteps,
		"failed_steps": test.FailedSteps,
		"created_at":   test.CreatedAt.Format(time.RFC3339),
	}
	if test.TestID.Valid {
		testPayload["test_id"] = test.TestID.UUID.String()
	}
	if test.ErrorMessage.Valid {
		testPayload["error_message"] = test.ErrorMessage.String
	}
	if test.StartedAt.Valid {
		testPayload["started_at"] = test.StartedAt.Time.Format(time.RFC3339)
	}
	if test.EndedAt.Valid {
		testPayload["ended_at"] = test.EndedAt.Time.Format(time.RFC3339)
	}
	if test.DurationMs.Valid {
		testPayload["duration_ms"] = test.DurationMs.Int64
	}

	payload := map[string]interface{}{
		"test": testPayload,
		"run":  buildRunPayload(run),
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleTestRunLogs handles GET /api/test-runs/{runTestId}/logs?limit=500
func (s *Server) handleTestRunLogs(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runTestID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Verify test run belongs to org by getting parent run
	result, err := s.store.GetRunTestWithRun(r.Context(), principal.OrgID, runTestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "test run not found")
			return
		}
		log.Printf("failed to verify test run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify test run")
		return
	}
	_ = result // org check passed

	limit := 500
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	logs, err := s.store.ListRunLogsByTest(r.Context(), runTestID, limit)
	if err != nil {
		log.Printf("failed to list test run logs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list logs")
		return
	}

	payload := make([]map[string]interface{}, 0, len(logs))
	for _, logEntry := range logs {
		item := map[string]interface{}{
			"id":          logEntry.ID.String(),
			"run_id":      logEntry.RunID,
			"run_test_id": runTestID.String(),
			"level":       logEntry.Level,
			"message":     logEntry.Message,
			"logged_at":   logEntry.LoggedAt.Format(time.RFC3339),
		}
		if logEntry.RunStepID.Valid {
			item["run_step_id"] = logEntry.RunStepID.UUID.String()
		}
		if logEntry.Metadata != nil {
			item["metadata"] = logEntry.Metadata
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleRunRoutesDispatch dispatches /api/runs/* routes
func (s *Server) handleRunRoutesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/runs/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/runs/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "run ID required")
		return
	}

	runID := segments[0]

	// If just run ID, return run detail
	if len(segments) == 1 {
		s.handleRunDetail(w, r, principal, runID)
		return
	}

	// Handle sub-resources
	switch segments[1] {
	case "tests":
		s.handleRunTests(w, r, principal, runID)
	case "logs":
		s.handleRunLogs(w, r, principal, runID)
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// handleTestRunSteps handles GET /api/test-runs/{runTestId}/steps
func (s *Server) handleTestRunSteps(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, runTestID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Verify test run belongs to org by getting parent run
	result, err := s.store.GetRunTestWithRun(r.Context(), principal.OrgID, runTestID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "test run not found")
			return
		}
		log.Printf("failed to verify test run: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to verify test run")
		return
	}
	_ = result // org check passed

	steps, err := s.store.ListRunSteps(r.Context(), runTestID)
	if err != nil {
		log.Printf("failed to list test run steps: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list steps")
		return
	}

	payload := make([]map[string]interface{}, 0, len(steps))
	for _, step := range steps {
		item := map[string]interface{}{
			"id":                step.ID.String(),
			"run_test_id":       step.RunTestID.String(),
			"step_index":        step.StepIndex,
			"name":              step.Name,
			"plugin":            step.Plugin,
			"status":            step.Status,
			"assertions_passed": step.AssertionsPassed,
			"assertions_failed": step.AssertionsFailed,
			"created_at":        step.CreatedAt.Format(time.RFC3339),
		}
		if step.ErrorMessage.Valid {
			item["error_message"] = step.ErrorMessage.String
		}
		if step.StartedAt.Valid {
			item["started_at"] = step.StartedAt.Time.Format(time.RFC3339)
		}
		if step.EndedAt.Valid {
			item["ended_at"] = step.EndedAt.Time.Format(time.RFC3339)
		}
		if step.DurationMs.Valid {
			item["duration_ms"] = step.DurationMs.Int64
		}
		// Include request/response data for HTTP plugin steps (UI display)
		if len(step.RequestData) > 0 {
			item["request_data"] = step.RequestData
		}
		if len(step.ResponseData) > 0 {
			item["response_data"] = step.ResponseData
		}
		// Include extended data for rich step details UI
		if len(step.AssertionsData) > 0 {
			item["assertions_data"] = step.AssertionsData
		}
		if len(step.VariablesData) > 0 {
			item["variables_data"] = step.VariablesData
		}
		if len(step.StepConfig) > 0 {
			item["step_config"] = step.StepConfig
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleTestRunRoutesDispatch dispatches /api/test-runs/* routes
func (s *Server) handleTestRunRoutesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/test-runs/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/test-runs/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "test run ID required")
		return
	}

	runTestID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid test run ID")
		return
	}

	// If just test run ID, return test run detail
	if len(segments) == 1 {
		s.handleTestRunDetail(w, r, principal, runTestID)
		return
	}

	// Handle sub-resources
	switch segments[1] {
	case "logs":
		s.handleTestRunLogs(w, r, principal, runTestID)
	case "steps":
		s.handleTestRunSteps(w, r, principal, runTestID)
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// buildRunPayload converts a RunRecord to a JSON-friendly map
func buildRunPayload(run persistence.RunRecord) map[string]interface{} {
	payload := map[string]interface{}{
		"id":            run.ID,
		"status":        run.Status,
		"suite_name":    run.SuiteName,
		"initiator":     run.Initiator,
		"trigger":       run.Trigger,
		"schedule_name": run.ScheduleName,
		"config_source": run.ConfigSource,
		"source":        run.Source,
		"branch":        run.Branch,
		"environment":   run.Environment,
		"total_tests":   run.TotalTests,
		"passed_tests":  run.PassedTests,
		"failed_tests":  run.FailedTests,
		"timeout_tests": run.TimeoutTests,
		"skipped_tests": run.SkippedTests,
		"created_at":    run.CreatedAt.Format(time.RFC3339),
		"updated_at":    run.UpdatedAt.Format(time.RFC3339),
	}

	if run.ProjectID.Valid {
		payload["project_id"] = run.ProjectID.UUID.String()
	}
	if run.CommitSHA.Valid {
		payload["commit_sha"] = run.CommitSHA.String
	}
	if run.BundleSHA.Valid {
		payload["bundle_sha"] = run.BundleSHA.String
	}
	if run.CommitMessage.Valid {
		payload["commit_message"] = run.CommitMessage.String
	}
	if run.EnvironmentID.Valid {
		payload["environment_id"] = run.EnvironmentID.UUID.String()
	}
	if run.ScheduleID.Valid {
		payload["schedule_id"] = run.ScheduleID.UUID.String()
	}
	if run.StartedAt.Valid {
		payload["started_at"] = run.StartedAt.Time.Format(time.RFC3339)
	}
	if run.EndedAt.Valid {
		payload["ended_at"] = run.EndedAt.Time.Format(time.RFC3339)
	}

	// Compute duration_ms if both started and ended are present
	if run.StartedAt.Valid && run.EndedAt.Valid {
		payload["duration_ms"] = run.EndedAt.Time.Sub(run.StartedAt.Time).Milliseconds()
	}

	// Derive initiator_type
	var initiatorType string
	if run.ScheduleID.Valid || run.ScheduleName != "" {
		initiatorType = "schedule"
	} else if run.Trigger == "webhook" || run.Source == "ci-branch" {
		initiatorType = "ci"
	} else {
		initiatorType = "manual"
	}
	payload["initiator_type"] = initiatorType

	// For manual runs, include the initiator name
	if initiatorType == "manual" && run.Initiator != "" {
		payload["initiator_name"] = run.Initiator
	}

	return payload
}
