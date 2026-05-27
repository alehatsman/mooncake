package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestSavePlanToFile_JSON tests saving plan to JSON format
func TestSavePlanToFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.json")

	// Create a test plan
	plan := &Plan{
		RootFile: "test.yml",
		Steps: []config.Step{
			{
				Name: "Test Step",
				Log:  &config.PrintAction{Msg: "test message"},
			},
		},
	}

	// Save to file
	err := SavePlanToFile(plan, filePath)
	if err != nil {
		t.Fatalf("SavePlanToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Plan file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read plan file: %v", err)
	}

	// Verify JSON format
	contentStr := string(content)
	if !strings.Contains(contentStr, "root_file") && !strings.Contains(contentStr, "RootFile") {
		t.Error("Expected JSON format with root_file field")
	}
	if !strings.Contains(contentStr, "test.yml") {
		t.Error("Expected RootFile value in content")
	}
}

// TestSavePlanToFile_YAML tests saving plan to YAML format
func TestSavePlanToFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.yaml")

	// Create a test plan
	plan := &Plan{
		RootFile: "test.yml",
		Steps: []config.Step{
			{
				Name: "Test Step",
				Log:  &config.PrintAction{Msg: "test message"},
			},
		},
	}

	// Save to file
	err := SavePlanToFile(plan, filePath)
	if err != nil {
		t.Fatalf("SavePlanToFile failed: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Plan file was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read plan file: %v", err)
	}

	// Verify YAML format
	contentStr := string(content)
	if !strings.Contains(contentStr, "root_file:") && !strings.Contains(contentStr, "RootFile:") {
		t.Error("Expected YAML format with root_file field")
	}
	if !strings.Contains(contentStr, "test.yml") {
		t.Error("Expected RootFile value in content")
	}
}

// TestSavePlanToFile_YML tests saving plan to .yml format
func TestSavePlanToFile_YML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.yml")

	// Create a test plan
	plan := &Plan{
		RootFile: "test.yml",
		Steps:    []config.Step{},
	}

	// Save to file
	err := SavePlanToFile(plan, filePath)
	if err != nil {
		t.Fatalf("SavePlanToFile failed for .yml: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("Plan file was not created")
	}
}

// TestSavePlanToFile_UnsupportedFormat tests unsupported format error
func TestSavePlanToFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.txt")

	plan := &Plan{
		RootFile: "test.yml",
		Steps:    []config.Step{},
	}

	err := SavePlanToFile(plan, filePath)
	if err == nil {
		t.Error("SavePlanToFile should fail for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("Expected 'unsupported file format' error, got: %v", err)
	}
}

// TestSavePlanToFile_InvalidPath tests invalid path error
func TestSavePlanToFile_InvalidPath(t *testing.T) {
	// Use a path that can't be created
	filePath := "/nonexistent/directory/plan.json"

	plan := &Plan{
		RootFile: "test.yml",
		Steps:    []config.Step{},
	}

	err := SavePlanToFile(plan, filePath)
	if err == nil {
		t.Error("SavePlanToFile should fail for invalid path")
	}
	if !strings.Contains(err.Error(), "failed to create") {
		t.Errorf("Expected 'failed to create' error, got: %v", err)
	}
}

// TestLoadPlanFromFile_JSON tests loading plan from JSON format
func TestLoadPlanFromFile_JSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.json")

	// Create a JSON plan file
	jsonContent := `{
  "root_file": "test.yml",
  "steps": [
    {
      "name": "Test Step"
    }
  ]
}`
	if err := os.WriteFile(filePath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load plan
	plan, err := LoadPlanFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadPlanFromFile failed: %v", err)
	}

	// Verify plan content
	if plan.RootFile != "test.yml" {
		t.Errorf("Expected RootFile 'test.yml', got %s", plan.RootFile)
	}
	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}
	if plan.Steps[0].Name != "Test Step" {
		t.Errorf("Expected step name 'Test Step', got %s", plan.Steps[0].Name)
	}
}

// TestLoadPlanFromFile_YAML tests loading plan from YAML format
func TestLoadPlanFromFile_YAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.yaml")

	// Create a YAML plan file
	yamlContent := `root_file: test.yml
steps:
  - name: Test Step
`
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load plan
	plan, err := LoadPlanFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadPlanFromFile failed: %v", err)
	}

	// Verify plan content
	if plan.RootFile != "test.yml" {
		t.Errorf("Expected RootFile 'test.yml', got %s", plan.RootFile)
	}
	if len(plan.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(plan.Steps))
	}
}

// TestLoadPlanFromFile_YML tests loading plan from .yml format
func TestLoadPlanFromFile_YML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.yml")

	// Create a YAML plan file
	yamlContent := `root_file: test.yml
steps: []
`
	if err := os.WriteFile(filePath, []byte(yamlContent), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Load plan
	plan, err := LoadPlanFromFile(filePath)
	if err != nil {
		t.Fatalf("LoadPlanFromFile failed for .yml: %v", err)
	}

	// Verify plan loaded
	if plan.RootFile != "test.yml" {
		t.Errorf("Expected RootFile 'test.yml', got %s", plan.RootFile)
	}
}

// TestLoadPlanFromFile_FileNotFound tests file not found error
func TestLoadPlanFromFile_FileNotFound(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.json")

	_, err := LoadPlanFromFile(filePath)
	if err == nil {
		t.Error("LoadPlanFromFile should fail for non-existent file")
	}
	if !strings.Contains(err.Error(), "failed to read") {
		t.Errorf("Expected 'failed to read' error, got: %v", err)
	}
}

// TestLoadPlanFromFile_InvalidJSON tests invalid JSON error
func TestLoadPlanFromFile_InvalidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.json")

	// Create invalid JSON
	invalidJSON := `{
  "root_file": "test.yml",
  "steps": [unclosed
`
	if err := os.WriteFile(filePath, []byte(invalidJSON), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := LoadPlanFromFile(filePath)
	if err == nil {
		t.Error("LoadPlanFromFile should fail for invalid JSON")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("Expected 'failed to decode' error, got: %v", err)
	}
}

// TestLoadPlanFromFile_InvalidYAML tests invalid YAML error
func TestLoadPlanFromFile_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "invalid.yaml")

	// Create invalid YAML
	invalidYAML := `root_file: test.yml
steps:
  - name: [unclosed
`
	if err := os.WriteFile(filePath, []byte(invalidYAML), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := LoadPlanFromFile(filePath)
	if err == nil {
		t.Error("LoadPlanFromFile should fail for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to decode") {
		t.Errorf("Expected 'failed to decode' error, got: %v", err)
	}
}

// TestLoadPlanFromFile_UnsupportedFormat tests unsupported format error
func TestLoadPlanFromFile_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "plan.txt")

	// Create a file with unsupported format
	if err := os.WriteFile(filePath, []byte("some content"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	_, err := LoadPlanFromFile(filePath)
	if err == nil {
		t.Error("LoadPlanFromFile should fail for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported file format") {
		t.Errorf("Expected 'unsupported file format' error, got: %v", err)
	}
}

// TestSaveAndLoadRoundTrip tests saving and loading the same plan
func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmpDir := t.TempDir()

	// Original plan
	originalPlan := &Plan{
		RootFile: "test.yml",
		Steps: []config.Step{
			{
				Name: "First Step",
				Log:  &config.PrintAction{Msg: "First message"},
			},
			{
				Name: "Second Step",
				Log:  &config.PrintAction{Msg: "Second message"},
			},
		},
	}

	// Test JSON round-trip
	jsonPath := filepath.Join(tmpDir, "plan.json")
	if err := SavePlanToFile(originalPlan, jsonPath); err != nil {
		t.Fatalf("SavePlanToFile (JSON) failed: %v", err)
	}

	loadedJSON, err := LoadPlanFromFile(jsonPath)
	if err != nil {
		t.Fatalf("LoadPlanFromFile (JSON) failed: %v", err)
	}

	if loadedJSON.RootFile != originalPlan.RootFile {
		t.Errorf("JSON round-trip: RootFile mismatch")
	}
	if len(loadedJSON.Steps) != len(originalPlan.Steps) {
		t.Errorf("JSON round-trip: Steps count mismatch")
	}

	// Test YAML round-trip
	yamlPath := filepath.Join(tmpDir, "plan.yaml")
	if err := SavePlanToFile(originalPlan, yamlPath); err != nil {
		t.Fatalf("SavePlanToFile (YAML) failed: %v", err)
	}

	loadedYAML, err := LoadPlanFromFile(yamlPath)
	if err != nil {
		t.Fatalf("LoadPlanFromFile (YAML) failed: %v", err)
	}

	if loadedYAML.RootFile != originalPlan.RootFile {
		t.Errorf("YAML round-trip: RootFile mismatch")
	}
	if len(loadedYAML.Steps) != len(originalPlan.Steps) {
		t.Errorf("YAML round-trip: Steps count mismatch")
	}
}

// TestF056_SavePlanToFile_Perms0o600 pins F056: plan files must be
// written at 0o600 (owner-read/write only), not the umask-default
// 0o644. Plans carry secret refs + the full playbook structure
// (every package, service, sysctl, ssh_key targeted on the host);
// on a multi-user host or a packaged-product worker that other
// services can read(2), 0o644 leaks recon to anyone with shell
// access. F037 closed this for pilot saved plans; F056 closes it
// for the SavePlanToFile path.
func TestF056_SavePlanToFile_Perms0o600(t *testing.T) {
	tmpDir := t.TempDir()
	p := &Plan{Version: "1.0", RootFile: "fixture.yml"}

	for _, ext := range []string{"json", "yaml"} {
		t.Run(ext, func(t *testing.T) {
			path := filepath.Join(tmpDir, "plan."+ext)
			if err := SavePlanToFile(p, path); err != nil {
				t.Fatalf("SavePlanToFile: %v", err)
			}
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			got := info.Mode().Perm()
			if got != 0o600 {
				t.Errorf("file mode = %v, want 0o600", got)
			}
		})
	}
}

// TestF056_SavePlanToFile_AtomicOnFailure: a failure mid-write must
// not corrupt an existing plan at the destination. The pre-cleanup
// path called os.Create(filePath) directly → a write that errored
// halfway left a partial/empty file at the destination. The post-
// cleanup path writes to <path>.tmp and renames; a write failure
// leaves the original file untouched and the tmp file cleaned up.
//
// We can't easily inject a Write failure from outside, so we test
// the success-path invariant: the .tmp file is gone after a
// successful save (cleaned up by rename moving it out).
func TestF056_SavePlanToFile_AtomicCleanup(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.json")
	if err := SavePlanToFile(&Plan{Version: "1.0"}, path); err != nil {
		t.Fatalf("SavePlanToFile: %v", err)
	}
	// No <path>.tmp orphan after a successful write.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected %s.tmp to be cleaned up; stat err = %v", path, err)
	}
}

// TestLoadPlanFromFile_RejectsUnknownYAMLFields covers the strict-
// decode change: pre-fix, yaml.Unmarshal silently dropped unknown
// fields. A user-edited plan with a typo'd field (e.g. `step:`
// instead of `steps:`) decoded into an empty Steps slice and the
// apply proceeded as a no-op — surprising and indistinguishable
// from "I told it to do nothing." Post-fix, the decode errors and
// the operator hears about the typo.
func TestLoadPlanFromFile_RejectsUnknownYAMLFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.yaml")
	// `step` instead of `steps` — the common typo.
	if err := os.WriteFile(path, []byte("version: \"1.0\"\nstep:\n  - { name: oops }\n"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := LoadPlanFromFile(path); err == nil {
		t.Fatal("expected error decoding plan with unknown YAML field; got nil")
	}
}

// TestLoadPlanFromFile_RejectsUnknownJSONFields covers the JSON
// branch of the same strict-decode change.
func TestLoadPlanFromFile_RejectsUnknownJSONFields(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "plan.json")
	if err := os.WriteFile(path, []byte(`{"version":"1.0","step":[{"name":"oops"}]}`), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	if _, err := LoadPlanFromFile(path); err == nil {
		t.Fatal("expected error decoding plan with unknown JSON field; got nil")
	}
}
