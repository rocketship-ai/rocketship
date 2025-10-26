//go:build unix

package playwright

import (
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

func setupProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}

	pgid, err := syscall.Getpgid(cmd.Process.Pid)
	if err != nil {
		_ = cmd.Process.Kill()
		return
	}

	_ = syscall.Kill(-pgid, syscall.SIGTERM)
	go func() {
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
	}()
}

func terminateProcessTree(pid int) error {
	// Send SIGTERM to process group
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return nil // Process already dead
		}
		return err
	}

	// Wait up to 3 seconds for graceful shutdown
	for i := 0; i < 30; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(-pid, 0); err == syscall.ESRCH {
			return nil // Process is dead
		}
	}

	// Force kill if still alive
	_ = syscall.Kill(-pid, syscall.SIGKILL)

	// Wait up to 2 more seconds for force kill
	for i := 0; i < 20; i++ {
		time.Sleep(100 * time.Millisecond)
		if err := syscall.Kill(-pid, 0); err == syscall.ESRCH {
			return nil // Process is dead
		}
	}

	return fmt.Errorf("process %d did not terminate after SIGKILL", pid)
}
