package cli

import (
	"encoding/json"
	"fmt"
	"log"
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

	// First pass: send SIGTERM to all processes
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			log.Printf("Sending SIGTERM to process %d (%s)", proc.Process.Pid, proc.Path)
			if err := proc.Process.Signal(syscall.SIGTERM); err != nil {
				log.Printf("Failed to send SIGTERM to process %d: %v", proc.Process.Pid, err)
			}
		}
	}

	// Give processes time to shut down gracefully
	time.Sleep(2 * time.Second)

	// Second pass: check if processes are still running and force kill if necessary
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			// Check if process is still running
			if err := proc.Process.Signal(syscall.Signal(0)); err == nil {
				log.Printf("Process %d still running, sending SIGKILL", proc.Process.Pid)
				if err := proc.Process.Kill(); err != nil {
					log.Printf("Failed to kill process %d: %v", proc.Process.Pid, err)
				}
			} else {
				log.Printf("Process %d has exited", proc.Process.Pid)
			}
		}
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
		log.Printf("Looking for process %d (%s)", ps.PID, ps.Path)
		proc, err := os.FindProcess(ps.PID)
		if err != nil {
			log.Printf("Failed to find process %d: %v", ps.PID, err)
			continue
		}

		// Check if process is actually running
		if err := proc.Signal(syscall.Signal(0)); err != nil {
			log.Printf("Process %d is not running: %v", ps.PID, err)
			continue
		}

		cmd := exec.Command(ps.Path)
		cmd.Process = proc
		pm.Add(cmd)
		log.Printf("Found running process %d", ps.PID)
	}

	return pm, nil
}
