//go:build windows

package process

import (
	"os"
	"os/exec"
)

func setSysProcAttr(_ *exec.Cmd) {}

func isProcessAlive(proc *os.Process) bool {
	// On Windows, os.FindProcess always succeeds; send signal 0 equivalent
	// is not available. Use a Kill(0)-style probe: try to open the handle.
	// The simplest portable check: attempt to signal and treat any error
	// (including "process already finished") as not alive.
	err := proc.Signal(os.Interrupt)
	return err == nil
}

func killProcess(proc *os.Process) error {
	return proc.Kill()
}
