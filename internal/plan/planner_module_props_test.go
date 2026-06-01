package plan

import (
	"os"
	"path/filepath"
	"testing"
)

// TestPlanner_ModuleDefaultProps_Merged verifies that #52 module-level
// default props are layered under a `use: <alias>` step at plan time. The
// alias defers expansion to apply time, so the use: step survives in the
// plan with its props already merged + rendered.
func TestPlanner_ModuleDefaultProps_Merged(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
vars:
  GO_TAGS: sqlite_fts5
modules:
  goq:
    source: "127.0.0.1:8080/owner/go-quality@v0.1.1"
    props:
      go_tags: "{{ GO_TAGS }}"
      dir: "."
tasks:
  lint:
    steps:
      - use: goq/lint
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath, TaskName: "lint"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan.Steps len = %d, want 1 (the deferred use: goq/lint)", len(plan.Steps))
	}
	props := plan.Steps[0].Props
	if props == nil {
		t.Fatalf("use step has nil Props; module defaults were not merged")
	}
	if got := props["go_tags"]; got != "sqlite_fts5" {
		t.Errorf("props[go_tags] = %v, want sqlite_fts5 (templated module default)", got)
	}
	if got := props["dir"]; got != "." {
		t.Errorf("props[dir] = %v, want .", got)
	}
}

// TestPlanner_ModuleDefaultProps_PerCallWins verifies a per-call prop
// overrides the module default for the same key, while other defaults
// still fill in.
func TestPlanner_ModuleDefaultProps_PerCallWins(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
modules:
  tq:
    source: "127.0.0.1:8080/owner/ts-quality@v0.1.0"
    props:
      dir: web
      strict: true
tasks:
  lint:
    steps:
      - use: tq/lint
        props:
          dir: other
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath, TaskName: "lint"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan.Steps len = %d, want 1", len(plan.Steps))
	}
	props := plan.Steps[0].Props
	if got := props["dir"]; got != "other" {
		t.Errorf("props[dir] = %v, want other (per-call overrides module default)", got)
	}
	if got := props["strict"]; got != true {
		t.Errorf("props[strict] = %v, want true (module default fills the gap)", got)
	}
}

// TestPlanner_BareStringModule_NoProps confirms the back-compat bare-string
// modules form parses and contributes no default props.
func TestPlanner_BareStringModule_NoProps(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
modules:
  goq: "127.0.0.1:8080/owner/go-quality@v0.1.1"
tasks:
  lint:
    steps:
      - use: goq/lint
        props:
          dir: "."
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath, TaskName: "lint"})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("plan.Steps len = %d, want 1", len(plan.Steps))
	}
	props := plan.Steps[0].Props
	if got := props["dir"]; got != "." {
		t.Errorf("props[dir] = %v, want .", got)
	}
	if _, extra := props["go_tags"]; extra {
		t.Errorf("bare-string module contributed unexpected default props: %v", props)
	}
}
