package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// writeFile is a test helper that writes the given content at path,
// creating parent dirs as needed. Fails the test on any I/O error so
// the table-driven cases below stay flat.
func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func TestDiscoverTasksConfig_PrefersTasksYml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.yml"), `
version: '1'
tasks:
  hello:
    steps:
      - shell: { cmd: "echo hi" }
`)
	writeFile(t, filepath.Join(dir, "mooncake.yml"), `
version: '1'
steps:
  - shell: { cmd: "echo from apply" }
`)

	path, shadowed, err := DiscoverTasksConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverTasksConfig: %v", err)
	}
	if filepath.Base(path) != "tasks.yml" {
		t.Errorf("got %q, want tasks.yml to win", path)
	}
	if shadowed != "" {
		t.Errorf("shadowed = %q, want empty (mooncake.yml has no tasks:)", shadowed)
	}
}

func TestDiscoverTasksConfig_ShadowWarning(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "tasks.yml"), `
version: '1'
tasks:
  hello:
    steps:
      - shell: { cmd: "echo hi" }
`)
	writeFile(t, filepath.Join(dir, "mooncake.yml"), `
version: '1'
tasks:
  shadowed:
    steps:
      - shell: { cmd: "echo shadowed" }
`)

	path, shadowed, err := DiscoverTasksConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverTasksConfig: %v", err)
	}
	if filepath.Base(path) != "tasks.yml" {
		t.Errorf("path = %q, want tasks.yml", path)
	}
	if filepath.Base(shadowed) != "mooncake.yml" {
		t.Errorf("shadowed = %q, want mooncake.yml (defines tasks: but is suppressed)", shadowed)
	}
}

func TestDiscoverTasksConfig_FallbackToMooncakeYml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "mooncake.yml"), `
version: '1'
tasks:
  hello:
    steps:
      - shell: { cmd: "echo hi" }
`)

	path, shadowed, err := DiscoverTasksConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverTasksConfig: %v", err)
	}
	if filepath.Base(path) != "mooncake.yml" {
		t.Errorf("got %q, want mooncake.yml", path)
	}
	if shadowed != "" {
		t.Errorf("shadowed = %q, want empty", shadowed)
	}
}

func TestDiscoverTasksConfig_NoTasksAnywhere(t *testing.T) {
	dir := t.TempDir()
	// mooncake.yml exists but has no tasks: — should not satisfy the
	// fallback. Discovery should error.
	writeFile(t, filepath.Join(dir, "mooncake.yml"), `
version: '1'
steps:
  - shell: { cmd: "echo hi" }
`)

	_, _, err := DiscoverTasksConfig(dir)
	var nfe *ErrNoConfigFound
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want *ErrNoConfigFound", err)
	}
}

func TestDiscoverTasksConfig_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	_, _, err := DiscoverTasksConfig(dir)
	var nfe *ErrNoConfigFound
	if !errors.As(err, &nfe) {
		t.Fatalf("err = %v, want *ErrNoConfigFound", err)
	}
}

func TestDiscoverTasksConfig_TasksYamlExtension(t *testing.T) {
	dir := t.TempDir()
	// Confirm .yaml also wins (parity with the .yml form).
	writeFile(t, filepath.Join(dir, "tasks.yaml"), `
version: '1'
tasks:
  hello:
    steps:
      - shell: { cmd: "echo hi" }
`)
	path, _, err := DiscoverTasksConfig(dir)
	if err != nil {
		t.Fatalf("DiscoverTasksConfig: %v", err)
	}
	if filepath.Base(path) != "tasks.yaml" {
		t.Errorf("got %q, want tasks.yaml", path)
	}
}
