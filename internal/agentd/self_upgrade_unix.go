//go:build linux || darwin

package agentd

import (
	"fmt"
	"os"
	"syscall"
)

// swapBinary atomically replaces dst with src on Linux/macOS. POSIX
// rename within the same filesystem is atomic — readers (the running
// process) keep their open inode via the old path's mtime, new
// processes pick up the freshly-named file. Different filesystems
// (e.g. /tmp vs /usr/local/bin) fail with EXDEV; the upgrade dir lives
// under <state_dir> for exactly this reason.
func swapBinary(src, dst string) error {
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}
	return nil
}

// reExec replaces this process with a fresh execution of binPath,
// keeping the same os.Args and environ. file-descriptor inheritance
// across exec means the listening sockets are passed along; net/http's
// Serve loop in the old process is interrupted by the syscall.Exec
// boundary, and the new process re-listens on a fresh socket bound to
// the same address. SO_REUSEADDR (default for net.Listen) makes that
// rebind succeed without a TIME_WAIT delay.
//
// On success this function does NOT return — the process is replaced.
// On failure it returns the syscall error.
func reExec(binPath string) error {
	return syscall.Exec(binPath, os.Args, os.Environ())
}
