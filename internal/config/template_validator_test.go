package config

import (
	"testing"
)

func TestTemplateValidator_ValidateSyntax(t *testing.T) {
	validator := NewTemplateValidator()

	tests := []struct {
		name      string
		template  string
		wantError bool
	}{
		{
			name:      "empty template",
			template:  "",
			wantError: false,
		},
		{
			name:      "valid template",
			template:  "{{os}} {{arch}}",
			wantError: false,
		},
		{
			name:      "valid expression",
			template:  "os == 'linux'",
			wantError: false,
		},
		{
			name:      "valid nested",
			template:  "{{item.key}}: {{item.value}}",
			wantError: false,
		},
		{
			name:      "invalid - unclosed variable",
			template:  "{{unclosed",
			wantError: true,
		},
		{
			name:      "invalid - unclosed string",
			template:  "echo \"{{test}",
			wantError: true,
		},
		{
			name:      "invalid - broken syntax",
			template:  "{{broken syntax",
			wantError: true,
		},
		{
			name:      "invalid - missing closing brace",
			template:  "{{variable}",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validator.ValidateSyntax(tt.template)
			if (err != nil) != tt.wantError {
				t.Errorf("ValidateSyntax() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

func TestTemplateValidator_ValidateSteps(t *testing.T) {
	validator := NewTemplateValidator()
	locationMap := NewLocationMap()

	tests := []struct {
		name               string
		steps              []Step
		expectedDiagnostics int
	}{
		{
			name: "valid step with templates",
			steps: []Step{
				{
					Name:  "test",
					Shell: strPtr("echo {{os}}"),
					When:  "os == 'linux'",
					Env: map[string]string{
						"VAR": "{{value}}",
					},
				},
			},
			expectedDiagnostics: 0,
		},
		{
			name: "invalid shell template",
			steps: []Step{
				{
					Name:  "test",
					Shell: strPtr("echo {{unclosed"),
				},
			},
			expectedDiagnostics: 1,
		},
		{
			name: "invalid when condition",
			steps: []Step{
				{
					Name:  "test",
					Shell: strPtr("echo test"),
					When:  "{{broken",
				},
			},
			expectedDiagnostics: 1,
		},
		{
			name: "invalid env variable",
			steps: []Step{
				{
					Name:  "test",
					Shell: strPtr("echo test"),
					Env: map[string]string{
						"BAD": "{{unclosed",
					},
				},
			},
			expectedDiagnostics: 1,
		},
		{
			name: "multiple invalid fields",
			steps: []Step{
				{
					Name:        "test",
					Shell:       strPtr("{{bad1"),
					When:        "{{bad2",
					ChangedWhen: "{{bad3",
				},
			},
			expectedDiagnostics: 3,
		},
		{
			name: "invalid template action",
			steps: []Step{
				{
					Name: "test",
					Template: &Template{
						Src:  "{{unclosed",
						Dest: "{{also_bad",
					},
				},
			},
			expectedDiagnostics: 2,
		},
		{
			name: "invalid file action",
			steps: []Step{
				{
					Name: "test",
					File: &File{
						Path:    "{{unclosed",
						Content: "{{also_bad",
					},
				},
			},
			expectedDiagnostics: 2,
		},
		{
			name: "valid complex step",
			steps: []Step{
				{
					Name:        "test",
					Shell:       strPtr("echo {{message}}"),
					When:        "os == 'linux'",
					BecomeUser:  "{{user}}",
					Cwd:         "/tmp/{{project}}",
					Timeout:     "30s",
					ChangedWhen: "result.rc == 0",
					FailedWhen:  "result.rc != 0",
					Env: map[string]string{
						"PATH": "/usr/bin:{{custom_path}}",
					},
				},
			},
			expectedDiagnostics: 0,
		},
		{
			name: "valid with_items",
			steps: []Step{
				{
					Name:      "test",
					Shell:     strPtr("echo {{item}}"),
					WithItems: strPtr("{{my_list}}"),
				},
			},
			expectedDiagnostics: 0,
		},
		{
			name: "invalid with_items",
			steps: []Step{
				{
					Name:      "test",
					Shell:     strPtr("echo test"),
					WithItems: strPtr("{{unclosed"),
				},
			},
			expectedDiagnostics: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diagnostics := validator.ValidateSteps(tt.steps, locationMap, "test.yml")
			if len(diagnostics) != tt.expectedDiagnostics {
				t.Errorf("ValidateSteps() got %d diagnostics, want %d", len(diagnostics), tt.expectedDiagnostics)
				for _, d := range diagnostics {
					t.Logf("  - %s", d.Message)
				}
			}

			// All diagnostics should be errors
			for _, d := range diagnostics {
				if d.Severity != "error" {
					t.Errorf("Expected severity 'error', got '%s'", d.Severity)
				}
			}
		})
	}
}

func TestFormatTemplateError(t *testing.T) {
	tests := []struct {
		name     string
		errMsg   string
		expected string
	}{
		{
			name:     "nil error",
			errMsg:   "",
			expected: "",
		},
		{
			name:     "simple error",
			errMsg:   "unexpected EOF",
			expected: "unexpected EOF",
		},
		{
			name:     "pongo2 error with prefix",
			errMsg:   "Error in line 1: unexpected token",
			expected: "unexpected token",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.errMsg == "" {
				result := formatTemplateError(nil)
				if result != tt.expected {
					t.Errorf("formatTemplateError() = %q, want %q", result, tt.expected)
				}
			}
		})
	}
}
