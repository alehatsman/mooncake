package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestTask_StringShorthand_ExpandsToUseStep verifies the #53 shorthand:
// a string task value becomes a single-step task with that `use:` ref and
// an empty Desc (so the listing can fall back to the component description).
func TestTask_StringShorthand_ExpandsToUseStep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yml")
	if err := os.WriteFile(path, []byte(`
version: "1"
tasks:
  ui-lint: tq/lint
  build:
    desc: full form
    steps:
      - shell: { cmd: "echo hi" }
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	parsed, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	short, ok := parsed.Tasks["ui-lint"]
	if !ok {
		t.Fatal(`Tasks["ui-lint"] missing`)
	}
	if short.Desc != "" {
		t.Errorf("shorthand Desc = %q, want empty (so listing falls back to component desc)", short.Desc)
	}
	if len(short.Steps) != 1 || short.Steps[0].Use != "tq/lint" {
		t.Fatalf("shorthand Steps = %+v, want one step with Use=tq/lint", short.Steps)
	}

	// Full-map form is unchanged.
	full := parsed.Tasks["build"]
	if full.Desc != "full form" || len(full.Steps) != 1 {
		t.Errorf("full-form task altered: %+v", full)
	}
}

// TestTask_StringShorthand_RejectsAmbiguous rejects a string task value that
// can't be a single `use:` reference (contains whitespace) with a clear error.
func TestTask_StringShorthand_RejectsAmbiguous(t *testing.T) {
	var task Task
	err := yaml.Unmarshal([]byte(`"echo hello world"`), &task)
	if err == nil {
		t.Fatal("expected error for whitespace-containing string task value")
	}
}

// TestTask_FullMapWithSingleUse keeps the explicit form (use + props) working;
// this is the path that survives once a one-liner needs extra props.
func TestTask_FullMapWithSingleUse(t *testing.T) {
	var task Task
	in := "steps:\n  - use: goq/ai-lint\n    props:\n      all: true\n"
	if err := yaml.Unmarshal([]byte(in), &task); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(task.Steps) != 1 || task.Steps[0].Use != "goq/ai-lint" {
		t.Fatalf("Steps = %+v", task.Steps)
	}
	if task.Steps[0].Props["all"] != true {
		t.Errorf("props.all = %v, want true", task.Steps[0].Props["all"])
	}
}
