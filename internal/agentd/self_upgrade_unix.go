//go:build linux || darwin

package agentd

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// swapBinary atomically replaces dst with src on Linux/macOS.
//
// POSIX rename within the same filesystem is atomic — readers (the
// running process) keep their open inode via the old path's mtime,
// new processes pick up the freshly-named file. Different filesystems
// (e.g. `<state_dir>/upgrade/` on /var vs `/usr/local/bin/`) fail with
// EXDEV. WSL, containers, and any multi-partition setup ship the
// upgrade dir on a different filesystem from the install dir; in those
// environments the bare rename always fails (issue #12).
//
// Strategy:
//
//  1. Try os.Rename(src, dst). Fast path — same-fs case, atomic.
//  2. On EXDEV: stage a copy at `<dst>.upgrade-tmp`, rename it into
//     place (now same-fs by construction), then best-effort remove
//     the original src so the staging dir doesn't accumulate
//     leftovers.
//
// The temp filename is fixed (`<dst>.upgrade-tmp`) so a previous
// failed upgrade's leftover gets clobbered rather than building up
// over time. Same atomicity guarantee as the fast path because the
// final rename is still within one filesystem.
func swapBinary(src, dst string) error {
	if err := renameFunc(src, dst); err == nil {
		return nil
	} else if !errors.Is(err, syscall.EXDEV) {
		return fmt.Errorf("rename %s → %s: %w", src, dst, err)
	}

	tmp := dst + ".upgrade-tmp"
	if err := copyFile(src, tmp); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cross-fs swap: copy %s → %s: %w", src, tmp, err)
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("cross-fs swap: rename %s → %s: %w", tmp, dst, err)
	}
	// GC the staged source so /var/lib/.../upgrade/ doesn't fill up over
	// time. Best-effort: the swap already succeeded, leaving a stale
	// staged file is a janitorial concern, not a correctness one.
	_ = os.Remove(src)
	return nil
}

// renameFunc is a test seam for the initial cross-fs detection. The
// EXDEV-fallback copy uses os.Rename directly (the temp + dst live in
// the same directory, so EXDEV can't recur there). Tests inject a
// stub that returns syscall.EXDEV to exercise the fallback path
// without needing an actual cross-fs mount.
var renameFunc = os.Rename

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
