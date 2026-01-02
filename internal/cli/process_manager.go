package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// ComponentType identifies the type of server component
type ComponentType int

const (
	Temporal ComponentType = iota
	Worker
	Engine
)

func (c ComponentType) String() string {
	switch c {
	case Temporal:
		return "temporal"
	case Worker:
		return "worker"
	case Engine:
		return "engine"
	default:
		return "unknown"
	}
}

// ServerState represents the state of all server components
type ServerState struct {
	Components map[ComponentType]*ProcessInfo `json:"components"`
	UpdatedAt  time.Time                      `json:"updated_at"`
}

// ProcessInfo represents information about a running process
type ProcessInfo struct {
	PID        int       `json:"pid"`
	StartTime  time.Time `json:"start_time"`
	BinaryPath string    `json:"binary_path"`
}

// processManager handles the lifecycle of server components
type processManager struct {
	stateFile string
	mu        sync.Mutex
}

var (
	globalPM     *processManager
	globalPMOnce sync.Once
)

// GetProcessManager returns the singleton process manager instance
func GetProcessManager() *processManager {
	globalPMOnce.Do(func() {
		globalPM = &processManager{
			stateFile: filepath.Join(os.TempDir(), "rocketship-server-state.json"),
		}
	})
	return globalPM
}

// loadState loads the current server state from file
func (pm *processManager) loadState() (*ServerState, error) {
	data, err := os.ReadFile(pm.stateFile)
	if os.IsNotExist(err) {
		return &ServerState{
			Components: make(map[ComponentType]*ProcessInfo),
			UpdatedAt:  time.Now(),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file: %w", err)
	}

	var state ServerState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file: %w", err)
	}

	return &state, nil
}

// saveState saves the current server state to file
func (pm *processManager) saveState(state *ServerState) error {
	state.UpdatedAt = time.Now()
	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	tempFile := pm.stateFile + ".tmp"
	if err := os.WriteFile(tempFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write state file: %w", err)
	}

	return os.Rename(tempFile, pm.stateFile)
}

// Add adds a new process to be managed
func (pm *processManager) Add(cmd *exec.Cmd, component ComponentType) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Load current state
	state, err := pm.loadState()
	if err != nil {
		return fmt.Errorf("failed to load state: %w", err)
	}

	// Check if component is already running
	if info, exists := state.Components[component]; exists {
		if pm.isProcessRunning(info.PID) {
			return fmt.Errorf("component %s is already running (PID: %d)", component, info.PID)
		}
	}

	// Set platform-specific process attributes
	setPlatformSpecificAttrs(cmd)

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	// Update state
	state.Components[component] = &ProcessInfo{
		PID:        cmd.Process.Pid,
		StartTime:  time.Now(),
		BinaryPath: cmd.Path,
	}

	if err := pm.saveState(state); err != nil {
		return fmt.Errorf("failed to save state: %w", err)
	}

	Logger.Debug("started process", "component", component, "pid", cmd.Process.Pid)
	return nil
}

// IsComponentRunning checks if a component is already running
func (pm *processManager) IsComponentRunning(component ComponentType) bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	state, err := pm.loadState()
	if err != nil {
		Logger.Debug("failed to load state", "error", err)
		return false
	}

	info, exists := state.Components[component]
	if !exists {
		return false
	}

	return pm.isProcessRunning(info.PID)
}

// isProcessRunning checks if a process is running
func (pm *processManager) isProcessRunning(pid int) bool {
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}

	err = process.Signal(syscall.Signal(0))
	return err == nil
}

// sendSignal sends the appropriate signal to a process based on the platform
func (pm *processManager) sendSignal(pid int, sig syscall.Signal) error {
	process, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return sendPlatformSignal(process, sig)
}

// Cleanup terminates all managed processes
func (pm *processManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	state, err := pm.loadState()
	if err != nil {
		Logger.Debug("failed to load state during cleanup", "error", err)
		return
	}

	Logger.Debug("cleaning up processes")

	// First attempt graceful shutdown
	for component, info := range state.Components {
		if pm.isProcessRunning(info.PID) {
			Logger.Debug("sending SIGTERM", "component", component, "pid", info.PID)
			if err := pm.sendSignal(info.PID, syscall.SIGTERM); err != nil {
				Logger.Debug("SIGTERM failed", "component", component, "error", err)
			}
		}
	}

	// Give processes time to shut down gracefully (but don't return while they're still running).
	waitForExit := func(maxWait time.Duration) {
		deadline := time.Now().Add(maxWait)
		for {
			allStopped := true
			for _, info := range state.Components {
				if pm.isProcessRunning(info.PID) {
					allStopped = false
					break
				}
			}
			if allStopped || time.Now().After(deadline) {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
	}

	waitForExit(5 * time.Second)

	// Force kill any remaining processes
	for component, info := range state.Components {
		if pm.isProcessRunning(info.PID) {
			Logger.Debug("sending SIGKILL", "component", component, "pid", info.PID)
			if err := pm.sendSignal(info.PID, syscall.SIGKILL); err != nil {
				Logger.Debug("SIGKILL failed", "component", component, "error", err)
			}
		}
	}

	waitForExit(3 * time.Second)

	// Remove the state file
	if err := os.Remove(pm.stateFile); err != nil && !os.IsNotExist(err) {
		Logger.Debug("failed to remove state file", "error", err)
	}

	Logger.Debug("cleanup complete")
}
