package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/interpreter"
	"go.temporal.io/sdk/client"
	yaml "gopkg.in/yaml.v3"
)

func (e *Engine) CreateRun(ctx context.Context, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.YamlPayload) == 0 {
		return nil, fmt.Errorf("YAML payload cannot be empty")
	}

	principal, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	slog.Debug("CreateRun called", "payload_size", len(req.YamlPayload), "org_id", orgID.String())

	runID, err := generateID()
	if err != nil {
		slog.Error("Failed to generate run ID", "error", err)
		return nil, fmt.Errorf("failed to generate run ID: %w", err)
	}

	run, err := dsl.ParseYAML(req.YamlPayload)
	if err != nil {
		slog.Error("Failed to parse YAML", "error", err)
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	if len(run.Tests) == 0 {
		return nil, fmt.Errorf("test run must contain at least one test")
	}

	runContext := extractRunContext(req.Context)
	startTime := time.Now().UTC()
	configSource := detectConfigSource(runContext)
	envSlug := detectEnvironment(runContext)
	initiator := determineInitiator(principal)

	// Environment resolution variables - will be set after project_id is resolved
	var envSecrets map[string]string
	var envConfigVars map[string]interface{}

	if orgID != uuid.Nil && e.runStore != nil {
		// Extract bundle_sha from metadata if present (set by CLI for uncommitted files)
		bundleSHA := sql.NullString{}
		if runContext.Metadata != nil {
			if sha := strings.TrimSpace(runContext.Metadata["rs_bundle_sha"]); sha != "" {
				bundleSHA = sql.NullString{String: sha, Valid: true}
				slog.Debug("CreateRun: using bundle_sha from CLI metadata", "bundle_sha", sha[:12])
			}
		}

		record := persistence.RunRecord{
			ID:             runID,
			OrganizationID: orgID,
			Status:         "RUNNING",
			SuiteName:      run.Name,
			Initiator:      initiator,
			Trigger:        strings.TrimSpace(runContext.Trigger),
			ScheduleName:   strings.TrimSpace(runContext.ScheduleName),
			ConfigSource:   configSource,
			Source:         strings.TrimSpace(runContext.Source),
			Branch:         strings.TrimSpace(runContext.Branch),
			CommitSHA:      makeNullString(runContext.CommitSHA),
			CommitMessage:  makeNullString(runContext.Metadata["rs_commit_message"]),
			BundleSHA:      bundleSHA,
			TotalTests:     len(run.Tests),
			PassedTests:    0,
			FailedTests:    0,
			TimeoutTests:   0,
			StartedAt:      sql.NullTime{Time: startTime, Valid: true},
		}

		// Parse project_id as UUID if it's a valid UUID
		if projectID, err := uuid.Parse(runContext.ProjectID); err == nil && projectID != uuid.Nil {
			record.ProjectID = uuid.NullUUID{UUID: projectID, Valid: true}
		} else if runContext.Metadata != nil {
			// Fallback: resolve project_id from metadata (rs_repo_url + rs_path_scope_json)
			repoURL := runContext.Metadata["rs_repo_url"]
			pathScopeJSON := runContext.Metadata["rs_path_scope_json"]
			if repoURL != "" && pathScopeJSON != "" {
				var pathScope []string
				if err := json.Unmarshal([]byte(pathScopeJSON), &pathScope); err != nil {
					slog.Debug("CreateRun: failed to parse rs_path_scope_json", "error", err)
				} else {
					project, found, err := e.runStore.FindProjectByRepoAndPathScope(ctx, orgID, repoURL, pathScope)
					if err != nil {
						slog.Debug("CreateRun: failed to lookup project by repo/path_scope", "error", err)
					} else if found {
						record.ProjectID = uuid.NullUUID{UUID: project.ID, Valid: true}
						slog.Debug("CreateRun: resolved project_id from metadata",
							"project_id", project.ID,
							"repo_url", repoURL,
							"path_scope", pathScope)
					} else {
						slog.Debug("CreateRun: no matching project found for repo/path_scope",
							"repo_url", repoURL,
							"path_scope", pathScope)
					}
				}
			}
		}

		// Resolve environment after project_id is set
		if record.ProjectID.Valid {
			var env persistence.ProjectEnvironment
			var found bool

			if envSlug != "" {
				// Try to get environment by slug
				env, err = e.runStore.GetEnvironmentBySlug(ctx, record.ProjectID.UUID, envSlug)
				if err != nil {
					if !errors.Is(err, sql.ErrNoRows) {
						slog.Debug("CreateRun: failed to lookup environment by slug", "slug", envSlug, "error", err)
					}
				} else {
					found = true
				}
			}

			// If no env slug specified or not found, try default environment
			if !found && envSlug == "" {
				env, err = e.runStore.GetDefaultEnvironment(ctx, record.ProjectID.UUID)
				if err != nil {
					if !errors.Is(err, sql.ErrNoRows) {
						slog.Debug("CreateRun: failed to lookup default environment", "error", err)
					}
				} else {
					found = true
				}
			}

			if found {
				envSecrets = env.EnvSecrets
				envConfigVars = env.ConfigVars
				record.EnvironmentID = uuid.NullUUID{UUID: env.ID, Valid: true}
				record.Environment = env.Slug
				slog.Debug("CreateRun: resolved environment",
					"env_id", env.ID,
					"env_slug", env.Slug,
					"secrets_count", len(envSecrets),
					"config_vars_count", len(envConfigVars))
			} else if envSlug != "" {
				// If user explicitly specified an environment slug but it wasn't found, fail fast
				// This is different from empty slug (where we try default env as best-effort)
				slog.Error("CreateRun: environment not found", "slug", envSlug, "project_id", record.ProjectID.UUID)
				return nil, fmt.Errorf("unknown environment %q for this project; hint: create it in the console /environments page", envSlug)
			}
		} else if envSlug != "" {
			// No project_id but env slug was specified - just store the slug
			record.Environment = envSlug
		}

		if _, err := e.runStore.InsertRun(ctx, record); err != nil {
			slog.Error("CreateRun: failed to persist run metadata", "run_id", runID, "error", err)
			return nil, fmt.Errorf("failed to persist run metadata: %w", err)
		}
	}

	slog.Debug("Starting run",
		"name", run.Name,
		"test_count", len(run.Tests),
		"project_id", runContext.ProjectID,
		"source", runContext.Source,
		"branch", runContext.Branch)

	// Resolve suite_id and build test name → test_id map for linking
	var resolvedProjectID, resolvedSuiteID uuid.UUID
	testIDMap := make(map[string]uuid.UUID)
	if orgID != uuid.Nil && e.runStore != nil {
		// Get resolved project_id from context or metadata lookup above
		if projectID, err := uuid.Parse(runContext.ProjectID); err == nil && projectID != uuid.Nil {
			resolvedProjectID = projectID
		} else if runContext.Metadata != nil {
			repoURL := runContext.Metadata["rs_repo_url"]
			pathScopeJSON := runContext.Metadata["rs_path_scope_json"]
			if repoURL != "" && pathScopeJSON != "" {
				var pathScope []string
				if err := json.Unmarshal([]byte(pathScopeJSON), &pathScope); err == nil {
					project, found, err := e.runStore.FindProjectByRepoAndPathScope(ctx, orgID, repoURL, pathScope)
					if err == nil && found {
						resolvedProjectID = project.ID
					}
				}
			}
		}

		// If we have a project_id, look up suite_id and test_ids
		if resolvedProjectID != uuid.Nil {
			sourceRef := strings.TrimSpace(runContext.Branch)
			if sourceRef == "" {
				sourceRef = "main" // fallback
			}
			suite, found, err := e.runStore.GetSuiteByName(ctx, resolvedProjectID, run.Name, sourceRef)
			if err != nil {
				slog.Debug("CreateRun: failed to lookup suite", "error", err)
			} else if found {
				resolvedSuiteID = suite.ID
				slog.Debug("CreateRun: resolved suite_id", "suite_id", suite.ID, "suite_name", run.Name, "source_ref", sourceRef)

				// Build test name → test_id map
				tests, err := e.runStore.ListTestsBySuite(ctx, suite.ID)
				if err != nil {
					slog.Debug("CreateRun: failed to list tests for suite", "error", err)
				} else {
					for _, t := range tests {
						// Use lowercase name for case-insensitive matching
						key := strings.ToLower(t.Name)
						testIDMap[key] = t.ID
					}
					slog.Debug("CreateRun: built test_id map", "count", len(testIDMap))
				}
			} else {
				slog.Debug("CreateRun: suite not found", "suite_name", run.Name, "source_ref", sourceRef)
			}
		}
	}

	// Merge environment config vars with run.Vars (environment is lowest precedence)
	// Use deep merge to properly handle nested config vars (like Helm values)
	// Base = envConfigVars (lowest), Overlay = run.Vars (higher)
	mergedVars := dsl.MergeInterfaceMaps(envConfigVars, run.Vars)

	// Server-side: Apply .vars.* substitution using merged vars (includes env config vars)
	// Important: Do NOT substitute .env.* here - leave for plugin-time resolution with env secrets
	// Re-parse the YAML after template substitution so tests have resolved values
	if len(mergedVars) > 0 {
		// Parse YAML to interface{} for template processing
		var yamlDoc interface{}
		if err := json.Unmarshal(req.YamlPayload, &yamlDoc); err != nil {
			// Try YAML unmarshal if JSON fails
			if yamlErr := yaml.Unmarshal(req.YamlPayload, &yamlDoc); yamlErr != nil {
				slog.Debug("CreateRun: failed to parse YAML for vars substitution", "error", yamlErr)
			} else {
				// Use ProcessVarsOnlyRecursive to only substitute .vars.* (not .env.*)
				processedDoc, err := dsl.ProcessVarsOnlyRecursive(yamlDoc, mergedVars)
				if err != nil {
					slog.Warn("CreateRun: failed to process config variables", "error", err)
				} else {
					// Re-marshal and re-parse
					processedYaml, err := yaml.Marshal(processedDoc)
					if err != nil {
						slog.Warn("CreateRun: failed to marshal processed YAML", "error", err)
					} else {
						newRun, err := dsl.ParseYAML(processedYaml)
						if err != nil {
							slog.Warn("CreateRun: failed to re-parse processed YAML", "error", err)
						} else {
							run = newRun
							slog.Debug("CreateRun: applied server-side vars substitution", "vars_count", len(mergedVars))
						}
					}
				}
			}
		} else {
			// JSON parse succeeded - use ProcessVarsOnlyRecursive to only substitute .vars.* (not .env.*)
			processedDoc, err := dsl.ProcessVarsOnlyRecursive(yamlDoc, mergedVars)
			if err != nil {
				slog.Warn("CreateRun: failed to process config variables (JSON)", "error", err)
			} else {
				processedYaml, err := yaml.Marshal(processedDoc)
				if err != nil {
					slog.Warn("CreateRun: failed to marshal processed YAML (JSON)", "error", err)
				} else {
					newRun, err := dsl.ParseYAML(processedYaml)
					if err != nil {
						slog.Warn("CreateRun: failed to re-parse processed YAML (JSON)", "error", err)
					} else {
						run = newRun
						slog.Debug("CreateRun: applied server-side vars substitution (JSON)", "vars_count", len(mergedVars))
					}
				}
			}
		}
	}

	runInfo := &RunInfo{
		ID:             runID,
		Name:           run.Name,
		Status:         "RUNNING",
		StartedAt:      startTime,
		Tests:          make(map[string]*TestInfo),
		Context:        runContext,
		SuiteCleanup:   run.Cleanup,
		Vars:           cloneInterfaceMap(mergedVars),
		SuiteOpenAPI:   run.OpenAPI,
		OrganizationID: orgID,
		ProjectID:      resolvedProjectID,
		SuiteID:        resolvedSuiteID,
		TestIDs:        testIDMap,
		EnvSecrets:     envSecrets,
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("Starting test run \"%s\"... 🚀 [%s/%s]", run.Name, runContext.ProjectID, runContext.Source),
				Color: "purple",
				Bold:  true,
			},
		},
	}

	e.mu.Lock()
	e.runs[runID] = runInfo
	e.mu.Unlock()

	var suiteGlobals map[string]string
	if len(run.Init) > 0 {
		e.addLog(runID, "Running suite init...", "n/a", false)
		initGlobals, initErr := e.runSuiteInitWorkflow(ctx, runID, run.Name, run.Init, runInfo.Vars, run.OpenAPI, runInfo.EnvSecrets)
		if initErr != nil {
			e.handleSuiteInitFailure(runID, runInfo, initErr)
			return &generated.CreateRunResponse{RunId: runID}, nil
		}

		suiteGlobals = initGlobals

		e.mu.Lock()
		runInfo.SuiteGlobals = cloneStringMap(suiteGlobals)
		runInfo.SuiteInitCompleted = true
		e.mu.Unlock()

		e.addLog(runID, "Suite init completed", "green", true)
	} else {
		e.mu.Lock()
		runInfo.SuiteInitCompleted = true
		e.mu.Unlock()
	}

	if suiteGlobals == nil {
		suiteGlobals = make(map[string]string)
	}

	for _, test := range run.Tests {
		testID, err := generateID()
		if err != nil {
			log.Printf("[ERROR] Failed to generate test ID: %v", err)
			return nil, fmt.Errorf("failed to generate test ID: %w", err)
		}
		testStartTime := time.Now().UTC()

		// Look up discovered test_id for linking
		var discoveredTestID uuid.UUID
		if len(runInfo.TestIDs) > 0 {
			key := strings.ToLower(test.Name)
			if tid, ok := runInfo.TestIDs[key]; ok {
				discoveredTestID = tid
			}
		}

		testInfo := &TestInfo{
			WorkflowID: testID,
			Name:       test.Name,
			Status:     "PENDING",
			StartedAt:  testStartTime,
			RunID:      runID,
			TestID:     discoveredTestID,
		}

		// Persist run_test record
		if orgID != uuid.Nil && e.runStore != nil {
			runTest := persistence.RunTest{
				RunID:      runID,
				WorkflowID: testID,
				Name:       test.Name,
				Status:     "PENDING",
				StartedAt:  sql.NullTime{Time: testStartTime, Valid: true},
				StepCount:  len(test.Steps),
			}
			// Set test_id if we found a matching discovered test
			if discoveredTestID != uuid.Nil {
				runTest.TestID = uuid.NullUUID{UUID: discoveredTestID, Valid: true}
			}
			if _, err := e.runStore.InsertRunTest(ctx, runTest); err != nil {
				slog.Error("CreateRun: failed to persist run_test", "run_id", runID, "workflow_id", testID, "error", err)
				// Don't fail the run, just log the error
			}
		}

		workflowOptions := client.StartWorkflowOptions{
			ID:        testID,
			TaskQueue: "test-workflows",
		}

		slog.Debug("Starting workflow with search attributes",
			"workflow_id", testID,
			"project_id", runContext.ProjectID,
			"suite_name", run.Name,
			"source", runContext.Source)

		suiteGlobalsCopy := cloneStringMap(suiteGlobals)
		envSecretsCopy := cloneStringMap(runInfo.EnvSecrets)
		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, runInfo.Vars, runID, run.OpenAPI, suiteGlobalsCopy, envSecretsCopy)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start test \"%s\": %v", test.Name, err), "red", true)
			e.triggerSuiteCleanup(runID, true)

			if orgID != uuid.Nil && e.runStore != nil {
				status := "FAILED"
				ended := time.Now().UTC()
				totals := &persistence.RunTotals{Total: len(run.Tests)}
				if _, updErr := e.runStore.UpdateRun(ctx, persistence.RunUpdate{
					RunID:          runID,
					OrganizationID: orgID,
					Status:         &status,
					EndedAt:        &ended,
					Totals:         totals,
				}); updErr != nil {
					slog.Error("CreateRun: failed to mark run as failed after workflow start error", "run_id", runID, "error", updErr)
				}
			}

			return nil, fmt.Errorf("failed to start workflow: %w", err)
		}

		e.mu.Lock()
		runInfo.Tests[testID] = testInfo
		runInfo.Logs = append(runInfo.Logs, LogLine{
			Msg:   fmt.Sprintf("Running test: \"%s\"...", test.Name),
			Color: "n/a",
			Bold:  false,
		})
		e.mu.Unlock()

		go e.monitorWorkflow(runID, execution.GetID(), execution.GetRunID())
	}

	return &generated.CreateRunResponse{RunId: runID}, nil
}

func (e *Engine) runSuiteInitWorkflow(ctx context.Context, runID, runName string, initSteps []dsl.Step, vars map[string]interface{}, suiteOpenAPI *dsl.OpenAPISuiteConfig, envSecrets map[string]string) (map[string]string, error) {
	if len(initSteps) == 0 {
		return make(map[string]string), nil
	}

	suiteTest := dsl.Test{
		Name:  fmt.Sprintf("%s::suite-init", runName),
		Init:  initSteps,
		Steps: []dsl.Step{},
	}

	workflowOptions := client.StartWorkflowOptions{
		ID:        fmt.Sprintf("%s_suite_init", runID),
		TaskQueue: "test-workflows",
	}

	execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", suiteTest, vars, runID, suiteOpenAPI, map[string]string(nil), envSecrets)
	if err != nil {
		return nil, err
	}

	var state map[string]string
	if err := execution.Get(ctx, &state); err != nil {
		return nil, err
	}

	return extractSavedValues(state), nil
}

func (e *Engine) handleSuiteInitFailure(runID string, runInfo *RunInfo, initErr error) {
	log.Printf("[ERROR] Suite init failed for run %s: %v", runID, initErr)

	ended := time.Now().UTC()

	e.mu.Lock()
	runInfo.Status = "FAILED"
	runInfo.SuiteInitFailed = true
	runInfo.EndedAt = ended
	e.mu.Unlock()

	e.addLog(runID, fmt.Sprintf("Suite init failed: %v", initErr), "red", true)
	e.addLog(runID, "Skipping all tests because suite init failed.", "red", true)
	e.addLog(runID, fmt.Sprintf("Test run: \"%s\" ended without executing any tests.", runInfo.Name), "n/a", true)

	if runInfo.OrganizationID != uuid.Nil && e.runStore != nil {
		if _, err := e.runStore.UpdateRun(context.Background(), persistence.RunUpdate{
			RunID:          runID,
			OrganizationID: runInfo.OrganizationID,
			Status:         stringPtr("FAILED"),
			EndedAt:        timePtr(ended),
			Totals:         &persistence.RunTotals{Total: len(runInfo.Tests)},
		}); err != nil {
			slog.Error("handleSuiteInitFailure: failed to persist failure state", "run_id", runID, "error", err)
		}
	}

	e.triggerSuiteCleanup(runID, true)
}

func (e *Engine) triggerSuiteCleanup(runID string, hasFailure bool) {
	slog.Info("triggerSuiteCleanup: Starting", "run_id", runID, "has_failure", hasFailure)

	e.mu.Lock()
	runInfo, exists := e.runs[runID]
	if !exists {
		e.mu.Unlock()
		slog.Warn("triggerSuiteCleanup: Run not found", "run_id", runID)
		return
	}

	if runInfo.SuiteCleanup == nil {
		e.mu.Unlock()
		slog.Debug("triggerSuiteCleanup: No suite cleanup configured", "run_id", runID)
		return
	}

	if runInfo.SuiteCleanupRan {
		e.mu.Unlock()
		slog.Debug("triggerSuiteCleanup: Suite cleanup already ran", "run_id", runID)
		return
	}

	runInfo.SuiteCleanupRan = true
	cleanupSpec := runInfo.SuiteCleanup
	varsCopy := cloneInterfaceMap(runInfo.Vars)
	suiteGlobalsCopy := cloneStringMap(runInfo.SuiteGlobals)
	suiteOpenAPI := runInfo.SuiteOpenAPI
	envSecretsCopy := cloneStringMap(runInfo.EnvSecrets)
	e.mu.Unlock()

	slog.Info("triggerSuiteCleanup: Starting suite cleanup workflow", "run_id", runID)

	// Track suite cleanup workflow so server waits for completion before shutdown
	e.cleanupWg.Add(1)
	slog.Debug("triggerSuiteCleanup: Added to cleanupWg", "run_id", runID)
	go func() {
		defer func() {
			e.cleanupWg.Done()
			slog.Debug("triggerSuiteCleanup: Removed from cleanupWg (Done called)", "run_id", runID)
		}()

		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Minute)
		defer cancel()

		options := client.StartWorkflowOptions{
			ID:        fmt.Sprintf("%s_suite_cleanup", runID),
			TaskQueue: "test-workflows",
		}

		params := interpreter.SuiteCleanupParams{
			RunID:          runID,
			TestName:       "suite-cleanup",
			Cleanup:        cleanupSpec,
			Vars:           varsCopy,
			SuiteOpenAPI:   suiteOpenAPI,
			SuiteGlobals:   suiteGlobalsCopy,
			TreatAsFailure: hasFailure,
			EnvSecrets:     envSecretsCopy,
		}

		slog.Debug("triggerSuiteCleanup: Executing suite cleanup workflow", "run_id", runID, "workflow_id", options.ID)
		execution, err := e.temporal.ExecuteWorkflow(ctx, options, "SuiteCleanupWorkflow", params)
		if err != nil {
			slog.Error("triggerSuiteCleanup: Failed to start suite cleanup workflow", "run_id", runID, "error", err)
			log.Printf("[ERROR] Failed to start suite cleanup workflow for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start suite cleanup: %v", err), "red", true)
			return
		}

		slog.Debug("triggerSuiteCleanup: Suite cleanup workflow started, waiting for completion", "run_id", runID)
		if err := execution.Get(ctx, nil); err != nil {
			slog.Error("triggerSuiteCleanup: Suite cleanup workflow failed", "run_id", runID, "error", err)
			log.Printf("[ERROR] Suite cleanup workflow failed for run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Suite cleanup workflow failed: %v", err), "red", true)
			return
		}

		slog.Info("triggerSuiteCleanup: Suite cleanup completed successfully", "run_id", runID)
		e.addLog(runID, "Suite cleanup completed", "green", true)
	}()
}

func (e *Engine) ListRuns(ctx context.Context, req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	slog.Debug("ListRuns called",
		"project_id", req.ProjectId,
		"source", req.Source,
		"branch", req.Branch,
		"status", req.Status,
		"limit", req.Limit)

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	if orgID == uuid.Nil || e.runStore == nil {
		return e.listRunsInMemory(req)
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}
	records, err := e.runStore.ListRuns(ctx, orgID, limit)
	if err != nil {
		slog.Error("ListRuns: failed to load runs", "error", err)
		return nil, fmt.Errorf("failed to list runs: %w", err)
	}

	filtered := make([]*generated.RunSummary, 0, len(records))
	for _, rec := range records {
		if req.Status != "" && !strings.EqualFold(rec.Status, req.Status) {
			continue
		}
		if req.ProjectId != "" {
			if !rec.ProjectID.Valid || rec.ProjectID.UUID.String() != req.ProjectId {
				continue
			}
		}
		if req.Source != "" && !strings.EqualFold(rec.Source, req.Source) {
			continue
		}
		if req.Branch != "" && !strings.EqualFold(rec.Branch, req.Branch) {
			continue
		}
		if req.ScheduleName != "" && !strings.EqualFold(rec.ScheduleName, req.ScheduleName) {
			continue
		}

		filtered = append(filtered, mapRunRecordToSummary(rec))
	}

	sortRuns(filtered, req.OrderBy, !req.Descending)

	result := &generated.ListRunsResponse{
		Runs:       filtered,
		TotalCount: int32(len(filtered)),
	}

	slog.Debug("Returning ListRuns response", "runs_count", len(filtered))
	return result, nil
}

func (e *Engine) GetRun(ctx context.Context, req *generated.GetRunRequest) (*generated.GetRunResponse, error) {
	slog.Debug("GetRun called", "run_id", req.RunId)

	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.RLock()
	if runInfo, exists := e.runs[req.RunId]; exists {
		if orgID == uuid.Nil || runInfo.OrganizationID == orgID {
			e.mu.RUnlock()
			slog.Debug("Found active run in memory", "run_id", req.RunId)
			return mapRunInfoToRunDetails(runInfo), nil
		}
	}
	e.mu.RUnlock()

	if orgID == uuid.Nil || e.runStore == nil {
		return e.getRunInMemory(req.RunId)
	}

	rec, err := e.runStore.GetRun(ctx, orgID, req.RunId)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("run not found: %s", req.RunId)
		}
		slog.Error("GetRun: failed to load run", "run_id", req.RunId, "error", err)
		return nil, fmt.Errorf("failed to load run: %w", err)
	}

	resp := mapRunRecordToRunDetails(rec)

	// Load persisted run_tests for this run
	runTests, err := e.runStore.ListRunTests(ctx, req.RunId)
	if err != nil {
		slog.Warn("GetRun: failed to load run_tests, returning empty tests", "run_id", req.RunId, "error", err)
	} else if len(runTests) > 0 {
		resp.Run.Tests = mapRunTestsToTestDetails(runTests)
	}

	return resp, nil
}

// CancelRun cancels all workflows for a given run and marks it as cancelled
func (e *Engine) CancelRun(ctx context.Context, req *generated.CancelRunRequest) (*generated.CancelRunResponse, error) {
	if req.RunId == "" {
		return nil, fmt.Errorf("run_id is required")
	}

	slog.Info("CancelRun: Starting cancellation", "run_id", req.RunId)

	_, orgID, err := e.resolvePrincipalAndOrg(ctx)
	if err != nil {
		return nil, err
	}

	e.mu.Lock()
	runInfo, exists := e.runs[req.RunId]
	if !exists {
		e.mu.Unlock()
		slog.Warn("CancelRun: Run not found", "run_id", req.RunId)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", req.RunId),
		}, nil
	}
	if orgID != uuid.Nil && runInfo.OrganizationID != orgID {
		e.mu.Unlock()
		slog.Warn("CancelRun: Run not accessible for caller", "run_id", req.RunId)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("run not found: %s", req.RunId),
		}, nil
	}

	runInfo.Status = "CANCELLED"
	slog.Debug("CancelRun: Marked run as CANCELLED", "run_id", req.RunId)

	testWorkflows := make([]string, 0, len(runInfo.Tests))
	for workflowID, testInfo := range runInfo.Tests {
		testWorkflows = append(testWorkflows, workflowID)
		slog.Debug("CancelRun: Found test workflow to cancel", "workflow_id", workflowID, "test_name", testInfo.Name, "status", testInfo.Status)
	}
	e.mu.Unlock()

	slog.Info("CancelRun: Cancelling workflows", "run_id", req.RunId, "workflow_count", len(testWorkflows))

	var cancelErrors []string
	for _, workflowID := range testWorkflows {
		slog.Debug("CancelRun: Attempting to cancel workflow", "workflow_id", workflowID)
		if err := e.temporal.CancelWorkflow(ctx, workflowID, ""); err != nil {
			slog.Warn("CancelRun: Failed to cancel workflow", "workflow_id", workflowID, "error", err)
			cancelErrors = append(cancelErrors, fmt.Sprintf("workflow %s: %v", workflowID, err))
			continue
		}
		slog.Info("CancelRun: Successfully cancelled workflow", "workflow_id", workflowID)

		slog.Debug("CancelRun: Waiting for workflow to complete cleanup", "workflow_id", workflowID)
		workflowRun := e.temporal.GetWorkflow(ctx, workflowID, "")
		var result interface{}
		if err := workflowRun.Get(ctx, &result); err != nil {
			slog.Debug("CancelRun: Workflow completed with cancellation", "workflow_id", workflowID, "error", err)
		} else {
			slog.Debug("CancelRun: Workflow completed successfully", "workflow_id", workflowID)
		}
	}

	e.addLog(req.RunId, "Run cancelled by user (Ctrl+C)", "yellow", true)
	slog.Debug("CancelRun: Triggering suite cleanup", "run_id", req.RunId)
	e.triggerSuiteCleanup(req.RunId, true)

	ended := time.Now().UTC()
	e.mu.Lock()
	if runInfo, ok := e.runs[req.RunId]; ok {
		runInfo.EndedAt = ended
	}
	e.mu.Unlock()

	counts, err := e.getTestStatusCounts(req.RunId)
	if err != nil {
		slog.Warn("CancelRun: unable to compute test counts", "run_id", req.RunId, "error", err)
		counts = TestStatusCounts{}
	}

	if orgID != uuid.Nil && e.runStore != nil {
		if _, updErr := e.runStore.UpdateRun(ctx, persistence.RunUpdate{
			RunID:          req.RunId,
			OrganizationID: orgID,
			Status:         stringPtr("CANCELLED"),
			EndedAt:        timePtr(ended),
			Totals:         makeRunTotals(counts),
		}); updErr != nil {
			slog.Error("CancelRun: failed to persist cancellation", "run_id", req.RunId, "error", updErr)
		}
	}

	if len(cancelErrors) > 0 {
		slog.Warn("CancelRun: Completed with errors", "run_id", req.RunId, "errors", cancelErrors)
		return &generated.CancelRunResponse{
			Success: false,
			Message: fmt.Sprintf("cancelled with errors: %s", strings.Join(cancelErrors, "; ")),
		}, nil
	}

	slog.Info("CancelRun: Completed successfully", "run_id", req.RunId)
	return &generated.CancelRunResponse{
		Success: true,
		Message: "run cancelled successfully",
	}, nil
}

func (e *Engine) listRunsInMemory(req *generated.ListRunsRequest) (*generated.ListRunsResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	runs := make([]*generated.RunSummary, 0)

	for _, runInfo := range e.runs {
		if req.Status != "" && runInfo.Status != req.Status {
			continue
		}
		if req.ProjectId != "" && runInfo.Context.ProjectID != req.ProjectId {
			continue
		}
		if req.Source != "" && runInfo.Context.Source != req.Source {
			continue
		}
		if req.Branch != "" && !strings.EqualFold(runInfo.Context.Branch, req.Branch) {
			continue
		}
		if req.ScheduleName != "" && runInfo.Context.ScheduleName != req.ScheduleName {
			continue
		}

		var passed, failed, timeout int32
		for _, test := range runInfo.Tests {
			switch test.Status {
			case "PASSED":
				passed++
			case "FAILED":
				failed++
			case "TIMEOUT":
				timeout++
			}
		}

		duration := int64(0)
		if !runInfo.EndedAt.IsZero() {
			duration = runInfo.EndedAt.Sub(runInfo.StartedAt).Milliseconds()
		}

		runs = append(runs, &generated.RunSummary{
			RunId:        runInfo.ID,
			SuiteName:    runInfo.Name,
			Status:       runInfo.Status,
			StartedAt:    runInfo.StartedAt.Format(time.RFC3339),
			EndedAt:      runInfo.EndedAt.Format(time.RFC3339),
			DurationMs:   duration,
			TotalTests:   int32(len(runInfo.Tests)),
			PassedTests:  passed,
			FailedTests:  failed,
			TimeoutTests: timeout,
			Context: &generated.RunContext{
				ProjectId:    runInfo.Context.ProjectID,
				Source:       runInfo.Context.Source,
				Branch:       runInfo.Context.Branch,
				CommitSha:    runInfo.Context.CommitSHA,
				Trigger:      runInfo.Context.Trigger,
				ScheduleName: runInfo.Context.ScheduleName,
				Metadata:     runInfo.Context.Metadata,
			},
		})
	}

	sortRuns(runs, req.OrderBy, !req.Descending)

	limit := req.Limit
	if limit <= 0 {
		limit = 50
	}
	if len(runs) > int(limit) {
		runs = runs[:limit]
	}

	return &generated.ListRunsResponse{
		Runs:       runs,
		TotalCount: int32(len(runs)),
	}, nil
}

func (e *Engine) getRunInMemory(runID string) (*generated.GetRunResponse, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if runInfo, exists := e.runs[runID]; exists {
		slog.Debug("Found active run in memory", "run_id", runID)
		return mapRunInfoToRunDetails(runInfo), nil
	}

	if len(runID) <= 12 {
		slog.Debug("Searching for run by prefix", "prefix", runID)
		for fullID, runInfo := range e.runs {
			if strings.HasPrefix(fullID, runID) {
				slog.Debug("Found run by prefix match", "prefix", runID, "full_id", fullID)
				return mapRunInfoToRunDetails(runInfo), nil
			}
		}
	}

	return nil, fmt.Errorf("run not found: %s", runID)
}
