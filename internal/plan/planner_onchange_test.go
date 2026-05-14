package plan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPlanner_OnChange_ExpandsAsSiblings verifies the planner expands an
// on_change child list into sibling plan-steps that immediately follow
// the parent, each tagged with TriggeredBy = parent.ID. The executor's
// gate on TriggeredBy then handles the run/skip decision at apply time.
func TestPlanner_OnChange_ExpandsAsSiblings(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")
	configContent := `version: "1.0"
steps:
  - name: write config
    shell: echo hello
    on_change:
      - name: reload service
        shell: echo reload
      - name: post-reload log
        shell: echo logged
  - name: standalone after parent
    shell: echo standalone
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("new planner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}

	// Expect 4 plan steps: parent + 2 on_change children + the standalone
	// sibling. on_change children must appear in declaration order
	// immediately after their parent, before the next top-level step.
	if got, want := len(plan.Steps), 4; got != want {
		t.Fatalf("plan steps = %d, want %d:\n%v", got, want, planNames(plan))
	}

	parent := plan.Steps[0]
	if parent.Name != "write config" {
		t.Fatalf("step 0 = %q, want 'write config'", parent.Name)
	}
	if parent.TriggeredBy != "" {
		t.Errorf("parent should not have TriggeredBy; got %q", parent.TriggeredBy)
	}
	// Parent's OnChange field is cleared after expansion (would otherwise
	// double-render in the plan).
	if len(parent.OnChange) != 0 {
		t.Errorf("parent OnChange should be cleared after expansion; got %d", len(parent.OnChange))
	}

	for i, name := range []string{"reload service", "post-reload log"} {
		child := plan.Steps[1+i]
		if child.Name != name {
			t.Errorf("step %d name = %q, want %q", 1+i, child.Name, name)
		}
		if child.TriggeredBy != parent.ID {
			t.Errorf("step %d TriggeredBy = %q, want parent.ID %q",
				1+i, child.TriggeredBy, parent.ID)
		}
	}

	standalone := plan.Steps[3]
	if standalone.Name != "standalone after parent" {
		t.Errorf("step 3 = %q, want 'standalone after parent'", standalone.Name)
	}
	if standalone.TriggeredBy != "" {
		t.Errorf("standalone should not have TriggeredBy; got %q", standalone.TriggeredBy)
	}
}

// TestPlanner_OnChange_NestedChildren guards that an on_change child can
// itself have on_change. Spec §259: "each on_change scopes to its
// immediate parent." Recursive expansion gets us that automatically.
func TestPlanner_OnChange_NestedChildren(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nested.yml")
	configContent := `version: "1.0"
steps:
  - name: outer
    shell: echo outer
    on_change:
      - name: middle
        shell: echo middle
        on_change:
          - name: inner
            shell: echo inner
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len(plan.Steps) != 3 {
		t.Fatalf("plan steps = %d, want 3:\n%v", len(plan.Steps), planNames(plan))
	}
	outer, middle, inner := plan.Steps[0], plan.Steps[1], plan.Steps[2]
	if outer.TriggeredBy != "" {
		t.Errorf("outer TriggeredBy = %q, want empty", outer.TriggeredBy)
	}
	if middle.TriggeredBy != outer.ID {
		t.Errorf("middle TriggeredBy = %q, want outer.ID %q", middle.TriggeredBy, outer.ID)
	}
	if inner.TriggeredBy != middle.ID {
		t.Errorf("inner TriggeredBy = %q, want middle.ID %q", inner.TriggeredBy, middle.ID)
	}
}

// TestPlanner_OnChange_EmptyChildrenIsHarmless: an `on_change: []` (or
// the field absent entirely) leaves the parent as a regular plan step
// with no extras appended. Defensive case — empty lists creep in via
// templating.
func TestPlanner_OnChange_EmptyChildrenIsHarmless(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "empty.yml")
	configContent := `version: "1.0"
steps:
  - name: solo
    shell: echo solo
    on_change: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	planner, _ := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("build plan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Errorf("plan steps = %d, want 1 (empty on_change)", len(plan.Steps))
	}
}

func planNames(p *Plan) []string {
	out := make([]string, len(p.Steps))
	for i, s := range p.Steps {
		out[i] = s.Name
	}
	return out
}
