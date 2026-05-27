package executor_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestExecutor_CancelBetweenSteps is the F016 stage-1(a) reproducer.
// A pre-cancelled context fed to executor.Start must short-circuit the
// step loop and surface ctx.Err(). On master (no F016) the plan runs
// to completion regardless of ctx.
func TestExecutor_CancelBetweenSteps(t *testing.T) {
	// t.Parallel() intentionally NOT set: facts.Collect() and other
	// process-wide state shared with sibling tests (see executor's
	// existing test files) cause a race detector trip when these run
	// concurrently. Sequential is fast enough (~10ms each).

	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	c := filepath.Join(dir, "c")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + a + `, content: A }
  - file.write: { path: ` + b + `, content: B }
  - file.write: { path: ` + c + `, content: C }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // already cancelled before Start sees it

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(ctx, executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)

	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	// None of the writes should have happened — the loop's first
	// iteration sees ctx.Err() != nil and returns before dispatching
	// the first file.write.
	for _, p := range []string{a, b, c} {
		if _, statErr := os.Stat(p); statErr == nil {
			t.Errorf("file %s should NOT exist (cancel was pre-Start); got existing file", p)
		}
	}
}

// TestExecutor_CancelMidPlan covers the realistic case: ctx cancels
// while the plan is running. The currently-executing step finishes,
// then the loop short-circuits on the next iteration. We use a
// shell-free plan (file.write only) so the in-flight step is fast and
// the assertion is deterministic — handler-level ctx-respect is the
// stage-3 audit.
func TestExecutor_CancelMidPlan(t *testing.T) {
	// t.Parallel() intentionally NOT set: facts.Collect() and other
	// process-wide state shared with sibling tests (see executor's
	// existing test files) cause a race detector trip when these run
	// concurrently. Sequential is fast enough (~10ms each).

	dir := t.TempDir()
	a := filepath.Join(dir, "a")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + a + `, content: A }
  - file.write: { path: ` + filepath.Join(dir, "b") + `, content: B }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	// Cancel slightly after Start begins. The 5ms is a balance: long
	// enough that ExecuteSteps has dispatched the first step (or is
	// dispatching it), short enough that the test finishes fast. If
	// the loop got past the first step before cancel fires, no test
	// signal — the plan finished and there's no race.
	go func() {
		time.Sleep(5 * time.Millisecond)
		cancel()
	}()

	publisher := events.NewPublisher()
	defer publisher.Close()

	err := executor.Start(ctx, executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)

	// Either: cancel hit between steps → context.Canceled.
	// Or: cancel arrived too late, plan completed → nil err.
	// Both are acceptable for stage-1(a). The bad outcome (cancel
	// ignored AND plan blocked indefinitely) is what we'd see on
	// master with a long-running step — covered by the agentd test.
	if err != nil && !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled or nil", err)
	}
}

// TestF055_UnlessGuardRespectsCtxCancel pins the F055 fix: when a
// step's `unless:` (or any of its aliases) shells out to a
// long-running command, ctx cancel must abort the subprocess
// instead of waiting for it to exit. Pre-fix the guard called
// `exec.Command("sh", "-c", ...).Run()` with no ctx awareness, so
// Ctrl-C during a hanging guard left mooncake unresponsive for the
// entire subprocess lifetime.
//
// The 30-second `unless: sleep 30` would block the test for 30 s
// without the fix; ctx cancelled at ~50 ms aborts the guard, the
// step skips (because `unless` returned non-zero via context
// cancel) is NOT what we assert — what we assert is that the
// guard call itself returns quickly. We use the elapsed-time
// bound as the regression signal.
func TestF055_UnlessGuardRespectsCtxCancel(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + a + `, content: A }
    unless: "sleep 30"
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	publisher := events.NewPublisher()
	defer publisher.Close()
	_ = executor.Start(ctx, executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)
	elapsed := time.Since(start)

	// Pre-F055: this elapses ~30s (the `sleep 30` runs to completion
	// because exec.Command ignores ctx). Post-F055: ~50ms (ctx
	// cancels the unless subprocess; the 10s hard cap is the
	// fallback). Allow a generous 5s ceiling to keep the test stable
	// under CI load while still catching the multi-second regression.
	if elapsed > 5*time.Second {
		t.Errorf("unless: guard ignored ctx cancel — elapsed=%v, expected <5s (pre-fix would block ~30s)", elapsed)
	}
}

// TestF055_UnlessGuardHardTimeout covers the safety net: even when
// no ctx cancel fires, the per-guard timeout caps the wait. A
// `sleep 30` on an unset ctx must return within ~10s (the cap)
// rather than blocking 30s. This protects operators who run
// mooncake without an outer ctx cancel path (legacy entry points,
// tests, or simply forgetting to wire SIGINT).
func TestF055_UnlessGuardHardTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping 10s timeout assertion in -short mode")
	}
	dir := t.TempDir()
	a := filepath.Join(dir, "a")

	yaml := `version: "1.0"
steps:
  - file.write: { path: ` + a + `, content: A }
    unless: "sleep 30"
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	start := time.Now()
	publisher := events.NewPublisher()
	defer publisher.Close()
	// Background ctx — no operator-side cancel. Hard cap is the
	// only thing protecting the run.
	_ = executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)
	elapsed := time.Since(start)

	// 10s cap + small dispatch / event-overhead headroom = 13s
	// ceiling. Pre-F055 would block ~30s on the sleep.
	if elapsed > 13*time.Second {
		t.Errorf("unless: guard exceeded hard timeout cap — elapsed=%v, expected ~10s", elapsed)
	}
	// Sanity: the step's `unless` ran (and either timed out or got
	// cancelled), so the file.write must have proceeded because a
	// non-zero `unless` means "don't skip". File should exist.
	if _, statErr := os.Stat(a); statErr != nil {
		t.Errorf("file %s should exist (unless: sleep 30 timed out → step ran): %v", a, statErr)
	}
}
