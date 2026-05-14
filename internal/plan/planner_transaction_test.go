package plan_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/plan"
	// Register all action handlers so the planner's reversibility check
	// can look them up via actions.Get(). The internal-package tests
	// don't need this (no handler introspection happens for the regular
	// path), but anything that exercises actions.Get in plan-mode must
	// pull register in or every action looks "unknown". Lives in an
	// _test.go package suffix to avoid the
	// plan ← actions/file ← register ← plan import cycle.
	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestPlanner_Transaction_ExpandsAsCompoundParent verifies that a Step
// with non-empty Transaction emits a parent plan step (no action)
// followed by each child as a sibling with TxnParent set.
func TestPlanner_Transaction_ExpandsAsCompoundParent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "txn.yml")
	configContent := `version: "1.0"
steps:
  - name: bootstrap demo
    transaction:
      - name: write a
        file.write:
          path: /tmp/mc-txn-test-a
          content: A
      - name: write b
        file.write:
          path: /tmp/mc-txn-test-b
          content: B
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("planner: %v", err)
	}
	plan, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	// 3 plan steps: parent + 2 children.
	if got, want := len(plan.Steps), 3; got != want {
		var names []string
		for _, s := range plan.Steps {
			names = append(names, s.Name)
		}
		t.Fatalf("plan steps = %d, want %d:\n%v", got, want, names)
	}

	parent := plan.Steps[0]
	if parent.Name != "bootstrap demo" {
		t.Errorf("step 0 = %q, want 'bootstrap demo'", parent.Name)
	}
	if parent.TxnParent != "" {
		t.Errorf("parent must not carry TxnParent; got %q", parent.TxnParent)
	}
	// Parent's Transaction field is cleared after expansion (linkage
	// survives via the children's TxnParent).
	if len(parent.Transaction) != 0 {
		t.Errorf("parent Transaction should be cleared after expansion; got %d children", len(parent.Transaction))
	}

	for i, name := range []string{"write a", "write b"} {
		child := plan.Steps[1+i]
		if child.Name != name {
			t.Errorf("step %d name = %q, want %q", 1+i, child.Name, name)
		}
		if child.TxnParent != parent.ID {
			t.Errorf("step %d TxnParent = %q, want parent.ID %q",
				1+i, child.TxnParent, parent.ID)
		}
	}
}

// TestPlanner_Transaction_RejectsIrreversibleChild verifies the spec-30
// §76 reversibility check: a transaction containing a step whose
// handler does not implement actions.Reverser is rejected at plan
// time. shell: is the canonical example — irreversible by nature.
func TestPlanner_Transaction_RejectsIrreversibleChild(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "txn-bad.yml")
	configContent := `version: "1.0"
steps:
  - name: deploy
    transaction:
      - name: write a config
        file.write:
          path: /tmp/mc-txn-bad-a
          content: A
      - name: run a side effect
        shell: echo "this cannot be reversed"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := plan.NewPlanner()
	_, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err == nil {
		t.Fatal("expected planner error for irreversible child")
	}
	if !strings.Contains(err.Error(), "does not implement Reverser") {
		t.Errorf("err = %v, want substring 'does not implement Reverser'", err)
	}
	// Should name the offending step for actionability.
	if !strings.Contains(err.Error(), "run a side effect") {
		t.Errorf("err should name the failing step; got: %v", err)
	}
}

// TestPlanner_Transaction_AllowIrreversibleBypassesCheck: when the user
// opts in, the same plan that previously failed now succeeds. The shell
// step is still in the plan; rollback semantics (PR B) will decline to
// reverse it.
func TestPlanner_Transaction_AllowIrreversibleBypassesCheck(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "txn-allow.yml")
	configContent := `version: "1.0"
steps:
  - name: deploy with shell
    allow_irreversible: true
    transaction:
      - name: write a
        file.write:
          path: /tmp/mc-txn-allow-a
          content: A
      - name: do a thing
        shell: echo doing
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := plan.NewPlanner()
	plan, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("allow_irreversible should bypass: %v", err)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 plan steps (parent + 2 children); got %d", len(plan.Steps))
	}
}

// TestPlanner_Transaction_RejectsNestedTransactions: nested
// transactions are out of scope per spec-30 §202.
func TestPlanner_Transaction_RejectsNestedTransactions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "txn-nested.yml")
	configContent := `version: "1.0"
steps:
  - name: outer
    transaction:
      - name: inner
        transaction:
          - name: nested
            file.write:
              path: /tmp/mc-txn-nest
              content: x
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := plan.NewPlanner()
	_, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err == nil {
		t.Fatal("expected planner error for nested transaction")
	}
	if !strings.Contains(err.Error(), "nested transactions") {
		t.Errorf("err = %v, want substring 'nested transactions'", err)
	}
}

// TestPlanner_Transaction_OnRollbackExpandsAsSibling: on_rollback
// children appear in the plan after the transaction body children,
// also tagged with TxnParent. Distinguished by the rollback: prefix on
// Name (PR B will replace this with a typed flag).
func TestPlanner_Transaction_OnRollbackExpandsAsSibling(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "txn-rb.yml")
	configContent := `version: "1.0"
steps:
  - name: txn-with-rollback
    transaction:
      - name: body step
        file.write:
          path: /tmp/mc-txn-rb-body
          content: x
    on_rollback:
      - name: notify
        file.write:
          path: /tmp/mc-txn-rb-notice
          content: rolled-back
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := plan.NewPlanner()
	plan, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	// 3 entries: parent, body, on_rollback.
	if len(plan.Steps) != 3 {
		t.Fatalf("expected 3 plan steps; got %d", len(plan.Steps))
	}
	rollback := plan.Steps[2]
	if !strings.HasPrefix(rollback.Name, "rollback:") {
		t.Errorf("on_rollback step should be prefixed with 'rollback:'; got %q", rollback.Name)
	}
	if rollback.TxnParent != plan.Steps[0].ID {
		t.Errorf("rollback step TxnParent = %q, want %q", rollback.TxnParent, plan.Steps[0].ID)
	}
}
