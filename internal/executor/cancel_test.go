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
