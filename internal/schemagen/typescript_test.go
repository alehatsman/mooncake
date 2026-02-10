package schemagen

import (
	"strings"
	"testing"
)

// TestGenerateTypeScript tests the complete TypeScript generation.
func TestGenerateTypeScript(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Verify basic structure
	if !strings.Contains(tsContent, "/**") {
		t.Error("TypeScript output missing JSDoc comments")
	}

	if !strings.Contains(tsContent, "export interface") {
		t.Error("TypeScript output missing interface declarations")
	}

	if !strings.Contains(tsContent, "export type MooncakeConfig = Step[];") {
		t.Error("TypeScript output missing MooncakeConfig type")
	}

	// Verify file header
	if !strings.Contains(tsContent, "TypeScript definitions for Mooncake configuration") {
		t.Error("TypeScript output missing file header")
	}

	if !strings.Contains(tsContent, "Auto-generated from action metadata") {
		t.Error("TypeScript output missing auto-generation warning")
	}
}

// TestGenerateTypeScriptActionInterfaces tests that all actions get interfaces.
func TestGenerateTypeScriptActionInterfaces(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Check for key action interfaces (PascalCase)
	expectedInterfaces := []string{
		"ShellAction",
		"CommandAction",
		"FileAction",
		"TemplateAction",
		"CopyAction",
		"DownloadAction",
		"UnarchiveAction",
		"ServiceAction",
		"AssertAction",
		"PresetAction",
		"PrintAction",
		"VarsAction",
		"IncludeVarsAction",
	}

	for _, interfaceName := range expectedInterfaces {
		if !strings.Contains(tsContent, "export interface "+interfaceName) {
			t.Errorf("TypeScript output missing interface: %s", interfaceName)
		}
	}
}

// TestGenerateTypeScriptStepInterface tests the Step interface generation.
func TestGenerateTypeScriptStepInterface(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Check for Step interface
	if !strings.Contains(tsContent, "export interface Step {") {
		t.Error("TypeScript output missing Step interface")
	}

	// Check for universal fields
	universalFields := []string{
		"name?:",
		"when?:",
		"creates?:",
		"unless?:",
		"become?:",
		"tags?:",
		"register?:",
		"with_filetree?:",
		"with_items?:",
		"env?:",
		"cwd?:",
		"timeout?:",
		"retries?:",
		"retry_delay?:",
		"changed_when?:",
		"failed_when?:",
		"become_user?:",
		"include?:",
	}

	for _, field := range universalFields {
		if !strings.Contains(tsContent, field) {
			t.Errorf("TypeScript Step interface missing field: %s", field)
		}
	}

	// Check for action fields (optional)
	actionFields := []string{
		"shell?:",
		"command?:",
		"file?:",
		"template?:",
		"service?:",
		"preset?:",
	}

	for _, field := range actionFields {
		if !strings.Contains(tsContent, field) {
			t.Errorf("TypeScript Step interface missing action field: %s", field)
		}
	}
}

// TestGenerateTypeScriptJSDocComments tests JSDoc comment generation.
func TestGenerateTypeScriptJSDocComments(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Check for JSDoc tags
	jsdocTags := []string{
		"@platforms",
		"@category",
		"@values",
	}

	for _, tag := range jsdocTags {
		if !strings.Contains(tsContent, tag) {
			t.Errorf("TypeScript output missing JSDoc tag: %s", tag)
		}
	}
}

// TestGenerateTypeScriptEnumTypes tests enum as union types.
func TestGenerateTypeScriptEnumTypes(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Check for union type syntax (enum values as string literals)
	if !strings.Contains(tsContent, `"present" | "absent"`) &&
		!strings.Contains(tsContent, `"absent" | "present"`) {
		// Order might vary due to sorting
		if !strings.Contains(tsContent, `"present"`) || !strings.Contains(tsContent, `"absent"`) {
			t.Error("TypeScript output missing enum union types")
		}
	}
}

// TestGenerateTypeScriptArrayTypes tests array type generation.
func TestGenerateTypeScriptArrayTypes(t *testing.T) {
	generator := NewGenerator(GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
	})

	tsContent, err := generator.GenerateTypeScript()
	if err != nil {
		t.Fatalf("GenerateTypeScript() failed: %v", err)
	}

	// Check for array syntax (T[])
	if !strings.Contains(tsContent, "string[]") {
		t.Error("TypeScript output missing string array type")
	}
}

// TestToPascalCase tests the PascalCase conversion utility.
func TestToPascalCase(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"shell", "Shell"},
		{"shell_action", "ShellAction"},
		{"include_vars", "IncludeVars"},
		{"my_action_name", "MyActionName"},
		{"", ""},
		{"a", "A"},
	}

	for _, tt := range tests {
		result := toPascalCase(tt.input)
		if result != tt.expected {
			t.Errorf("toPascalCase(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

// TestWrapText tests the text wrapping utility.
func TestWrapText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		width    int
		expected string
	}{
		{
			name:     "short text",
			input:    "Hello world",
			width:    50,
			expected: "Hello world",
		},
		{
			name:     "exact width",
			input:    "This is exactly fifty characters long text here!",
			width:    50,
			expected: "This is exactly fifty characters long text here!",
		},
		{
			name:     "needs wrapping",
			input:    "This is a very long text that should be wrapped at the specified width",
			width:    30,
			expected: "This is a very long text that\nshould be wrapped at the\nspecified width",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := wrapText(tt.input, tt.width)
			if result != tt.expected {
				t.Errorf("wrapText() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// TestGenerateTypeScriptPropertyTypes tests various property type conversions.
func TestGenerateTypeScriptPropertyTypes(t *testing.T) {
	gen := &TypeScriptGenerator{
		schema: &Schema{
			Definitions: make(map[string]*Definition),
		},
	}

	tests := []struct {
		name     string
		prop     *Property
		expected string
	}{
		{
			name:     "string type",
			prop:     &Property{Type: "string"},
			expected: "string",
		},
		{
			name:     "number type",
			prop:     &Property{Type: "number"},
			expected: "number",
		},
		{
			name:     "integer type",
			prop:     &Property{Type: "integer"},
			expected: "number",
		},
		{
			name:     "boolean type",
			prop:     &Property{Type: "boolean"},
			expected: "boolean",
		},
		{
			name: "array of strings",
			prop: &Property{
				Type:  "array",
				Items: &Property{Type: "string"},
			},
			expected: "string[]",
		},
		{
			name: "object type",
			prop: &Property{Type: "object"},
			expected: "object",
		},
		{
			name: "enum type",
			prop: &Property{
				Type: "string",
				Enum: []interface{}{"present", "absent"},
			},
			expected: `"present" | "absent"`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.propertyToTypeScript(tt.prop)
			if result != tt.expected {
				t.Errorf("propertyToTypeScript() = %q, want %q", result, tt.expected)
			}
		})
	}
}
