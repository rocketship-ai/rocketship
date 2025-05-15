package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
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

// processState represents the serializable state of a process
type processState struct {
	PID       int           `json:"pid"`
	PGID      int           `json:"pgid"`
	Path      string        `json:"path"`
	Component ComponentType `json:"component"`
}

// processManager handles the lifecycle of child processes
type processManager struct {
	processes []*exec.Cmd
	mu        sync.Mutex
}

func newProcessManager() *processManager {
	return &processManager{
		processes: make([]*exec.Cmd, 0),
	}
}

// IsComponentRunning checks if a component is already running
func (pm *processManager) IsComponentRunning(component ComponentType) bool {
	var pattern string
	switch component {
	case Temporal:
		pattern = "temporal.*server.*start-dev"
	case Worker:
		pattern = "rocketship.*worker"
	case Engine:
		pattern = "rocketship.*engine"
	default:
		return false
	}

	// Use pgrep with -f to match against the full command line
	cmd := exec.Command("pgrep", "-f", pattern)
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// If we found any processes, the component is running
	return len(strings.TrimSpace(string(output))) > 0
}

func (pm *processManager) Add(cmd *exec.Cmd, component ComponentType) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// Set process group
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	Logger.Debug("started process", "pid", cmd.Process.Pid, "component", component)
	pm.processes = append(pm.processes, cmd)
	return nil
}

func (pm *processManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// First pass: send SIGTERM to all process groups
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			pgid, err := syscall.Getpgid(proc.Process.Pid)
			if err != nil {
				Logger.Debug("failed to get pgid", "pid", proc.Process.Pid, "error", err)
				continue
			}

			Logger.Debug("sending SIGTERM", "pgid", pgid, "pid", proc.Process.Pid)
			if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				Logger.Warn("failed to send SIGTERM", "pgid", pgid, "error", err)
			}
		}
	}

	// Give processes time to shut down gracefully
	time.Sleep(2 * time.Second)

	// Second pass: force kill any remaining processes
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			pgid, err := syscall.Getpgid(proc.Process.Pid)
			if err != nil {
				continue // Process might have already exited
			}

			// Check if process group is still running
			if err := syscall.Kill(-pgid, syscall.Signal(0)); err == nil {
				Logger.Debug("sending SIGKILL", "pgid", pgid)
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
					Logger.Debug("failed to kill process group", "pgid", pgid, "error", err)
				}
			}
		}
	}

	// Cleanup any orphaned processes with more specific patterns
	componentPatterns := map[ComponentType]string{
		Temporal: "temporal.*server.*start-dev",
		Worker:   "rocketship.*worker",
		Engine:   "rocketship.*engine",
	}

	for _, pattern := range componentPatterns {
		// First try SIGTERM
		cmd := exec.Command("pkill", "-f", pattern)
		_ = cmd.Run()

		// Give processes a moment to terminate gracefully
		time.Sleep(500 * time.Millisecond)

		// Then force kill any remaining matches
		cmd = exec.Command("pkill", "-9", "-f", pattern)
		_ = cmd.Run()
	}

	// Clear the processes list
	pm.processes = make([]*exec.Cmd, 0)
}

// SaveToFile saves the process manager state to a file
func (pm *processManager) SaveToFile(path string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var state []processState
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			pgid, err := syscall.Getpgid(proc.Process.Pid)
			if err != nil {
				Logger.Debug("failed to get pgid", "pid", proc.Process.Pid, "error", err)
				continue
			}

			state = append(state, processState{
				PID:       proc.Process.Pid,
				PGID:      pgid,
				Path:      proc.Path,
				Component: getComponentType(proc.Path),
			})
		}
	}

	data, err := json.Marshal(state)
	if err != nil {
		return fmt.Errorf("failed to marshal process state: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write process state: %w", err)
	}

	Logger.Debug("saved process state", "path", path)
	return nil
}

// getComponentType determines the component type from the process path
func getComponentType(path string) ComponentType {
	switch {
	case strings.Contains(path, "temporal"):
		return Temporal
	case strings.Contains(path, "worker"):
		return Worker
	case strings.Contains(path, "engine"):
		return Engine
	default:
		return -1
	}
}

// LoadFromFile loads process manager state from a file
func LoadFromFile(path string) (*processManager, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read process state: %w", err)
	}

	var state []processState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to unmarshal process state: %w", err)
	}

	pm := newProcessManager()
	for _, ps := range state {
		Logger.Debug("checking process group", "pgid", ps.PGID, "pid", ps.PID)

		// Try to signal the process group to check if it exists
		if err := syscall.Kill(-ps.PGID, syscall.Signal(0)); err != nil {
			Logger.Debug("process group not running", "pgid", ps.PGID, "error", err)
			continue
		}

		// Create a dummy command just to track the process group
		cmd := exec.Command(ps.Path)
		cmd.Process, _ = os.FindProcess(ps.PID)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pgid: ps.PGID,
		}

		if err := pm.Add(cmd, ps.Component); err != nil {
			Logger.Debug("failed to add process to manager", "pid", ps.PID, "error", err)
			continue
		}
		Logger.Debug("found running process group", "pgid", ps.PGID)
	}

	return pm, nil
}
