package config

import (
	"os"
	"path/filepath"
	"testing"
)

// TestReadConfig_TasksOnlyFile verifies a file with `tasks:` but no
// top-level `steps:` parses cleanly. This is the dedicated-tasks.yml
// happy path that depends on the schema's anyOf [steps, tasks].
func TestReadConfig_TasksOnlyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yml")
	if err := os.WriteFile(path, []byte(`
version: '1'
vars:
  GREETING: Hello
tasks:
  hello:
    desc: Print a friendly greeting
    vars:
      NAME: World
    steps:
      - shell: { cmd: "echo {{ GREETING }} {{ NAME }}" }
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	parsed, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}

	if got := len(parsed.Tasks); got != 1 {
		t.Fatalf("Tasks len = %d, want 1", got)
	}
	task, ok := parsed.Tasks["hello"]
	if !ok {
		t.Fatalf("Tasks[\"hello\"] missing")
	}
	if task.Desc != "Print a friendly greeting" {
		t.Errorf("Desc = %q", task.Desc)
	}
	if task.Vars["NAME"] != "World" {
		t.Errorf("Vars[NAME] = %v, want World", task.Vars["NAME"])
	}
	if got := len(task.Steps); got != 1 {
		t.Fatalf("Steps len = %d, want 1", got)
	}
	if task.Steps[0].Shell == nil || task.Steps[0].Shell.Cmd == "" {
		t.Errorf("Steps[0].Shell.Cmd not populated")
	}
}

// TestReadConfig_StepsAndTasksTogether verifies both top-level shapes
// can coexist in mooncake.yml. apply will look at Steps; task will
// look at Tasks.
func TestReadConfig_StepsAndTasksTogether(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(path, []byte(`
version: '1'
steps:
  - shell: { cmd: "echo apply" }
tasks:
  build:
    steps:
      - shell: { cmd: "echo build" }
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	parsed, err := ReadConfig(path)
	if err != nil {
		t.Fatalf("ReadConfig: %v", err)
	}
	if len(parsed.Steps) != 1 {
		t.Errorf("Steps len = %d, want 1", len(parsed.Steps))
	}
	if _, ok := parsed.Tasks["build"]; !ok {
		t.Errorf("Tasks[\"build\"] missing")
	}
}

// TestReadConfig_EmptyConfig is the negative case: a file with neither
// steps nor tasks should fail the schema's anyOf gate.
func TestReadConfig_EmptyConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(path, []byte(`
version: '1'
vars:
  X: y
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ReadConfig(path)
	if err == nil {
		t.Fatalf("expected validation error for steps-less + tasks-less file, got nil")
	}
}

// TestReadConfig_TaskUnknownField verifies strict-mode rejects a
// typo'd field inside a task body — the same protection users get on
// top-level steps.
func TestReadConfig_TaskUnknownField(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "tasks.yml")
	if err := os.WriteFile(path, []byte(`
tasks:
  build:
    description: typo of "desc"
    steps:
      - shell: { cmd: "echo hi" }
`), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := ReadConfig(path)
	if err == nil {
		t.Fatalf("expected strict-mode error for unknown task field, got nil")
	}
}
