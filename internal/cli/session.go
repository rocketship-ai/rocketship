package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Session struct {
	EngineAddress string    `json:"engine_address"`
	SessionID     string    `json:"session_id"`
	CreatedAt     time.Time `json:"created_at"`
}

func getSessionPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(home, ".rocketship", "session.json"), nil
}

func SaveSession(session *Session) error {
	sessionPath, err := getSessionPath()
	if err != nil {
		return err
	}

	// Create .rocketship directory if it doesn't exist
	dir := filepath.Dir(sessionPath)

	// First try to create the directory with user write permissions
	if err := os.MkdirAll(dir, 0700); err != nil {
		// If directory exists but we can't write to it, try to fix permissions
		if os.IsPermission(err) {
			if chmodErr := os.Chmod(dir, 0700); chmodErr != nil {
				return fmt.Errorf("failed to set directory permissions: %w", chmodErr)
			}
		} else {
			return fmt.Errorf("failed to create directory: %w", err)
		}
	}

	data, err := json.MarshalIndent(session, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Try to write the file with user-only read/write permissions
	if err := os.WriteFile(sessionPath, data, 0600); err != nil {
		// If file exists but we can't write to it, try to fix permissions
		if os.IsPermission(err) {
			if chmodErr := os.Chmod(sessionPath, 0600); chmodErr != nil {
				return fmt.Errorf("failed to set file permissions: %w", chmodErr)
			}
			// Try writing again after fixing permissions
			if writeErr := os.WriteFile(sessionPath, data, 0600); writeErr != nil {
				return fmt.Errorf("failed to write session file after fixing permissions: %w", writeErr)
			}
		} else {
			return fmt.Errorf("failed to write session file: %w", err)
		}
	}

	return nil
}

func LoadSession() (*Session, error) {
	sessionPath, err := getSessionPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(sessionPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("no active session found")
		}
		return nil, fmt.Errorf("failed to read session file: %w", err)
	}

	var session Session
	if err := json.Unmarshal(data, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &session, nil
}
