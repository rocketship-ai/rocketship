//go:build windows

package browser

import (
	"os/exec"
)

// setupProcessGroup is a no-op on Windows
func setupProcessGroup(cmd *exec.Cmd) {
	// Windows doesn't have Unix process groups, but we can set up process creation flags
	// for now, we'll keep it simple and just do nothing
}

// killProcessGroup attempts to kill the process and its children on Windows
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	
	// On Windows, just kill the main process
	// TODO: Could use taskkill /F /T /PID to kill process tree, but for now keep it simple
	_ = cmd.Process.Kill()
}