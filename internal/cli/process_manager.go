package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// processState represents the serializable state of a process
type processState struct {
	PID  int    `json:"pid"`
	Path string `json:"path"`
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

func (pm *processManager) Add(cmd *exec.Cmd) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	pm.processes = append(pm.processes, cmd)
}

func (pm *processManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			// Send SIGTERM first for graceful shutdown
			_ = proc.Process.Signal(syscall.SIGTERM)

			// Give process time to shutdown gracefully
			time.Sleep(2 * time.Second)

			// Force kill if still running
			_ = proc.Process.Kill()
		}
	}
}

// SaveToFile saves the process manager state to a file
func (pm *processManager) SaveToFile(path string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	var state []processState
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			state = append(state, processState{
				PID:  proc.Process.Pid,
				Path: proc.Path,
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

	return nil
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
		proc, err := os.FindProcess(ps.PID)
		if err == nil {
			cmd := exec.Command(ps.Path)
			cmd.Process = proc
			pm.Add(cmd)
		}
	}

	return pm, nil
}
