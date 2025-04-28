package orchestrator

import (
	"context"
	"testing"
	"time"

	"github.com/rocketship-ai/rocketship/internal/api/generated"
	"go.temporal.io/sdk/client"
)

type MockTemporalClient struct {
	client.Client
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
}

func TestListRuns(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := NewEngine(mockClient)

	runInfo := &RunInfo{
		ID:        "test-run-id",
		Status:    "PASSED",
		StartedAt: time.Now(),
		EndedAt:   time.Now().Add(5 * time.Second),
	}
	engine.runs["test-run-id"] = runInfo

	ctx := context.Background()
	resp, err := engine.ListRuns(ctx, &generated.ListRunsRequest{})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if len(resp.Runs) != 1 {
		t.Fatalf("Expected 1 run, got %d", len(resp.Runs))
	}

	if resp.Runs[0].RunId != "test-run-id" {
		t.Errorf("Expected run ID 'test-run-id', got '%s'", resp.Runs[0].RunId)
	}

	if resp.Runs[0].Status != "PASSED" {
		t.Errorf("Expected status 'PASSED', got '%s'", resp.Runs[0].Status)
	}
}

func TestGenerateID(t *testing.T) {
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
}

func TestAddLog(t *testing.T) {
	mockClient := &MockTemporalClient{}
	engine := NewEngine(mockClient)

	runInfo := &RunInfo{
		ID:     "test-run-id",
		Status: "RUNNING",
		Logs:   []string{},
	}
	engine.runs["test-run-id"] = runInfo

	engine.addLog("test-run-id", "Test log message")

	if len(runInfo.Logs) != 1 {
		t.Fatalf("Expected 1 log message, got %d", len(runInfo.Logs))
	}

	if runInfo.Logs[0] != "Test log message" {
		t.Errorf("Expected log message 'Test log message', got '%s'", runInfo.Logs[0])
	}
}
