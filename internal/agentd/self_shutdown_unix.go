//go:build !windows

package agentd

import (
	"fmt"
	"os/exec"
)

// realShutdownExec invokes `shutdown -h now` on Linux/macOS/BSD. The
// daemon typically runs as root under systemd/launchd; if it doesn't,
// the exec will fail with EPERM and the self_shutdown handler re-arms
// so the operator can investigate. The command does not block on
// shutdown completion — the kernel asks init to broadcast and then
// signals processes.
func realShutdownExec() error {
	out, err := exec.Command("shutdown", "-h", "now").CombinedOutput()
	if err != nil {
		return fmt.Errorf("shutdown -h now: %w (output: %s)", err, out)
	}
	return nil
}
