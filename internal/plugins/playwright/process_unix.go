//go:build unix

package playwright

import (
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
	if err := syscall.Kill(-pid, syscall.SIGTERM); err != nil {
		if err == syscall.ESRCH {
			return nil
		}
		return err
	}

	go func() {
		time.Sleep(2 * time.Second)
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	}()

	return nil
}
