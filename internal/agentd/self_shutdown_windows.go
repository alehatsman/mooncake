//go:build windows

package agentd

import (
	"fmt"
	"os/exec"
)

// realShutdownExec invokes Windows `shutdown /s /t 0`. /s = shutdown
// (not reboot), /t 0 = immediate (no countdown dialog). Requires the
// daemon's service principal to hold SeShutdownPrivilege, which the
// LocalSystem account does by default. The exec returns immediately;
// Windows handles the actual power-off asynchronously.
func realShutdownExec() error {
	out, err := exec.Command("shutdown", "/s", "/t", "0").CombinedOutput()
	if err != nil {
		return fmt.Errorf("shutdown /s /t 0: %w (output: %s)", err, out)
	}
	return nil
}
