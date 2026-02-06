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
				Name:  "Test Step",
				Print: &config.PrintAction{Msg: "test message"},
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
				Name:  "Test Step",
				Print: &config.PrintAction{Msg: "test message"},
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
				Name:  "First Step",
				Print: &config.PrintAction{Msg: "First message"},
			},
			{
				Name:  "Second Step",
				Print: &config.PrintAction{Msg: "Second message"},
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
