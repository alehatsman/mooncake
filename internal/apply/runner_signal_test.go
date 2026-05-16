package apply_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestApply_NoSignalHandling is the F020 regression guard. apply.Runner
// is the kernel boundary; embedded callers (agentd, MCP, future SDK)
// must control their own signal handling — the kernel calling
// os.Exit(143) on SIGTERM killed daemon graceful-shutdown stone-dead.
//
// The signal-handler reintroduction is mechanical: someone copy-pastes
// `signal.Notify` back in to "make Ctrl-C work" and ships it without
// noticing it also kills the daemon. This test fails fast if any apply
// source file imports os/signal or defines installSignalHandler. The
// CLI side (cmd/mooncake.go) is the right home for signal handling and
// is unaffected by this guard.
func TestApply_NoSignalHandling(t *testing.T) {
	t.Parallel()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		src := string(body)

		// Ban the import lines + the legacy helper. Function-call
		// strings (signal.Notify(, signal.NotifyContext() are not
		// banned by literal match — the import audit is the real
		// guard, and doc comments are allowed to reference the
		// stdlib functions callers should use.
		for _, banned := range []string{
			`"os/signal"`,
			`"syscall"`,
			`func installSignalHandler`,
		} {
			if strings.Contains(src, banned) {
				t.Errorf("%s: F020 regression — %q must not appear in apply package source (signal handling belongs in cmd/)", f, banned)
			}
		}
	}
}
