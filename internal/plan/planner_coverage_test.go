package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestRenderActionTemplates_AllActionTypes tests the renderActionTemplates function
// for all action types to improve coverage from 26.2% to 80%+
func TestRenderActionTemplates_AllActionTypes(t *testing.T) {
	tmpDir := t.TempDir()
	planner := NewPlanner()

	// Create a test file for template/copy/unarchive actions
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("test content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name   string
		step   config.Step
		vars   map[string]interface{}
		verify func(*testing.T, config.Step)
	}{
		{
			name: "shell action",
			step: config.Step{
				Shell: &config.ShellAction{
					Cmd: "echo {{ message }}",
				},
			},
			vars: map[string]interface{}{"message": "hello"},
			verify: func(t *testing.T, step config.Step) {
				if step.Shell.Cmd != "echo hello" {
					t.Errorf("Expected 'echo hello', got '%s'", step.Shell.Cmd)
				}
			},
		},
		{
			name: "file action with path and content",
			step: config.Step{
				File: &config.File{
					Path:    "{{ dir }}/file.txt",
					Content: "Content: {{ value }}",
					State:   "present",
				},
			},
			vars: map[string]interface{}{"dir": "/tmp", "value": "test"},
			verify: func(t *testing.T, step config.Step) {
				if step.File.Path != "/tmp/file.txt" {
					t.Errorf("Expected '/tmp/file.txt', got '%s'", step.File.Path)
				}
				if step.File.Content != "Content: test" {
					t.Errorf("Expected 'Content: test', got '%s'", step.File.Content)
				}
			},
		},
		{
			name: "file action with path only",
			step: config.Step{
				File: &config.File{
					Path:  "{{ dir }}/file2.txt",
					State: "absent",
				},
			},
			vars: map[string]interface{}{"dir": "/var"},
			verify: func(t *testing.T, step config.Step) {
				if step.File.Path != "/var/file2.txt" {
					t.Errorf("Expected '/var/file2.txt', got '%s'", step.File.Path)
				}
			},
		},
		{
			name: "template action with absolute path",
			step: config.Step{
				Template: &config.Template{
					Src:  "/absolute/{{ name }}.j2",
					Dest: "{{ output }}/result",
				},
			},
			vars: map[string]interface{}{"name": "template", "output": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				if step.Template.Src != "/absolute/template.j2" {
					t.Errorf("Expected '/absolute/template.j2', got '%s'", step.Template.Src)
				}
				if step.Template.Dest != "/tmp/result" {
					t.Errorf("Expected '/tmp/result', got '%s'", step.Template.Dest)
				}
			},
		},
		{
			name: "template action with relative path",
			step: config.Step{
				Template: &config.Template{
					Src:  "{{ name }}.j2",
					Dest: "{{ output }}/result",
				},
			},
			vars: map[string]interface{}{"name": "template", "output": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path based on tmpDir
				if !filepath.IsAbs(step.Template.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.Template.Src)
				}
				if step.Template.Dest != "/tmp/result" {
					t.Errorf("Expected '/tmp/result', got '%s'", step.Template.Dest)
				}
			},
		},
		{
			name: "copy action with absolute path",
			step: config.Step{
				Copy: &config.Copy{
					Src:  "/absolute/{{ file }}.txt",
					Dest: "{{ dst }}/copy.txt",
				},
			},
			vars: map[string]interface{}{"file": "source", "dst": "/dest"},
			verify: func(t *testing.T, step config.Step) {
				if step.Copy.Src != "/absolute/source.txt" {
					t.Errorf("Expected '/absolute/source.txt', got '%s'", step.Copy.Src)
				}
				if step.Copy.Dest != "/dest/copy.txt" {
					t.Errorf("Expected '/dest/copy.txt', got '%s'", step.Copy.Dest)
				}
			},
		},
		{
			name: "copy action with relative path",
			step: config.Step{
				Copy: &config.Copy{
					Src:  "{{ file }}.txt",
					Dest: "{{ dst }}/copy.txt",
				},
			},
			vars: map[string]interface{}{"file": "source", "dst": "/dest"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path
				if !filepath.IsAbs(step.Copy.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.Copy.Src)
				}
				if step.Copy.Dest != "/dest/copy.txt" {
					t.Errorf("Expected '/dest/copy.txt', got '%s'", step.Copy.Dest)
				}
			},
		},
		{
			name: "unarchive action with absolute path",
			step: config.Step{
				Unarchive: &config.Unarchive{
					Src:  "/archive/{{ name }}.tar.gz",
					Dest: "{{ extract }}/files",
				},
			},
			vars: map[string]interface{}{"name": "backup", "extract": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				if step.Unarchive.Src != "/archive/backup.tar.gz" {
					t.Errorf("Expected '/archive/backup.tar.gz', got '%s'", step.Unarchive.Src)
				}
				if step.Unarchive.Dest != "/tmp/files" {
					t.Errorf("Expected '/tmp/files', got '%s'", step.Unarchive.Dest)
				}
			},
		},
		{
			name: "unarchive action with relative path",
			step: config.Step{
				Unarchive: &config.Unarchive{
					Src:  "{{ name }}.tar.gz",
					Dest: "{{ extract }}/files",
				},
			},
			vars: map[string]interface{}{"name": "backup", "extract": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path
				if !filepath.IsAbs(step.Unarchive.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.Unarchive.Src)
				}
				if step.Unarchive.Dest != "/tmp/files" {
					t.Errorf("Expected '/tmp/files', got '%s'", step.Unarchive.Dest)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ExpansionContext{
				Variables:  tt.vars,
				CurrentDir: tmpDir,
			}

			// Make a copy of the step to avoid modifying the test case
			stepCopy := tt.step

			err := planner.renderActionTemplates(&stepCopy, ctx)
			if err != nil {
				t.Fatalf("renderActionTemplates failed: %v", err)
			}

			tt.verify(t, stepCopy)
		})
	}
}


// TestConvertToSliceExtended tests additional edge cases for convertToSlice
func TestConvertToSliceExtended(t *testing.T) {
	tests := []struct {
		name        string
		input       interface{}
		expr        string
		expectError bool
		expectLen   int
	}{
		{
			name:        "not a slice - map",
			input:       map[string]string{"key": "value"},
			expr:        "items",
			expectError: true,
		},
		{
			name:        "not a slice - bool",
			input:       true,
			expr:        "items",
			expectError: true,
		},
		{
			name:        "not a slice - float",
			input:       3.14,
			expr:        "items",
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := convertToSlice(tt.input, tt.expr)

			if tt.expectError {
				if err == nil {
					t.Fatal("Expected error, got nil")
				}
				if !contains(err.Error(), "not a list") {
					t.Errorf("Expected error to contain 'not a list', got: %v", err)
				}
			} else {
				if err != nil {
					t.Fatalf("Unexpected error: %v", err)
				}
				if len(result) != tt.expectLen {
					t.Errorf("Expected length %d, got %d", tt.expectLen, len(result))
				}
			}
		})
	}
}

// TestExpandStep_ErrorPath tests error handling in expandStep for better coverage
func TestExpandStep_VarsWithWhen(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	configContent := `version: "1.0"
vars:
  env: prod

steps:
  - vars:
      debug: false
    when: env == "dev"

  - name: Production step
    shell: echo "prod"
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Should skip the vars step because when condition is false
	// Should only have the production step
	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].Name != "Production step" {
		t.Errorf("Expected 'Production step', got '%s'", plan.Steps[0].Name)
	}
}

// TestExpandStep_IncludeVarsWithWhen tests include_vars with when condition
func TestExpandStep_IncludeVarsWithWhen(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")
	varsPath := filepath.Join(tmpDir, "vars.yml")

	varsContent := `extra: value`
	err := os.WriteFile(varsPath, []byte(varsContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write vars file: %v", err)
	}

	configContent := `version: "1.0"
vars:
  load_vars: false

steps:
  - include_vars: vars.yml
    when: load_vars == true

  - name: Test step
    shell: echo "test"
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	planner := NewPlanner()
	plan, err := planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
		Variables:  nil,
		Tags:       nil,
	})

	if err != nil {
		t.Fatalf("Failed to build plan: %v", err)
	}

	// Should skip include_vars and only have test step
	if len(plan.Steps) != 1 {
		t.Fatalf("Expected 1 step, got %d", len(plan.Steps))
	}

	if plan.Steps[0].Name != "Test step" {
		t.Errorf("Expected 'Test step', got '%s'", plan.Steps[0].Name)
	}
}

// TestSavePlanToFile_ErrorCases tests error handling in SavePlanToFile
func TestSavePlanToFile_ErrorCases(t *testing.T) {
	// Try to save to a directory that doesn't exist
	err := SavePlanToFile(&Plan{Steps: []config.Step{}}, "/nonexistent/path/plan.json")
	if err == nil {
		t.Fatal("Expected error when saving to non-existent directory, got nil")
	}
}

// TestReadRunConfig_MissingFile tests error handling when config file is missing
func TestReadRunConfig_MissingFile(t *testing.T) {
	planner := NewPlanner()
	_, err := planner.readRunConfig("/nonexistent/config.yml")
	if err == nil {
		t.Fatal("Expected error for missing config file, got nil")
	}
}

// TestReadRunConfig_InvalidYAML tests error handling for invalid YAML
func TestReadRunConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "invalid.yml")

	// Write invalid YAML
	invalidContent := `
version: "1.0"
steps:
  - name: Invalid
    shell: "command"
    invalid_field: [unclosed
`
	err := os.WriteFile(configPath, []byte(invalidContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write invalid config: %v", err)
	}

	planner := NewPlanner()
	_, err = planner.readRunConfig(configPath)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

// TestBuildPlan_ExpandStepsError tests error propagation in BuildPlan
func TestBuildPlan_ExpandStepsError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "test.yml")

	// Config with include that references a non-existent file
	configContent := `version: "1.0"
steps:
  - include: missing.yml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	planner := NewPlanner()
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
	})

	if err == nil {
		t.Fatal("Expected error from expandSteps, got nil")
	}
}
