package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestYAMLReader_ReadConfig(t *testing.T) {
	reader := NewYAMLConfigReader()

	t.Run("valid config", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: test step
  shell: echo hello

- name: create file
  file.write:
    path: /tmp/test.txt
    state: present
`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		steps := parsedConfig.Steps
		if len(steps) != 2 {
			t.Errorf("ReadConfig() got %d steps, want 2", len(steps))
		}

		// Verify first step
		if steps[0].Name != "test step" {
			t.Errorf("step[0].Name = %q, want 'test step'", steps[0].Name)
		}
		if steps[0].Shell == nil || steps[0].Shell.Cmd != "echo hello" {
			t.Error("step[0].Shell not correctly parsed")
		}

		// Verify second step
		if steps[1].Name != "create file" {
			t.Errorf("step[1].Name = %q, want 'create file'", steps[1].Name)
		}
		if steps[1].FileWrite == nil {
			t.Error("step[1].File should not be nil")
		}
	})

	t.Run("empty config", func(t *testing.T) {
		tmpFile := createTempYAML(t, "[]")
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(parsedConfig.Steps) != 0 {
			t.Errorf("ReadConfig() got %d steps, want 0", len(parsedConfig.Steps))
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpFile := createTempYAML(t, "invalid: yaml: syntax:")
		defer os.Remove(tmpFile)

		_, err := reader.ReadConfig(tmpFile)
		if err == nil {
			t.Error("ReadConfig() should return error for invalid YAML")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := reader.ReadConfig("/nonexistent/file.yml")
		if err == nil {
			t.Error("ReadConfig() should return error for nonexistent file")
		}
	})

	t.Run("config with template", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: render template
  file.template:
    src: /tmp/template.j2
    dest: /tmp/output.txt
`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		steps := parsedConfig.Steps
		if len(steps) != 1 {
			t.Errorf("ReadConfig() got %d steps, want 1", len(steps))
		}

		if steps[0].FileTemplate == nil {
			t.Error("step[0].Template should not be nil")
		}
	})

	t.Run("config with when condition", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: conditional step
  shell: echo test
  when: os == 'linux'
`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		steps := parsedConfig.Steps
		if steps[0].When != "os == 'linux'" {
			t.Errorf("step[0].When = %q, want \"os == 'linux'\"", steps[0].When)
		}
	})
}

func TestYAMLReader_ReadVariables(t *testing.T) {
	reader := NewYAMLConfigReader()

	t.Run("valid variables", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
name: test
version: 1.0
debug: true
count: 42
`)
		defer os.Remove(tmpFile)

		vars, err := reader.ReadVariables(tmpFile)
		if err != nil {
			t.Fatalf("ReadVariables() error = %v", err)
		}

		if vars["name"] != "test" {
			t.Errorf("vars['name'] = %v, want 'test'", vars["name"])
		}
		if vars["version"] != 1.0 {
			t.Errorf("vars['version'] = %v, want 1.0", vars["version"])
		}
		if vars["debug"] != true {
			t.Errorf("vars['debug'] = %v, want true", vars["debug"])
		}
		if vars["count"] != 42 {
			t.Errorf("vars['count'] = %v, want 42", vars["count"])
		}
	})

	t.Run("empty path returns empty map", func(t *testing.T) {
		vars, err := reader.ReadVariables("")
		if err != nil {
			t.Fatalf("ReadVariables() error = %v", err)
		}

		if len(vars) != 0 {
			t.Errorf("ReadVariables() got %d vars, want 0", len(vars))
		}
	})

	t.Run("empty file", func(t *testing.T) {
		tmpFile := createTempYAML(t, "")
		defer os.Remove(tmpFile)

		_, err := reader.ReadVariables(tmpFile)
		// Empty YAML should not error, just return empty map or nil
		if err != nil {
			// Some YAML parsers might return error, some might return empty
			// Either is acceptable
			return
		}
	})

	t.Run("invalid yaml", func(t *testing.T) {
		tmpFile := createTempYAML(t, "invalid: yaml: syntax:")
		defer os.Remove(tmpFile)

		_, err := reader.ReadVariables(tmpFile)
		if err == nil {
			t.Error("ReadVariables() should return error for invalid YAML")
		}
	})

	t.Run("nonexistent file", func(t *testing.T) {
		_, err := reader.ReadVariables("/nonexistent/vars.yml")
		if err == nil {
			t.Error("ReadVariables() should return error for nonexistent file")
		}
	})

	t.Run("nested variables", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
user:
  name: alice
  age: 30
  settings:
    theme: dark
`)
		defer os.Remove(tmpFile)

		vars, err := reader.ReadVariables(tmpFile)
		if err != nil {
			t.Fatalf("ReadVariables() error = %v", err)
		}

		user, ok := vars["user"].(map[string]interface{})
		if !ok {
			t.Fatal("vars['user'] should be a map")
		}

		if user["name"] != "alice" {
			t.Errorf("user.name = %v, want 'alice'", user["name"])
		}
	})
}

func TestNewYAMLConfigReader(t *testing.T) {
	reader := NewYAMLConfigReader()
	if reader == nil {
		t.Error("NewYAMLConfigReader() returned nil")
	}

	// Verify it implements Reader interface
	var _ Reader = reader
}

func TestPackageLevelFunctions(t *testing.T) {
	// Test backward-compatible package-level functions

	t.Run("ReadConfig package function", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: test
  shell: echo test
`)
		defer os.Remove(tmpFile)

		parsedConfig, err := ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(parsedConfig.Steps) != 1 {
			t.Errorf("ReadConfig() got %d steps, want 1", len(parsedConfig.Steps))
		}
	})

	t.Run("ReadVariables package function", func(t *testing.T) {
		tmpFile := createTempYAML(t, "test: value")
		defer os.Remove(tmpFile)

		vars, err := ReadVariables(tmpFile)
		if err != nil {
			t.Fatalf("ReadVariables() error = %v", err)
		}

		if vars["test"] != "value" {
			t.Errorf("ReadVariables() test = %v, want 'value'", vars["test"])
		}
	})

	t.Run("ReadConfigWithValidation package function - valid", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: test step
  shell: echo hello
`)
		defer os.Remove(tmpFile)

		parsedConfig, diagnostics, err := ReadConfigWithValidation(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfigWithValidation() error = %v", err)
		}

		if len(parsedConfig.Steps) != 1 {
			t.Errorf("ReadConfigWithValidation() got %d steps, want 1", len(parsedConfig.Steps))
		}

		if len(diagnostics) != 0 {
			t.Errorf("ReadConfigWithValidation() got %d diagnostics, want 0", len(diagnostics))
		}
	})

	t.Run("ReadConfigWithValidation package function - invalid", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: invalid step
  shell: echo hello
  file.write:
    path: /tmp/test
`)
		defer os.Remove(tmpFile)

		_, diagnostics, err := ReadConfigWithValidation(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfigWithValidation() error = %v", err)
		}

		// Should have diagnostics for multiple actions
		if len(diagnostics) == 0 {
			t.Error("ReadConfigWithValidation() should return diagnostics for invalid config")
		}
	})

	t.Run("ReadConfigWithValidation package function - missing action", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: no action step
  when: os == 'linux'
`)
		defer os.Remove(tmpFile)

		_, diagnostics, err := ReadConfigWithValidation(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfigWithValidation() error = %v", err)
		}

		// Should have diagnostics for missing action
		if len(diagnostics) == 0 {
			t.Error("ReadConfigWithValidation() should return diagnostics for step with no action")
		}
	})
}

func TestYAMLReader_ReadConfigComplexSteps(t *testing.T) {
	reader := NewYAMLConfigReader()

	tmpFile := createTempYAML(t, `
- name: create directory
  file.write:
    path: /tmp/testdir
    state: directory
    mode: "0755"

- name: template file
  file.template:
    src: template.j2
    dest: /tmp/output.txt
    mode: "0644"
    vars:
      key: value

- name: run with sudo
  shell: apt-get update
  as_user: root

- name: include other file
  import: other.yml

- name: loop over files
  file.write:
    path: "{{ item.path }}"
    state: present
  for_each_file: /tmp/files

- name: set variables
  vars:
    env: production
    version: 2.0
`)
	defer os.Remove(tmpFile)

	parsedConfig, err := reader.ReadConfig(tmpFile)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	steps := parsedConfig.Steps
	if len(steps) != 6 {
		t.Errorf("ReadConfig() got %d steps, want 6", len(steps))
	}

	// Verify file step with mode
	if steps[0].FileWrite != nil && steps[0].FileWrite.Mode != "0755" {
		t.Errorf("file mode = %q, want '0755'", steps[0].FileWrite.Mode)
	}

	// Verify as_user was parsed correctly. (Don't call ShouldBecome here:
	// it now returns false when the test process is already running as
	// root targeting root — that runtime short-circuit is tested separately.)
	if steps[2].AsUser != "root" {
		t.Errorf("step[2].AsUser = %q, want %q", steps[2].AsUser, "root")
	}

	// Verify include
	if steps[3].Import == nil {
		t.Error("include step should have Include field set")
	}

	// Verify with_filetree
	if steps[4].ForEachFile == nil {
		t.Error("with_filetree step should have WithFileTree field set")
	}

	// Verify vars
	if steps[5].Vars == nil {
		t.Error("vars step should have Vars field set")
	}
}

// MT-73: empty / whitespace-only config files must produce a
// human-readable error, not the raw "EOF" string the yaml decoder
// returns. Operators new to the tool need to see what shape was
// expected.
func TestReadConfigWithValidation_EmptyFileGivesClearError(t *testing.T) {
	for name, content := range map[string]string{
		"truly empty":           "",
		"newlines only":         "\n\n\n",
		"spaces and newlines":   "   \n   \n",
	} {
		t.Run(name, func(t *testing.T) {
			tmpFile := createTempYAML(t, content)
			defer os.Remove(tmpFile)

			_, _, err := ReadConfigWithValidation(tmpFile)
			if err == nil {
				t.Fatal("expected error for empty config")
			}
			msg := err.Error()
			if !strings.Contains(msg, "empty") {
				t.Errorf("error should mention 'empty', got: %s", msg)
			}
			if !strings.Contains(msg, tmpFile) {
				t.Errorf("error should cite the config path, got: %s", msg)
			}
			// Sanity: the raw "EOF" leak from MT-73's original report
			// must NOT be the whole error any more.
			if strings.TrimSpace(msg) == "EOF" {
				t.Errorf("error regressed to raw EOF: %s", msg)
			}
		})
	}
}

// Helper function to create temporary YAML file
func createTempYAML(t *testing.T, content string) string {
	t.Helper()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yml")

	if err := os.WriteFile(tmpFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}

	return tmpFile
}

func TestYAMLReader_ReadConfig_RunConfigFormat(t *testing.T) {
	tests := []struct {
		name          string
		yamlContent   string
		expectSteps   int
		expectError   bool
	}{
		{
			name: "new format with version and vars",
			yamlContent: `version: "1.0"
vars:
  app_name: "myapp"
  port: 8080
steps:
  - name: Test step
    shell: echo "{{app_name}}"
  - name: Second step
    shell: echo "Port {{port}}"`,
			expectSteps: 2,
			expectError: false,
		},
		{
			name: "new format with just steps",
			yamlContent: `version: "1.0"
steps:
  - name: Test step
    shell: echo "test"`,
			expectSteps: 1,
			expectError: false,
		},
		{
			name: "old format still works",
			yamlContent: `- name: Test step
  shell: echo "old format"
- name: Second step
  shell: echo "still works"`,
			expectSteps: 2,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file
			tmpFile, err := os.CreateTemp("", "runconfig-test-*.yml")
			if err != nil {
				t.Fatal(err)
			}
			defer os.Remove(tmpFile.Name())

			if _, err := tmpFile.Write([]byte(tt.yamlContent)); err != nil {
				t.Fatal(err)
			}
			if err := tmpFile.Close(); err != nil {
				t.Fatal(err)
			}

			// Read config
			reader := NewYAMLConfigReader()
			parsedConfig, err := reader.ReadConfig(tmpFile.Name())

			if tt.expectError {
				if err == nil {
					t.Error("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			if len(parsedConfig.Steps) != tt.expectSteps {
				t.Errorf("Expected %d steps, got %d", tt.expectSteps, len(parsedConfig.Steps))
			}
		})
	}
}

func TestParsedConfig_GlobalVars(t *testing.T) {
	reader := NewYAMLConfigReader()

	t.Run("new format with global vars", func(t *testing.T) {
		tmpFile := createTempYAML(t, `version: "1.0"
vars:
  app_name: "myapp"
  port: 8080
  debug: true
steps:
  - name: Test step
    shell: echo "{{app_name}}"`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		// Verify global vars
		if parsedConfig.GlobalVars == nil {
			t.Fatal("GlobalVars should not be nil")
		}
		if parsedConfig.GlobalVars["app_name"] != "myapp" {
			t.Errorf("GlobalVars[app_name] = %v, want 'myapp'", parsedConfig.GlobalVars["app_name"])
		}
		if parsedConfig.GlobalVars["port"] != 8080 {
			t.Errorf("GlobalVars[port] = %v, want 8080", parsedConfig.GlobalVars["port"])
		}
		if parsedConfig.GlobalVars["debug"] != true {
			t.Errorf("GlobalVars[debug] = %v, want true", parsedConfig.GlobalVars["debug"])
		}

		// Verify version
		if parsedConfig.Version != "1.0" {
			t.Errorf("Version = %q, want '1.0'", parsedConfig.Version)
		}

		// Verify steps
		if len(parsedConfig.Steps) != 1 {
			t.Errorf("got %d steps, want 1", len(parsedConfig.Steps))
		}
	})

	t.Run("old format without global vars", func(t *testing.T) {
		tmpFile := createTempYAML(t, `- name: Test step
  shell: echo test`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		// Verify global vars is empty
		if parsedConfig.GlobalVars == nil {
			t.Fatal("GlobalVars should not be nil, should be empty map")
		}
		if len(parsedConfig.GlobalVars) != 0 {
			t.Errorf("GlobalVars should be empty, got %d entries", len(parsedConfig.GlobalVars))
		}

		// Verify version is empty
		if parsedConfig.Version != "" {
			t.Errorf("Version = %q, want empty string", parsedConfig.Version)
		}

		// Verify steps
		if len(parsedConfig.Steps) != 1 {
			t.Errorf("got %d steps, want 1", len(parsedConfig.Steps))
		}
	})

	t.Run("new format without vars", func(t *testing.T) {
		tmpFile := createTempYAML(t, `version: "1.0"
steps:
  - name: Test step
    shell: echo test`)
		defer os.Remove(tmpFile)

		parsedConfig, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		// Verify global vars is empty (but not nil)
		if parsedConfig.GlobalVars == nil {
			t.Fatal("GlobalVars should not be nil, should be empty map")
		}
		if len(parsedConfig.GlobalVars) != 0 {
			t.Errorf("GlobalVars should be empty, got %d entries", len(parsedConfig.GlobalVars))
		}

		// Verify version
		if parsedConfig.Version != "1.0" {
			t.Errorf("Version = %q, want '1.0'", parsedConfig.Version)
		}
	})
}

func TestIsArrayFormat(t *testing.T) {
	tests := []struct {
		name        string
		yamlContent string
		expectArray bool
	}{
		{
			name: "array format",
			yamlContent: `- name: Step 1
  shell: echo test`,
			expectArray: true,
		},
		{
			name: "object format",
			yamlContent: `version: "1.0"
steps:
  - name: Step 1
    shell: echo test`,
			expectArray: false,
		},
		{
			name: "empty array",
			yamlContent: `[]`,
			expectArray: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rootNode yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlContent), &rootNode)
			if err != nil {
				t.Fatal(err)
			}

			result := isArrayFormat(&rootNode)
			if result != tt.expectArray {
				t.Errorf("isArrayFormat() = %v, want %v", result, tt.expectArray)
			}
		})
	}
}
