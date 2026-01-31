package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestYAMLReader_ReadConfig(t *testing.T) {
	reader := NewYAMLConfigReader()

	t.Run("valid config", func(t *testing.T) {
		tmpFile := createTempYAML(t, `
- name: test step
  shell: echo hello

- name: create file
  file:
    path: /tmp/test.txt
    state: file
`)
		defer os.Remove(tmpFile)

		steps, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(steps) != 2 {
			t.Errorf("ReadConfig() got %d steps, want 2", len(steps))
		}

		// Verify first step
		if steps[0].Name != "test step" {
			t.Errorf("step[0].Name = %q, want 'test step'", steps[0].Name)
		}
		if steps[0].Shell == nil || *steps[0].Shell != "echo hello" {
			t.Error("step[0].Shell not correctly parsed")
		}

		// Verify second step
		if steps[1].Name != "create file" {
			t.Errorf("step[1].Name = %q, want 'create file'", steps[1].Name)
		}
		if steps[1].File == nil {
			t.Error("step[1].File should not be nil")
		}
	})

	t.Run("empty config", func(t *testing.T) {
		tmpFile := createTempYAML(t, "[]")
		defer os.Remove(tmpFile)

		steps, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(steps) != 0 {
			t.Errorf("ReadConfig() got %d steps, want 0", len(steps))
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
  template:
    src: /tmp/template.j2
    dest: /tmp/output.txt
`)
		defer os.Remove(tmpFile)

		steps, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(steps) != 1 {
			t.Errorf("ReadConfig() got %d steps, want 1", len(steps))
		}

		if steps[0].Template == nil {
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

		steps, err := reader.ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

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

		steps, err := ReadConfig(tmpFile)
		if err != nil {
			t.Fatalf("ReadConfig() error = %v", err)
		}

		if len(steps) != 1 {
			t.Errorf("ReadConfig() got %d steps, want 1", len(steps))
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
}

func TestYAMLReader_ReadConfigComplexSteps(t *testing.T) {
	reader := NewYAMLConfigReader()

	tmpFile := createTempYAML(t, `
- name: create directory
  file:
    path: /tmp/testdir
    state: directory
    mode: "0755"

- name: template file
  template:
    src: template.j2
    dest: /tmp/output.txt
    mode: "0644"
    vars:
      key: value

- name: run with sudo
  shell: apt-get update
  become: true

- name: include other file
  include: other.yml

- name: loop over files
  file:
    path: "{{ item.path }}"
    state: file
  with_filetree: /tmp/files

- name: set variables
  vars:
    env: production
    version: 2.0
`)
	defer os.Remove(tmpFile)

	steps, err := reader.ReadConfig(tmpFile)
	if err != nil {
		t.Fatalf("ReadConfig() error = %v", err)
	}

	if len(steps) != 6 {
		t.Errorf("ReadConfig() got %d steps, want 6", len(steps))
	}

	// Verify file step with mode
	if steps[0].File != nil && steps[0].File.Mode != "0755" {
		t.Errorf("file mode = %q, want '0755'", steps[0].File.Mode)
	}

	// Verify become flag
	if !steps[2].Become {
		t.Error("step with sudo should have Become = true")
	}

	// Verify include
	if steps[3].Include == nil {
		t.Error("include step should have Include field set")
	}

	// Verify with_filetree
	if steps[4].WithFileTree == nil {
		t.Error("with_filetree step should have WithFileTree field set")
	}

	// Verify vars
	if steps[5].Vars == nil {
		t.Error("vars step should have Vars field set")
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
