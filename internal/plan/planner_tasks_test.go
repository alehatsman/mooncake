package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestPlanner_TaskName_SelectsTaskSteps verifies that setting TaskName
// on PlannerConfig replaces the top-level Steps list with the named
// task's Steps. The output plan should contain ONLY the task's steps,
// not the file's top-level steps (which here is empty anyway — the
// next test covers the two-list case).
func TestPlanner_TaskName_SelectsTaskSteps(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
tasks:
  build:
    desc: build the thing
    steps:
      - name: compile
        shell: { cmd: "echo compiling" }
      - name: link
        shell: { cmd: "echo linking" }
  test:
    steps:
      - name: run-tests
        shell: { cmd: "echo testing" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		TaskName:   "build",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if got := len(plan.Steps); got != 2 {
		t.Fatalf("plan.Steps len = %d, want 2 (compile, link)", got)
	}
	if plan.Steps[0].Name != "compile" {
		t.Errorf("plan.Steps[0].Name = %q, want compile", plan.Steps[0].Name)
	}
	if plan.Steps[1].Name != "link" {
		t.Errorf("plan.Steps[1].Name = %q, want link", plan.Steps[1].Name)
	}
}

// TestPlanner_TaskName_IgnoresTopLevelSteps proves the splice fully
// replaces — top-level Steps are dropped when a task is selected.
func TestPlanner_TaskName_IgnoresTopLevelSteps(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mooncake.yml")
	content := `version: "1"
steps:
  - name: top-level-1
    shell: { cmd: "echo top" }
tasks:
  build:
    steps:
      - name: task-step
        shell: { cmd: "echo task" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		TaskName:   "build",
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if got := len(plan.Steps); got != 1 {
		t.Fatalf("plan.Steps len = %d, want 1", got)
	}
	if plan.Steps[0].Name != "task-step" {
		t.Errorf("plan.Steps[0].Name = %q, want task-step", plan.Steps[0].Name)
	}
}

// TestPlanner_TaskName_UnknownErrors verifies the error message lists
// the available tasks so users can fix typos without re-reading the
// file.
func TestPlanner_TaskName_UnknownErrors(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
tasks:
  build:
    steps:
      - shell: { cmd: "echo a" }
  test:
    steps:
      - shell: { cmd: "echo b" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		TaskName:   "buil", // typo
	})
	if err == nil {
		t.Fatal("BuildPlan succeeded with unknown task, want error")
	}
	msg := err.Error()
	if !strings.Contains(msg, `"buil"`) {
		t.Errorf("error %q should quote the typo'd name", msg)
	}
	if !strings.Contains(msg, "build") || !strings.Contains(msg, "test") {
		t.Errorf("error %q should list available tasks (build, test)", msg)
	}
}

// TestPlanner_TaskName_VarPrecedence verifies the merge order:
// file-level vars < task vars < caller-supplied Variables.
func TestPlanner_TaskName_VarPrecedence(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "tasks.yml")
	content := `version: "1"
vars:
  COMMON: file-value
  OVERRIDABLE: file-loses
tasks:
  build:
    vars:
      OVERRIDABLE: task-wins
      TASK_ONLY: task-set
    steps:
      - shell: { cmd: "echo {{ COMMON }} {{ OVERRIDABLE }} {{ TASK_ONLY }} {{ CLI_ONLY }}" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		TaskName:   "build",
		Variables: map[string]interface{}{
			"CLI_ONLY":    "cli-set",
			"OVERRIDABLE": "cli-wins",
		},
	})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if plan.InitialVars["COMMON"] != "file-value" {
		t.Errorf("COMMON = %v, want file-value", plan.InitialVars["COMMON"])
	}
	if plan.InitialVars["TASK_ONLY"] != "task-set" {
		t.Errorf("TASK_ONLY = %v, want task-set", plan.InitialVars["TASK_ONLY"])
	}
	if plan.InitialVars["CLI_ONLY"] != "cli-set" {
		t.Errorf("CLI_ONLY = %v, want cli-set", plan.InitialVars["CLI_ONLY"])
	}
	if plan.InitialVars["OVERRIDABLE"] != "cli-wins" {
		t.Errorf("OVERRIDABLE = %v, want cli-wins (CLI > task > file)", plan.InitialVars["OVERRIDABLE"])
	}
}

// TestPlanner_NoTaskName_UsesTopLevelSteps confirms the existing
// behavior is unchanged when TaskName is empty: top-level Steps run
// as before, tasks block is ignored.
func TestPlanner_NoTaskName_UsesTopLevelSteps(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "mooncake.yml")
	content := `version: "1"
steps:
  - name: apply-step
    shell: { cmd: "echo apply" }
tasks:
  build:
    steps:
      - name: build-step
        shell: { cmd: "echo build" }
`
	if err := os.WriteFile(configPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Name != "apply-step" {
		names := make([]string, 0, len(plan.Steps))
		for _, s := range plan.Steps {
			names = append(names, s.Name)
		}
		t.Fatalf("got Steps=%v, want exactly [apply-step]", names)
	}
}
