package sessionfile

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	defaultRunDirName = ".rocketship"
	sessionDir        = "tmp/browser_sessions"
	sessionExtension  = ".json"
)

type sessionPayload struct {
	WSEndpoint string `json:"wsEndpoint"`
	PID        int    `json:"pid"`
	CreatedAt  string `json:"createdAt"`
}

func Path(runDir, sessionID string) string {
	return filepath.Join(runDir, sessionDir, sessionID+sessionExtension)
}

func Read(ctx context.Context, sessionID string) (string, int, error) {
	if err := ctx.Err(); err != nil {
		return "", 0, err
	}

	runDir, err := resolveRunDir()
	if err != nil {
		return "", 0, err
	}

	path := Path(runDir, sessionID)
	data, err := os.ReadFile(path)
	if err != nil {
		return "", 0, fmt.Errorf("failed to read session file %s: %w", path, err)
	}

	var payload sessionPayload
	if err := json.Unmarshal(data, &payload); err != nil {
		return "", 0, fmt.Errorf("failed to parse session file %s: %w", path, err)
	}

	if payload.WSEndpoint == "" {
		return "", 0, fmt.Errorf("session file %s missing wsEndpoint", path)
	}
	if payload.PID <= 0 {
		return "", 0, fmt.Errorf("session file %s has invalid pid %d", path, payload.PID)
	}
	if payload.CreatedAt == "" {
		return "", 0, fmt.Errorf("session file %s missing createdAt", path)
	}
	if _, err := time.Parse(time.RFC3339, payload.CreatedAt); err != nil {
		return "", 0, fmt.Errorf("session file %s has invalid createdAt: %w", path, err)
	}

	return payload.WSEndpoint, payload.PID, nil
}

func Write(ctx context.Context, sessionID, wsEndpoint string, pid int) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sessionID == "" {
		return errors.New("sessionID is required")
	}
	if wsEndpoint == "" {
		return errors.New("wsEndpoint is required")
	}
	if pid <= 0 {
		return fmt.Errorf("pid must be > 0, got %d", pid)
	}

	runDir, err := resolveRunDir()
	if err != nil {
		return err
	}

	if err := EnsureDir(); err != nil {
		return err
	}

	payload := sessionPayload{
		WSEndpoint: wsEndpoint,
		PID:        pid,
		CreatedAt:  time.Now().UTC().Format(time.RFC3339),
	}

	data, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session data: %w", err)
	}

	path := Path(runDir, sessionID)
	tempPath := path + ".tmp"

	if err := os.WriteFile(tempPath, data, 0o600); err != nil {
		return fmt.Errorf("failed to write session file %s: %w", tempPath, err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("failed to finalize session file %s: %w", path, err)
	}

	return nil
}

func Remove(ctx context.Context, sessionID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if sessionID == "" {
		return errors.New("sessionID is required")
	}

	runDir, err := resolveRunDir()
	if err != nil {
		return err
	}

	path := Path(runDir, sessionID)
	if err := os.Remove(path); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("session file %s not found", path)
		}
		return fmt.Errorf("failed to remove session file %s: %w", path, err)
	}

	return nil
}

func EnsureDir() error {
	runDir, err := resolveRunDir()
	if err != nil {
		return err
	}

	target := filepath.Join(runDir, sessionDir)
	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("failed to create session directory %s: %w", target, err)
	}

	return nil
}

func BaseDir() (string, error) {
	return resolveRunDir()
}

func resolveRunDir() (string, error) {
	if dir := os.Getenv("ROCKETSHIP_RUN_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", fmt.Errorf("failed to resolve ROCKETSHIP_RUN_DIR %s: %w", dir, err)
		}
		return abs, nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("failed to get current working directory: %w", err)
	}

	return filepath.Join(cwd, defaultRunDirName), nil
}
