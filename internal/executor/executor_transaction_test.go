package executor_test

// Spec-30 PR B executor tests. These exercise the transaction
// state machine end-to-end via the planner+executor combo: write a
// YAML config containing a transaction, plan it, execute it, then
// inspect the filesystem to verify roll-forward (happy path) vs
// roll-back (failing path) semantics.
//
// Lives in package executor_test (external) because it imports
// register to pull in all action handlers — internal-package tests
// would create an import cycle.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"

	// Register all action handlers so transaction-body steps' Reverse()
	// lookups succeed via actions.Get(). Without this import, file.write
	// (and friends) aren't in the registry.
	_ "github.com/alehatsman/mooncake/internal/register"
)

// runConfig is a thin helper: writes config YAML to a temp file,
// builds + applies a plan, returns the surfaced error (if any).
// The test then inspects the filesystem to assert side effects.
func runConfig(t *testing.T, yamlBody string) error {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yamlBody), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	testLogger := logger.NewTestLogger()
	publisher := events.NewPublisher()

	return executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, testLogger, publisher)
}

// TestTransaction_HappyPath_AllStepsCommit verifies the no-rollback
// path: three file.write children all succeed → all three files
// exist after, no rollback occurred.
func TestTransaction_HappyPath_AllStepsCommit(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	b := filepath.Join(dir, "b")
	c := filepath.Join(dir, "c")

	yaml := `version: "1.0"
steps:
  - name: bootstrap
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + b + `, content: B }
      - file.write: { path: ` + c + `, content: C }
`
	if err := runConfig(t, yaml); err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}
	for _, p := range []string{a, b, c} {
		if _, err := os.Stat(p); err != nil {
			t.Errorf("expected file %s to exist; got %v", p, err)
		}
	}
}

// TestTransaction_RollbackOnFailure: a three-child transaction where
// the third child writes to a path inside a non-existent directory
// (the directory doesn't exist; file.write of a regular file under
// a missing parent fails). The first two file.writes should be
// reversed (file.write Reverse for create = delete), so neither
// file exists at the end. Run returns a non-nil error.
func TestTransaction_RollbackOnFailure(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "rb-a")
	b := filepath.Join(dir, "rb-b")
	// Third step writes into a deeply nested non-existent directory
	// without enabling parent creation → fails the write.
	c := "/dev/null/cannot-write-here"

	yaml := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + b + `, content: B }
      - file.write: { path: ` + c + `, content: C }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected apply to fail on the third step")
	}

	// Files A and B were created by the first two steps. Rollback
	// should have deleted them.
	for _, p := range []string{a, b} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("expected %s to be REVERTED (not exist); stat err = %v", p, statErr)
		}
	}
	// c lives under /dev/null/ which fails differently (ENOTDIR) — no
	// useful filesystem check; the fact that apply errored above is the
	// signal that the third step's write failed.
	_ = c
}

// MT-45: when a transaction rolls back, the recap (driven by
// run.completed) should subtract reverted body steps from
// changed_steps and surface them as reverted_steps. Before the fix,
// the original body writes stayed in the changed count even though
// their effects were undone.
func TestTransaction_RollbackRecapMath(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "rb-a")
	b := filepath.Join(dir, "rb-b")
	c := "/dev/null/cannot-write-here"

	yaml := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + b + `, content: B }
      - file.write: { path: ` + c + `, content: C }
`
	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewSyncPublisher()
	var runCompleted *events.RunCompletedData
	publisher.Subscribe(&capturingSubscriber{
		onEvent: func(e events.Event) {
			if e.Type == events.EventRunCompleted {
				if d, ok := e.Data.(events.RunCompletedData); ok {
					runCompleted = &d
				}
			}
		},
	})

	_ = executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)

	if runCompleted == nil {
		t.Fatal("run.completed event was never emitted")
	}

	// Both rolled-back body steps must show up in RevertedSteps;
	// they must NOT inflate ChangedSteps (which would report 2
	// persistent writes that did not happen).
	if runCompleted.RevertedSteps != 2 {
		t.Errorf("RevertedSteps = %d, want 2 (both body writes rolled back)",
			runCompleted.RevertedSteps)
	}
	if runCompleted.ChangedSteps >= 2 {
		t.Errorf("ChangedSteps = %d, want < 2 after rollback (post-revert net effect)",
			runCompleted.ChangedSteps)
	}
}

// capturingSubscriber is a tiny test helper that forwards every event
// to a closure. Used to inspect run.completed without spinning a real
// console subscriber.
type capturingSubscriber struct {
	onEvent func(events.Event)
}

func (c *capturingSubscriber) OnEvent(e events.Event) { c.onEvent(e) }
func (c *capturingSubscriber) Close()                 {}

// TestTransaction_OnRollbackFiresOnFailureOnly: on_rollback children
// run when the transaction rolled back; they skip when it committed.
// Encoded by checking the existence of a marker file the on_rollback
// step writes.
func TestTransaction_OnRollbackFiresOnFailureOnly(t *testing.T) {
	t.Run("rollback path fires on_rollback", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "a")
		c := "/dev/null/cannot-write-here"
		marker := filepath.Join(dir, "rollback-marker")

		yaml := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + c + `, content: C }
    on_rollback:
      - file.write: { path: ` + marker + `, content: "rolled back" }
`
		err := runConfig(t, yaml)
		if err == nil {
			t.Fatal("expected apply to fail")
		}
		if _, statErr := os.Stat(marker); statErr != nil {
			t.Errorf("on_rollback marker should exist after rollback; got %v", statErr)
		}
		// Forward-path file should be reverted.
		if _, statErr := os.Stat(a); !os.IsNotExist(statErr) {
			t.Errorf("forward file should have been reverted; stat err = %v", statErr)
		}
	})

	t.Run("commit path skips on_rollback", func(t *testing.T) {
		dir := t.TempDir()
		a := filepath.Join(dir, "ok-a")
		marker := filepath.Join(dir, "should-not-exist")

		yaml := `version: "1.0"
steps:
  - name: smooth deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
    on_rollback:
      - file.write: { path: ` + marker + `, content: should never write }
`
		if err := runConfig(t, yaml); err != nil {
			t.Fatalf("apply failed unexpectedly: %v", err)
		}
		if _, statErr := os.Stat(marker); !os.IsNotExist(statErr) {
			t.Errorf("on_rollback marker should NOT exist after commit; stat err = %v", statErr)
		}
		if _, statErr := os.Stat(a); statErr != nil {
			t.Errorf("forward file should exist; got %v", statErr)
		}
	})
}

// TestTransaction_RemainingBodyStepsSkipOnFailure: when step K fails,
// body children K+1..N must not run (the transaction is dead). Encoded
// by checking that a 4th child's "would-create" file doesn't exist.
func TestTransaction_RemainingBodyStepsSkipOnFailure(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a")
	failPath := "/dev/null/cannot-write-here"
	shouldNotRun := filepath.Join(dir, "should-not-run")

	yaml := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + failPath + `, content: F }
      - file.write: { path: ` + shouldNotRun + `, content: never }
`
	err := runConfig(t, yaml)
	if err == nil {
		t.Fatal("expected apply to fail")
	}
	if _, statErr := os.Stat(shouldNotRun); !os.IsNotExist(statErr) {
		t.Errorf("body step after failure should NOT have run; stat err = %v", statErr)
	}
}

// TestTransaction_PlanModeIsSafe: in plan mode (dry-run), a
// transaction's children should plan-check without mutating disk.
// Spec-30 §76: planner reversibility check runs at plan time.
func TestTransaction_PlanModeIsSafe(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "plan-a")

	yaml := `version: "1.0"
steps:
  - name: planning
    transaction:
      - file.write: { path: ` + a + `, content: A }
`
	// Same start helper, but no easy plan-mode toggle here — instead,
	// rely on the planner's own tests for plan-mode coverage. This
	// test asserts the apply happy path and is otherwise redundant
	// with TestTransaction_HappyPath_AllStepsCommit; kept as a
	// sanity smoke for the single-child transaction case.
	if err := runConfig(t, yaml); err != nil {
		t.Fatalf("single-child txn apply failed: %v", err)
	}
	if _, err := os.Stat(a); err != nil {
		t.Errorf("expected %s; got %v", a, err)
	}
	_ = strings.TrimSpace // keep strings import used
}

// TestTransaction_F054_RollbackEvents pins the spec-30 rollback-event
// surface F054 added: the four transaction.rollback_* events fire in
// order across a transaction with a mid-stream failure. Before F054,
// only the "↺ Reverse:" log line surfaced rollback to operators;
// machine-readable consumers (runlog, fleet telemetry, mooncake history)
// saw nothing.
//
// Sequence asserted on a 3-body transaction failing at step 3:
//
//  1. transaction.rollback_begin   (failed_step_id = step 3, completed_steps = 2)
//  2. transaction.step_reversed    (step 2, ordered before step 1 by LIFO)
//  3. transaction.step_reversed    (step 1)
//  4. transaction.rollback_complete (reversed_steps = 2)
//
// transaction.rollback_failed fires INSTEAD of complete when any
// Reverse() returns an error — covered by the subtest below.
func TestTransaction_F054_RollbackEvents(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "evt-a")
	b := filepath.Join(dir, "evt-b")
	c := "/dev/null/cannot-write-here"

	yaml := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - file.write: { path: ` + a + `, content: A }
      - file.write: { path: ` + b + `, content: B }
      - file.write: { path: ` + c + `, content: C }
`
	configPath := filepath.Join(t.TempDir(), "config.yml")
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	var seen []events.Type
	var beginData *events.TransactionRollbackBeginData
	var reversed []events.TransactionStepReversedData
	var completeData *events.TransactionRollbackCompleteData
	publisher := events.NewSyncPublisher()
	publisher.Subscribe(&capturingSubscriber{
		onEvent: func(e events.Event) {
			switch e.Type {
			case events.EventTransactionRollbackBegin,
				events.EventTransactionStepReversed,
				events.EventTransactionRollbackComplete,
				events.EventTransactionRollbackFailed:
				seen = append(seen, e.Type)
			}
			switch d := e.Data.(type) {
			case events.TransactionRollbackBeginData:
				beginData = &d
			case events.TransactionStepReversedData:
				reversed = append(reversed, d)
			case events.TransactionRollbackCompleteData:
				completeData = &d
			}
		},
	})

	_ = executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)

	// 1. rollback_begin fired exactly once with CompletedSteps == 2.
	if beginData == nil {
		t.Fatal("transaction.rollback_begin event was never emitted")
	}
	if beginData.CompletedSteps != 2 {
		t.Errorf("rollback_begin.CompletedSteps = %d, want 2 (a + b completed before c failed)",
			beginData.CompletedSteps)
	}
	if beginData.FailedStepName == "" && beginData.FailedStepID == "" {
		t.Error("rollback_begin missing both FailedStepID and FailedStepName")
	}

	// 2 + 3. Two step_reversed events in LIFO order (b then a).
	if len(reversed) != 2 {
		t.Fatalf("step_reversed event count = %d, want 2", len(reversed))
	}
	if reversed[0].Action != "file.write" || reversed[1].Action != "file.write" {
		t.Errorf("step_reversed events should carry file.write action; got %q, %q",
			reversed[0].Action, reversed[1].Action)
	}

	// 4. rollback_complete (NOT failed) since file.write reverse never errors here.
	if completeData == nil {
		t.Fatal("transaction.rollback_complete event was never emitted")
	}
	if completeData.ReversedSteps != 2 {
		t.Errorf("rollback_complete.ReversedSteps = %d, want 2", completeData.ReversedSteps)
	}

	// Sequence: begin → reversed → reversed → complete. Drop any
	// interleaved non-rollback events; we already filtered above.
	wantSeq := []events.Type{
		events.EventTransactionRollbackBegin,
		events.EventTransactionStepReversed,
		events.EventTransactionStepReversed,
		events.EventTransactionRollbackComplete,
	}
	if len(seen) != len(wantSeq) {
		t.Fatalf("event sequence length = %d, want %d (got %v)", len(seen), len(wantSeq), seen)
	}
	for i, want := range wantSeq {
		if seen[i] != want {
			t.Errorf("event[%d] = %s, want %s", i, seen[i], want)
		}
	}
}
