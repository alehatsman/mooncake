package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/template"
)

// mustNewRenderer creates a renderer or panics
func mustNewRenderer() template.Renderer {
	r, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return r
}

// TestRenderActionTemplates_AllActionTypes tests the renderActionTemplates function
// for all action types to improve coverage from 26.2% to 80%+
func TestRenderActionTemplates_AllActionTypes(t *testing.T) {
	tmpDir := t.TempDir()
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}

	// Create a test file for template/copy/unarchive actions
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test content"), 0644)
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
				FileWrite: &config.File{
					Path:    "{{ dir }}/file.txt",
					Content: "Content: {{ value }}",
					State:   "present",
				},
			},
			vars: map[string]interface{}{"dir": "/tmp", "value": "test"},
			verify: func(t *testing.T, step config.Step) {
				if step.FileWrite.Path != "/tmp/file.txt" {
					t.Errorf("Expected '/tmp/file.txt', got '%s'", step.FileWrite.Path)
				}
				if step.FileWrite.Content != "Content: test" {
					t.Errorf("Expected 'Content: test', got '%s'", step.FileWrite.Content)
				}
			},
		},
		{
			name: "file action with path only",
			step: config.Step{
				FileWrite: &config.File{
					Path:  "{{ dir }}/file2.txt",
					State: "absent",
				},
			},
			vars: map[string]interface{}{"dir": "/var"},
			verify: func(t *testing.T, step config.Step) {
				if step.FileWrite.Path != "/var/file2.txt" {
					t.Errorf("Expected '/var/file2.txt', got '%s'", step.FileWrite.Path)
				}
			},
		},
		{
			name: "template action with absolute path",
			step: config.Step{
				FileTemplate: &config.Template{
					Src:  "/absolute/{{ name }}.j2",
					Dest: "{{ output }}/result",
				},
			},
			vars: map[string]interface{}{"name": "template", "output": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				if step.FileTemplate.Src != "/absolute/template.j2" {
					t.Errorf("Expected '/absolute/template.j2', got '%s'", step.FileTemplate.Src)
				}
				if step.FileTemplate.Dest != "/tmp/result" {
					t.Errorf("Expected '/tmp/result', got '%s'", step.FileTemplate.Dest)
				}
			},
		},
		{
			name: "template action with relative path",
			step: config.Step{
				FileTemplate: &config.Template{
					Src:  "{{ name }}.j2",
					Dest: "{{ output }}/result",
				},
			},
			vars: map[string]interface{}{"name": "template", "output": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path based on tmpDir
				if !filepath.IsAbs(step.FileTemplate.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.FileTemplate.Src)
				}
				if step.FileTemplate.Dest != "/tmp/result" {
					t.Errorf("Expected '/tmp/result', got '%s'", step.FileTemplate.Dest)
				}
			},
		},
		{
			name: "copy action with absolute path",
			step: config.Step{
				FileCopy: &config.Copy{
					Src:  "/absolute/{{ file }}.txt",
					Dest: "{{ dst }}/copy.txt",
				},
			},
			vars: map[string]interface{}{"file": "source", "dst": "/dest"},
			verify: func(t *testing.T, step config.Step) {
				if step.FileCopy.Src != "/absolute/source.txt" {
					t.Errorf("Expected '/absolute/source.txt', got '%s'", step.FileCopy.Src)
				}
				if step.FileCopy.Dest != "/dest/copy.txt" {
					t.Errorf("Expected '/dest/copy.txt', got '%s'", step.FileCopy.Dest)
				}
			},
		},
		{
			name: "copy action with relative path",
			step: config.Step{
				FileCopy: &config.Copy{
					Src:  "{{ file }}.txt",
					Dest: "{{ dst }}/copy.txt",
				},
			},
			vars: map[string]interface{}{"file": "source", "dst": "/dest"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path
				if !filepath.IsAbs(step.FileCopy.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.FileCopy.Src)
				}
				if step.FileCopy.Dest != "/dest/copy.txt" {
					t.Errorf("Expected '/dest/copy.txt', got '%s'", step.FileCopy.Dest)
				}
			},
		},
		{
			name: "unarchive action with absolute path",
			step: config.Step{
				FileUnarchive: &config.Unarchive{
					Src:  "/archive/{{ name }}.tar.gz",
					Dest: "{{ extract }}/files",
				},
			},
			vars: map[string]interface{}{"name": "backup", "extract": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				if step.FileUnarchive.Src != "/archive/backup.tar.gz" {
					t.Errorf("Expected '/archive/backup.tar.gz', got '%s'", step.FileUnarchive.Src)
				}
				if step.FileUnarchive.Dest != "/tmp/files" {
					t.Errorf("Expected '/tmp/files', got '%s'", step.FileUnarchive.Dest)
				}
			},
		},
		{
			name: "unarchive action with relative path",
			step: config.Step{
				FileUnarchive: &config.Unarchive{
					Src:  "{{ name }}.tar.gz",
					Dest: "{{ extract }}/files",
				},
			},
			vars: map[string]interface{}{"name": "backup", "extract": "/tmp"},
			verify: func(t *testing.T, step config.Step) {
				// Should be resolved to absolute path
				if !filepath.IsAbs(step.FileUnarchive.Src) {
					t.Errorf("Expected absolute path, got relative: '%s'", step.FileUnarchive.Src)
				}
				if step.FileUnarchive.Dest != "/tmp/files" {
					t.Errorf("Expected '/tmp/files', got '%s'", step.FileUnarchive.Dest)
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

// TestRenderActionTemplates_UseAndProps verifies that the spec-67 `use:`
// string action and its sibling `props:` map both get template-rendered at
// plan time. Regression: skipping non-pointer action fields used to leave
// {{ }} expressions in props unrendered, so component-side enum/required
// checks at apply time would see literal templates instead of values.
func TestRenderActionTemplates_UseAndProps(t *testing.T) {
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Use: "{{ component_path }}",
		Props: map[string]interface{}{
			"variant":  "{{ palette_variant }}",
			"port":     "{{ port }}",
			"literal":  "no_template_here",
			"nested":   map[string]interface{}{"inner": "{{ inner }}"},
			"list":     []interface{}{"{{ first }}", "second"},
			"int_pass": 42,
		},
	}
	ctx := &ExpansionContext{
		Variables: map[string]interface{}{
			"component_path":  "./components/palette/index.yml",
			"palette_variant": "monokai_dark",
			"port":            "5432",
			"inner":           "deep",
			"first":           "alpha",
		},
	}

	if err := planner.renderActionTemplates(&step, ctx); err != nil {
		t.Fatalf("renderActionTemplates: %v", err)
	}
	if step.Use != "./components/palette/index.yml" {
		t.Errorf("Use = %q, want rendered path", step.Use)
	}
	if got := step.Props["variant"]; got != "monokai_dark" {
		t.Errorf("Props[variant] = %v, want monokai_dark", got)
	}
	if got := step.Props["port"]; got != "5432" {
		t.Errorf("Props[port] = %v, want 5432", got)
	}
	if got := step.Props["literal"]; got != "no_template_here" {
		t.Errorf("Props[literal] = %v, want passthrough", got)
	}
	nested, ok := step.Props["nested"].(map[string]interface{})
	if !ok {
		t.Fatalf("Props[nested] not a map: %T", step.Props["nested"])
	}
	if got := nested["inner"]; got != "deep" {
		t.Errorf("Props[nested][inner] = %v, want deep", got)
	}
	list, ok := step.Props["list"].([]interface{})
	if !ok {
		t.Fatalf("Props[list] not a slice: %T", step.Props["list"])
	}
	if list[0] != "alpha" || list[1] != "second" {
		t.Errorf("Props[list] = %v, want [alpha second]", list)
	}
	if got := step.Props["int_pass"]; got != 42 {
		t.Errorf("Props[int_pass] = %v, want 42 (non-string passthrough)", got)
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

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
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
  - vars.load: vars.yml
    when: load_vars == true

  - name: Test step
    shell: echo "test"
`
	err = os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
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
	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
	_, err = planner.readRunConfig("/nonexistent/config.yml")
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

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
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
  - import: missing.yml
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	if err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("Failed to create planner: %v", err)
	}
	_, err = planner.BuildPlan(PlannerConfig{
		ConfigPath: configPath,
	})

	if err == nil {
		t.Fatal("Expected error from expandSteps, got nil")
	}
}
