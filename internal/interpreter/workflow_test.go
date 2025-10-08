package interpreter

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/rocketship-ai/rocketship/internal/dsl"
	"github.com/rocketship-ai/rocketship/internal/plugins/http"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func TestHandleDelayStep(t *testing.T) {
	tests := []struct {
		name    string
		step    dsl.Step
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid duration",
			step: dsl.Step{
				Name: "test-delay",
				Config: map[string]interface{}{
					"duration": "1s",
				},
			},
			wantErr: false,
		},
		{
			name: "missing duration",
			step: dsl.Step{
				Name:   "test-delay",
				Config: map[string]interface{}{},
			},
			wantErr: true,
			errMsg:  "duration is required and must be a string",
		},
		{
			name: "invalid duration type",
			step: dsl.Step{
				Name: "test-delay",
				Config: map[string]interface{}{
					"duration": 123,
				},
			},
			wantErr: true,
			errMsg:  "duration is required and must be a string",
		},
		{
			name: "invalid duration format",
			step: dsl.Step{
				Name: "test-delay",
				Config: map[string]interface{}{
					"duration": "invalid",
				},
			},
			wantErr: true,
			errMsg:  "invalid duration format",
		},
		{
			name: "complex duration",
			step: dsl.Step{
				Name: "test-delay",
				Config: map[string]interface{}{
					"duration": "1h30m45s",
				},
			},
			wantErr: false,
		},
	}

	// Test each case concurrently for speed
	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// Use temporal test suite for workflow context
			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestWorkflowEnvironment()

			// Run the handleDelayStep function in a test workflow
			env.ExecuteWorkflow(func(ctx workflow.Context) error {
				return handleDelayStep(ctx, tt.step, "test-name", "test-run-id")
			})

			if tt.wantErr {
				if env.IsWorkflowCompleted() {
					err := env.GetWorkflowError()
					if err == nil {
						t.Error("Expected error, got nil")
						return
					}
					if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
					}
				} else {
					t.Error("Expected workflow to complete with error")
				}
			} else {
				if !env.IsWorkflowCompleted() {
					t.Error("Expected workflow to complete successfully")
					return
				}
				if err := env.GetWorkflowError(); err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestTestWorkflow(t *testing.T) {
	tests := []struct {
		name     string
		test     dsl.Test
		wantErr  bool
		errMsg   string
		setupEnv func(*testsuite.TestWorkflowEnvironment)
	}{
		{
			name: "workflow with delay step",
			test: dsl.Test{
				Name: "test-workflow",
				Steps: []dsl.Step{
					{
						Name:   "delay-step",
						Plugin: "delay",
						Config: map[string]interface{}{
							"duration": "100ms",
						},
					},
				},
			},
			wantErr: false,
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				// Register LogForwarderActivity for step logging
				env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
				// Mock LogForwarderActivity to return success
				env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
					map[string]interface{}{"forwarded": true}, nil)
			},
		},
		{
			name: "workflow with HTTP step",
			test: dsl.Test{
				Name: "test-workflow",
				Steps: []dsl.Step{
					{
						Name:   "http-step",
						Plugin: "http",
						Config: map[string]interface{}{
							"method": "GET",
							"url":    "http://example.com",
						},
					},
				},
			},
			wantErr: false,
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				// Register activities
				env.RegisterActivityWithOptions((&http.HTTPPlugin{}).Activity, activity.RegisterOptions{Name: "http"})
				env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
				env.OnActivity("http", mock.Anything, mock.Anything).Return(
					&http.ActivityResponse{
						Response: &http.HTTPResponse{StatusCode: 200},
						Saved:    map[string]string{},
					}, nil)
				// Mock LogForwarderActivity to return success
				env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
					map[string]interface{}{"forwarded": true}, nil)
			},
		},
		{
			name: "workflow with unknown plugin",
			test: dsl.Test{
				Name: "test-workflow",
				Steps: []dsl.Step{
					{
						Name:   "unknown-step",
						Plugin: "unknown",
					},
				},
			},
			wantErr: true,
			errMsg:  "unknown plugin",
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				// Register LogForwarderActivity for step logging
				env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
				// Mock LogForwarderActivity to return success
				env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
					map[string]interface{}{"forwarded": true}, nil)
			},
		},
		{
			name: "workflow with multiple steps",
			test: dsl.Test{
				Name: "test-workflow",
				Steps: []dsl.Step{
					{
						Name:   "delay-step",
						Plugin: "delay",
						Config: map[string]interface{}{
							"duration": "50ms",
						},
					},
					{
						Name:   "http-step",
						Plugin: "http",
						Config: map[string]interface{}{
							"method": "POST",
							"url":    "http://example.com/users",
							"body":   `{"name": "{{ user_name }}"}`,
						},
						Save: []map[string]interface{}{
							{
								"json_path": ".id",
								"as":        "user_id",
							},
						},
					},
					{
						Name:   "second-http-step",
						Plugin: "http",
						Config: map[string]interface{}{
							"method": "GET",
							"url":    "http://example.com/users/{{ user_id }}",
						},
					},
				},
			},
			wantErr: false,
			setupEnv: func(env *testsuite.TestWorkflowEnvironment) {
				// Register activities
				env.RegisterActivityWithOptions((&http.HTTPPlugin{}).Activity, activity.RegisterOptions{Name: "http"})
				env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
				// Mock LogForwarderActivity to return success
				env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
					map[string]interface{}{"forwarded": true}, nil)
				// First HTTP call returns user creation
				env.OnActivity("http", mock.Anything, mock.MatchedBy(func(params map[string]interface{}) bool {
					config := params["config"].(map[string]interface{})
					return config["method"] == "POST"
				})).Return(
					&http.ActivityResponse{
						Response: &http.HTTPResponse{StatusCode: 201},
						Saved:    map[string]string{"user_id": "456"},
					}, nil)

				// Second HTTP call uses the saved user_id
				env.OnActivity("http", mock.Anything, mock.MatchedBy(func(params map[string]interface{}) bool {
					config := params["config"].(map[string]interface{})
					return config["method"] == "GET"
				})).Return(
					&http.ActivityResponse{
						Response: &http.HTTPResponse{StatusCode: 200},
						Saved:    map[string]string{},
					}, nil)
			},
		},
	}

	t.Parallel()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestWorkflowEnvironment()

			if tt.setupEnv != nil {
				tt.setupEnv(env)
			}

			env.ExecuteWorkflow(TestWorkflow, tt.test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))

			if tt.wantErr {
				if env.IsWorkflowCompleted() {
					err := env.GetWorkflowError()
					if err == nil {
						t.Error("Expected error, got nil")
						return
					}
					if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
						t.Errorf("Expected error containing '%s', got '%s'", tt.errMsg, err.Error())
					}
				} else {
					t.Error("Expected workflow to complete with error")
				}
			} else {
				if !env.IsWorkflowCompleted() {
					t.Error("Expected workflow to complete successfully")
					return
				}
				if err := env.GetWorkflowError(); err != nil {
					t.Errorf("Expected no error, got %v", err)
				}
			}
		})
	}
}

func TestWorkflowStatePropagation(t *testing.T) {
	// Test that state is properly propagated between HTTP steps
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities with proper names
	env.RegisterActivityWithOptions((&http.HTTPPlugin{}).Activity, activity.RegisterOptions{Name: "http"})
	env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})

	// Mock LogForwarderActivity to return success
	env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
		map[string]interface{}{"forwarded": true}, nil)

	// Track the calls to verify state propagation
	var callOrder []map[string]interface{}
	var mu sync.Mutex

	env.OnActivity("http", mock.Anything, mock.Anything).Return(
		func(ctx context.Context, params map[string]interface{}) (interface{}, error) {
			mu.Lock()
			callOrder = append(callOrder, params)
			mu.Unlock()

			// Return different saved values based on call order
			if len(callOrder) == 1 {
				return &http.ActivityResponse{
					Response: &http.HTTPResponse{StatusCode: 200},
					Saved:    map[string]string{"token": "abc123", "user_id": "789"},
				}, nil
			} else {
				return &http.ActivityResponse{
					Response: &http.HTTPResponse{StatusCode: 200},
					Saved:    map[string]string{},
				}, nil
			}
		})

	test := dsl.Test{
		Name: "state-propagation-test",
		Steps: []dsl.Step{
			{
				Name:   "first-step",
				Plugin: "http",
				Config: map[string]interface{}{
					"method": "POST",
					"url":    "http://example.com/login",
				},
				Save: []map[string]interface{}{
					{
						"json_path": ".token",
						"as":        "token",
					},
					{
						"json_path": ".user_id",
						"as":        "user_id",
					},
				},
			},
			{
				Name:   "second-step",
				Plugin: "http",
				Config: map[string]interface{}{
					"method": "GET",
					"url":    "http://example.com/profile",
					"headers": map[string]interface{}{
						"Authorization": "Bearer {{ token }}",
					},
				},
			},
		},
	}

	env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))

	if !env.IsWorkflowCompleted() {
		t.Fatal("Expected workflow to complete successfully")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	// Verify we made the expected number of calls
	if len(callOrder) != 2 {
		t.Fatalf("Expected 2 activity calls, got %d", len(callOrder))
	}

	// Verify first call has empty state
	firstCallState := callOrder[0]["state"].(map[string]interface{})
	if len(firstCallState) != 0 {
		t.Errorf("Expected first call to have empty state, got %v", firstCallState)
	}

	// Verify second call has state from first call
	secondCallState := callOrder[1]["state"].(map[string]interface{})
	if secondCallState["token"] != "abc123" {
		t.Errorf("Expected token 'abc123' in second call state, got '%v'", secondCallState["token"])
	}
	if secondCallState["user_id"] != "789" {
		t.Errorf("Expected user_id '789' in second call state, got '%v'", secondCallState["user_id"])
	}
}

func TestWorkflowConcurrency(t *testing.T) {
	// Test that workflows can run concurrently without interference
	numWorkflows := 10
	var wg sync.WaitGroup
	errors := make(chan error, numWorkflows)

	for i := 0; i < numWorkflows; i++ {
		wg.Add(1)
		go func(workflowID int) {
			defer wg.Done()

			testSuite := &testsuite.WorkflowTestSuite{}
			env := testSuite.NewTestWorkflowEnvironment()

			// Register activities
			env.RegisterActivityWithOptions((&http.HTTPPlugin{}).Activity, activity.RegisterOptions{Name: "http"})
			env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})

			// Mock LogForwarderActivity to return success
			env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
				map[string]interface{}{"forwarded": true}, nil)

			// Each workflow should have its own isolated state
			expectedValue := fmt.Sprintf("value-%d", workflowID)

			env.OnActivity("http", mock.Anything, mock.Anything).Return(
				&http.ActivityResponse{
					Response: &http.HTTPResponse{StatusCode: 200},
					Saved:    map[string]string{"workflow_id": expectedValue},
				}, nil)

			test := dsl.Test{
				Name: fmt.Sprintf("concurrent-test-%d", workflowID),
				Steps: []dsl.Step{
					{
						Name:   "delay-step",
						Plugin: "delay",
						Config: map[string]interface{}{
							"duration": "1ms", // Very short delay
						},
					},
					{
						Name:   "http-step",
						Plugin: "http",
						Config: map[string]interface{}{
							"method": "GET",
							"url":    "http://example.com",
						},
						Save: []map[string]interface{}{
							{
								"json_path": ".workflow_id",
								"as":        "workflow_id",
							},
						},
					},
				},
			}

			env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))

			if !env.IsWorkflowCompleted() {
				errors <- fmt.Errorf("workflow %d did not complete", workflowID)
				return
			}
			if err := env.GetWorkflowError(); err != nil {
				errors <- fmt.Errorf("workflow %d failed: %v", workflowID, err)
				return
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Error(err)
	}
}

func TestWorkflowWithComplexState(t *testing.T) {
	// Test workflow with complex state transitions and variable substitution
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	// Register activities
	env.RegisterActivityWithOptions((&http.HTTPPlugin{}).Activity, activity.RegisterOptions{Name: "http"})
	env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})

	// Mock LogForwarderActivity to return success
	env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).Return(
		map[string]interface{}{"forwarded": true}, nil)

	// Mock responses for different steps
	env.OnActivity("http", mock.Anything, mock.MatchedBy(func(params map[string]interface{}) bool {
		config := params["config"].(map[string]interface{})
		return config["url"] == "http://api.example.com/auth"
	})).Return(
		&http.ActivityResponse{
			Response: &http.HTTPResponse{StatusCode: 200},
			Saved:    map[string]string{"auth_token": "token123", "user_id": "user456"},
		}, nil)

	env.OnActivity("http", mock.Anything, mock.MatchedBy(func(params map[string]interface{}) bool {
		config := params["config"].(map[string]interface{})
		return contains(config["url"].(string), "/users/")
	})).Return(
		&http.ActivityResponse{
			Response: &http.HTTPResponse{StatusCode: 200},
			Saved:    map[string]string{"profile_id": "profile789"},
		}, nil)

	env.OnActivity("http", mock.Anything, mock.MatchedBy(func(params map[string]interface{}) bool {
		config := params["config"].(map[string]interface{})
		return contains(config["url"].(string), "/profiles/")
	})).Return(
		&http.ActivityResponse{
			Response: &http.HTTPResponse{StatusCode: 200},
			Saved:    map[string]string{},
		}, nil)

	test := dsl.Test{
		Name: "complex-state-test",
		Steps: []dsl.Step{
			{
				Name:   "authenticate",
				Plugin: "http",
				Config: map[string]interface{}{
					"method": "POST",
					"url":    "http://api.example.com/auth",
					"body":   `{"username": "test", "password": "pass"}`,
				},
				Save: []map[string]interface{}{
					{"json_path": ".token", "as": "auth_token"},
					{"json_path": ".user_id", "as": "user_id"},
				},
			},
			{
				Name:   "get-user-profile",
				Plugin: "http",
				Config: map[string]interface{}{
					"method": "GET",
					"url":    "http://api.example.com/users/{{ user_id }}",
					"headers": map[string]interface{}{
						"Authorization": "Bearer {{ auth_token }}",
					},
				},
				Save: []map[string]interface{}{
					{"json_path": ".profile_id", "as": "profile_id"},
				},
			},
			{
				Name:   "update-profile",
				Plugin: "http",
				Config: map[string]interface{}{
					"method": "PUT",
					"url":    "http://api.example.com/profiles/{{ profile_id }}",
					"headers": map[string]interface{}{
						"Authorization": "Bearer {{ auth_token }}",
					},
					"body": `{"user_id": "{{ user_id }}", "updated": true}`,
				},
			},
		},
	}

	env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))

	if !env.IsWorkflowCompleted() {
		t.Fatal("Expected workflow to complete successfully")
	}
	if err := env.GetWorkflowError(); err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
}

func TestTestWorkflow_InitFailureTriggersCleanup(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var mu sync.Mutex
	startCounts := make(map[string]int)
	successCounts := make(map[string]int)
	failureCounts := make(map[string]int)

	env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
	env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).
		Return(map[string]interface{}{"forwarded": true}, nil).
		Run(func(args mock.Arguments) {
			params, _ := args.Get(1).(map[string]interface{})
			stepName, _ := params["step_name"].(string)
			message, _ := params["message"].(string)
			if stepName == "" {
				return
			}
			mu.Lock()
			switch {
			case strings.Contains(message, "Starting"):
				startCounts[stepName]++
			case strings.Contains(message, "completed"):
				successCounts[stepName]++
			case strings.Contains(message, "failed"):
				failureCounts[stepName]++
			}
			mu.Unlock()
		})

	test := dsl.Test{
		Name: "init failure test",
		Init: []dsl.Step{
			{
				Name:   "suite-setup",
				Plugin: "delay",
				Config: map[string]interface{}{},
			},
		},
		Steps: []dsl.Step{
			{
				Name:   "main-step",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "1s"},
			},
		},
		Cleanup: &dsl.CleanupSpec{
			OnFailure: []dsl.Step{
				{
					Name:   "cleanup-on-failure",
					Plugin: "delay",
					Config: map[string]interface{}{"duration": "1s"},
				},
			},
			Always: []dsl.Step{
				{
					Name:   "cleanup-always",
					Plugin: "delay",
					Config: map[string]interface{}{"duration": "1s"},
				},
			},
		},
	}

	env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))
	err := env.GetWorkflowError()
	assert.Error(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, failureCounts["suite-setup"], 1)
	assert.Zero(t, startCounts["main-step"], "main-step should not run when init fails")
	assert.GreaterOrEqual(t, startCounts["cleanup-on-failure"], 1)
	assert.GreaterOrEqual(t, successCounts["cleanup-on-failure"], 1)
	assert.GreaterOrEqual(t, startCounts["cleanup-always"], 1)
	assert.GreaterOrEqual(t, successCounts["cleanup-always"], 1)
}

func TestTestWorkflow_SuccessRunsCleanupAlwaysOnly(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var mu sync.Mutex
	startCounts := make(map[string]int)

	env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
	env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).
		Return(map[string]interface{}{"forwarded": true}, nil).
		Run(func(args mock.Arguments) {
			params, _ := args.Get(1).(map[string]interface{})
			stepName, _ := params["step_name"].(string)
			message, _ := params["message"].(string)
			if stepName == "" {
				return
			}
			if strings.Contains(message, "Starting") {
				mu.Lock()
				startCounts[stepName]++
				mu.Unlock()
			}
		})

	test := dsl.Test{
		Name: "success cleanup test",
		Init: []dsl.Step{
			{
				Name:   "suite-setup",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "1s"},
			},
		},
		Steps: []dsl.Step{
			{
				Name:   "main-step",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "1s"},
			},
		},
		Cleanup: &dsl.CleanupSpec{
			OnFailure: []dsl.Step{
				{
					Name:   "cleanup-on-failure",
					Plugin: "delay",
					Config: map[string]interface{}{"duration": "1s"},
				},
			},
			Always: []dsl.Step{
				{
					Name:   "cleanup-always",
					Plugin: "delay",
					Config: map[string]interface{}{"duration": "1s"},
				},
			},
		},
	}

	env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "test-run-id", (*dsl.OpenAPISuiteConfig)(nil), map[string]string(nil))
	err := env.GetWorkflowError()
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, startCounts["cleanup-always"], 1)
	assert.Zero(t, startCounts["cleanup-on-failure"], "on_failure cleanup should not run on success")
}

func TestTestWorkflowInjectsSuiteGlobalsIntoState(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
	env.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).
		Return(map[string]interface{}{"forwarded": true}, nil)

	test := dsl.Test{
		Name: "suite globals state",
		Steps: []dsl.Step{
			{
				Name:   "noop",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "0s"},
			},
		},
	}

	suiteGlobals := map[string]string{"api_token": "abc123"}

	env.ExecuteWorkflow(TestWorkflow, test, make(map[string]interface{}), "run-id", (*dsl.OpenAPISuiteConfig)(nil), suiteGlobals)
	assert.NoError(t, env.GetWorkflowError())

	var state map[string]string
	err := env.GetWorkflowResult(&state)
	assert.NoError(t, err)
	assert.Equal(t, "abc123", state["api_token"])
	_, hasLegacyKey := state["suite.saved.api_token"]
	assert.False(t, hasLegacyKey)
}

func TestSuiteCleanupWorkflowHonorsFailureFlag(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var mu sync.Mutex
	startCounts := make(map[string]int)

	registerLogCapture := func(environment *testsuite.TestWorkflowEnvironment) {
		environment.RegisterActivityWithOptions(LogForwarderActivity, activity.RegisterOptions{Name: "LogForwarderActivity"})
		environment.OnActivity("LogForwarderActivity", mock.Anything, mock.Anything).
			Return(map[string]interface{}{"forwarded": true}, nil).
			Run(func(args mock.Arguments) {
				params, _ := args.Get(1).(map[string]interface{})
				stepName, _ := params["step_name"].(string)
				message, _ := params["message"].(string)
				if stepName == "" {
					return
				}
				if strings.Contains(message, "Starting") {
					mu.Lock()
					startCounts[stepName]++
					mu.Unlock()
				}
			})
	}

	registerLogCapture(env)
	cleanup := &dsl.CleanupSpec{
		OnFailure: []dsl.Step{
			{
				Name:   "suite-on-failure",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "1s"},
			},
		},
		Always: []dsl.Step{
			{
				Name:   "suite-always",
				Plugin: "delay",
				Config: map[string]interface{}{"duration": "1s"},
			},
		},
	}

	env.ExecuteWorkflow(SuiteCleanupWorkflow, SuiteCleanupParams{
		RunID:          "run-id",
		TestName:       "suite-cleanup",
		Cleanup:        cleanup,
		Vars:           map[string]interface{}{},
		SuiteGlobals:   map[string]string{"foo": "bar"},
		TreatAsFailure: true,
	})
	err := env.GetWorkflowError()
	assert.NoError(t, err)

	mu.Lock()
	assert.GreaterOrEqual(t, startCounts["suite-always"], 1)
	assert.GreaterOrEqual(t, startCounts["suite-on-failure"], 1)
	mu.Unlock()

	// Reset state and rerun without failure flag
	mu.Lock()
	for k := range startCounts {
		startCounts[k] = 0
	}
	mu.Unlock()

	env = testSuite.NewTestWorkflowEnvironment()
	registerLogCapture(env)

	env.ExecuteWorkflow(SuiteCleanupWorkflow, SuiteCleanupParams{
		RunID:          "run-id",
		TestName:       "suite-cleanup",
		Cleanup:        cleanup,
		Vars:           map[string]interface{}{},
		SuiteGlobals:   map[string]string{},
		TreatAsFailure: false,
	})
	err = env.GetWorkflowError()
	assert.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()
	assert.GreaterOrEqual(t, startCounts["suite-always"], 1)
	assert.Zero(t, startCounts["suite-on-failure"], "suite on_failure should not run when not flagged")
}

// Helper function for string containment
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > len(substr) && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
