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
	PID      int    `json:"pid"`
	PGID     int    `json:"pgid"`
	Path     string `json:"path"`
	IsLeader bool   `json:"is_leader"`
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

	// Set process group ID to be the same as the process ID
	// This makes the process a group leader and helps us manage child processes
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	pm.processes = append(pm.processes, cmd)
}

func (pm *processManager) Cleanup() {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// First pass: send SIGTERM to all process groups
	for _, proc := range pm.processes {
		if proc != nil && proc.Process != nil {
			pgid, err := syscall.Getpgid(proc.Process.Pid)
			if err != nil {
				log.Printf("Failed to get pgid for process %d: %v", proc.Process.Pid, err)
				continue
			}

			log.Printf("Sending SIGTERM to process group %d (leader: %d)", pgid, proc.Process.Pid)
			if err := syscall.Kill(-pgid, syscall.SIGTERM); err != nil {
				log.Printf("Failed to send SIGTERM to process group %d: %v", pgid, err)
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
				log.Printf("Process group %d still running, sending SIGKILL", pgid)
				if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
					log.Printf("Failed to kill process group %d: %v", pgid, err)
				}
			} else {
				log.Printf("Process group %d has exited", pgid)
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
			pgid, err := syscall.Getpgid(proc.Process.Pid)
			if err != nil {
				log.Printf("Warning: Failed to get pgid for process %d: %v", proc.Process.Pid, err)
				continue
			}

			state = append(state, processState{
				PID:      proc.Process.Pid,
				PGID:     pgid,
				Path:     proc.Path,
				IsLeader: true,
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
		log.Printf("Looking for process group %d (leader: %d)", ps.PGID, ps.PID)

		// Try to signal the process group to check if it exists
		if err := syscall.Kill(-ps.PGID, syscall.Signal(0)); err != nil {
			log.Printf("Process group %d is not running: %v", ps.PGID, err)
			continue
		}

		// Create a dummy command just to track the process group
		cmd := exec.Command(ps.Path)
		cmd.Process, _ = os.FindProcess(ps.PID)
		cmd.SysProcAttr = &syscall.SysProcAttr{
			Pgid: ps.PGID,
		}

		pm.Add(cmd)
		log.Printf("Found running process group %d", ps.PGID)
	}

	return pm, nil
}
