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

// handleProjectSchedules handles /api/projects/{projectId}/schedules
// GET: List all project schedules for this project
func (s *Server) handleProjectSchedules(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Check project access
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

	schedules, err := s.store.ListProjectSchedulesByProject(ctx, projectID)
	if err != nil {
		log.Printf("failed to list project schedules: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}

	payload := make([]map[string]interface{}, 0, len(schedules))
	for _, sched := range schedules {
		item := map[string]interface{}{
			"id":              sched.ID.String(),
			"project_id":     sched.ProjectID.String(),
			"environment_id": sched.EnvironmentID.String(),
			"name":           sched.Name,
			"cron_expression": sched.CronExpression,
			"timezone":       sched.Timezone,
			"enabled":        sched.Enabled,
			"created_at":     sched.CreatedAt.Format(time.RFC3339),
			"updated_at":     sched.UpdatedAt.Format(time.RFC3339),
			"environment": map[string]interface{}{
				"name": sched.EnvironmentName,
				"slug": sched.EnvironmentSlug,
			},
		}
		if sched.NextRunAt.Valid {
			item["next_run_at"] = sched.NextRunAt.Time.Format(time.RFC3339)
		}
		if sched.LastRunAt.Valid {
			item["last_run_at"] = sched.LastRunAt.Time.Format(time.RFC3339)
		}
		if sched.LastRunID.Valid {
			item["last_run_id"] = sched.LastRunID.String
		}
		if sched.LastRunStatus.Valid {
			item["last_run_status"] = sched.LastRunStatus.String
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleCreateProjectSchedule handles POST /api/projects/{projectId}/project-schedules
func (s *Server) handleCreateProjectSchedule(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, projectID uuid.UUID) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Check write access to project
	hasWrite, err := s.store.UserHasProjectWriteAccess(ctx, principal.OrgID, principal.UserID, projectID)
	if err != nil {
		log.Printf("failed to check project write access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !hasWrite {
		writeError(w, http.StatusForbidden, "write access required")
		return
	}

	var body struct {
		EnvironmentID  string `json:"environment_id"`
		Name           string `json:"name"`
		CronExpression string `json:"cron_expression"`
		Timezone       string `json:"timezone"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	environmentID, err := uuid.Parse(body.EnvironmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment_id")
		return
	}

	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if strings.TrimSpace(body.CronExpression) == "" {
		writeError(w, http.StatusBadRequest, "cron_expression required")
		return
	}
	if strings.TrimSpace(body.Timezone) == "" {
		body.Timezone = "UTC"
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	input := persistence.CreateProjectScheduleInput{
		ProjectID:      projectID,
		EnvironmentID:  environmentID,
		Name:           strings.TrimSpace(body.Name),
		CronExpression: strings.TrimSpace(body.CronExpression),
		Timezone:       strings.TrimSpace(body.Timezone),
		Enabled:        enabled,
		CreatedBy:      principal.UserID,
	}

	schedule, err := s.store.CreateProjectSchedule(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			writeError(w, http.StatusConflict, "a schedule already exists for this project and environment")
			return
		}
		if strings.Contains(err.Error(), "invalid cron") || strings.Contains(err.Error(), "invalid timezone") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("failed to create project schedule: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to create schedule")
		return
	}

	payload := map[string]interface{}{
		"id":              schedule.ID.String(),
		"project_id":     schedule.ProjectID.String(),
		"environment_id": schedule.EnvironmentID.String(),
		"name":           schedule.Name,
		"cron_expression": schedule.CronExpression,
		"timezone":       schedule.Timezone,
		"enabled":        schedule.Enabled,
		"created_at":     schedule.CreatedAt.Format(time.RFC3339),
		"updated_at":     schedule.UpdatedAt.Format(time.RFC3339),
	}
	if schedule.NextRunAt.Valid {
		payload["next_run_at"] = schedule.NextRunAt.Time.Format(time.RFC3339)
	}

	writeJSON(w, http.StatusCreated, payload)
}

// handleProjectScheduleByID handles PUT/DELETE /api/project-schedules/{id}
func (s *Server) handleProjectScheduleByID(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, scheduleID uuid.UUID) {
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// First fetch the schedule to check access
	schedule, err := s.store.GetProjectSchedule(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		log.Printf("failed to get project schedule: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}

	// Check write access to the schedule's project
	hasWrite, err := s.store.UserHasProjectWriteAccess(ctx, principal.OrgID, principal.UserID, schedule.ProjectID)
	if err != nil {
		log.Printf("failed to check project write access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !hasWrite {
		writeError(w, http.StatusForbidden, "write access required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		payload := map[string]interface{}{
			"id":              schedule.ID.String(),
			"project_id":     schedule.ProjectID.String(),
			"environment_id": schedule.EnvironmentID.String(),
			"name":           schedule.Name,
			"cron_expression": schedule.CronExpression,
			"timezone":       schedule.Timezone,
			"enabled":        schedule.Enabled,
			"created_at":     schedule.CreatedAt.Format(time.RFC3339),
			"updated_at":     schedule.UpdatedAt.Format(time.RFC3339),
			"environment": map[string]interface{}{
				"name": schedule.EnvironmentName,
				"slug": schedule.EnvironmentSlug,
			},
		}
		if schedule.NextRunAt.Valid {
			payload["next_run_at"] = schedule.NextRunAt.Time.Format(time.RFC3339)
		}
		if schedule.LastRunAt.Valid {
			payload["last_run_at"] = schedule.LastRunAt.Time.Format(time.RFC3339)
		}
		if schedule.LastRunID.Valid {
			payload["last_run_id"] = schedule.LastRunID.String
		}
		if schedule.LastRunStatus.Valid {
			payload["last_run_status"] = schedule.LastRunStatus.String
		}
		writeJSON(w, http.StatusOK, payload)

	case http.MethodPut:
		var body struct {
			Name           *string `json:"name"`
			CronExpression *string `json:"cron_expression"`
			Timezone       *string `json:"timezone"`
			Enabled        *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}

		input := persistence.UpdateProjectScheduleInput{
			Name:           body.Name,
			CronExpression: body.CronExpression,
			Timezone:       body.Timezone,
			Enabled:        body.Enabled,
		}

		updated, err := s.store.UpdateProjectSchedule(ctx, scheduleID, input)
		if err != nil {
			if strings.Contains(err.Error(), "invalid cron") || strings.Contains(err.Error(), "invalid timezone") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("failed to update project schedule: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update schedule")
			return
		}

		payload := map[string]interface{}{
			"id":              updated.ID.String(),
			"project_id":     updated.ProjectID.String(),
			"environment_id": updated.EnvironmentID.String(),
			"name":           updated.Name,
			"cron_expression": updated.CronExpression,
			"timezone":       updated.Timezone,
			"enabled":        updated.Enabled,
			"created_at":     updated.CreatedAt.Format(time.RFC3339),
			"updated_at":     updated.UpdatedAt.Format(time.RFC3339),
		}
		if updated.NextRunAt.Valid {
			payload["next_run_at"] = updated.NextRunAt.Time.Format(time.RFC3339)
		}
		writeJSON(w, http.StatusOK, payload)

	case http.MethodDelete:
		if err := s.store.DeleteProjectSchedule(ctx, scheduleID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "schedule not found")
				return
			}
			log.Printf("failed to delete project schedule: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to delete schedule")
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleProjectSchedulesDispatch routes /api/project-schedules/* requests
func (s *Server) handleProjectSchedulesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/project-schedules/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/project-schedules/")
	scheduleID, err := uuid.Parse(strings.Trim(trimmed, "/"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	s.handleProjectScheduleByID(w, r, principal, scheduleID)
}

// ==================== Suite Schedule Routes (Overrides) ====================

// handleSuiteSchedules handles /api/suites/{suiteId}/schedules
// GET: List all schedule overrides for this suite
// POST: Create or update a schedule override for an environment (upsert semantics)
func (s *Server) handleSuiteSchedules(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, suiteID uuid.UUID) {
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// Fetch suite to get project_id and verify access
	suite, err := s.store.GetSuiteByID(ctx, suiteID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "suite not found")
			return
		}
		log.Printf("failed to get suite: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get suite")
		return
	}

	// Check project access
	canAccess, err := s.store.UserCanAccessProject(ctx, principal.OrgID, principal.UserID, suite.ProjectID)
	if err != nil {
		log.Printf("failed to check project access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !canAccess {
		writeError(w, http.StatusNotFound, "suite not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		s.handleListSuiteSchedules(w, r, principal, suiteID)
	case http.MethodPost:
		s.handleCreateSuiteSchedule(w, r, principal, suiteID, suite.ProjectID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleListSuiteSchedules handles GET /api/suites/{suiteId}/schedules
func (s *Server) handleListSuiteSchedules(w http.ResponseWriter, r *http.Request, _ brokerPrincipal, suiteID uuid.UUID) {
	ctx := r.Context()

	schedules, err := s.store.ListSuiteSchedulesBySuiteWithEnv(ctx, suiteID)
	if err != nil {
		log.Printf("failed to list suite schedules: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list schedules")
		return
	}

	payload := make([]map[string]interface{}, 0, len(schedules))
	for _, sched := range schedules {
		item := map[string]interface{}{
			"id":              sched.ID.String(),
			"suite_id":        sched.SuiteID.String(),
			"project_id":      sched.ProjectID.String(),
			"environment_id":  sched.EnvironmentID.UUID.String(),
			"name":            sched.Name,
			"cron_expression": sched.CronExpression,
			"timezone":        sched.Timezone,
			"enabled":         sched.Enabled,
			"created_at":      sched.CreatedAt.Format(time.RFC3339),
			"updated_at":      sched.UpdatedAt.Format(time.RFC3339),
			"environment": map[string]interface{}{
				"name": sched.EnvironmentName,
				"slug": sched.EnvironmentSlug,
			},
		}
		if sched.NextRunAt.Valid {
			item["next_run_at"] = sched.NextRunAt.Time.Format(time.RFC3339)
		}
		if sched.LastRunAt.Valid {
			item["last_run_at"] = sched.LastRunAt.Time.Format(time.RFC3339)
		}
		if sched.LastRunID.Valid {
			item["last_run_id"] = sched.LastRunID.String
		}
		if sched.LastRunStatus.Valid {
			item["last_run_status"] = sched.LastRunStatus.String
		}
		payload = append(payload, item)
	}

	writeJSON(w, http.StatusOK, payload)
}

// handleCreateSuiteSchedule handles POST /api/suites/{suiteId}/schedules
// Uses upsert semantics: creates new override or updates existing one for the environment
func (s *Server) handleCreateSuiteSchedule(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, suiteID, projectID uuid.UUID) {
	ctx := r.Context()

	// Check write access to the suite's project
	hasWrite, err := s.store.UserHasProjectWriteAccess(ctx, principal.OrgID, principal.UserID, projectID)
	if err != nil {
		log.Printf("failed to check project write access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !hasWrite {
		writeError(w, http.StatusForbidden, "write access required")
		return
	}

	var body struct {
		EnvironmentID  string `json:"environment_id"`
		Name           string `json:"name"`
		CronExpression string `json:"cron_expression"`
		Timezone       string `json:"timezone"`
		Enabled        *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json payload")
		return
	}

	environmentID, err := uuid.Parse(body.EnvironmentID)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid environment_id")
		return
	}

	if strings.TrimSpace(body.Name) == "" {
		writeError(w, http.StatusBadRequest, "name required")
		return
	}
	if strings.TrimSpace(body.CronExpression) == "" {
		writeError(w, http.StatusBadRequest, "cron_expression required")
		return
	}
	if strings.TrimSpace(body.Timezone) == "" {
		body.Timezone = "UTC"
	}

	enabled := true
	if body.Enabled != nil {
		enabled = *body.Enabled
	}

	input := persistence.CreateSuiteScheduleInput{
		SuiteID:        suiteID,
		ProjectID:      projectID,
		EnvironmentID:  environmentID,
		Name:           strings.TrimSpace(body.Name),
		CronExpression: strings.TrimSpace(body.CronExpression),
		Timezone:       strings.TrimSpace(body.Timezone),
		Enabled:        enabled,
		CreatedBy:      principal.UserID,
	}

	// Use upsert semantics
	schedule, created, err := s.store.UpsertSuiteScheduleOverride(ctx, input)
	if err != nil {
		if strings.Contains(err.Error(), "invalid cron") || strings.Contains(err.Error(), "invalid timezone") {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		log.Printf("failed to upsert suite schedule: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to save schedule")
		return
	}

	payload := map[string]interface{}{
		"id":              schedule.ID.String(),
		"suite_id":        schedule.SuiteID.String(),
		"project_id":      schedule.ProjectID.String(),
		"environment_id":  schedule.EnvironmentID.UUID.String(),
		"name":            schedule.Name,
		"cron_expression": schedule.CronExpression,
		"timezone":        schedule.Timezone,
		"enabled":         schedule.Enabled,
		"created_at":      schedule.CreatedAt.Format(time.RFC3339),
		"updated_at":      schedule.UpdatedAt.Format(time.RFC3339),
		"environment": map[string]interface{}{
			"name": schedule.EnvironmentName,
			"slug": schedule.EnvironmentSlug,
		},
	}
	if schedule.NextRunAt.Valid {
		payload["next_run_at"] = schedule.NextRunAt.Time.Format(time.RFC3339)
	}

	statusCode := http.StatusOK
	if created {
		statusCode = http.StatusCreated
	}
	writeJSON(w, statusCode, payload)
}

// handleSuiteScheduleByID handles GET/PUT/DELETE /api/suite-schedules/{id}
func (s *Server) handleSuiteScheduleByID(w http.ResponseWriter, r *http.Request, principal brokerPrincipal, scheduleID uuid.UUID) {
	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	ctx := r.Context()

	// First fetch the schedule to check access
	schedule, err := s.store.GetSuiteScheduleWithEnv(ctx, scheduleID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, http.StatusNotFound, "schedule not found")
			return
		}
		log.Printf("failed to get suite schedule: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to get schedule")
		return
	}

	// Check write access to the schedule's project
	hasWrite, err := s.store.UserHasProjectWriteAccess(ctx, principal.OrgID, principal.UserID, schedule.ProjectID)
	if err != nil {
		log.Printf("failed to check project write access: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to check access")
		return
	}
	if !hasWrite {
		writeError(w, http.StatusForbidden, "write access required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		payload := map[string]interface{}{
			"id":              schedule.ID.String(),
			"suite_id":        schedule.SuiteID.String(),
			"project_id":      schedule.ProjectID.String(),
			"environment_id":  schedule.EnvironmentID.UUID.String(),
			"name":            schedule.Name,
			"cron_expression": schedule.CronExpression,
			"timezone":        schedule.Timezone,
			"enabled":         schedule.Enabled,
			"created_at":      schedule.CreatedAt.Format(time.RFC3339),
			"updated_at":      schedule.UpdatedAt.Format(time.RFC3339),
			"environment": map[string]interface{}{
				"name": schedule.EnvironmentName,
				"slug": schedule.EnvironmentSlug,
			},
		}
		if schedule.NextRunAt.Valid {
			payload["next_run_at"] = schedule.NextRunAt.Time.Format(time.RFC3339)
		}
		if schedule.LastRunAt.Valid {
			payload["last_run_at"] = schedule.LastRunAt.Time.Format(time.RFC3339)
		}
		if schedule.LastRunID.Valid {
			payload["last_run_id"] = schedule.LastRunID.String
		}
		if schedule.LastRunStatus.Valid {
			payload["last_run_status"] = schedule.LastRunStatus.String
		}
		writeJSON(w, http.StatusOK, payload)

	case http.MethodPut:
		var body struct {
			Name           *string `json:"name"`
			CronExpression *string `json:"cron_expression"`
			Timezone       *string `json:"timezone"`
			Enabled        *bool   `json:"enabled"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeError(w, http.StatusBadRequest, "invalid json payload")
			return
		}

		input := persistence.UpdateSuiteScheduleInput{
			Name:           body.Name,
			CronExpression: body.CronExpression,
			Timezone:       body.Timezone,
			Enabled:        body.Enabled,
		}

		updated, err := s.store.UpdateSuiteScheduleOverride(ctx, scheduleID, input)
		if err != nil {
			if strings.Contains(err.Error(), "invalid cron") || strings.Contains(err.Error(), "invalid timezone") {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			log.Printf("failed to update suite schedule: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to update schedule")
			return
		}

		payload := map[string]interface{}{
			"id":              updated.ID.String(),
			"suite_id":        updated.SuiteID.String(),
			"project_id":      updated.ProjectID.String(),
			"environment_id":  updated.EnvironmentID.UUID.String(),
			"name":            updated.Name,
			"cron_expression": updated.CronExpression,
			"timezone":        updated.Timezone,
			"enabled":         updated.Enabled,
			"created_at":      updated.CreatedAt.Format(time.RFC3339),
			"updated_at":      updated.UpdatedAt.Format(time.RFC3339),
			"environment": map[string]interface{}{
				"name": updated.EnvironmentName,
				"slug": updated.EnvironmentSlug,
			},
		}
		if updated.NextRunAt.Valid {
			payload["next_run_at"] = updated.NextRunAt.Time.Format(time.RFC3339)
		}
		writeJSON(w, http.StatusOK, payload)

	case http.MethodDelete:
		if err := s.store.DeleteSuiteScheduleOverride(ctx, scheduleID); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeError(w, http.StatusNotFound, "schedule not found")
				return
			}
			log.Printf("failed to delete suite schedule: %v", err)
			writeError(w, http.StatusInternalServerError, "failed to delete schedule")
			return
		}
		w.WriteHeader(http.StatusNoContent)

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleSuiteSchedulesDispatch routes /api/suite-schedules/* requests
func (s *Server) handleSuiteSchedulesDispatch(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if !strings.HasPrefix(r.URL.Path, "/api/suite-schedules/") {
		writeError(w, http.StatusNotFound, "not found")
		return
	}

	trimmed := strings.TrimPrefix(r.URL.Path, "/api/suite-schedules/")
	scheduleID, err := uuid.Parse(strings.Trim(trimmed, "/"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid schedule id")
		return
	}

	s.handleSuiteScheduleByID(w, r, principal, scheduleID)
}
