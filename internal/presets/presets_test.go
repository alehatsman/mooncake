package presets

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestPresetSearchPaths tests the preset search path ordering
func TestPresetSearchPaths(t *testing.T) {
	paths := PresetSearchPaths()

	if len(paths) < 2 {
		t.Errorf("Expected at least 2 search paths, got %d", len(paths))
	}

	// First path should always be ./presets
	if paths[0] != "./presets" {
		t.Errorf("First search path should be ./presets, got %s", paths[0])
	}

	// Should contain user home path (if available)
	home, err := os.UserHomeDir()
	if err == nil {
		expectedHome := filepath.Join(home, ".mooncake", "presets")
		found := false
		for _, p := range paths {
			if p == expectedHome {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected user home path %s in search paths", expectedHome)
		}
	}

	// Should contain system paths
	expectedPaths := []string{
		"/usr/local/share/mooncake/presets",
		"/usr/share/mooncake/presets",
	}
	for _, expected := range expectedPaths {
		found := false
		for _, p := range paths {
			if p == expected {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("Expected system path %s in search paths", expected)
		}
	}
}

// TestLoadPreset_FlatStructure tests loading a flat preset file
func TestLoadPreset_FlatStructure(t *testing.T) {
	// Create temp directory with preset
	tmpDir := t.TempDir()
	presetPath := filepath.Join(tmpDir, "test-preset.yml")

	presetContent := `name: test-preset
description: Test preset
version: 1.0.0
steps:
  - name: Step 1
    print: "Hello from preset"
`

	if err := os.WriteFile(presetPath, []byte(presetContent), 0644); err != nil {
		t.Fatalf("Failed to create test preset: %v", err)
	}

	// Create presets directory and copy file
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	testPresetPath := filepath.Join(presetsDir, "test-preset.yml")
	data, _ := os.ReadFile(presetPath)
	os.WriteFile(testPresetPath, data, 0644)

	// Load preset
	preset, err := LoadPreset("test-preset")
	if err != nil {
		t.Fatalf("LoadPreset failed: %v", err)
	}

	// Verify preset fields
	if preset.Name != "test-preset" {
		t.Errorf("Expected name 'test-preset', got %s", preset.Name)
	}
	if preset.Description != "Test preset" {
		t.Errorf("Expected description 'Test preset', got %s", preset.Description)
	}
	if preset.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", preset.Version)
	}
	if len(preset.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(preset.Steps))
	}
}

// TestLoadPreset_DirectoryStructure tests loading a preset from a directory
func TestLoadPreset_DirectoryStructure(t *testing.T) {
	// Create presets directory
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	// Create preset directory and file
	presetDir := filepath.Join(presetsDir, "dir-preset")
	os.MkdirAll(presetDir, 0755)

	presetContent := `name: dir-preset
description: Directory preset
steps:
  - name: Step 1
    print: "Hello"
`

	presetFile := filepath.Join(presetDir, "preset.yml")
	if err := os.WriteFile(presetFile, []byte(presetContent), 0644); err != nil {
		t.Fatalf("Failed to create preset file: %v", err)
	}

	// Load preset
	preset, err := LoadPreset("dir-preset")
	if err != nil {
		t.Fatalf("LoadPreset failed: %v", err)
	}

	// Verify preset loaded correctly
	if preset.Name != "dir-preset" {
		t.Errorf("Expected name 'dir-preset', got %s", preset.Name)
	}

	// Verify base directory is set
	if preset.BaseDir == "" {
		t.Error("Expected BaseDir to be set")
	}
}

// TestLoadPreset_EmptyName tests that empty preset name returns error
func TestLoadPreset_EmptyName(t *testing.T) {
	_, err := LoadPreset("")
	if err == nil {
		t.Error("LoadPreset should fail with empty name")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

// TestLoadPreset_NotFound tests preset not found error
func TestLoadPreset_NotFound(t *testing.T) {
	_, err := LoadPreset("nonexistent-preset")
	if err == nil {
		t.Error("LoadPreset should fail for non-existent preset")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestLoadPreset_InvalidYAML tests invalid YAML error
func TestLoadPreset_InvalidYAML(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	// Create invalid YAML file
	presetPath := filepath.Join(presetsDir, "invalid.yml")
	invalidContent := `name: invalid
steps: [unclosed
`
	os.WriteFile(presetPath, []byte(invalidContent), 0644)

	_, err := LoadPreset("invalid")
	if err == nil {
		t.Error("LoadPreset should fail for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Expected 'failed to parse' error, got: %v", err)
	}
}

// TestLoadPreset_MissingName tests missing name field error
func TestLoadPreset_MissingName(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "no-name.yml")
	content := `description: Missing name
steps:
  - name: Step 1
    print: "test"
`
	os.WriteFile(presetPath, []byte(content), 0644)

	_, err := LoadPreset("no-name")
	if err == nil {
		t.Error("LoadPreset should fail for preset without name")
	}
	if !strings.Contains(err.Error(), "missing required field 'name'") {
		t.Errorf("Expected 'missing required field' error, got: %v", err)
	}
}

// TestLoadPreset_NameMismatch tests name mismatch error
func TestLoadPreset_NameMismatch(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "expected-name.yml")
	content := `name: actual-name
steps:
  - name: Step 1
    print: "test"
`
	os.WriteFile(presetPath, []byte(content), 0644)

	_, err := LoadPreset("expected-name")
	if err == nil {
		t.Error("LoadPreset should fail for name mismatch")
	}
	if !strings.Contains(err.Error(), "name mismatch") {
		t.Errorf("Expected 'name mismatch' error, got: %v", err)
	}
}

// TestLoadPreset_NoSteps tests error when no steps defined
func TestLoadPreset_NoSteps(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "no-steps.yml")
	content := `name: no-steps
description: No steps
`
	os.WriteFile(presetPath, []byte(content), 0644)

	_, err := LoadPreset("no-steps")
	if err == nil {
		t.Error("LoadPreset should fail when no steps defined")
	}
	if !strings.Contains(err.Error(), "no steps defined") {
		t.Errorf("Expected 'no steps defined' error, got: %v", err)
	}
}

// TestLoadPreset_NestedPreset tests that nesting is detected and rejected
func TestLoadPreset_NestedPreset(t *testing.T) {
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "nested.yml")
	content := `name: nested
steps:
  - name: Call other preset
    preset: other-preset
`
	os.WriteFile(presetPath, []byte(content), 0644)

	_, err := LoadPreset("nested")
	if err == nil {
		t.Error("LoadPreset should fail for nested presets")
	}
	if !strings.Contains(err.Error(), "nesting not supported") {
		t.Errorf("Expected 'nesting not supported' error, got: %v", err)
	}
}

// TestValidateParameters_RequiredParameter tests required parameter validation
func TestValidateParameters_RequiredParameter(t *testing.T) {
	definition := &config.PresetDefinition{
		Name: "test",
		Parameters: map[string]config.PresetParameter{
			"required_param": {
				Type:     "string",
				Required: true,
			},
		},
	}

	// Missing required parameter
	_, err := ValidateParameters(definition, map[string]interface{}{})
	if err == nil {
		t.Error("ValidateParameters should fail for missing required parameter")
	}
	if !strings.Contains(err.Error(), "required parameter") {
		t.Errorf("Expected 'required parameter' error, got: %v", err)
	}

	// Provided required parameter
	validated, err := ValidateParameters(definition, map[string]interface{}{
		"required_param": "value",
	})
	if err != nil {
		t.Fatalf("ValidateParameters failed: %v", err)
	}
	if validated["required_param"] != "value" {
		t.Error("Required parameter should be in validated params")
	}
}

// TestValidateParameters_DefaultValue tests default value application
func TestValidateParameters_DefaultValue(t *testing.T) {
	definition := &config.PresetDefinition{
		Name: "test",
		Parameters: map[string]config.PresetParameter{
			"optional_param": {
				Type:    "string",
				Default: "default_value",
			},
		},
	}

	// Without providing parameter
	validated, err := ValidateParameters(definition, map[string]interface{}{})
	if err != nil {
		t.Fatalf("ValidateParameters failed: %v", err)
	}
	if validated["optional_param"] != "default_value" {
		t.Errorf("Expected default value, got %v", validated["optional_param"])
	}

	// With provided parameter (should override default)
	validated, err = ValidateParameters(definition, map[string]interface{}{
		"optional_param": "custom_value",
	})
	if err != nil {
		t.Fatalf("ValidateParameters failed: %v", err)
	}
	if validated["optional_param"] != "custom_value" {
		t.Error("Provided value should override default")
	}
}

// TestValidateParameters_UnknownParameter tests unknown parameter detection
func TestValidateParameters_UnknownParameter(t *testing.T) {
	definition := &config.PresetDefinition{
		Name:       "test",
		Parameters: map[string]config.PresetParameter{},
	}

	_, err := ValidateParameters(definition, map[string]interface{}{
		"unknown_param": "value",
	})
	if err == nil {
		t.Error("ValidateParameters should fail for unknown parameter")
	}
	if !strings.Contains(err.Error(), "unknown parameter") {
		t.Errorf("Expected 'unknown parameter' error, got: %v", err)
	}
}

// TestValidateParameters_TypeValidation tests type validation
func TestValidateParameters_TypeValidation(t *testing.T) {
	tests := []struct {
		name          string
		paramType     string
		validValue    interface{}
		invalidValue  interface{}
		expectedError string
	}{
		{
			name:          "string type",
			paramType:     "string",
			validValue:    "text",
			invalidValue:  123,
			expectedError: "must be a string",
		},
		{
			name:          "bool type",
			paramType:     "bool",
			validValue:    true,
			invalidValue:  "not a bool",
			expectedError: "must be a boolean",
		},
		{
			name:          "array type",
			paramType:     "array",
			validValue:    []interface{}{"a", "b"},
			invalidValue:  "not an array",
			expectedError: "must be an array",
		},
		{
			name:          "object type",
			paramType:     "object",
			validValue:    map[string]interface{}{"key": "value"},
			invalidValue:  "not an object",
			expectedError: "must be an object",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			definition := &config.PresetDefinition{
				Name: "test",
				Parameters: map[string]config.PresetParameter{
					"param": {
						Type:     tt.paramType,
						Required: true,
					},
				},
			}

			// Valid value should succeed
			_, err := ValidateParameters(definition, map[string]interface{}{
				"param": tt.validValue,
			})
			if err != nil {
				t.Errorf("ValidateParameters failed for valid %s: %v", tt.paramType, err)
			}

			// Invalid value should fail
			_, err = ValidateParameters(definition, map[string]interface{}{
				"param": tt.invalidValue,
			})
			if err == nil {
				t.Errorf("ValidateParameters should fail for invalid %s", tt.paramType)
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected '%s' error, got: %v", tt.expectedError, err)
			}
		})
	}
}

// TestValidateParameters_EnumValidation tests enum constraint validation
func TestValidateParameters_EnumValidation(t *testing.T) {
	definition := &config.PresetDefinition{
		Name: "test",
		Parameters: map[string]config.PresetParameter{
			"state": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"started", "stopped", "restarted"},
			},
		},
	}

	// Valid enum value
	_, err := ValidateParameters(definition, map[string]interface{}{
		"state": "started",
	})
	if err != nil {
		t.Fatalf("ValidateParameters failed for valid enum value: %v", err)
	}

	// Invalid enum value
	_, err = ValidateParameters(definition, map[string]interface{}{
		"state": "invalid",
	})
	if err == nil {
		t.Error("ValidateParameters should fail for invalid enum value")
	}
	if !strings.Contains(err.Error(), "invalid value") {
		t.Errorf("Expected 'invalid value' error, got: %v", err)
	}
}

// TestValidateParameters_NilDefinition tests nil definition handling
func TestValidateParameters_NilDefinition(t *testing.T) {
	_, err := ValidateParameters(nil, map[string]interface{}{})
	if err == nil {
		t.Error("ValidateParameters should fail for nil definition")
	}
	if !strings.Contains(err.Error(), "definition is nil") {
		t.Errorf("Expected 'definition is nil' error, got: %v", err)
	}
}

// TestExpandPreset tests preset expansion
func TestExpandPreset(t *testing.T) {
	// Create test preset
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "expand-test.yml")
	content := `name: expand-test
parameters:
  message:
    type: string
    required: true
steps:
  - name: Print message
    print: "{{ parameters.message }}"
`
	os.WriteFile(presetPath, []byte(content), 0644)

	// Expand preset
	invocation := &config.PresetInvocation{
		Name: "expand-test",
		With: map[string]interface{}{
			"message": "Hello World",
		},
	}

	steps, namespace, baseDir, err := ExpandPreset(invocation)
	if err != nil {
		t.Fatalf("ExpandPreset failed: %v", err)
	}

	// Verify steps
	if len(steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(steps))
	}

	// Verify namespace contains parameters
	if namespace["parameters"] == nil {
		t.Error("Expected parameters namespace to be set")
	}

	params := namespace["parameters"].(map[string]interface{})
	if params["message"] != "Hello World" {
		t.Errorf("Expected message parameter, got %v", params["message"])
	}

	// Verify base directory is set
	if baseDir == "" {
		t.Error("Expected base directory to be set")
	}
}

// TestExpandPreset_NilInvocation tests nil invocation handling
func TestExpandPreset_NilInvocation(t *testing.T) {
	_, _, _, err := ExpandPreset(nil)
	if err == nil {
		t.Error("ExpandPreset should fail for nil invocation")
	}
	if !strings.Contains(err.Error(), "invocation is nil") {
		t.Errorf("Expected 'invocation is nil' error, got: %v", err)
	}
}

// TestExpandPreset_PresetNotFound tests preset not found error
func TestExpandPreset_PresetNotFound(t *testing.T) {
	invocation := &config.PresetInvocation{
		Name: "nonexistent",
	}

	_, _, _, err := ExpandPreset(invocation)
	if err == nil {
		t.Error("ExpandPreset should fail for non-existent preset")
	}
	if !strings.Contains(err.Error(), "failed to load preset") {
		t.Errorf("Expected 'failed to load preset' error, got: %v", err)
	}
}

// TestExpandPreset_ParameterValidationFailed tests parameter validation failure
func TestExpandPreset_ParameterValidationFailed(t *testing.T) {
	// Create test preset with required parameter
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "required-param.yml")
	content := `name: required-param
parameters:
  required_field:
    type: string
    required: true
steps:
  - name: Step
    print: "test"
`
	os.WriteFile(presetPath, []byte(content), 0644)

	// Try to expand without providing required parameter
	invocation := &config.PresetInvocation{
		Name: "required-param",
		With: map[string]interface{}{}, // Missing required_field
	}

	_, _, _, err := ExpandPreset(invocation)
	if err == nil {
		t.Error("ExpandPreset should fail for missing required parameter")
	}
	if !strings.Contains(err.Error(), "parameter validation failed") {
		t.Errorf("Expected 'parameter validation failed' error, got: %v", err)
	}
}

// TestExpandPreset_NilWith tests expansion with nil parameters
func TestExpandPreset_NilWith(t *testing.T) {
	// Create test preset without required parameters
	presetsDir := filepath.Join(".", "presets")
	os.MkdirAll(presetsDir, 0755)
	defer os.RemoveAll(presetsDir)

	presetPath := filepath.Join(presetsDir, "no-params.yml")
	content := `name: no-params
steps:
  - name: Step
    print: "test"
`
	os.WriteFile(presetPath, []byte(content), 0644)

	// Expand with nil parameters
	invocation := &config.PresetInvocation{
		Name: "no-params",
		With: nil,
	}

	steps, namespace, _, err := ExpandPreset(invocation)
	if err != nil {
		t.Fatalf("ExpandPreset should succeed with nil parameters: %v", err)
	}

	if len(steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(steps))
	}

	if namespace["parameters"] == nil {
		t.Error("Expected parameters namespace even with nil With")
	}
}

// TestGetValueType tests the type detection helper
func TestGetValueType(t *testing.T) {
	tests := []struct {
		value    interface{}
		expected string
	}{
		{nil, "null"},
		{"string", "string"},
		{true, "bool"},
		{false, "bool"},
		{42, "number"},
		{int8(1), "number"},
		{int16(1), "number"},
		{int32(1), "number"},
		{int64(1), "number"},
		{uint(1), "number"},
		{uint8(1), "number"},
		{uint16(1), "number"},
		{uint32(1), "number"},
		{uint64(1), "number"},
		{float32(3.14), "number"},
		{float64(3.14), "number"},
		{[]interface{}{"a", "b"}, "array"},
		{map[string]interface{}{"key": "value"}, "object"},
	}

	for _, tt := range tests {
		actual := getValueType(tt.value)
		if actual != tt.expected {
			t.Errorf("getValueType(%v) = %s, want %s", tt.value, actual, tt.expected)
		}
	}
}
