//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func setPlatformSpecificAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
}

func sendPlatformSignal(process *os.Process, sig syscall.Signal) error {
	return syscall.Kill(process.Pid, sig)
}
