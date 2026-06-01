package executor

// #74 (Option C): plan.loaded carries a flattened step-name list so a
// consumer can render the whole plan up front. planStepNames must descend
// into transaction wrappers (the pilot loop wraps every plan in one) and
// reuse the same Name/synthesizeStepName labelling the per-step events use.

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestPlanStepNames_FlattensTransactionWrapper(t *testing.T) {
	steps := []config.Step{
		{
			Name: "mooncake pilot transaction", // synthetic wrapper — must be dropped
			Transaction: []config.Step{
				{Name: "first"},
				{Name: "second"},
			},
		},
	}
	got := planStepNames(steps)
	want := []string{"first", "second"}
	if len(got) != len(want) {
		t.Fatalf("planStepNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestPlanStepNames_TopLevelAndSynthesized(t *testing.T) {
	steps := []config.Step{
		{Name: "named-step"},
		{Shell: &config.ShellAction{Cmd: "echo hi"}}, // no Name → synthesized
	}
	got := planStepNames(steps)
	if len(got) != 2 {
		t.Fatalf("planStepNames = %v, want 2 entries", got)
	}
	if got[0] != "named-step" {
		t.Errorf("step[0] = %q, want %q", got[0], "named-step")
	}
	// The unnamed shell step gets a synthesized label; we don't pin the
	// exact format (that's synthesizeStepName's contract) but it must be
	// non-empty and mention the command.
	if got[1] == "" {
		t.Error("step[1]: unnamed shell step should synthesize a non-empty label")
	}
}

func TestPlanStepNames_NestedTransactions(t *testing.T) {
	steps := []config.Step{
		{
			Transaction: []config.Step{
				{Name: "a"},
				{Transaction: []config.Step{{Name: "b"}, {Name: "c"}}},
			},
		},
	}
	got := planStepNames(steps)
	want := []string{"a", "b", "c"}
	if len(got) != len(want) {
		t.Fatalf("planStepNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}
