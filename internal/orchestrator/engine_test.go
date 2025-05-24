package orchestrator

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
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
	engine := NewEngine(mockClient)

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

		if len(id1) != 16 {
			t.Errorf("Expected ID length to be 16, got %d", len(id1))
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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

		// Should not panic when run doesn't exist
		engine.addLog("nonexistent-run-id", "Test log message", "green", true)
		// Test passes if no panic occurs
	})
}

func TestGetTestStatusCounts(t *testing.T) {
	t.Run("empty run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

		_, err := engine.getTestStatusCounts("nonexistent-run-id")
		if err == nil {
			t.Error("Expected error for nonexistent run")
		}
	})
}

func TestCreateRunValidation(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := NewEngine(mockClient)
	ctx := context.Background()

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
version: "v1.0.0"
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
	engine := NewEngine(mockClient)
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
	engine := NewEngine(mockClient)

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
	engine := NewEngine(mockClient)

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
	engine := NewEngine(mockClient)

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
	engine := NewEngine(mockClient)

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
	engine := NewEngine(mockClient)

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
	engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

		// Should not panic when run doesn't exist
		engine.checkIfRunFinished("nonexistent-run-id")
		// Test passes if no panic occurs
	})
}

func TestUpdateTestStatusEdgeCases(t *testing.T) {
	t.Run("nonexistent run", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := NewEngine(mockClient)

		// Should not panic when run doesn't exist
		engine.updateTestStatus("nonexistent-run-id", "test-id", nil)
		// Test passes if no panic occurs
	})

	t.Run("nonexistent test", func(t *testing.T) {
		mockClient := &MockTemporalClient{}
		engine := NewEngine(mockClient)

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
		engine := NewEngine(mockClient)

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
