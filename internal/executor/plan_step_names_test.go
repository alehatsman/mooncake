package executor

// #74 (Option C): plan.loaded carries a flattened step-name list so a
// consumer can render the whole plan up front. #78: planStepNames runs on
// the *compiled* plan, where a compound's children are already expanded as
// sibling steps and the parent is a structural no-op marker. It must list
// the leaves and skip the markers — descending into a marker's Transaction
// would list the children twice (once via the parent, once as siblings) and
// inflate the count past the leaves that actually execute.

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// wrappedPlanSteps mirrors what the planner's expandTransaction produces for
// a transaction: the parent marker (Transaction populated, TxnRole empty)
// followed by each child expanded as a sibling (TxnParent set, TxnRole
// "body"). This is the shape executePlanWithCapture actually hands
// planStepNames — not the un-expanded config shape.
func wrappedPlanSteps(parentName string, children ...config.Step) []config.Step {
	parent := config.Step{Name: parentName, Transaction: children}
	steps := []config.Step{parent}
	for _, c := range children {
		c.TxnParent = "txn-parent"
		c.TxnRole = "body"
		steps = append(steps, c)
	}
	return steps
}

func TestPlanStepNames_SkipsTransactionMarker_ListsLeaves(t *testing.T) {
	steps := wrappedPlanSteps("agent apply",
		config.Step{Name: "first"},
		config.Step{Name: "second"},
	)
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

// Regression for #78: a single-leaf agent plan. The compiled plan is the
// "agent apply" transaction marker plus one expanded child. plan.loaded must
// list the leaf exactly once and count it exactly once — the bug listed it
// twice (descended into the parent AND listed the sibling) and reported
// total_steps: 2 against a single executed leaf.
func TestPlanStepNames_SingleLeafWrapper_NoDuplication(t *testing.T) {
	steps := wrappedPlanSteps("agent apply",
		config.Step{Name: "log test issue"},
	)
	if got := planStepNames(steps); len(got) != 1 || got[0] != "log test issue" {
		t.Fatalf("planStepNames = %v, want [\"log test issue\"]", got)
	}
	if got := executableStepCount(steps); got != 1 {
		t.Fatalf("executableStepCount = %d, want 1 (only the leaf executes, not the wrapper)", got)
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

// A try-parent is the other structural marker the executor short-circuits;
// like a transaction parent it must be skipped, with its expanded branch
// siblings carrying the leaf names.
func TestPlanStepNames_SkipsTryMarker_ListsBranches(t *testing.T) {
	steps := []config.Step{
		{Try: []config.Step{{Name: "attempt"}}}, // try-parent marker (TryRole empty)
		{Name: "attempt", TryParent: "try-parent", TryRole: "try"},
		{Name: "recover", TryParent: "try-parent", TryRole: "catch"},
	}
	got := planStepNames(steps)
	want := []string{"attempt", "recover"}
	if len(got) != len(want) {
		t.Fatalf("planStepNames = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("step[%d] = %q, want %q", i, got[i], want[i])
		}
	}
	if got := executableStepCount(steps); got != 2 {
		t.Errorf("executableStepCount = %d, want 2", got)
	}
}

// executableStepCount counts every leaf and skips only the structural
// markers; a plan with no compounds counts all its steps.
func TestExecutableStepCount_NoCompounds(t *testing.T) {
	steps := []config.Step{{Name: "a"}, {Name: "b"}, {Name: "c"}}
	if got := executableStepCount(steps); got != 3 {
		t.Errorf("executableStepCount = %d, want 3", got)
	}
}
