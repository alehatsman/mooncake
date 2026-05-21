package preset

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/presets"
)

// TestLocalComponent_FromPath covers the spec-67 local-component dispatch
// path: `use: ./components/foo.yml` loads a component from an explicit file
// instead of going through search paths.
func TestLocalComponent_FromPath(t *testing.T) {
	dir := t.TempDir()
	componentsDir := filepath.Join(dir, "components")
	if err := os.MkdirAll(componentsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	componentPath := filepath.Join(componentsDir, "setup-user.yml")
	content := `name: setup-user
props:
  username: { type: string, required: true }
steps:
  - name: greet
    log: "{{ props.username }}"
`
	if err := os.WriteFile(componentPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	name := "./components/setup-user.yml"
	props := map[string]interface{}{"username": "alice"}

	// Simulate the handler: detect Kind, resolve relative to playbook dir, expand.
	if config.ComponentRefKindOf(name) != config.ComponentRefLocalPath {
		t.Fatalf("Kind = %v, want LocalPath", config.ComponentRefKindOf(name))
	}
	absPath := filepath.Join(dir, name)
	steps, ns, baseDir, err := presets.ExpandPresetFromPath(name, props, absPath)
	if err != nil {
		t.Fatalf("ExpandPresetFromPath: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
	if baseDir != componentsDir {
		t.Errorf("baseDir = %q, want %q", baseDir, componentsDir)
	}
	gotProps := ns["props"].(map[string]interface{})
	if gotProps["username"] != "alice" {
		t.Errorf("props.username = %v, want alice", gotProps["username"])
	}
}

// TestLocalComponent_MissingFile verifies the spec's error message for a
// missing local component file.
func TestLocalComponent_MissingFile(t *testing.T) {
	_, _, _, err := presets.ExpandPresetFromPath("./components/missing.yml", nil, "/tmp/nonexistent/missing.yml")
	if err == nil {
		t.Fatal("expected error for missing component")
	}
	if !strings.Contains(err.Error(), "component not found") {
		t.Errorf("error = %q, want 'component not found'", err.Error())
	}
}

// TestLocalComponent_ValidationFailure verifies prop validation runs.
func TestLocalComponent_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	componentPath := filepath.Join(dir, "req.yml")
	content := `name: req
props:
  needed: { type: string, required: true }
steps:
  - name: noop
    log: "x"
`
	if err := os.WriteFile(componentPath, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err := presets.ExpandPresetFromPath("./req.yml", nil, componentPath)
	if err == nil {
		t.Fatal("expected error for missing required prop")
	}
	if !strings.Contains(err.Error(), "required parameter") {
		t.Errorf("error = %q, want 'required parameter' phrase", err.Error())
	}
}
