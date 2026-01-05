package controlplane

import (
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
)

// handleTestHealth handles GET /api/test-health
// Returns test health data for the Test Health page.
//
// Query params:
//   - project_ids: comma-separated project IDs (optional, default = all accessible)
//   - environment_id: single env UUID (optional)
//   - suite_ids: comma-separated suite IDs (optional)
//   - plugins: comma-separated plugin names (OR semantics, optional)
//   - search: test name search (ILIKE, optional)
//   - limit: max rows (default 100, max 500)
func (s *Server) handleTestHealth(w http.ResponseWriter, r *http.Request, principal brokerPrincipal) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if principal.RequiresOrgMembership() {
		writeError(w, http.StatusForbidden, "organization membership required")
		return
	}

	// Parse query params
	params := persistence.TestHealthParams{}

	// project_ids
	if projectIDsStr := r.URL.Query().Get("project_ids"); projectIDsStr != "" {
		for _, idStr := range strings.Split(projectIDsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if id, err := uuid.Parse(idStr); err == nil {
				params.ProjectIDs = append(params.ProjectIDs, id)
			}
		}
	}

	// environment_id
	if envIDStr := r.URL.Query().Get("environment_id"); envIDStr != "" {
		if envID, err := uuid.Parse(envIDStr); err == nil {
			params.EnvironmentID = uuid.NullUUID{UUID: envID, Valid: true}
		}
	}

	// suite_ids
	if suiteIDsStr := r.URL.Query().Get("suite_ids"); suiteIDsStr != "" {
		for _, idStr := range strings.Split(suiteIDsStr, ",") {
			idStr = strings.TrimSpace(idStr)
			if id, err := uuid.Parse(idStr); err == nil {
				params.SuiteIDs = append(params.SuiteIDs, id)
			}
		}
	}

	// plugins
	if pluginsStr := r.URL.Query().Get("plugins"); pluginsStr != "" {
		for _, p := range strings.Split(pluginsStr, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				params.Plugins = append(params.Plugins, p)
			}
		}
	}

	// search
	params.Search = r.URL.Query().Get("search")

	// limit
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			params.Limit = limit
		}
	}

	// Execute query
	tests, suiteOptions, err := s.store.ListTestHealth(r.Context(), principal.OrgID, principal.UserID, params)
	if err != nil {
		log.Printf("failed to list test health: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to list test health")
		return
	}

	// Build response payload
	testsPayload := make([]map[string]interface{}, 0, len(tests))
	for _, t := range tests {
		item := map[string]interface{}{
			"id":             t.TestID.String(),
			"name":           t.TestName,
			"step_count":     t.StepCount,
			"plugins":        t.TestPlugins,
			"suite_id":       t.SuiteID.String(),
			"suite_name":     t.SuiteName,
			"project_id":     t.ProjectID.String(),
			"project_name":   t.ProjectName,
			"recent_results": t.LatestResults,
			"is_live":        t.IsLive,
		}

		// success_rate
		if t.SuccessPercent.Valid {
			item["success_rate"] = strconv.Itoa(int(t.SuccessPercent.Int32)) + "%"
		} else {
			item["success_rate"] = nil
		}

		// last_run_at (ISO 8601)
		if t.LastRunAt.Valid {
			item["last_run_at"] = t.LastRunAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		} else {
			item["last_run_at"] = nil
		}

		// next_run_at (ISO 8601)
		if t.NextRunAt.Valid {
			item["next_run_at"] = t.NextRunAt.Time.UTC().Format("2006-01-02T15:04:05Z")
		} else {
			item["next_run_at"] = nil
		}

		testsPayload = append(testsPayload, item)
	}

	// Build suite options payload
	suitesPayload := make([]map[string]interface{}, 0, len(suiteOptions))
	for _, s := range suiteOptions {
		suitesPayload = append(suitesPayload, map[string]interface{}{
			"id":   s.ID.String(),
			"name": s.Name,
		})
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tests":  testsPayload,
		"suites": suitesPayload,
	})
}
