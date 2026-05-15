package plan_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/plan"
	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestPlanner_Try_ExpandsAsCompoundParent verifies the spec-23 §2
// expansion: a Step with non-empty Try emits a parent plan step (no
// action; carries Try/Catch/Finally verbatim) followed by each
// branch's children as siblings tagged with TryParent + TryRole.
// Order is fixed: try children first, then catch, then finally.
func TestPlanner_Try_ExpandsAsCompoundParent(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "try.yml")
	configContent := `version: "1.0"
steps:
  - name: deploy app
    try:
      - name: write config
        file.write:
          path: /tmp/mc-try-cfg
          content: cfg
      - name: write seed
        file.write:
          path: /tmp/mc-try-seed
          content: seed
    catch:
      - name: log failure
        log: "deploy failed"
    finally:
      - name: notify
        log: "deploy done"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("planner: %v", err)
	}
	p, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	// 5 plan steps: parent + 2 try + 1 catch + 1 finally.
	if got, want := len(p.Steps), 5; got != want {
		var names []string
		for _, s := range p.Steps {
			names = append(names, s.Name)
		}
		t.Fatalf("plan steps = %d, want %d:\n%v", got, want, names)
	}

	parent := p.Steps[0]
	if parent.Name != "deploy app" {
		t.Errorf("step 0 = %q, want 'deploy app'", parent.Name)
	}
	if parent.TryParent != "" {
		t.Errorf("parent must not carry TryParent; got %q", parent.TryParent)
	}
	if parent.TryRole != "" {
		t.Errorf("parent must not carry TryRole; got %q", parent.TryRole)
	}
	if len(parent.Try) != 2 || len(parent.Catch) != 1 || len(parent.Finally) != 1 {
		t.Errorf("parent branches preserved on plan entry: try=%d catch=%d finally=%d", len(parent.Try), len(parent.Catch), len(parent.Finally))
	}

	expect := []struct {
		name string
		role string
	}{
		{"write config", "try"},
		{"write seed", "try"},
		{"log failure", "catch"},
		{"notify", "finally"},
	}
	for i, e := range expect {
		child := p.Steps[1+i]
		if child.Name != e.name {
			t.Errorf("step %d name = %q, want %q", 1+i, child.Name, e.name)
		}
		if child.TryParent != parent.ID {
			t.Errorf("step %d TryParent = %q, want parent.ID %q", 1+i, child.TryParent, parent.ID)
		}
		if child.TryRole != e.role {
			t.Errorf("step %d TryRole = %q, want %q", 1+i, child.TryRole, e.role)
		}
	}
}

// TestPlanner_Try_AcceptsTryOnly verifies that catch and finally are
// both optional. A try-only compound is a valid shape.
func TestPlanner_Try_AcceptsTryOnly(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "try-only.yml")
	configContent := `version: "1.0"
steps:
  - name: maybe
    try:
      - name: write
        file.write:
          path: /tmp/mc-try-only
          content: x
`
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := plan.NewPlanner()
	p, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	// parent + 1 try child.
	if got := len(p.Steps); got != 2 {
		t.Fatalf("plan steps = %d, want 2", got)
	}
	if p.Steps[1].TryRole != "try" {
		t.Errorf("child TryRole = %q, want 'try'", p.Steps[1].TryRole)
	}
}
