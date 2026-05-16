package facts

import (
	"context"
	"os/exec"
	"time"
)

// probeTimeout caps how long a single facts probe can wait for its
// subprocess to exit. 5 seconds is generous for every probe in the
// package (most complete in < 100 ms) except `system_profiler
// SPDisplaysDataType` on macOS with many external displays, where it
// can take 30+ seconds — that one is accepted as a truncation (the
// probe returns ("", false) and the GPU section falls back to what
// was already collected). Without this cap a stuck NFS mount /
// wedged GPU driver / systemd-dbus deadlock hangs the entire
// `mooncake apply` or `mooncake doctor` invocation indefinitely
// (F042).
const probeTimeout = 5 * time.Second

// probeOutput is the facts-package replacement for
// `exec.Command(name, args...).Output()`. It wraps the call in
// `exec.CommandContext` with a per-call timeout so a stuck
// subprocess can't hang `facts.Collect`. Returns the same
// (output, error) pair as Output(), with the error being
// context.DeadlineExceeded if the timeout fires.
//
// literal strings (systemctl, df, sysctl, etc.) or from PATH
// lookups performed inside facts/ itself, never from user input.
//
//nolint:gosec // G204: name and args come from this package's
func probeOutput(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).Output()
}

// probeCombined is the facts-package replacement for
// `exec.Command(name, args...).CombinedOutput()`. Same timeout
// semantics as probeOutput.
//
//nolint:gosec // G204: see probeOutput.
func probeCombined(name string, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(context.Background(), probeTimeout)
	defer cancel()
	return exec.CommandContext(ctx, name, args...).CombinedOutput()
}
