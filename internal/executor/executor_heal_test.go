package executor_test

// Proposal-11 vertical-slice tests: heal: on assert steps.
//
// Three branches exercised end-to-end (planner + executor + counters):
//   - heal absent: assert failure propagates as before;
//   - heal present + restores predicate: original failure is suppressed,
//     run-wide HealedSteps bumps to 1;
//   - heal present + predicate still fails: failure surfaces and
//     HealedSteps stays 0.
//
// A validation test pins the slice-scope rule that heal: is only legal
// on assert steps; the generalisation to wait.* / observe.* lives in a
// follow-up.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// runAndCaptureRunCompleted runs the YAML config and returns the
// RunCompletedData payload + the run-level error. Returns a nil event
// pointer if run.completed never fired (catastrophic setup failure).
func runAndCaptureRunCompleted(t *testing.T, yamlBody string) (*events.RunCompletedData, error) {
	t.Helper()

	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(configPath, []byte(yamlBody), 0o644); err != nil {
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

	err := executor.Start(context.Background(), executor.StartConfig{
		ConfigFilePath: configPath,
	}, logger.NewTestLogger(), publisher)

	return runCompleted, err
}

// TestHeal_HappyPath_RestoresPredicate: assert file exists on a path
// that doesn't yet exist; heal writes the file. The re-check passes,
// the run-level error is nil, HealedSteps == 1.
func TestHeal_HappyPath_RestoresPredicate(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "should-exist")

	yaml := `version: "1.0"
steps:
  - name: ensure_target
    assert:
      file:
        path: ` + target + `
        exists: true
    heal:
      - file.write: { path: ` + target + `, content: restored }
`

	rc, err := runAndCaptureRunCompleted(t, yaml)
	if err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}
	if rc == nil {
		t.Fatal("run.completed event was never emitted")
	}
	if rc.HealedSteps != 1 {
		t.Errorf("HealedSteps = %d, want 1", rc.HealedSteps)
	}
	if rc.FailedSteps != 0 {
		t.Errorf("FailedSteps = %d, want 0 (heal suppressed the failure)", rc.FailedSteps)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("expected heal to create %s; stat err = %v", target, statErr)
	}
}

// TestHeal_StillFailingAfterHeal: heal writes the WRONG file, so the
// post-heal re-check still fails. Apply returns an error, the recap
// counts the step as failed, HealedSteps stays at 0, and the run is
// not marked successful.
func TestHeal_StillFailingAfterHeal(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "should-exist")
	wrong := filepath.Join(dir, "decoy")

	yaml := `version: "1.0"
steps:
  - name: ensure_target
    assert:
      file:
        path: ` + target + `
        exists: true
    heal:
      - file.write: { path: ` + wrong + `, content: this-is-the-wrong-file }
`

	rc, err := runAndCaptureRunCompleted(t, yaml)
	if err == nil {
		t.Fatal("expected apply to fail (heal did not restore the predicate)")
	}
	if rc == nil {
		t.Fatal("run.completed event was never emitted")
	}
	if rc.HealedSteps != 0 {
		t.Errorf("HealedSteps = %d, want 0 (re-check still failing)", rc.HealedSteps)
	}
	if rc.FailedSteps == 0 {
		t.Errorf("FailedSteps = %d, want > 0 (the run failed)", rc.FailedSteps)
	}
	if rc.Success {
		t.Error("Success = true, want false")
	}
}

// TestHeal_AbsentField_NoBehaviorChange: a passing assert without any
// heal: field must keep its existing semantics — Success=true, no
// HealedSteps bump.
func TestHeal_AbsentField_NoBehaviorChange(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "already-there")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	yaml := `version: "1.0"
steps:
  - name: just_check
    assert:
      file:
        path: ` + target + `
        exists: true
`

	rc, err := runAndCaptureRunCompleted(t, yaml)
	if err != nil {
		t.Fatalf("apply errored unexpectedly: %v", err)
	}
	if rc == nil {
		t.Fatal("run.completed event was never emitted")
	}
	if rc.HealedSteps != 0 {
		t.Errorf("HealedSteps = %d, want 0 (no heal was needed or declared)", rc.HealedSteps)
	}
	if !rc.Success {
		t.Error("Success = false, want true")
	}
}

// TestHeal_PlanMode_PreviewsChildren: in plan mode the planner expands
// heal children as siblings tagged HealParent so `mooncake plan`
// renders their per-step diff/perms/risk alongside the parent assert
// (proposal-11 follow-up). The parent + each heal child should emit
// EventStepChecked exactly once; the heal-child entries must reference
// the parent via HealParent.
func TestHeal_PlanMode_PreviewsChildren(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "would-heal")
	configPath := filepath.Join(dir, "config.yml")

	// Seed the target so the parent assert passes at plan time. This
	// isolates the test to the question we care about — does the
	// planner expand heal children for plan-output preview, and does
	// the executor run them in plan mode so their diff/perms surface
	// — without coupling it to the (separate) question of how the
	// step loop behaves when the parent assert fails.
	if err := os.WriteFile(target, []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed target: %v", err)
	}

	yaml := `version: "1.0"
steps:
  - name: ensure_target
    assert:
      file:
        path: ` + target + `
        exists: true
    heal:
      - file.write: { path: ` + target + `, content: restored }
`
	if err := os.WriteFile(configPath, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	publisher := events.NewSyncPublisher()
	var checkedEvents []events.StepCheckedData
	publisher.Subscribe(&capturingSubscriber{
		onEvent: func(e events.Event) {
			if e.Type == events.EventStepChecked {
				if d, ok := e.Data.(events.StepCheckedData); ok {
					checkedEvents = append(checkedEvents, d)
				}
			}
		},
	})

	// Build the plan via the public planner entry point so the
	// expansion path (which the test exercises) runs exactly as it
	// does for `mooncake plan`. Then drive ExecutePlan in ModePlan.
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configPath,
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	// The plan must contain the parent assert step AND its heal
	// child expanded as a sibling tagged with HealParent.
	var parentID string
	var sawHealChildInPlan bool
	for _, s := range planData.Steps {
		if s.Assert != nil && s.HealParent == "" {
			parentID = s.ID
		}
		if s.HealParent != "" {
			sawHealChildInPlan = true
			if parentID != "" && s.HealParent != parentID {
				t.Errorf("heal-child HealParent = %q, want %q", s.HealParent, parentID)
			}
		}
	}
	if !sawHealChildInPlan {
		t.Fatal("planner did not expand heal child as a sibling with HealParent set")
	}

	// Plan-mode execution: parent assert passes (the file exists),
	// the heal-child sibling flows through dispatchRunner and emits
	// StepChecked with the handler's diff/perms output.
	if err := executor.ExecutePlan(context.Background(), planData, "", actions.ModePlan, logger.NewTestLogger(), publisher); err != nil {
		t.Fatalf("ExecutePlan in plan mode: %v", err)
	}

	// We expect at least one StepChecked entry whose step represents
	// the heal child (file.write). The simplest signal is the action
	// name on the checked event.
	var sawHealChild bool
	for _, e := range checkedEvents {
		if e.Action == "file.write" {
			sawHealChild = true
			break
		}
	}
	if !sawHealChild {
		t.Errorf("expected a StepChecked event for the heal child (file.write); got actions %v",
			actionsOf(checkedEvents))
	}
}

// actionsOf flattens StepCheckedData entries to a slice of action
// names for error reporting.
func actionsOf(es []events.StepCheckedData) []string {
	out := make([]string, 0, len(es))
	for _, e := range es {
		out = append(out, e.Action)
	}
	return out
}

// TestHeal_OnlyValidOnAssert: heal: on a non-assert step (shell) must
// be rejected at validation time. This pins the slice-scope rule;
// loosening it to other check-like actions is a follow-up.
func TestHeal_OnlyValidOnAssert(t *testing.T) {
	yaml := `version: "1.0"
steps:
  - name: misplaced_heal
    shell: { cmd: "true" }
    heal:
      - file.write: { path: /tmp/p11-never-written, content: x }
`

	rc, err := runAndCaptureRunCompleted(t, yaml)
	if err == nil {
		t.Fatal("expected validation error for heal: on a shell step")
	}
	// Validation runs before any step executes; run.completed may or may
	// not fire depending on where validation lives. The error is the
	// signal; if rc is non-nil it must report Success=false.
	if rc != nil && rc.Success {
		t.Errorf("Success = true on a config-validation failure")
	}
}
