package cli

import (
	"os/exec"
	"sync"
	"syscall"
	"time"
)

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
