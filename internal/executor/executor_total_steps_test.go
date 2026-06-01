package executor_test

// #78 end-to-end guard: a plan's advertised step total must count only the
// executable leaves, so plan.loaded, run.completed, and the actual step
// stream all agree and the recap's stat buckets sum to the total. The bug:
// the implicit transaction wrapper (and any user transaction parent) was
// counted as a step and its child name listed twice, so total_steps
// over-reported by the number of wrappers and never matched the executed
// leaves.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"

	_ "github.com/alehatsman/mooncake/internal/register"
)

type eventCollector struct{ events []events.Event }

func (c *eventCollector) OnEvent(e events.Event) { c.events = append(c.events, e) }
func (c *eventCollector) Close()                 {}

func (c *eventCollector) only(t *testing.T, typ events.Type) events.Event {
	t.Helper()
	var found []events.Event
	for _, e := range c.events {
		if e.Type == typ {
			found = append(found, e)
		}
	}
	if len(found) != 1 {
		t.Fatalf("expected exactly one %s event, got %d", typ, len(found))
	}
	return found[0]
}

func (c *eventCollector) count(typ events.Type) int {
	n := 0
	for _, e := range c.events {
		if e.Type == typ {
			n++
		}
	}
	return n
}

// A transaction with three reversible children: the compiled plan is one
// wrapper marker + three leaves. total_steps must be 3 (not 4), the leaf
// names must each appear once (no duplication, no wrapper name), the step
// stream must emit exactly three step.completed, and the run.completed stat
// buckets must sum to 3.
func TestRunTotals_TransactionCountsLeavesNotWrapper(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	c := filepath.Join(dir, "c")

	yaml := `version: "1.0"
steps:
  - name: bootstrap
    transaction:
      - name: write a
        file.write: { path: ` + a + `, content: A }
      - name: write b
        file.write: { path: ` + b + `, content: B }
      - name: write c
        file.write: { path: ` + c + `, content: C }
`
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	pub := events.NewSyncPublisher()
	col := &eventCollector{}
	pub.Subscribe(col)

	if err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), pub); err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}

	const wantLeaves = 3

	loaded := col.only(t, events.EventPlanLoaded).Data.(events.PlanLoadedData)
	if loaded.TotalSteps != wantLeaves {
		t.Errorf("plan.loaded total_steps = %d, want %d (leaves only, not the wrapper)", loaded.TotalSteps, wantLeaves)
	}
	if len(loaded.Steps) != wantLeaves {
		t.Errorf("plan.loaded steps = %v, want %d entries", loaded.Steps, wantLeaves)
	}
	// Names must be the leaves, each once — not the "bootstrap" wrapper and
	// not a duplicated child.
	seen := map[string]int{}
	for _, n := range loaded.Steps {
		seen[n]++
	}
	for _, want := range []string{"write a", "write b", "write c"} {
		if seen[want] != 1 {
			t.Errorf("plan.loaded steps: %q appears %d times, want 1 (got %v)", want, seen[want], loaded.Steps)
		}
	}
	if seen["bootstrap"] != 0 {
		t.Errorf("plan.loaded steps lists the transaction wrapper %q; it should not", "bootstrap")
	}

	if got := col.count(events.EventStepCompleted); got != wantLeaves {
		t.Errorf("step.completed count = %d, want %d (one per executed leaf)", got, wantLeaves)
	}

	completed := col.only(t, events.EventRunCompleted).Data.(events.RunCompletedData)
	if completed.TotalSteps != wantLeaves {
		t.Errorf("run.completed total_steps = %d, want %d", completed.TotalSteps, wantLeaves)
	}
	if completed.TotalSteps != loaded.TotalSteps {
		t.Errorf("plan.loaded total_steps (%d) and run.completed total_steps (%d) disagree", loaded.TotalSteps, completed.TotalSteps)
	}
	bucketSum := completed.OkSteps + completed.ChangedSteps + completed.FailedSteps +
		completed.SkippedSteps + completed.RevertedSteps + completed.CancelledSteps
	if bucketSum != completed.TotalSteps {
		t.Errorf("run.completed stat buckets sum to %d but total_steps = %d (must reconcile)", bucketSum, completed.TotalSteps)
	}
}
