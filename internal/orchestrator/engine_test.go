package orchestrator

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"github.com/rocketship-ai/rocketship/internal/authbroker/persistence"
	"go.temporal.io/sdk/client"
)

type MockTemporalClient struct {
	client.Client
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestNewEngine(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	if engine == nil {
		t.Fatal("Expected engine to be created, got nil")
	}

	if engine.temporal == nil {
		t.Error("Expected temporal client to be set")
	}

	if engine.runs == nil {
		t.Error("Expected runs map to be initialized")
	}

	if len(engine.runs) != 0 {
		t.Error("Expected runs map to be empty initially")
	}
}

func TestGenerateID(t *testing.T) {
	t.Run("successful generation", func(t *testing.T) {
		id1, err := generateID()
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		id2, err := generateID()
		if err != nil {
			t.Fatalf("Failed to generate ID: %v", err)
		}

		if id1 == id2 {
			t.Error("Expected different IDs to be generated")
		}

		if len(id1) != 26 {
			t.Errorf("Expected ID length to be 26, got %d", len(id1))
		}
	})

	t.Run("uniqueness over many generations", func(t *testing.T) {
		ids := make(map[string]bool)
		for i := 0; i < 1000; i++ {
			id, err := generateID()
			if err != nil {
				t.Fatalf("Failed to generate ID: %v", err)
			}
			if ids[id] {
				t.Errorf("Duplicate ID generated: %s", id)
			}
			ids[id] = true
		}
	})
}

func TestAddLog(t *testing.T) {
	t.Run("successful log addition", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Status: "RUNNING",
			Logs:   []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.addLog("test-run-id", "Test log message", "green", true)

		if len(runInfo.Logs) != 1 {
			t.Fatalf("Expected 1 log message, got %d", len(runInfo.Logs))
		}

		if runInfo.Logs[0].Msg != "Test log message" {
			t.Errorf("Expected log message 'Test log message', got '%s'", runInfo.Logs[0].Msg)
		}

		if runInfo.Logs[0].Color != "green" {
			t.Errorf("Expected color 'green', got '%s'", runInfo.Logs[0].Color)
		}

		if !runInfo.Logs[0].Bold {
			t.Error("Expected bold to be true")
		}
	})

	t.Run("multiple log additions", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Status: "RUNNING",
			Logs:   []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.addLog("test-run-id", "First message", "green", true)
		engine.addLog("test-run-id", "Second message", "red", false)

		if len(runInfo.Logs) != 2 {
			t.Fatalf("Expected 2 log messages, got %d", len(runInfo.Logs))
		}

		if runInfo.Logs[1].Msg != "Second message" {
			t.Errorf("Expected second log message 'Second message', got '%s'", runInfo.Logs[1].Msg)
		}
	})

	t.Run("nonexistent run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		// Should not panic when run doesn't exist
		engine.addLog("nonexistent-run-id", "Test log message", "green", true)
		// Test passes if no panic occurs
	})
}

func TestGetTestStatusCounts(t *testing.T) {
	t.Run("empty run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:    "test-run-id",
			Tests: make(map[string]*TestInfo),
		}
		engine.runs["test-run-id"] = runInfo

		counts, err := engine.getTestStatusCounts("test-run-id")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if counts.Total != 0 {
			t.Errorf("Expected total 0, got %d", counts.Total)
		}
	})

	t.Run("various test statuses", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID: "test-run-id",
			Tests: map[string]*TestInfo{
				"test1": {Status: "PASSED"},
				"test2": {Status: "FAILED"},
				"test3": {Status: "PENDING"},
				"test4": {Status: "TIMEOUT"},
				"test5": {Status: "PASSED"},
			},
		}
		engine.runs["test-run-id"] = runInfo

		counts, err := engine.getTestStatusCounts("test-run-id")
		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if counts.Total != 5 {
			t.Errorf("Expected total 5, got %d", counts.Total)
		}
		if counts.Passed != 2 {
			t.Errorf("Expected passed 2, got %d", counts.Passed)
		}
		if counts.Failed != 1 {
			t.Errorf("Expected failed 1, got %d", counts.Failed)
		}
		if counts.Pending != 1 {
			t.Errorf("Expected pending 1, got %d", counts.Pending)
		}
		if counts.TimedOut != 1 {
			t.Errorf("Expected timed out 1, got %d", counts.TimedOut)
		}
	})

	t.Run("nonexistent run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		_, err := engine.getTestStatusCounts("nonexistent-run-id")
		if err == nil {
			t.Error("Expected error for nonexistent run")
		}
	})
}

func TestCreateRunValidation(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)
	ctx := contextWithPrincipal(context.Background(), &Principal{
		Subject: "tester",
		Email:   "tester@example.com",
		OrgID:   uuid.New().String(),
		Roles:   []string{"owner"},
	})

	t.Run("nil request", func(t *testing.T) {
		_, err := engine.CreateRun(ctx, nil)
		if err == nil {
			t.Error("Expected error for nil request")
		}
		if err.Error() != "request cannot be nil" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("empty payload", func(t *testing.T) {
		req := &generated.CreateRunRequest{
			YamlPayload: []byte{},
		}
		_, err := engine.CreateRun(ctx, req)
		if err == nil {
			t.Error("Expected error for empty payload")
		}
		if err.Error() != "YAML payload cannot be empty" {
			t.Errorf("Expected specific error message, got: %v", err)
		}
	})

	t.Run("invalid YAML", func(t *testing.T) {
		req := &generated.CreateRunRequest{
			YamlPayload: []byte("invalid: yaml: content: ["),
		}
		_, err := engine.CreateRun(ctx, req)
		if err == nil {
			t.Error("Expected error for invalid YAML")
		}
	})

	t.Run("no tests in run", func(t *testing.T) {
		validYAML := `name: "Empty Test Suite"
description: "A test suite with no tests"
tests: []`
		req := &generated.CreateRunRequest{
			YamlPayload: []byte(validYAML),
		}
		_, err := engine.CreateRun(ctx, req)
		if err == nil {
			t.Error("Expected error for run with no tests")
		}
		// The error could be from schema validation or our custom validation
		if !contains(err.Error(), "test") && !contains(err.Error(), "Array must have at least 1 items") {
			t.Errorf("Expected error related to tests, got: %v", err)
		}
	})
}

func TestHealthEndpoint(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)
	ctx := context.Background()

	resp, err := engine.Health(ctx, &generated.HealthRequest{})
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", resp.Status)
	}
}

// CONCURRENCY AND LOCK TESTING

func TestConcurrentAddLog(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	runInfo := &RunInfo{
		ID:     "test-run-id",
		Status: "RUNNING",
		Logs:   []LogLine{},
	}
	engine.runs["test-run-id"] = runInfo

	// Test concurrent log additions
	numGoroutines := 100
	numLogsPerGoroutine := 10
	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			for j := 0; j < numLogsPerGoroutine; j++ {
				msg := fmt.Sprintf("Log from goroutine %d, message %d", goroutineID, j)
				engine.addLog("test-run-id", msg, "green", true)
			}
		}(i)
	}

	wg.Wait()

	expectedLogCount := numGoroutines * numLogsPerGoroutine
	if len(runInfo.Logs) != expectedLogCount {
		t.Errorf("Expected %d log messages, got %d", expectedLogCount, len(runInfo.Logs))
	}
}

func TestConcurrentGetTestStatusCounts(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	runInfo := &RunInfo{
		ID: "test-run-id",
		Tests: map[string]*TestInfo{
			"test1": {Status: "PASSED"},
			"test2": {Status: "FAILED"},
			"test3": {Status: "PENDING"},
		},
	}
	engine.runs["test-run-id"] = runInfo

	// Test concurrent reads
	numGoroutines := 50
	var wg sync.WaitGroup
	errors := make(chan error, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			counts, err := engine.getTestStatusCounts("test-run-id")
			if err != nil {
				errors <- err
				return
			}
			if counts.Total != 3 {
				errors <- fmt.Errorf("expected total 3, got %d", counts.Total)
				return
			}
		}()
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent read error: %v", err)
	}
}

func TestConcurrentReadWriteOperations(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	runInfo := &RunInfo{
		ID: "test-run-id",
		Tests: map[string]*TestInfo{
			"test1": {Status: "PENDING"},
			"test2": {Status: "PENDING"},
		},
		Logs: []LogLine{},
	}
	engine.runs["test-run-id"] = runInfo

	// Mix of readers and writers
	numReaders := 25
	numWriters := 25
	var wg sync.WaitGroup
	errors := make(chan error, numReaders+numWriters)

	// Readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				_, err := engine.getTestStatusCounts("test-run-id")
				if err != nil {
					errors <- fmt.Errorf("reader %d: %v", readerID, err)
					return
				}
			}
		}(i)
	}

	// Writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				msg := fmt.Sprintf("Writer %d message %d", writerID, j)
				engine.addLog("test-run-id", msg, "blue", false)
			}
		}(i)
	}

	wg.Wait()
	close(errors)

	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
	}

	// Verify final state
	if len(runInfo.Logs) != numWriters*10 {
		t.Errorf("Expected %d logs, got %d", numWriters*10, len(runInfo.Logs))
	}
}

func TestUpdateTestStatusConcurrency(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	runInfo := &RunInfo{
		ID: "test-run-id",
		Tests: map[string]*TestInfo{
			"test1": {Name: "Test 1", Status: "PENDING"},
			"test2": {Name: "Test 2", Status: "PENDING"},
		},
		Logs: []LogLine{},
	}
	engine.runs["test-run-id"] = runInfo

	// Simulate concurrent test completions
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		engine.updateTestStatus("test-run-id", "test1", nil) // Success
	}()

	go func() {
		defer wg.Done()
		engine.updateTestStatus("test-run-id", "test2", fmt.Errorf("test failed")) // Failure
	}()

	wg.Wait()

	// Verify final states
	if runInfo.Tests["test1"].Status != "PASSED" {
		t.Errorf("Expected test1 to be PASSED, got %s", runInfo.Tests["test1"].Status)
	}
	if runInfo.Tests["test2"].Status != "FAILED" {
		t.Errorf("Expected test2 to be FAILED, got %s", runInfo.Tests["test2"].Status)
	}

	// Should have at least 2 logs (one for each test completion)
	if len(runInfo.Logs) < 2 {
		t.Errorf("Expected at least 2 logs, got %d", len(runInfo.Logs))
	}
}

// DEADLOCK PREVENTION TESTING

func TestNoDeadlockInMixedOperations(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	// Setup multiple runs to increase complexity
	for i := 0; i < 5; i++ {
		runID := fmt.Sprintf("run-%d", i)
		runInfo := &RunInfo{
			ID: runID,
			Tests: map[string]*TestInfo{
				fmt.Sprintf("test-%d-1", i): {Status: "PENDING"},
				fmt.Sprintf("test-%d-2", i): {Status: "PENDING"},
			},
			Logs: []LogLine{},
		}
		engine.runs[runID] = runInfo
	}

	// Mix of operations that could potentially deadlock
	numOperations := 100
	var wg sync.WaitGroup
	done := make(chan bool, 1)

	// Timeout mechanism to detect deadlocks
	go func() {
		time.Sleep(10 * time.Second)
		select {
		case <-done:
			return
		default:
			t.Error("Test timed out - possible deadlock detected")
		}
	}()

	for i := 0; i < numOperations; i++ {
		wg.Add(1)
		go func(opID int) {
			defer wg.Done()
			runID := fmt.Sprintf("run-%d", opID%5)

			switch opID % 4 {
			case 0:
				// Read operation
				_, _ = engine.getTestStatusCounts(runID)
			case 1:
				// Write operation
				engine.addLog(runID, fmt.Sprintf("Operation %d", opID), "green", false)
			case 2:
				// Status update
				testID := fmt.Sprintf("test-%d-1", opID%5)
				engine.updateTestStatus(runID, testID, nil)
			case 3:
				// Check if finished (mixed read operations)
				engine.checkIfRunFinished(runID)
			}
		}(i)
	}

	wg.Wait()
	done <- true
}

func TestStreamLogsNonBlocking(t *testing.T) {
	// This test ensures that StreamLogs doesn't hold locks for too long
	mockClient := &MockTemporalClient{}
	engine := newTestEngineWithClient(mockClient)

	runInfo := &RunInfo{
		ID:     "test-run-id",
		Status: "RUNNING",
		Logs:   []LogLine{{Msg: "Initial log", Color: "green", Bold: false}},
	}
	engine.runs["test-run-id"] = runInfo

	// Test that we can still perform operations while streaming is theoretically happening
	// (We can't easily test the actual streaming without complex gRPC setup, but we can
	// test that the log reading operations don't block other operations)

	var wg sync.WaitGroup

	// Simulate what StreamLogs does (reading logs)
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate the log reading operation from StreamLogs
			engine.mu.RLock()
			if runInfo, exists := engine.runs["test-run-id"]; exists {
				_ = len(runInfo.Logs) // Read operation
			}
			engine.mu.RUnlock()
		}()
	}

	// Concurrent write operations
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			engine.addLog("test-run-id", fmt.Sprintf("Concurrent log %d", i), "blue", false)
		}(i)
	}

	wg.Wait()

	// Verify we have the expected number of logs
	if len(runInfo.Logs) != 11 { // 1 initial + 10 concurrent
		t.Errorf("Expected 11 logs, got %d", len(runInfo.Logs))
	}
}

// ERROR HANDLING AND EDGE CASE TESTS

func TestCheckIfRunFinished(t *testing.T) {
	t.Run("all tests passed", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Name:   "Test Run",
			Status: "RUNNING",
			Tests: map[string]*TestInfo{
				"test1": {Status: "PASSED"},
				"test2": {Status: "PASSED"},
			},
			Logs: []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.checkIfRunFinished("test-run-id")

		if runInfo.Status != "PASSED" {
			t.Errorf("Expected run status to be PASSED, got %s", runInfo.Status)
		}
		if runInfo.EndedAt.IsZero() {
			t.Error("Expected EndedAt to be set")
		}
		if len(runInfo.Logs) == 0 {
			t.Error("Expected completion log to be added")
		}
	})

	t.Run("some tests failed", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Name:   "Test Run",
			Status: "RUNNING",
			Tests: map[string]*TestInfo{
				"test1": {Status: "PASSED"},
				"test2": {Status: "FAILED"},
			},
			Logs: []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.checkIfRunFinished("test-run-id")

		if runInfo.Status != "FAILED" {
			t.Errorf("Expected run status to be FAILED, got %s", runInfo.Status)
		}
	})

	t.Run("tests with timeout", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Name:   "Test Run",
			Status: "RUNNING",
			Tests: map[string]*TestInfo{
				"test1": {Status: "PASSED"},
				"test2": {Status: "TIMEOUT"},
			},
			Logs: []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.checkIfRunFinished("test-run-id")

		if runInfo.Status != "FAILED" {
			t.Errorf("Expected run status to be FAILED due to timeout, got %s", runInfo.Status)
		}
	})

	t.Run("pending tests - should not finish", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:     "test-run-id",
			Name:   "Test Run",
			Status: "RUNNING",
			Tests: map[string]*TestInfo{
				"test1": {Status: "PASSED"},
				"test2": {Status: "PENDING"},
			},
			Logs: []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.checkIfRunFinished("test-run-id")

		if runInfo.Status != "RUNNING" {
			t.Errorf("Expected run status to remain RUNNING, got %s", runInfo.Status)
		}
		if !runInfo.EndedAt.IsZero() {
			t.Error("Expected EndedAt to remain unset")
		}
	})

	t.Run("nonexistent run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		// Should not panic when run doesn't exist
		engine.checkIfRunFinished("nonexistent-run-id")
		// Test passes if no panic occurs
	})
}

func TestUpdateTestStatusEdgeCases(t *testing.T) {
	t.Run("nonexistent run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		// Should not panic when run doesn't exist
		engine.updateTestStatus("nonexistent-run-id", "test-id", nil)
		// Test passes if no panic occurs
	})

	t.Run("nonexistent test", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:    "test-run-id",
			Tests: make(map[string]*TestInfo),
			Logs:  []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		// Should not panic when test doesn't exist
		engine.updateTestStatus("test-run-id", "nonexistent-test-id", nil)
		// Test passes if no panic occurs
	})

	t.Run("timeout error handling", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID: "test-run-id",
			Tests: map[string]*TestInfo{
				"test-id": {Name: "Test", Status: "PENDING"},
			},
			Logs: []LogLine{},
		}
		engine.runs["test-run-id"] = runInfo

		engine.updateTestStatus("test-run-id", "test-id", fmt.Errorf("workflow monitoring timeout"))

		if runInfo.Tests["test-id"].Status != "TIMEOUT" {
			t.Errorf("Expected test status to be TIMEOUT, got %s", runInfo.Tests["test-id"].Status)
		}
	})
}

func TestRunInfoSuiteGlobalsStorage(t *testing.T) {
	t.Run("suite globals initialized empty", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:           "test-run",
			SuiteGlobals: make(map[string]string),
		}
		engine.runs["test-run"] = runInfo

		if runInfo.SuiteGlobals == nil {
			t.Error("Expected SuiteGlobals to be initialized")
		}
		if len(runInfo.SuiteGlobals) != 0 {
			t.Errorf("Expected empty SuiteGlobals, got %d items", len(runInfo.SuiteGlobals))
		}
	})

	t.Run("suite globals can be set and retrieved", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:           "test-run",
			SuiteGlobals: map[string]string{"api_token": "abc123", "database_id": "db456"},
		}
		engine.runs["test-run"] = runInfo

		engine.mu.RLock()
		defer engine.mu.RUnlock()

		if runInfo.SuiteGlobals["api_token"] != "abc123" {
			t.Errorf("Expected api_token 'abc123', got '%s'", runInfo.SuiteGlobals["api_token"])
		}
		if runInfo.SuiteGlobals["database_id"] != "db456" {
			t.Errorf("Expected database_id 'db456', got '%s'", runInfo.SuiteGlobals["database_id"])
		}
	})

	t.Run("suite globals persist across test executions", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:           "test-run",
			SuiteGlobals: map[string]string{"shared_token": "xyz789"},
			Tests: map[string]*TestInfo{
				"test1": {Status: "PENDING"},
				"test2": {Status: "PENDING"},
			},
		}
		engine.runs["test-run"] = runInfo

		// Simulate test1 completion
		engine.updateTestStatus("test-run", "test1", nil)

		// Verify suite globals still exist
		engine.mu.RLock()
		if runInfo.SuiteGlobals["shared_token"] != "xyz789" {
			t.Errorf("Expected suite globals to persist, got '%s'", runInfo.SuiteGlobals["shared_token"])
		}
		engine.mu.RUnlock()

		// Simulate test2 completion
		engine.updateTestStatus("test-run", "test2", nil)

		// Verify suite globals still exist
		engine.mu.RLock()
		if runInfo.SuiteGlobals["shared_token"] != "xyz789" {
			t.Errorf("Expected suite globals to persist after all tests, got '%s'", runInfo.SuiteGlobals["shared_token"])
		}
		engine.mu.RUnlock()
	})
}

func TestSuiteInitFlagsAndCleanup(t *testing.T) {
	t.Run("suite init completed flag", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:                 "test-run",
			SuiteInitCompleted: false,
		}
		engine.runs["test-run"] = runInfo

		if runInfo.SuiteInitCompleted {
			t.Error("Expected SuiteInitCompleted to be false initially")
		}

		engine.mu.Lock()
		runInfo.SuiteInitCompleted = true
		engine.mu.Unlock()

		if !runInfo.SuiteInitCompleted {
			t.Error("Expected SuiteInitCompleted to be true after setting")
		}
	})

	t.Run("suite init failed flag", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:              "test-run",
			SuiteInitFailed: false,
		}
		engine.runs["test-run"] = runInfo

		if runInfo.SuiteInitFailed {
			t.Error("Expected SuiteInitFailed to be false initially")
		}

		engine.mu.Lock()
		runInfo.SuiteInitFailed = true
		runInfo.Status = "FAILED"
		engine.mu.Unlock()

		if !runInfo.SuiteInitFailed {
			t.Error("Expected SuiteInitFailed to be true after failure")
		}
		if runInfo.Status != "FAILED" {
			t.Errorf("Expected run status to be FAILED, got %s", runInfo.Status)
		}
	})

	t.Run("suite cleanup ran flag", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := newTestEngineWithClient(mockClient)

		runInfo := &RunInfo{
			ID:              "test-run",
			SuiteCleanupRan: false,
		}
		engine.runs["test-run"] = runInfo

		if runInfo.SuiteCleanupRan {
			t.Error("Expected SuiteCleanupRan to be false initially")
		}

		engine.mu.Lock()
		runInfo.SuiteCleanupRan = true
		engine.mu.Unlock()

		if !runInfo.SuiteCleanupRan {
			t.Error("Expected SuiteCleanupRan to be true after cleanup")
		}
	})
}

func TestHelperFunctions(t *testing.T) {
	t.Run("cloneStringMap preserves values", func(t *testing.T) {
		original := map[string]string{
			"key1": "value1",
			"key2": "value2",
		}

		cloned := cloneStringMap(original)

		if len(cloned) != len(original) {
			t.Errorf("Expected cloned map to have %d items, got %d", len(original), len(cloned))
		}

		for k, v := range original {
			if cloned[k] != v {
				t.Errorf("Expected cloned[%s] = %s, got %s", k, v, cloned[k])
			}
		}

		// Modify clone, ensure original is unchanged
		cloned["key3"] = "value3"
		if _, exists := original["key3"]; exists {
			t.Error("Expected original map to be unaffected by clone modification")
		}
	})

	t.Run("cloneStringMap handles nil", func(t *testing.T) {
		var nilMap map[string]string
		cloned := cloneStringMap(nilMap)

		if cloned == nil {
			t.Error("Expected cloned map to be initialized, not nil")
		}
		if len(cloned) != 0 {
			t.Errorf("Expected empty map, got %d items", len(cloned))
		}
	})

	t.Run("extractSavedValues extracts all values", func(t *testing.T) {
		state := map[string]string{
			"user_id":    "123",
			"auth_token": "abc",
			"session_id": "xyz",
		}

		extracted := extractSavedValues(state)

		if len(extracted) != len(state) {
			t.Errorf("Expected %d extracted values, got %d", len(state), len(extracted))
		}

		for k, v := range state {
			if extracted[k] != v {
				t.Errorf("Expected extracted[%s] = %s, got %s", k, v, extracted[k])
			}
		}
	})
}

func TestRunAPIsAreOrgScoped(t *testing.T) {
	mockClient := &MockTemporalClient{}
	store := NewMemoryRunStore()
	engine := NewEngine(mockClient, store, true)
	engine.authConfig.mode = authModeOIDC

	orgA := uuid.New()
	orgB := uuid.New()

	now := time.Now().UTC()
	_, err := engine.runStore.InsertRun(context.Background(), persistence.RunRecord{
		ID:             "run-a",
		OrganizationID: orgA,
		Status:         "PASSED",
		SuiteName:      "Suite A",
		Initiator:      "owner@orga",
		Trigger:        "manual",
		ScheduleName:   "",
		ConfigSource:   "repo_commit",
		Source:         "cli-local",
		Branch:         "main",
		Environment:    "",
		CommitSHA:      sql.NullString{},
		BundleSHA:      sql.NullString{},
		TotalTests:     1,
		PassedTests:    1,
		FailedTests:    0,
		TimeoutTests:   0,
		StartedAt:      sql.NullTime{Time: now, Valid: true},
		EndedAt:        sql.NullTime{Time: now.Add(time.Second), Valid: true},
	})
	if err != nil {
		t.Fatalf("failed to insert run: %v", err)
	}

	ctxOrgA := contextWithPrincipal(context.Background(), &Principal{
		Subject: "user-a",
		Email:   "owner@orga",
		OrgID:   orgA.String(),
		Roles:   []string{"owner"},
	})

	resp, err := engine.ListRuns(ctxOrgA, &generated.ListRunsRequest{})
	if err != nil {
		t.Fatalf("ListRuns returned error: %v", err)
	}
	if len(resp.Runs) != 1 {
		t.Fatalf("expected 1 run for org A, got %d", len(resp.Runs))
	}
	if resp.Runs[0].RunId != "run-a" {
		t.Fatalf("expected run ID 'run-a', got %s", resp.Runs[0].RunId)
	}

	ctxOrgB := contextWithPrincipal(context.Background(), &Principal{
		Subject: "user-b",
		Email:   "owner@orgb",
		OrgID:   orgB.String(),
		Roles:   []string{"owner"},
	})

	respB, err := engine.ListRuns(ctxOrgB, &generated.ListRunsRequest{})
	if err != nil {
		t.Fatalf("ListRuns for org B returned error: %v", err)
	}
	if len(respB.Runs) != 0 {
		t.Fatalf("expected 0 runs for org B, got %d", len(respB.Runs))
	}

	if _, err := engine.GetRun(ctxOrgB, &generated.GetRunRequest{RunId: "run-a"}); err == nil {
		t.Fatal("expected GetRun for org B to fail, but it succeeded")
	}

	if _, err := engine.GetRun(ctxOrgA, &generated.GetRunRequest{RunId: "run-a"}); err != nil {
		t.Fatalf("GetRun for org A returned error: %v", err)
	}
}
