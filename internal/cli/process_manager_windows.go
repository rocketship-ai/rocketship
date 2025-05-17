//go:build windows

package cli

import (
	"os"
	"os/exec"
	"syscall"
)

func setPlatformSpecificAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
}

func sendPlatformSignal(process *os.Process, sig syscall.Signal) error {
	return process.Signal(sig)
}
