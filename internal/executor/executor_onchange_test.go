package executor_test

import (
	"os"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// newExecOnChangeContext builds an ExecutionContext appropriate for the
// on_change executor tests. Mirrors the pattern in TestExecuteSteps —
// minimal Svc with logger + template + evaluator + redactor + stats.
func newExecOnChangeContext(t *testing.T) *executor.ExecutionContext {
	t.Helper()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:    logger.NewTestLogger(),
			Template:  renderer,
			Evaluator: expression.NewGovaluateEvaluator(),
			PathUtil:  pathutil.NewPathExpander(renderer),
			Stats:     executor.NewExecutionStats(),
			Redactor:  security.NewRedactor(),
		},
		Scope:       executor.NewVariableScope(),
		CurrentDir:  os.TempDir(),
		CurrentFile: "test.yml",
	}
}

// TestExecutor_OnChange_RunsWhenParentChanged drives a 2-step sequence
// where the parent's outcome is `changed`, then the on_change child runs.
// Verifies the child's command was actually executed by checking the
// global step counter.
func TestExecutor_OnChange_RunsWhenParentChanged(t *testing.T) {
	ec := newExecOnChangeContext(t)

	// Parent step: a shell that exits 0 and reports `changed=true` via
	// shell action semantics (any successful run is "changed" today).
	// Child: a shell tagged TriggeredBy=parent.ID.
	steps := []config.Step{
		{
			ID:         "step-0001",
			Name:       "parent",
			ActionType: "shell",
			Shell:      &config.ShellAction{Cmd: "true"},
		},
		{
			ID:          "step-0002",
			Name:        "child",
			ActionType:  "shell",
			Shell:       &config.ShellAction{Cmd: "true"},
			TriggeredBy: "step-0001",
		},
	}

	if err := executor.ExecuteSteps(steps, ec); err != nil {
		t.Fatalf("ExecuteSteps: %v", err)
	}

	// Parent + child both ran → executed=2, skipped=0.
	if *ec.Svc.Stats.Global != 2 {
		t.Errorf("global step count = %d, want 2 (parent + child)", *ec.Svc.Stats.Global)
	}
	if *ec.Svc.Stats.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", *ec.Svc.Stats.Skipped)
	}
	// Sanity: ChangedByStepID was populated for the parent's ID.
	if !ec.ChangedByStepID["step-0001"] {
		t.Errorf("expected ChangedByStepID[step-0001]=true after parent ran")
	}
}

// TestExecutor_OnChange_SkipsWhenParentDidNotChange: a child whose parent
// has reported changed=false must skip with an actionable reason. The
// parent emits a "no-op" pattern via `unless_command: true` (always
// succeeds → skip → no change).
func TestExecutor_OnChange_SkipsWhenParentDidNotChange(t *testing.T) {
	ec := newExecOnChangeContext(t)
	// Seed the changed map directly to simulate a parent that ran but
	// reported changed=false. We can't easily produce changed=false from
	// a real shell action (shell always reports changed=true on success),
	// so seeding the map is the cleanest way to isolate the
	// TriggeredBy-gate behavior under test.
	ec.ChangedByStepID = map[string]bool{"step-0001": false}

	child := config.Step{
		ID:          "step-0002",
		Name:        "child of unchanged parent",
		ActionType:  "shell",
		Shell:       &config.ShellAction{Cmd: "true"},
		TriggeredBy: "step-0001",
	}

	if err := executor.ExecuteStep(child, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}

	if *ec.Svc.Stats.Global != 0 {
		t.Errorf("global step count = %d, want 0 (child should have skipped)", *ec.Svc.Stats.Global)
	}
	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1", *ec.Svc.Stats.Skipped)
	}
}

// TestExecutor_OnChange_SkipsWhenParentNeverRan: defensive guard. If a
// child's TriggeredBy points at a step ID that never recorded a result
// (e.g. parent was itself skipped), the child must skip too — never
// silently treat "unknown parent" as "parent changed".
func TestExecutor_OnChange_SkipsWhenParentNeverRan(t *testing.T) {
	ec := newExecOnChangeContext(t)
	// ec.ChangedByStepID is nil — parent never recorded anything.

	child := config.Step{
		ID:          "step-0002",
		Name:        "orphan child",
		ActionType:  "shell",
		Shell:       &config.ShellAction{Cmd: "true"},
		TriggeredBy: "step-0000-nonexistent",
	}

	if err := executor.ExecuteStep(child, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}

	if *ec.Svc.Stats.Skipped != 1 {
		t.Errorf("skipped = %d, want 1 (orphan must skip, not run)", *ec.Svc.Stats.Skipped)
	}
}

// TestExecutor_OnChange_NormalStepUnaffected: a step with no TriggeredBy
// is not gated and runs unconditionally. Pins the regression boundary
// for the conditional-dispatch logic.
func TestExecutor_OnChange_NormalStepUnaffected(t *testing.T) {
	ec := newExecOnChangeContext(t)
	step := config.Step{
		ID:         "step-0001",
		Name:       "standalone",
		ActionType: "shell",
		Shell:      &config.ShellAction{Cmd: "true"},
		// TriggeredBy intentionally empty.
	}
	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}
	if *ec.Svc.Stats.Global != 1 {
		t.Errorf("global step count = %d, want 1", *ec.Svc.Stats.Global)
	}
	if *ec.Svc.Stats.Skipped != 0 {
		t.Errorf("skipped = %d, want 0", *ec.Svc.Stats.Skipped)
	}
}
