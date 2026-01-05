package controlplane

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleConsoleProjects handles GET /api/projects (list projects the user can access)
func (s *Server) handleConsoleProjects(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Use scoped query - only returns projects user can access
	projects, err := s.store.ListProjectSummariesForUser(r.Context(), principal.OrgID, principal.UserID)
	if err != nil {
		log.Printf("failed to list project summaries: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list projects")
		return
	}

	payload := make([]map[string]interface{}, 0, len(projects))
	for _, p := range projects {
		item := map[string]interface{}{
			"id":             p.ID.String(),
			"name":           p.Name,
			"repo_url":       p.RepoURL,
			"default_branch": p.DefaultBranch,
			"path_scope":     p.PathScope,
			"source_ref":     p.SourceRef,
			"suite_count":    p.SuiteCount,
			"test_count":     p.TestCount,
		}
		if p.LastScan != nil {
			item["last_scan"] = map[string]interface{}{
				"status":        p.LastScan.Status,
				"created_at":    p.LastScan.CreatedAt.Format(time.RFC3339),
				"head_sha":      p.LastScan.HeadSHA,
				"error_message": p.LastScan.ErrorMessage,
				"suites_found":  p.LastScan.SuitesFound,
				"tests_found":   p.LastScan.TestsFound,
			}
		} else {
			item["last_scan"] = nil
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleConsoleProjectDetail handles GET /api/projects/{projectId}
func (s *Server) handleConsoleProjectDetail(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

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

	project, err := s.store.GetProjectWithOrgCheck(r.Context(), principal.OrgID, projectID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "project not found")
			return
		}
		log.Printf("failed to get project: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	// Get canonical suite and test counts (deduped by file_path, prefer default branch)
	suites, err := s.store.ListSuitesForProjectCanonical(r.Context(), projectID)
	if err != nil {
		log.Printf("failed to list canonical suites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get project details")
		return
	}

	suiteCount := len(suites)
	testCount := 0
	for _, suite := range suites {
		testCount += suite.TestCount
	}

	// Get latest scan
	lastScan, err := s.store.GetLatestScanForProject(r.Context(), principal.OrgID, project.RepoURL, project.SourceRef)
	if err != nil {
		log.Printf("failed to get latest scan: %v", err)
		// Non-fatal, continue without scan info
	}

	payload := map[string]interface{}{
		"id":             project.ID.String(),
		"name":           project.Name,
		"repo_url":       project.RepoURL,
		"default_branch": project.DefaultBranch,
		"path_scope":     project.PathScope,
		"source_ref":     project.SourceRef,
		"suite_count":    suiteCount,
		"test_count":     testCount,
	}

	if lastScan != nil {
		payload["last_scan"] = map[string]interface{}{
			"status":        lastScan.Status,
			"created_at":    lastScan.CreatedAt.Format(time.RFC3339),
			"head_sha":      lastScan.HeadSHA,
			"error_message": lastScan.ErrorMessage,
			"suites_found":  lastScan.SuitesFound,
			"tests_found":   lastScan.TestsFound,
		}
	} else {
		payload["last_scan"] = nil
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleConsoleProjectSuites handles GET /api/projects/{projectId}/suites
func (s *Server) handleConsoleProjectSuites(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

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

	// Use canonical suites list (deduped by file_path, prefer default branch)
	suites, err := s.store.ListSuitesForProjectCanonical(r.Context(), projectID)
	if err != nil {
		log.Printf("failed to list canonical suites: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list suites")
		return
	}

	payload := make([]map[string]interface{}, 0, len(suites))
	for _, suite := range suites {
		item := map[string]interface{}{
			"id":         suite.ID.String(),
			"name":       suite.Name,
			"source_ref": suite.SourceRef,
			"test_count": suite.TestCount,
		}
		if suite.Description.Valid {
			item["description"] = suite.Description.String
		}
		if suite.FilePath.Valid {
			item["file_path"] = suite.FilePath.String
		}
		if suite.LastRunStatus.Valid {
			item["last_run_status"] = suite.LastRunStatus.String
		}
		if suite.LastRunAt.Valid {
			item["last_run_at"] = suite.LastRunAt.Time.Format(time.RFC3339)
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleSuiteActivity handles GET /api/suites/activity
func (s *Server) handleSuiteActivity(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Use scoped query - only returns suites from projects user can access
	suites, err := s.store.ListSuitesForUserProjects(r.Context(), principal.OrgID, principal.UserID, 100)
	if err != nil {
		log.Printf("failed to list suites for org: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list suites")
		return
	}

	payload := make([]map[string]interface{}, 0, len(suites))
	for _, suite := range suites {
		item := map[string]interface{}{
			"suite_id":   suite.SuiteID.String(),
			"name":       suite.SuiteName,
			"source_ref": suite.SourceRef,
			"test_count": suite.TestCount,
			"project": map[string]interface{}{
				"id":       suite.ProjectID.String(),
				"name":     suite.ProjectName,
				"repo_url": suite.RepoURL,
			},
		}
		if suite.Description.Valid {
			item["description"] = suite.Description.String
		}
		if suite.FilePath.Valid {
			item["file_path"] = suite.FilePath.String
		}
		if suite.LastRunStatus.Valid {
			item["last_run"] = map[string]interface{}{
				"status": suite.LastRunStatus.String,
			}
			if suite.LastRunAt.Valid {
				item["last_run"].(map[string]interface{})["at"] = suite.LastRunAt.Time.Format(time.RFC3339)
			}
		} else {
			item["last_run"] = map[string]interface{}{
				"status": nil,
				"at":     nil,
			}
		}
		// Aggregate metrics (null if no run history)
		if suite.MedianDurationMs.Valid {
			item["median_duration_ms"] = suite.MedianDurationMs.Int64
		} else {
			item["median_duration_ms"] = nil
		}
		if suite.ReliabilityPct.Valid {
			item["reliability_pct"] = suite.ReliabilityPct.Float64
		} else {
			item["reliability_pct"] = nil
		}
		if suite.RunsPerWeek.Valid {
			item["runs_per_week"] = suite.RunsPerWeek.Int64
		} else {
			item["runs_per_week"] = nil
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleSuiteRuns handles GET /api/suites/{suiteId}/runs
// Returns run history for a suite across all branches/refs
// Query params:
//   - environment_id: (uuid) filter runs by environment
//   - branch: (string) when set, enter branch drilldown mode with pagination
//   - limit: (int) runs per page in branch mode (default 10, max 50)
//   - offset: (int) pagination offset in branch mode
//   - runs_per_branch: (int) runs per branch in summary mode (default 5, max 20)
//   - triggers: (csv) filter by trigger types (ci, manual, schedule)
//   - search: (string) search in commit message and SHA
func (s *Server) handleSuiteRuns(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, suiteID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	query := r.URL.Query()

	// Get suite detail (includes org check)
	suite, err := s.store.GetSuiteDetail(r.Context(), principal.OrgID, suiteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "suite not found")
			return
		}
		log.Printf("failed to get suite detail: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get suite")
		return
	}

	// Check project access for the suite's project
	canAccess, err := s.store.UserCanAccessProject(r.Context(), principal.OrgID, principal.UserID, suite.ProjectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "suite not found")
		return
	}

	// Get project to obtain repo_url and path_scope
	project, err := s.store.GetProjectWithOrgCheck(r.Context(), principal.OrgID, suite.ProjectID)
	if err != nil {
		log.Printf("failed to get project for suite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get project")
		return
	}

	// Find all project IDs in this org with the same repo/path_scope (same .rocketship directory)
	projectIDs, err := s.store.ListProjectIDsByRepoAndPathScope(r.Context(), principal.OrgID, project.RepoURL, project.PathScope)
	if err != nil {
		log.Printf("failed to list project IDs: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to find related projects")
		return
	}

	// Build filter from query params
	var filter persistence.SuiteRunsFilter

	// Environment filter: resolve UUID → slug for cross-project filtering
	if envIDStr := query.Get("environment_id"); envIDStr != "" {
		envID, err := uuid.Parse(envIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		// Look up environment to get its slug
		env, err := s.store.GetEnvironment(r.Context(), suite.ProjectID, envID)
		if err == nil {
			filter.EnvironmentSlug = env.Slug
		}
		// If env not found, filter.EnvironmentSlug remains empty (returns no runs, which is correct)
	}

	// Trigger filter: parse CSV, validate values
	if triggersStr := query.Get("triggers"); triggersStr != "" {
		validTriggers := map[string]bool{"ci": true, "manual": true, "schedule": true}
		for _, t := range strings.Split(triggersStr, ",") {
			t = strings.ToLower(strings.TrimSpace(t))
			if validTriggers[t] {
				filter.Triggers = append(filter.Triggers, t)
			}
		}
	}

	// Search filter
	filter.Search = strings.TrimSpace(query.Get("search"))

	// Branch param determines mode: summary (all branches) vs branch (single branch with pagination)
	branch := strings.TrimSpace(query.Get("branch"))

	var responsePayload map[string]interface{}

	if branch != "" {
		// Branch drilldown mode with pagination
		limit := 10
		if limitStr := query.Get("limit"); limitStr != "" {
			if _, err := stringToInt(limitStr, &limit); err != nil || limit <= 0 {
				limit = 10
			}
		}
		if limit > 50 {
			limit = 50
		}

		offset := 0
		if offsetStr := query.Get("offset"); offsetStr != "" {
			if _, err := stringToInt(offsetStr, &offset); err != nil || offset < 0 {
				offset = 0
			}
		}

		result, err := s.store.ListRunsForSuiteBranch(r.Context(), principal.OrgID, projectIDs, suite.Name, branch, filter, limit, offset)
		if err != nil {
			log.Printf("failed to list runs for suite branch: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to list runs")
			return
		}

		runsPayload := s.formatSuiteRuns(result.Runs)
		responsePayload = map[string]interface{}{
			"mode":   "branch",
			"runs":   runsPayload,
			"total":  result.Total,
			"limit":  result.Limit,
			"offset": result.Offset,
			"branch": result.Branch,
		}
	} else {
		// Summary mode: multiple branches
		runsPerBranch := 5
		if rStr := query.Get("runs_per_branch"); rStr != "" {
			if _, err := stringToInt(rStr, &runsPerBranch); err != nil || runsPerBranch <= 0 {
				runsPerBranch = 5
			}
		}
		if runsPerBranch > 20 {
			runsPerBranch = 20
		}

		runs, err := s.store.ListRunsForSuiteGroup(r.Context(), principal.OrgID, projectIDs, suite.Name, project.DefaultBranch, runsPerBranch, filter)
		if err != nil {
			log.Printf("failed to list runs for suite: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to list runs")
			return
		}

		runsPayload := s.formatSuiteRuns(runs)
		responsePayload = map[string]interface{}{
			"mode":   "summary",
			"runs":   runsPayload,
			"limit":  runsPerBranch,
			"offset": 0,
		}
	}

	writeJSON(w, http.StatusOK, responsePayload)
}

// formatSuiteRuns converts SuiteRunRow slice to API response format
func (s *Server) formatSuiteRuns(runs []persistence.SuiteRunRow) []map[string]interface{} {
	payload := make([]map[string]interface{}, 0, len(runs))
	for _, run := range runs {
		item := map[string]interface{}{
			"id":            run.ID,
			"status":        run.Status,
			"branch":        run.Branch,
			"environment":   run.Environment,
			"config_source": run.ConfigSource,
			"created_at":    run.CreatedAt.Format(time.RFC3339),
			"total_tests":   run.TotalTests,
			"passed_tests":  run.PassedTests,
			"failed_tests":  run.FailedTests,
			"timeout_tests": run.TimeoutTests,
			"skipped_tests": run.SkippedTests,
		}

		// Nullable fields
		if run.CommitSHA.Valid {
			item["commit_sha"] = run.CommitSHA.String
		}
		if run.CommitMessage.Valid {
			item["commit_message"] = run.CommitMessage.String
		}
		if run.StartedAt.Valid {
			item["started_at"] = run.StartedAt.Time.Format(time.RFC3339)
		}
		if run.EndedAt.Valid {
			item["ended_at"] = run.EndedAt.Time.Format(time.RFC3339)
		}

		// Compute duration_ms if both started and ended are present
		if run.StartedAt.Valid && run.EndedAt.Valid {
			item["duration_ms"] = run.EndedAt.Time.Sub(run.StartedAt.Time).Milliseconds()
		}

		// initiator_type is now the authoritative trigger value from DB
		// After migration 0029, trigger is always one of: 'manual', 'ci', 'schedule'
		initiatorType := run.Trigger
		if initiatorType == "" {
			initiatorType = "manual" // fallback for very old runs before trigger was set
		}
		item["initiator_type"] = initiatorType

		// For manual runs, parse initiator to extract username
		// New format: "user:<github_username>" → extract just the username
		if initiatorType == "manual" && run.Initiator != "" {
			item["initiator_name"] = strings.TrimPrefix(run.Initiator, "user:")
		}

		payload = append(payload, item)
	}
	return payload
}

// handleSuiteDetail handles GET /api/suites/{suiteId}
func (s *Server) handleSuiteDetail(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, suiteID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	suite, err := s.store.GetSuiteDetail(r.Context(), principal.OrgID, suiteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "suite not found")
			return
		}
		log.Printf("failed to get suite detail: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get suite")
		return
	}

	// Check project access for the suite's project
	canAccess, err := s.store.UserCanAccessProject(r.Context(), principal.OrgID, principal.UserID, suite.ProjectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "suite not found")
		return
	}

	tests, err := s.store.ListTestsBySuite(r.Context(), suiteID)
	if err != nil {
		log.Printf("failed to list tests: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get suite tests")
		return
	}

	testsPayload := make([]map[string]interface{}, 0, len(tests))
	for _, test := range tests {
		item := map[string]interface{}{
			"id":         test.ID.String(),
			"name":       test.Name,
			"source_ref": test.SourceRef,
			"step_count": test.StepCount,
		}

		// Always include step_summaries (empty array if none)
		stepSummaries := make([]map[string]interface{}, 0, len(test.StepSummaries))
		for _, s := range test.StepSummaries {
			stepSummaries = append(stepSummaries, map[string]interface{}{
				"step_index": s.StepIndex,
				"plugin":     s.Plugin,
				"name":       s.Name,
			})
		}
		item["step_summaries"] = stepSummaries

		if test.Description.Valid {
			item["description"] = test.Description.String
		}
		if test.LastRunStatus.Valid {
			item["last_run_status"] = test.LastRunStatus.String
		}
		if test.LastRunAt.Valid {
			item["last_run_at"] = test.LastRunAt.Time.Format(time.RFC3339)
		}
		if test.PassRate.Valid {
			item["pass_rate"] = test.PassRate.Float64
		}
		if test.AvgDurationMs.Valid {
			item["avg_duration_ms"] = test.AvgDurationMs.Int64
		}
		testsPayload = append(testsPayload, item)
	}

	payload := map[string]interface{}{
		"id":         suite.ID.String(),
		"name":       suite.Name,
		"source_ref": suite.SourceRef,
		"test_count": suite.TestCount,
		"tests":      testsPayload,
		"project": map[string]interface{}{
			"id":       suite.ProjectID.String(),
			"name":     suite.ProjectName,
			"repo_url": suite.RepoURL,
		},
	}
	if suite.Description.Valid {
		payload["description"] = suite.Description.String
	}
	if suite.FilePath.Valid {
		payload["file_path"] = suite.FilePath.String
	}
	if suite.LastRunStatus.Valid {
		payload["last_run_status"] = suite.LastRunStatus.String
	}
	if suite.LastRunAt.Valid {
		payload["last_run_at"] = suite.LastRunAt.Time.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleConsoleProjectRoutesDispatch extends handleProjectRoutes to support console APIs
// This is called for /api/projects/* and dispatches to the appropriate handler
func (s *Server) handleConsoleProjectRoutesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/projects/") {
		// If it's exactly /api/projects (list all), handle it here
		if r.URL.Path == "/api/projects" || r.URL.Path == "/api/projects/" {
			s.handleConsoleProjects(w, r, principal)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/projects/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		// /api/projects/ -> list all
		s.handleConsoleProjects(w, r, principal)
		return
	}

	projectID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid project id")
		return
	}

	// If just project ID, return project detail
	if len(segments) == 1 {
		s.handleConsoleProjectDetail(w, r, principal, projectID)
		return
	}

	// Route to sub-resources
	switch segments[1] {
	case "suites":
		s.handleConsoleProjectSuites(w, r, principal, projectID)
	case "members":
		// Delegate to existing members handler (requires org admin)
		s.handleProjectRoutes(w, r, principal)
	case "environments":
		// Handle environment management
		s.handleProjectEnvironments(w, r, principal, projectID, segments[2:])
	case "schedules":
		// List all project schedules
		s.handleProjectSchedules(w, r, principal, projectID)
	case "project-schedules":
		// Create project schedule
		s.handleCreateProjectSchedule(w, r, principal, projectID)
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// handleConsoleSuiteRoutesDispatch handles /api/suites/* routes
func (s *Server) handleConsoleSuiteRoutesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/suites/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/suites/")
	segments := strings.Split(strings.Trim(trimmed, "/"), "/")
	if len(segments) == 0 || segments[0] == "" {
		writeError(w, http.StatusNotFound, "suite resource required")
		return
	}

	// Handle special routes first
	if segments[0] == "activity" {
		s.handleSuiteActivity(w, r, principal)
		return
	}

	// Parse suite ID
	suiteID, err := uuid.Parse(segments[0])
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid suite id")
		return
	}

	// If just suite ID, return suite detail
	if len(segments) == 1 {
		s.handleSuiteDetail(w, r, principal, suiteID)
		return
	}

	// Handle sub-resources
	switch segments[1] {
	case "runs":
		s.handleSuiteRuns(w, r, principal, suiteID)
	case "schedules":
		s.handleSuiteSchedules(w, r, principal, suiteID)
	case "tests":
		// Could add /api/suites/{id}/tests later if needed
		writeError(w, http.StatusNotFound, "resource not found")
	default:
		writeError(w, http.StatusNotFound, "resource not found")
	}
}

// handleOverviewMetrics handles GET /api/overview/metrics
// Returns aggregated metrics for the overview dashboard
func (s *Server) handleOverviewMetrics(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Parse query params
	query := r.URL.Query()

	// Parse project_ids (comma-separated)
	var projectIDs []uuid.UUID
	if projectIDsStr := query.Get("project_ids"); projectIDsStr != "" {
		for _, idStr := range strings.Split(projectIDsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if idStr == "" {
				continue
			}
			id, err := uuid.Parse(idStr)
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid project_ids")
				return
			}
			projectIDs = append(projectIDs, id)
		}
	}

	// Parse environment_id (single UUID, optional)
	var environmentID *uuid.UUID
	if envIDStr := query.Get("environment_id"); envIDStr != "" {
		id, err := uuid.Parse(envIDStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid environment_id")
			return
		}
		environmentID = &id
	}

	// Parse days (default 7, cap 30)
	days := 7
	if daysStr := query.Get("days"); daysStr != "" {
		var d int
		if _, err := stringToInt(daysStr, &d); err == nil && d > 0 {
			days = d
		}
	}

	// Get metrics from store
	metrics, err := s.store.GetOverviewMetrics(r.Context(), principal.OrgID, principal.UserID, projectIDs, environmentID, days)
	if err != nil {
		log.Printf("failed to get overview metrics: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get metrics")
		return
	}

	// Build response
	response := map[string]interface{}{
		"now": map[string]interface{}{
			"failing_monitors":      metrics.FailingMonitors,
			"failing_tests_24h":     metrics.FailingTests24h,
			"runs_in_progress":      metrics.RunsInProgress,
			"pass_rate_24h":         metrics.PassRate24h,
			"median_duration_ms_24h": metrics.MedianDurationMs24h,
		},
		"pass_rate_over_time":    metrics.PassRateOverTime,
		"failures_by_suite_24h": metrics.FailuresBySuite24h,
	}

	writeJSON(w, http.StatusOK, response)
}

// stringToInt parses a string to int and writes to the target
func stringToInt(s string, target *int) (int, error) {
	var n int
	_, err := fmt.Sscanf(s, "%d", &n)
	if err != nil {
		return 0, err
	}
	*target = n
	return n, nil
}
