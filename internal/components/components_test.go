package components

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// TestComponentSearchPaths tests the component search path ordering
func TestComponentSearchPaths(t *testing.T) {
	paths := ComponentSearchPaths()

	if len(paths) < 2 {
		t.Errorf("Expected at least 2 search paths, got %d", len(paths))
	}

	// First path should always be ./components
	if paths[0] != "./components" {
		t.Errorf("First search path should be ./components, got %s", paths[0])
	}

	// Should contain user home path (if available)
	home, err := os.UserHomeDir()
	if err == nil {
		expectedHome := filepath.Join(home, ".mooncake", "components")
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
		"/usr/local/share/mooncake/components",
		"/usr/share/mooncake/components",
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

// TestLoadComponent_FlatStructure tests loading a flat component file
func TestLoadComponent_FlatStructure(t *testing.T) {
	// Create temp directory with component
	tmpDir := t.TempDir()
	componentPath := filepath.Join(tmpDir, "test-component.yml")

	componentContent := `name: test-component
description: Test component
version: 1.0.0
steps:
  - name: Step 1
    log: "Hello from component"
`

	if err := os.WriteFile(componentPath, []byte(componentContent), 0644); err != nil {
		t.Fatalf("Failed to create test component: %v", err)
	}

	// Create components directory and copy file
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	testComponentPath := filepath.Join(componentsDir, "test-component.yml")
	data, _ := os.ReadFile(componentPath)
	os.WriteFile(testComponentPath, data, 0644)

	// Load component
	component, err := LoadComponent("test-component")
	if err != nil {
		t.Fatalf("LoadComponent failed: %v", err)
	}

	// Verify component fields
	if component.Name != "test-component" {
		t.Errorf("Expected name 'test-component', got %s", component.Name)
	}
	if component.Description != "Test component" {
		t.Errorf("Expected description 'Test component', got %s", component.Description)
	}
	if component.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got %s", component.Version)
	}
	if len(component.Steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(component.Steps))
	}
}

// TestLoadComponent_DirectoryStructure tests loading a component from a directory
func TestLoadComponent_DirectoryStructure(t *testing.T) {
	// Create components directory
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	// Create component directory and file
	componentDir := filepath.Join(componentsDir, "dir-component")
	os.MkdirAll(componentDir, 0755)

	componentContent := `name: dir-component
description: Directory component
steps:
  - name: Step 1
    log: "Hello"
`

	componentFile := filepath.Join(componentDir, "component.yml")
	if err := os.WriteFile(componentFile, []byte(componentContent), 0644); err != nil {
		t.Fatalf("Failed to create component file: %v", err)
	}

	// Load component
	component, err := LoadComponent("dir-component")
	if err != nil {
		t.Fatalf("LoadComponent failed: %v", err)
	}

	// Verify component loaded correctly
	if component.Name != "dir-component" {
		t.Errorf("Expected name 'dir-component', got %s", component.Name)
	}

	// Verify base directory is set
	if component.BaseDir == "" {
		t.Error("Expected BaseDir to be set")
	}
}

// TestLoadComponent_EmptyName tests that empty component name returns error
func TestLoadComponent_EmptyName(t *testing.T) {
	_, err := LoadComponent("")
	if err == nil {
		t.Error("LoadComponent should fail with empty name")
	}
	if !strings.Contains(err.Error(), "cannot be empty") {
		t.Errorf("Expected 'cannot be empty' error, got: %v", err)
	}
}

// TestLoadComponent_NotFound tests component not found error
func TestLoadComponent_NotFound(t *testing.T) {
	_, err := LoadComponent("nonexistent-component")
	if err == nil {
		t.Error("LoadComponent should fail for non-existent component")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("Expected 'not found' error, got: %v", err)
	}
}

// TestLoadComponent_InvalidYAML tests invalid YAML error
func TestLoadComponent_InvalidYAML(t *testing.T) {
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	// Create invalid YAML file
	componentPath := filepath.Join(componentsDir, "invalid.yml")
	invalidContent := `name: invalid
steps: [unclosed
`
	os.WriteFile(componentPath, []byte(invalidContent), 0644)

	_, err := LoadComponent("invalid")
	if err == nil {
		t.Error("LoadComponent should fail for invalid YAML")
	}
	if !strings.Contains(err.Error(), "failed to parse") {
		t.Errorf("Expected 'failed to parse' error, got: %v", err)
	}
}

// TestLoadComponent_MissingName tests missing name field error
func TestLoadComponent_MissingName(t *testing.T) {
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "no-name.yml")
	content := `description: Missing name
steps:
  - name: Step 1
    log: "test"
`
	os.WriteFile(componentPath, []byte(content), 0644)

	_, err := LoadComponent("no-name")
	if err == nil {
		t.Error("LoadComponent should fail for component without name")
	}
	if !strings.Contains(err.Error(), "missing required field 'name'") {
		t.Errorf("Expected 'missing required field' error, got: %v", err)
	}
}

// TestLoadComponent_NameMismatch tests name mismatch error
func TestLoadComponent_NameMismatch(t *testing.T) {
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "expected-name.yml")
	content := `name: actual-name
steps:
  - name: Step 1
    log: "test"
`
	os.WriteFile(componentPath, []byte(content), 0644)

	_, err := LoadComponent("expected-name")
	if err == nil {
		t.Error("LoadComponent should fail for name mismatch")
	}
	if !strings.Contains(err.Error(), "name mismatch") {
		t.Errorf("Expected 'name mismatch' error, got: %v", err)
	}
}

// TestLoadComponent_NoSteps tests error when no steps defined
func TestLoadComponent_NoSteps(t *testing.T) {
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "no-steps.yml")
	content := `name: no-steps
description: No steps
`
	os.WriteFile(componentPath, []byte(content), 0644)

	_, err := LoadComponent("no-steps")
	if err == nil {
		t.Error("LoadComponent should fail when no steps defined")
	}
	if !strings.Contains(err.Error(), "no steps defined") {
		t.Errorf("Expected 'no steps defined' error, got: %v", err)
	}
}

// TestLoadComponent_NestedComponent tests that nesting is detected and rejected
func TestLoadComponent_NestedComponent(t *testing.T) {
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "nested.yml")
	content := `name: nested
steps:
  - name: Call other component
    use: other-component
`
	os.WriteFile(componentPath, []byte(content), 0644)

	_, err := LoadComponent("nested")
	if err == nil {
		t.Error("LoadComponent should fail for nested components")
	}
	if !strings.Contains(err.Error(), "nesting not supported") {
		t.Errorf("Expected 'nesting not supported' error, got: %v", err)
	}
}

// TestValidateParameters_RequiredParameter tests required parameter validation
func TestValidateParameters_RequiredParameter(t *testing.T) {
	definition := &config.ComponentDefinition{
		Name: "test",
		Props: map[string]config.ComponentProp{
			"required_param": {
				Type:     "string",
				Required: true,
			},
		},
	}

	// Missing required parameter
	_, err := ValidateProps(definition, map[string]interface{}{})
	if err == nil {
		t.Error("ValidateProps should fail for missing required parameter")
	}
	if !strings.Contains(err.Error(), "required parameter") {
		t.Errorf("Expected 'required parameter' error, got: %v", err)
	}

	// Provided required parameter
	validated, err := ValidateProps(definition, map[string]interface{}{
		"required_param": "value",
	})
	if err != nil {
		t.Fatalf("ValidateProps failed: %v", err)
	}
	if validated["required_param"] != "value" {
		t.Error("Required parameter should be in validated params")
	}
}

// TestValidateParameters_DefaultValue tests default value application
func TestValidateParameters_DefaultValue(t *testing.T) {
	definition := &config.ComponentDefinition{
		Name: "test",
		Props: map[string]config.ComponentProp{
			"optional_param": {
				Type:    "string",
				Default: "default_value",
			},
		},
	}

	// Without providing parameter
	validated, err := ValidateProps(definition, map[string]interface{}{})
	if err != nil {
		t.Fatalf("ValidateProps failed: %v", err)
	}
	if validated["optional_param"] != "default_value" {
		t.Errorf("Expected default value, got %v", validated["optional_param"])
	}

	// With provided parameter (should override default)
	validated, err = ValidateProps(definition, map[string]interface{}{
		"optional_param": "custom_value",
	})
	if err != nil {
		t.Fatalf("ValidateProps failed: %v", err)
	}
	if validated["optional_param"] != "custom_value" {
		t.Error("Provided value should override default")
	}
}

// TestValidateParameters_UnknownParameter tests unknown parameter detection
func TestValidateParameters_UnknownParameter(t *testing.T) {
	definition := &config.ComponentDefinition{
		Name:  "test",
		Props: map[string]config.ComponentProp{},
	}

	_, err := ValidateProps(definition, map[string]interface{}{
		"unknown_param": "value",
	})
	if err == nil {
		t.Error("ValidateProps should fail for unknown parameter")
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
			definition := &config.ComponentDefinition{
				Name: "test",
				Props: map[string]config.ComponentProp{
					"param": {
						Type:     tt.paramType,
						Required: true,
					},
				},
			}

			// Valid value should succeed
			_, err := ValidateProps(definition, map[string]interface{}{
				"param": tt.validValue,
			})
			if err != nil {
				t.Errorf("ValidateProps failed for valid %s: %v", tt.paramType, err)
			}

			// Invalid value should fail
			_, err = ValidateProps(definition, map[string]interface{}{
				"param": tt.invalidValue,
			})
			if err == nil {
				t.Errorf("ValidateProps should fail for invalid %s", tt.paramType)
			}
			if !strings.Contains(err.Error(), tt.expectedError) {
				t.Errorf("Expected '%s' error, got: %v", tt.expectedError, err)
			}
		})
	}
}

// TestValidateParameters_EnumValidation tests enum constraint validation
func TestValidateParameters_EnumValidation(t *testing.T) {
	definition := &config.ComponentDefinition{
		Name: "test",
		Props: map[string]config.ComponentProp{
			"state": {
				Type:     "string",
				Required: true,
				Enum:     []interface{}{"started", "stopped", "restarted"},
			},
		},
	}

	// Valid enum value
	_, err := ValidateProps(definition, map[string]interface{}{
		"state": "started",
	})
	if err != nil {
		t.Fatalf("ValidateProps failed for valid enum value: %v", err)
	}

	// Invalid enum value
	_, err = ValidateProps(definition, map[string]interface{}{
		"state": "invalid",
	})
	if err == nil {
		t.Error("ValidateProps should fail for invalid enum value")
	}
	if !strings.Contains(err.Error(), "invalid value") {
		t.Errorf("Expected 'invalid value' error, got: %v", err)
	}
}

// TestValidateParameters_NilDefinition tests nil definition handling
func TestValidateParameters_NilDefinition(t *testing.T) {
	_, err := ValidateProps(nil, map[string]interface{}{})
	if err == nil {
		t.Error("ValidateProps should fail for nil definition")
	}
	if !strings.Contains(err.Error(), "definition is nil") {
		t.Errorf("Expected 'definition is nil' error, got: %v", err)
	}
}

// TestExpandComponent tests component expansion
func TestExpandComponent(t *testing.T) {
	// Create test component
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "expand-test.yml")
	content := `name: expand-test
props:
  message:
    type: string
    required: true
steps:
  - name: Print message
    log: "{{ parameters.message }}"
`
	os.WriteFile(componentPath, []byte(content), 0644)

	// Expand component
	steps, namespace, baseDir, err := ExpandComponent("expand-test", map[string]interface{}{
		"message": "Hello World",
	})
	if err != nil {
		t.Fatalf("ExpandComponent failed: %v", err)
	}

	// Verify steps
	if len(steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(steps))
	}

	// Verify namespace contains props
	if namespace["props"] == nil {
		t.Error("Expected props namespace to be set")
	}

	params := namespace["props"].(map[string]interface{})
	if params["message"] != "Hello World" {
		t.Errorf("Expected message prop, got %v", params["message"])
	}

	// Verify base directory is set
	if baseDir == "" {
		t.Error("Expected base directory to be set")
	}
}

// TestExpandComponent_EmptyName tests empty-name handling
func TestExpandComponent_EmptyName(t *testing.T) {
	_, _, _, err := ExpandComponent("", nil)
	if err == nil {
		t.Error("ExpandComponent should fail for empty name")
	}
	if !strings.Contains(err.Error(), "empty name") {
		t.Errorf("Expected 'empty name' error, got: %v", err)
	}
}

// TestExpandComponent_ComponentNotFound tests component not found error
func TestExpandComponent_ComponentNotFound(t *testing.T) {
	_, _, _, err := ExpandComponent("nonexistent", nil)
	if err == nil {
		t.Error("ExpandComponent should fail for non-existent component")
	}
	if !strings.Contains(err.Error(), "failed to load component") {
		t.Errorf("Expected 'failed to load component' error, got: %v", err)
	}
}

// TestExpandComponent_ParameterValidationFailed tests parameter validation failure
func TestExpandComponent_ParameterValidationFailed(t *testing.T) {
	// Create test component with required parameter
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "required-param.yml")
	content := `name: required-param
props:
  required_field:
    type: string
    required: true
steps:
  - name: Step
    log: "test"
`
	os.WriteFile(componentPath, []byte(content), 0644)

	// Try to expand without providing required parameter
	_, _, _, err := ExpandComponent("required-param", map[string]interface{}{}) // Missing required_field
	if err == nil {
		t.Error("ExpandComponent should fail for missing required parameter")
	}
	if !strings.Contains(err.Error(), "parameter validation failed") {
		t.Errorf("Expected 'parameter validation failed' error, got: %v", err)
	}
}

// TestExpandComponent_NilWith tests expansion with nil parameters
func TestExpandComponent_NilWith(t *testing.T) {
	// Create test component without required parameters
	componentsDir := filepath.Join(".", "components")
	os.MkdirAll(componentsDir, 0755)
	defer os.RemoveAll(componentsDir)

	componentPath := filepath.Join(componentsDir, "no-params.yml")
	content := `name: no-params
steps:
  - name: Step
    log: "test"
`
	os.WriteFile(componentPath, []byte(content), 0644)

	// Expand with nil parameters
	steps, namespace, _, err := ExpandComponent("no-params", nil)
	if err != nil {
		t.Fatalf("ExpandComponent should succeed with nil parameters: %v", err)
	}

	if len(steps) != 1 {
		t.Errorf("Expected 1 step, got %d", len(steps))
	}

	if namespace["props"] == nil {
		t.Error("Expected props namespace even with nil With")
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

// TestDiscoverAllComponents_EmptyDirectories tests discovery with no components.
// Overrides ComponentSearchPaths to a hermetic tempdir so user/system
// component installations don't leak into the test (previously this test
// failed for anyone with components in ~/.mooncake/components or
// /usr/local/share/mooncake/components).
func TestDiscoverAllComponents_EmptyDirectories(t *testing.T) {
	componentsDir := filepath.Join(t.TempDir(), "components")
	if err := os.MkdirAll(componentsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	orig := ComponentSearchPaths
	ComponentSearchPaths = func() []string { return []string{componentsDir} }
	defer func() { ComponentSearchPaths = orig }()

	components, err := DiscoverAllComponents()
	if err != nil {
		t.Fatalf("DiscoverAllComponents failed: %v", err)
	}

	// Should return empty slice, not error
	if len(components) != 0 {
		t.Errorf("Expected 0 components, got %d", len(components))
	}
}

// TestDiscoverAllComponents_LocalComponents tests discovery of local components
func TestDiscoverAllComponents_LocalComponents(t *testing.T) {
	// Create local components directory
	componentsDir := filepath.Join(".", "components")
	os.RemoveAll(componentsDir)
	defer os.RemoveAll(componentsDir)
	os.MkdirAll(componentsDir, 0755)

	// Create test components
	component1 := filepath.Join(componentsDir, "component1.yml")
	component1Content := `name: component1
description: First component
version: 1.0.0
steps:
  - name: Step 1
    log: "test1"
`
	os.WriteFile(component1, []byte(component1Content), 0644)

	component2 := filepath.Join(componentsDir, "component2.yml")
	component2Content := `name: component2
description: Second component
version: 2.0.0
steps:
  - name: Step 1
    log: "test2"
`
	os.WriteFile(component2, []byte(component2Content), 0644)

	components, err := DiscoverAllComponents()
	if err != nil {
		t.Fatalf("DiscoverAllComponents failed: %v", err)
	}

	if len(components) < 2 {
		t.Errorf("Expected at least 2 components, got %d", len(components))
	}

	// Find our test components
	found1, found2 := false, false
	for _, p := range components {
		if p.Name == "component1" {
			found1 = true
			if p.Description != "First component" {
				t.Errorf("component1 description = %s, want 'First component'", p.Description)
			}
			if p.Version != "1.0.0" {
				t.Errorf("component1 version = %s, want '1.0.0'", p.Version)
			}
			if p.Source != "local" {
				t.Errorf("component1 source = %s, want 'local'", p.Source)
			}
		}
		if p.Name == "component2" {
			found2 = true
			if p.Description != "Second component" {
				t.Errorf("component2 description = %s, want 'Second component'", p.Description)
			}
			if p.Version != "2.0.0" {
				t.Errorf("component2 version = %s, want '2.0.0'", p.Version)
			}
		}
	}

	if !found1 {
		t.Error("component1 not found in discovered components")
	}
	if !found2 {
		t.Error("component2 not found in discovered components")
	}
}

// TestDiscoverAllComponents_DuplicateHandling tests that higher priority components override lower ones
func TestDiscoverAllComponents_DuplicateHandling(t *testing.T) {
	// Create local components directory
	componentsDir := filepath.Join(".", "components")
	os.RemoveAll(componentsDir)
	defer os.RemoveAll(componentsDir)
	os.MkdirAll(componentsDir, 0755)

	// Create local component
	localComponent := filepath.Join(componentsDir, "duplicate.yml")
	localContent := `name: duplicate
description: Local version
version: 1.0.0
steps:
  - name: Step 1
    log: "local"
`
	os.WriteFile(localComponent, []byte(localContent), 0644)

	// Note: We can't easily test actual duplicate handling with user/system components
	// without setting up those directories, but this tests the basic flow

	components, err := DiscoverAllComponents()
	if err != nil {
		t.Fatalf("DiscoverAllComponents failed: %v", err)
	}

	// Find our duplicate component (should be local version)
	found := false
	for _, p := range components {
		if p.Name == "duplicate" {
			found = true
			if p.Description != "Local version" {
				t.Errorf("Expected local version, got description: %s", p.Description)
			}
			if p.Source != "local" {
				t.Errorf("Expected source 'local', got: %s", p.Source)
			}
			break
		}
	}

	if !found {
		t.Error("duplicate component not found in discovered components")
	}
}

// TestDiscoverAllComponents_InvalidComponent tests handling of invalid component files
func TestDiscoverAllComponents_InvalidComponent(t *testing.T) {
	// Create local components directory
	componentsDir := filepath.Join(".", "components")
	os.RemoveAll(componentsDir)
	defer os.RemoveAll(componentsDir)
	os.MkdirAll(componentsDir, 0755)

	// Create valid component
	validComponent := filepath.Join(componentsDir, "valid.yml")
	validContent := `name: valid
description: Valid component
version: 1.0.0
steps:
  - name: Step 1
    log: "test"
`
	os.WriteFile(validComponent, []byte(validContent), 0644)

	// Create invalid component (missing required fields)
	invalidComponent := filepath.Join(componentsDir, "invalid.yml")
	invalidContent := `steps:
  - name: Step 1
    log: "test"
`
	os.WriteFile(invalidComponent, []byte(invalidContent), 0644)

	components, err := DiscoverAllComponents()
	if err != nil {
		t.Fatalf("DiscoverAllComponents failed: %v", err)
	}

	// Should still discover valid component, skip invalid one
	foundValid := false
	foundInvalid := false
	for _, p := range components {
		if p.Name == "valid" {
			foundValid = true
		}
		if p.Name == "invalid" {
			foundInvalid = true
		}
	}

	if !foundValid {
		t.Error("valid component should be discovered")
	}
	// Invalid component should be skipped (not cause error)
	if foundInvalid {
		t.Error("invalid component should be skipped")
	}
}

// config-json-input: component files may be authored as JSON too. The loader
// sniffs the content (not the extension) so a JSON document with .yml suffix
// still works — agents that write to the conventional path don't have to
// rename anything.
func TestLoadComponentFromPath_JSONContent(t *testing.T) {
	tmpDir := t.TempDir()
	componentPath := filepath.Join(tmpDir, "json-component.yml")

	jsonContent := `{
  "name": "json-component",
  "description": "JSON-authored component",
  "version": "1.0.0",
  "steps": [
    {"name": "Step 1", "log": {"msg": "hi"}}
  ]
}`
	if err := os.WriteFile(componentPath, []byte(jsonContent), 0644); err != nil {
		t.Fatalf("write component: %v", err)
	}

	component, err := LoadComponentFromPath(componentPath)
	if err != nil {
		t.Fatalf("LoadComponentFromPath: %v", err)
	}
	if component.Name != "json-component" {
		t.Errorf("name: got %q", component.Name)
	}
	if component.Description != "JSON-authored component" {
		t.Errorf("description: got %q", component.Description)
	}
	if len(component.Steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(component.Steps))
	}
}
