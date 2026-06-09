//go:build !windows

package process

import (
	"os"
	"os/exec"
	"syscall"
)

func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

func isProcessAlive(proc *os.Process) bool {
	err := proc.Signal(syscall.Signal(0))
	return err == nil
}

func killProcess(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
