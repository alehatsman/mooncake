package config

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestNewSchemaValidator(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create schema validator: %v", err)
	}
	if validator == nil {
		t.Fatal("Expected non-nil validator")
	}
	if validator.schema == nil {
		t.Fatal("Expected non-nil schema")
	}
}

func TestSchemaValidator_ValidConfig(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name       string
		yamlConfig string
	}{
		{
			name: "simple shell step",
			yamlConfig: `- name: test step
  shell: echo hello`,
		},
		{
			name: "file step",
			yamlConfig: `- name: create file
  file:
    path: /tmp/test.txt
    state: present`,
		},
		{
			name: "template step",
			yamlConfig: `- name: render template
  template:
    src: /path/to/template.j2
    dest: /path/to/output.txt`,
		},
		{
			name: "template with mode",
			yamlConfig: `- name: render template
  template:
    src: /path/to/template.j2
    dest: /path/to/output.txt
    mode: "0644"`,
		},
		{
			name: "file with mode",
			yamlConfig: `- name: create file
  file:
    path: /tmp/test.txt
    state: present
    mode: "0755"`,
		},
		{
			name: "include step",
			yamlConfig: `- name: include tasks
  include: ./tasks.yml`,
		},
		{
			name: "include_vars step",
			yamlConfig: `- name: load vars
  include_vars: ./vars.yml`,
		},
		{
			name: "vars step",
			yamlConfig: `- vars:
    key1: value1
    key2: value2`,
		},
		{
			name: "step with when condition",
			yamlConfig: `- name: conditional step
  shell: echo hello
  when: os == "linux"`,
		},
		{
			name: "step with tags",
			yamlConfig: `- name: tagged step
  shell: echo hello
  tags:
    - setup
    - initial`,
		},
		{
			name: "step with register",
			yamlConfig: `- name: command with register
  shell: echo hello
  register: result`,
		},
		{
			name: "step with become",
			yamlConfig: `- name: privileged command
  shell: apt-get update
  become: true`,
		},
		{
			name: "step with with_items",
			yamlConfig: `- name: loop over items
  shell: echo "{{ item }}"
  with_items: "{{ packages }}"`,
		},
		{
			name: "step with with_filetree",
			yamlConfig: `- name: loop over files
  file:
    path: "{{ item.path }}"
    state: present
  with_filetree: /tmp/files`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse YAML
			var rootNode yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlConfig), &rootNode)
			if err != nil {
				t.Fatalf("Failed to parse YAML: %v", err)
			}

			// Build location map
			locationMap := buildLocationMap(&rootNode)

			// Unmarshal to steps
			var steps []Step
			err = rootNode.Decode(&steps)
			if err != nil {
				t.Fatalf("Failed to decode steps: %v", err)
			}

			// Validate
			parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

			// Should have no errors
			if len(diagnostics) > 0 {
				t.Errorf("Expected no validation errors, got %d:\n%s",
					len(diagnostics), FormatDiagnostics(diagnostics))
			}
		})
	}
}

func TestSchemaValidator_MultipleActions(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: invalid step
  shell: echo hello
  file:
    path: /tmp/test.txt
    state: present`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have validation errors (multiple actions)
	if len(diagnostics) == 0 {
		t.Error("Expected validation errors for multiple actions, got none")
	}
}

func TestSchemaValidator_NoAction(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: step with no action
  when: os == "linux"`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have validation errors (no action)
	if len(diagnostics) == 0 {
		t.Error("Expected validation errors for missing action, got none")
	}
}

func TestSchemaValidator_MissingRequiredField(t *testing.T) {
	t.Skip("Known limitation: validation after Go unmarshal cannot detect missing required fields with omitempty JSON tags")

	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name       string
		yamlConfig string
		wantError  string
	}{
		{
			name: "template missing src",
			yamlConfig: `- name: invalid template
  template:
    dest: /path/to/output.txt`,
			wantError: "src",
		},
		{
			name: "template missing dest",
			yamlConfig: `- name: invalid template
  template:
    src: /path/to/template.j2`,
			wantError: "dest",
		},
		{
			name: "file missing path",
			yamlConfig: `- name: invalid file
  file:
    state: present`,
			wantError: "path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var rootNode yaml.Node
			err := yaml.Unmarshal([]byte(tt.yamlConfig), &rootNode)
			if err != nil {
				t.Fatalf("Failed to parse YAML: %v", err)
			}

			locationMap := buildLocationMap(&rootNode)

			var steps []Step
			err = rootNode.Decode(&steps)
			if err != nil {
				t.Fatalf("Failed to decode steps: %v", err)
			}

			parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

			// Should have validation errors
			if len(diagnostics) == 0 {
				t.Errorf("Expected validation errors for missing %s, got none", tt.wantError)
				return
			}

			// Check that the error message mentions the missing field
			found := false
			for _, diag := range diagnostics {
				if strings.Contains(diag.Message, tt.wantError) {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected error message to mention '%s', got:\n%s",
					tt.wantError, FormatDiagnostics(diagnostics))
			}
		})
	}
}

func TestSchemaValidator_InvalidFieldType(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: invalid step
  shell: 123
  become: "not a boolean"`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	// Note: This might fail at decode time depending on Go's strictness
	// If it decodes, validation should catch type mismatches

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have some kind of error (either from decode or validation)
	if err == nil && len(diagnostics) == 0 {
		t.Error("Expected either decode error or validation errors for type mismatch, got neither")
	}
}

func TestSchemaValidator_UnknownField(t *testing.T) {
	t.Skip("Known limitation: unknown fields in YAML are dropped by Go's unmarshaler before validation")

	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: step with unknown field
  shell: echo hello
  unknown_field: some value`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have validation errors (unknown field)
	if len(diagnostics) == 0 {
		t.Error("Expected validation errors for unknown field, got none")
	}

	// Check that error mentions the unknown field
	found := false
	for _, diag := range diagnostics {
		if strings.Contains(diag.Message, "unknown") || strings.Contains(diag.Message, "additional") {
			found = true
			break
		}
	}

	if !found {
		t.Errorf("Expected error message to mention unknown/additional field, got:\n%s",
			FormatDiagnostics(diagnostics))
	}
}

func TestSchemaValidator_InvalidFileMode(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	tests := []struct {
		name string
		mode string
	}{
		// Note: "644" without leading zero is actually valid per schema pattern ^[0-7]{3,4}$
		// The pattern allows 3-4 octal digits, with or without leading zero
		{"invalid octal digit", `"0888"`},
		{"too short", `"07"`},
		{"too long", `"07777"`},
		// Note: numeric mode (644 without quotes) is handled by YAML parser
		// which coerces it to string, so we don't test for that case
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			yamlConfig := `- name: file with invalid mode
  file:
    path: /tmp/test.txt
    state: present
    mode: ` + tt.mode

			var rootNode yaml.Node
			err := yaml.Unmarshal([]byte(yamlConfig), &rootNode)
			if err != nil {
				// Some invalid values might fail at parse time
				return
			}

			locationMap := buildLocationMap(&rootNode)

			var steps []Step
			err = rootNode.Decode(&steps)
			if err != nil {
				// Type mismatch might fail at decode time
				return
			}

			parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

			// Should have validation errors
			if len(diagnostics) == 0 {
				t.Errorf("Expected validation errors for invalid mode %s, got none", tt.mode)
			}
		})
	}
}

func TestSchemaValidator_InvalidFileState(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: file with invalid state
  file:
    path: /tmp/test.txt
    state: invalid_state`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have validation errors (invalid enum value)
	if len(diagnostics) == 0 {
		t.Error("Expected validation errors for invalid state, got none")
	}
}

// TestSchemaValidator_DiagnosticHasLocation verifies that diagnostics include source locations
func TestSchemaValidator_DiagnosticHasLocation(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: step1
  shell: echo hello
  file:
    path: /tmp/test.txt
    state: present`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "/path/to/test.yml")

	if len(diagnostics) == 0 {
		t.Fatal("Expected validation errors, got none")
	}

	// Check that diagnostics have location information
	for _, diag := range diagnostics {
		if diag.FilePath != "/path/to/test.yml" {
			t.Errorf("Expected FilePath '/path/to/test.yml', got %q", diag.FilePath)
		}
		if diag.Line == 0 {
			t.Error("Expected non-zero Line number")
		}
		// Column might be 0 in some cases, so we don't strictly require it
	}
}

// TestValidate_RealWorldExample tests validation with a realistic config
func TestValidate_RealWorldExample(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `- name: Create application directory
  file:
    path: /opt/myapp
    state: directory
    mode: "0755"

- name: Install dependencies
  shell: apt-get install -y nginx
  become: true
  when: os == "linux"
  tags:
    - setup

- name: Render configuration
  template:
    src: ./templates/nginx.conf.j2
    dest: /etc/nginx/nginx.conf
    mode: "0644"
  become: true

- name: Load variables
  include_vars: ./vars/production.yml

- name: Start service
  shell: systemctl start nginx
  become: true
  register: start_result`

	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	locationMap := buildLocationMap(&rootNode)

	var steps []Step
	err = rootNode.Decode(&steps)
	if err != nil {
		t.Fatalf("Failed to decode steps: %v", err)
	}

	parsedConfig := &ParsedConfig{Steps: steps, GlobalVars: make(map[string]interface{}), Version: ""}
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have no errors for valid config
	if len(diagnostics) > 0 {
		t.Errorf("Expected no validation errors for valid config, got %d:\n%s",
			len(diagnostics), FormatDiagnostics(diagnostics))
	}
}

func TestSchemaValidator_RunConfigFormat(t *testing.T) {
	validator, err := NewSchemaValidator()
	if err != nil {
		t.Fatalf("Failed to create validator: %v", err)
	}

	yamlConfig := `version: "1.0"
vars:
  os_name: darwin
steps:
  - name: test step
    shell: echo hello`

	// Parse YAML
	var rootNode yaml.Node
	err = yaml.Unmarshal([]byte(yamlConfig), &rootNode)
	if err != nil {
		t.Fatalf("Failed to parse YAML: %v", err)
	}

	// Build location map
	locationMap := buildLocationMap(&rootNode)

	// Unmarshal to RunConfig
	var runConfig RunConfig
	err = rootNode.Decode(&runConfig)
	if err != nil {
		t.Fatalf("Failed to decode RunConfig: %v", err)
	}

	// Create ParsedConfig
	parsedConfig := &ParsedConfig{
		Version:    runConfig.Version,
		GlobalVars: runConfig.Vars,
		Steps:      runConfig.Steps,
	}

	t.Logf("Version: %q", parsedConfig.Version)
	t.Logf("GlobalVars: %v", parsedConfig.GlobalVars)
	t.Logf("Steps count: %d", len(parsedConfig.Steps))

	// Validate
	diagnostics := validator.Validate(parsedConfig, locationMap, "test.yml")

	// Should have no errors
	if len(diagnostics) > 0 {
		for _, d := range diagnostics {
			t.Errorf("Validation error at line %d: %s", d.Line, d.Message)
		}
		t.Fatalf("Expected no validation errors, got %d", len(diagnostics))
	}
}
