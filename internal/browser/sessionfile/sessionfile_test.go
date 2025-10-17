package sessionfile

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestPath(t *testing.T) {
	runDir := "/tmp/rocketship"
	sessionID := "abc123"
	expected := filepath.Join(runDir, "tmp", "browser_sessions", sessionID+".json")
	if got := Path(runDir, sessionID); got != expected {
		t.Fatalf("Path() = %s, want %s", got, expected)
	}
}

func TestWriteReadRemove(t *testing.T) {
	runDir := t.TempDir()
	t.Setenv("ROCKETSHIP_RUN_DIR", runDir)
	ctx := context.Background()

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	baseDir, err := BaseDir()
	if err != nil {
		t.Fatalf("BaseDir() error = %v", err)
	}

	if baseDir != runDir {
		t.Fatalf("BaseDir() = %s, want %s", baseDir, runDir)
	}

	const (
		sessionID  = "test-session"
		wsEndpoint = "ws://localhost:1234/devtools/browser/123"
		pid        = 4242
	)

	if err := Write(ctx, sessionID, wsEndpoint, pid); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	gotEndpoint, gotPID, err := Read(ctx, sessionID)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if gotEndpoint != wsEndpoint {
		t.Fatalf("Read() wsEndpoint = %s, want %s", gotEndpoint, wsEndpoint)
	}
	if gotPID != pid {
		t.Fatalf("Read() pid = %d, want %d", gotPID, pid)
	}

	if err := Remove(ctx, sessionID); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}

	if _, _, err := Read(ctx, sessionID); err == nil {
		t.Fatalf("Read() expected error after removal, got nil")
	}
}

func TestReadValidation(t *testing.T) {
	t.Setenv("ROCKETSHIP_RUN_DIR", t.TempDir())
	ctx := context.Background()

	if err := EnsureDir(); err != nil {
		t.Fatalf("EnsureDir() error = %v", err)
	}

	runDir, err := resolveRunDir()
	if err != nil {
		t.Fatalf("resolveRunDir() error = %v", err)
	}

	sessionID := "bad-session"
	path := Path(runDir, sessionID)

	// Missing wsEndpoint and pid
	if err := os.WriteFile(path, []byte(`{"createdAt":"2024-01-01T00:00:00Z"}`), 0o600); err != nil {
		t.Fatalf("failed to write invalid session file: %v", err)
	}

	if _, _, err := Read(ctx, sessionID); err == nil {
		t.Fatalf("Read() expected validation error, got nil")
	}
}
