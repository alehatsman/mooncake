//go:build !windows

package shell

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestTimeout_KillsProcessGroup_NotJustShell guards issue #16. When a
// shell-compound command (`sleep N; echo done`) is run with a short
// timeout, the *entire process group* must die — not just the
// outer shell. Without the fix, bash would be killed on timeout, but
// the already-forked `sleep` kept running with init as its new parent
// — wall-clock then matched the sleep duration rather than the
// timeout, despite the step reporting "killed" promptly.
//
// We can't observe the orphan directly from inside the test, so we
// take an indirect measurement: the command writes a sentinel file
// AFTER the sleep. If the sleep gets killed by the group-wide signal,
// the sentinel never appears. If only bash dies and sleep runs to
// completion in the background, the sentinel will appear after the
// test's settling window.
func TestTimeout_KillsProcessGroup_NotJustShell(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("process groups behave differently on Windows")
	}
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "child-finished")
	// sleep 3s then write the sentinel. With a 500ms timeout the whole
	// chain should die before second 3 elapses.
	script := "sleep 3 ; echo done > " + sentinel

	h := &Handler{}
	step := &config.Step{
		Shell: &config.ShellAction{Cmd: script},
		Timeout: "500ms",
	}
	ctx := newCtx(t, false)
	start := time.Now()
	res, err := h.Run(ctx, step)
	elapsed := time.Since(start)

	if err == nil {
		t.Errorf("expected timeout error; got nil result=%v", res)
	}
	if elapsed > 1500*time.Millisecond {
		t.Errorf("step took %v — timeout did not fire promptly", elapsed)
	}

	// Settle: give the orphaned child (if any) up to 4s to flush its
	// sentinel. With the fix the sentinel must NOT appear.
	deadline := time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		if _, err := os.Stat(sentinel); err == nil {
			t.Fatalf("sentinel %s appeared after timeout — child was orphaned, not killed", sentinel)
		}
		time.Sleep(50 * time.Millisecond)
	}
}
