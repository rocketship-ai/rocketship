package orchestrator

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/controlplane/persistence"
	"github.com/rocketship-ai/rocketship/internal/dsl"
	"go.temporal.io/sdk/client"
	yaml "gopkg.in/yaml.v3"
)

// createRunInternal creates a run bypassing auth/context resolution.
// Used by the scheduler to create scheduled runs internally.
func (e *Engine) createRunInternal(ctx context.Context, orgID uuid.UUID, initiator string, req *generated.CreateRunRequest) (*generated.CreateRunResponse, error) {
	if req == nil {
		return nil, fmt.Errorf("request cannot be nil")
	}
	if len(req.YamlPayload) == 0 {
		return nil, fmt.Errorf("YAML payload cannot be empty")
	}
	if orgID == uuid.Nil {
		return nil, fmt.Errorf("organization ID is required for scheduled runs")
	}

	slog.Debug("createRunInternal called", "payload_size", len(req.YamlPayload), "org_id", orgID.String(), "initiator", initiator)

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

	// Environment resolution variables
	var envSecrets map[string]string
	var envConfigVars map[string]interface{}

	if e.runStore != nil {
		// Extract schedule_id from metadata if present
		var scheduleID uuid.NullUUID
		if runContext.Metadata != nil {
			if sid := strings.TrimSpace(runContext.Metadata["rs_schedule_id"]); sid != "" {
				if parsed, err := uuid.Parse(sid); err == nil {
					scheduleID = uuid.NullUUID{UUID: parsed, Valid: true}
				}
			}
		}

		// Extract environment_id from metadata if present
		var environmentID uuid.NullUUID
		if runContext.Metadata != nil {
			if eid := strings.TrimSpace(runContext.Metadata["rs_environment_id"]); eid != "" {
				if parsed, err := uuid.Parse(eid); err == nil {
					environmentID = uuid.NullUUID{UUID: parsed, Valid: true}
				}
			}
		}

		// Extract schedule_type from metadata if present (for project/suite schedules)
		var scheduleType sql.NullString
		if runContext.Metadata != nil {
			if st := strings.TrimSpace(runContext.Metadata["rs_schedule_type"]); st != "" {
				scheduleType = sql.NullString{String: st, Valid: true}
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
			ScheduleType:   scheduleType,
			ConfigSource:   configSource,
			Source:         strings.TrimSpace(runContext.Source),
			Branch:         strings.TrimSpace(runContext.Branch),
			TotalTests:     len(run.Tests),
			PassedTests:    0,
			FailedTests:    0,
			TimeoutTests:   0,
			StartedAt:      sql.NullTime{Time: startTime, Valid: true},
			ScheduleID:     scheduleID,
			EnvironmentID:  environmentID,
			Environment:    envSlug,
		}

		// Parse project_id as UUID if it's a valid UUID
		if projectID, err := uuid.Parse(runContext.ProjectID); err == nil && projectID != uuid.Nil {
			record.ProjectID = uuid.NullUUID{UUID: projectID, Valid: true}
		}

		// Resolve environment after project_id is set
		if record.ProjectID.Valid && envSlug != "" {
			env, err := e.runStore.GetEnvironmentBySlug(ctx, record.ProjectID.UUID, envSlug)
			if err != nil {
				if err == sql.ErrNoRows {
					slog.Warn("createRunInternal: environment not found", "slug", envSlug, "project_id", record.ProjectID.UUID)
				} else {
					slog.Debug("createRunInternal: failed to lookup environment", "slug", envSlug, "error", err)
				}
			} else {
				envSecrets = env.EnvSecrets
				envConfigVars = env.ConfigVars
				record.EnvironmentID = uuid.NullUUID{UUID: env.ID, Valid: true}
				record.Environment = env.Slug
				slog.Debug("createRunInternal: resolved environment",
					"env_id", env.ID,
					"env_slug", env.Slug,
					"secrets_count", len(envSecrets),
					"config_vars_count", len(envConfigVars))
			}
		}

		if _, err := e.runStore.InsertRun(ctx, record); err != nil {
			slog.Error("createRunInternal: failed to persist run metadata", "run_id", runID, "error", err)
			return nil, fmt.Errorf("failed to persist run metadata: %w", err)
		}
	}

	slog.Debug("Starting scheduled run",
		"name", run.Name,
		"test_count", len(run.Tests),
		"project_id", runContext.ProjectID,
		"source", runContext.Source,
		"branch", runContext.Branch,
		"schedule_name", runContext.ScheduleName)

	// Resolve suite_id and build test name → test_id map for linking
	var resolvedProjectID, resolvedSuiteID uuid.UUID
	testIDMap := make(map[string]uuid.UUID)
	if e.runStore != nil {
		if projectID, err := uuid.Parse(runContext.ProjectID); err == nil && projectID != uuid.Nil {
			resolvedProjectID = projectID
		}

		if resolvedProjectID != uuid.Nil {
			sourceRef := strings.TrimSpace(runContext.Branch)
			if sourceRef == "" {
				sourceRef = "main"
			}
			suite, found, err := e.runStore.GetSuiteByName(ctx, resolvedProjectID, run.Name, sourceRef)
			if err != nil {
				slog.Debug("createRunInternal: failed to lookup suite", "error", err)
			} else if found {
				resolvedSuiteID = suite.ID
				slog.Debug("createRunInternal: resolved suite_id", "suite_id", suite.ID, "suite_name", run.Name, "source_ref", sourceRef)

				tests, err := e.runStore.ListTestsBySuite(ctx, suite.ID)
				if err != nil {
					slog.Debug("createRunInternal: failed to list tests for suite", "error", err)
				} else {
					for _, t := range tests {
						key := strings.ToLower(t.Name)
						testIDMap[key] = t.ID
					}
					slog.Debug("createRunInternal: built test_id map", "count", len(testIDMap))
				}
			}
		}
	}

	// Merge environment config vars with run.Vars
	mergedVars := dsl.MergeInterfaceMaps(envConfigVars, run.Vars)

	// Server-side: Apply .vars.* substitution using merged vars
	if len(mergedVars) > 0 {
		var yamlDoc interface{}
		if err := json.Unmarshal(req.YamlPayload, &yamlDoc); err != nil {
			if yamlErr := yaml.Unmarshal(req.YamlPayload, &yamlDoc); yamlErr != nil {
				slog.Debug("createRunInternal: failed to parse YAML for vars substitution", "error", yamlErr)
			} else {
				processedDoc, err := dsl.ProcessVarsOnlyRecursive(yamlDoc, mergedVars)
				if err != nil {
					slog.Warn("createRunInternal: failed to process config variables", "error", err)
				} else {
					processedYaml, err := yaml.Marshal(processedDoc)
					if err != nil {
						slog.Warn("createRunInternal: failed to marshal processed YAML", "error", err)
					} else {
						newRun, err := dsl.ParseYAML(processedYaml)
						if err != nil {
							slog.Warn("createRunInternal: failed to re-parse processed YAML", "error", err)
						} else {
							run = newRun
							slog.Debug("createRunInternal: applied server-side vars substitution", "vars_count", len(mergedVars))
						}
					}
				}
			}
		} else {
			processedDoc, err := dsl.ProcessVarsOnlyRecursive(yamlDoc, mergedVars)
			if err != nil {
				slog.Warn("createRunInternal: failed to process config variables (JSON)", "error", err)
			} else {
				processedYaml, err := yaml.Marshal(processedDoc)
				if err != nil {
					slog.Warn("createRunInternal: failed to marshal processed YAML (JSON)", "error", err)
				} else {
					newRun, err := dsl.ParseYAML(processedYaml)
					if err != nil {
						slog.Warn("createRunInternal: failed to re-parse processed YAML (JSON)", "error", err)
					} else {
						run = newRun
						slog.Debug("createRunInternal: applied server-side vars substitution (JSON)", "vars_count", len(mergedVars))
					}
				}
			}
		}
	}

	// Extract schedule info for run completion callback
	var scheduleIDForRunInfo uuid.UUID
	var scheduleTypeForRunInfo string
	if runContext.Metadata != nil {
		if sid := strings.TrimSpace(runContext.Metadata["rs_schedule_id"]); sid != "" {
			if parsed, err := uuid.Parse(sid); err == nil {
				scheduleIDForRunInfo = parsed
			}
		}
		if st := strings.TrimSpace(runContext.Metadata["rs_schedule_type"]); st != "" {
			scheduleTypeForRunInfo = st
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
		ScheduleID:     scheduleIDForRunInfo,
		ScheduleType:   scheduleTypeForRunInfo,
		Logs: []LogLine{
			{
				Msg:   fmt.Sprintf("Starting scheduled run \"%s\"... 🚀 [schedule: %s]", run.Name, runContext.ScheduleName),
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
		if e.runStore != nil {
			runTest := persistence.RunTest{
				RunID:      runID,
				WorkflowID: testID,
				Name:       test.Name,
				Status:     "PENDING",
				StartedAt:  sql.NullTime{Time: testStartTime, Valid: true},
				StepCount:  len(test.Steps),
			}
			if discoveredTestID != uuid.Nil {
				runTest.TestID = uuid.NullUUID{UUID: discoveredTestID, Valid: true}
			}
			if _, err := e.runStore.InsertRunTest(ctx, runTest); err != nil {
				slog.Error("createRunInternal: failed to persist run_test", "run_id", runID, "workflow_id", testID, "error", err)
			}
		}

		workflowOptions := client.StartWorkflowOptions{
			ID:        testID,
			TaskQueue: "test-workflows",
		}

		suiteGlobalsCopy := cloneStringMap(suiteGlobals)
		envSecretsCopy := cloneStringMap(runInfo.EnvSecrets)
		execution, err := e.temporal.ExecuteWorkflow(ctx, workflowOptions, "TestWorkflow", test, runInfo.Vars, runID, run.OpenAPI, suiteGlobalsCopy, envSecretsCopy)
		if err != nil {
			log.Printf("[ERROR] Failed to start workflow for scheduled run %s: %v", runID, err)
			e.addLog(runID, fmt.Sprintf("Failed to start test \"%s\": %v", test.Name, err), "red", true)
			e.triggerSuiteCleanup(runID, true)

			if e.runStore != nil {
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
					slog.Error("createRunInternal: failed to mark run as failed after workflow start error", "run_id", runID, "error", updErr)
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
