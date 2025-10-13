//go:build unix

package browser

import (
	"os/exec"
	"syscall"
	"time"
)

// setupProcessGroup configures the command to run in its own process group (Unix only)
func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

// killProcessGroup attempts to kill the entire process group (Unix only)
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	// Kill the entire process group to ensure child processes (browsers) are also killed
	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		// Fallback to killing just the process
		_ = cmd.Process.Kill()
		return
	}

	// Send SIGTERM to the process group first (graceful)
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	// Wait a bit, then send SIGKILL if needed
	go func() {
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}()
}
