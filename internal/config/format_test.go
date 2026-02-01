package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterUserFriendlyDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics []Diagnostic
		expected    int
	}{
		{
			name: "filter out schema validation messages",
			diagnostics: []Diagnostic{
				{Message: "doesn't validate with #/definitions/step"},
				{Message: "Missing required field 'shell'"},
				{Message: "invalid value from https://mooncake.dev/schemas/v1"},
			},
			expected: 1, // Only "Missing required field 'shell'" should remain
		},
		{
			name: "keep user-friendly messages",
			diagnostics: []Diagnostic{
				{Message: "Missing required field 'src'"},
				{Message: "Unknown field 'command'"},
				{Message: "Invalid file state"},
			},
			expected: 3,
		},
		{
			name: "filter definitions messages except step must have",
			diagnostics: []Diagnostic{
				{Message: "some message with /definitions/ in it"},
				{Message: "step must have exactly one action"},
				{Message: "Invalid value"},
			},
			expected: 2, // Filter out definitions message, keep the other two
		},
		{
			name:        "empty diagnostics",
			diagnostics: []Diagnostic{},
			expected:    0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := filterUserFriendlyDiagnostics(tt.diagnostics)
			if len(result) != tt.expected {
				t.Errorf("filterUserFriendlyDiagnostics() returned %d diagnostics, want %d", len(result), tt.expected)
			}
		})
	}
}

func TestExtractStepName(t *testing.T) {
	tests := []struct {
		name      string
		lines     []string
		errorLine int
		expected  string
	}{
		{
			name: "find name when not on step start",
			lines: []string{
				"steps:",
				"  - shell: test",
				"    name: Install nginx",
				"    invalid_field: value",
			},
			errorLine: 4,
			expected:  "Install nginx",
		},
		{
			name: "find name with quotes",
			lines: []string{
				"steps:",
				"  - shell: test",
				"    name: 'Deploy app'",
				"    template:",
			},
			errorLine: 4,
			expected:  "Deploy app",
		},
		{
			name: "find name with double quotes",
			lines: []string{
				"steps:",
				"  - shell: test",
				`    name: "Create directory"`,
				"    file:",
			},
			errorLine: 4,
			expected:  "Create directory",
		},
		{
			name: "no name found",
			lines: []string{
				"steps:",
				"  - shell: echo hello",
				"    become: true",
			},
			errorLine: 3,
			expected:  "",
		},
		{
			name: "stop at previous step boundary",
			lines: []string{
				"steps:",
				"  - name: First step",
				"    shell: echo first",
				"  - shell: second",
				"    name: Second step",
				"    invalid_field: value",
			},
			errorLine: 6,
			expected:  "Second step",
		},
		{
			name: "error line out of range",
			lines: []string{
				"  - name: Test step",
				"    shell: test",
			},
			errorLine: 10,
			expected:  "",
		},
		{
			name: "error line is zero",
			lines: []string{
				"  - name: Test step",
				"    shell: test",
			},
			errorLine: 0,
			expected:  "",
		},
		{
			name: "look back maximum 10 lines",
			lines: []string{
				"steps:",
				"  - name: Far away step",
				"    shell: line1",
				"    field2: line2",
				"    field3: line3",
				"    field4: line4",
				"    field5: line5",
				"    field6: line6",
				"    field7: line7",
				"    field8: line8",
				"    field9: line9",
				"    field10: line10",
				"    field11: line11",
				"    invalid_field: value",
			},
			errorLine: 15,
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractStepName(tt.lines, tt.errorLine)
			if result != tt.expected {
				t.Errorf("extractStepName() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestLoadFileLines(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yml")

	content := `steps:
  - name: Test step
    shell: echo hello
    invalid: field`

	err := os.WriteFile(tmpFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name          string
		filePath      string
		expectedLines int
		shouldBeEmpty bool
	}{
		{
			name:          "valid file",
			filePath:      tmpFile,
			expectedLines: 4,
			shouldBeEmpty: false,
		},
		{
			name:          "non-existent file",
			filePath:      filepath.Join(tmpDir, "nonexistent.yml"),
			expectedLines: 0,
			shouldBeEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := loadFileLines(tt.filePath)
			if tt.shouldBeEmpty {
				if len(result) != 0 {
					t.Errorf("loadFileLines() should return empty slice for non-existent file, got %d lines", len(result))
				}
			} else {
				if len(result) != tt.expectedLines {
					t.Errorf("loadFileLines() returned %d lines, want %d", len(result), tt.expectedLines)
				}
			}
		})
	}
}

func TestFormatDiagnosticsWithContext(t *testing.T) {
	// Create a temporary file for testing
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.yml")

	content := `steps:
  - name: Test step
    shell: echo hello
    invalid: field`

	err := os.WriteFile(tmpFile, []byte(content), 0600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	tests := []struct {
		name        string
		diagnostics []Diagnostic
		expectEmpty bool
		expectError bool
	}{
		{
			name:        "empty diagnostics",
			diagnostics: []Diagnostic{},
			expectEmpty: true,
		},
		{
			name: "single diagnostic",
			diagnostics: []Diagnostic{
				{
					FilePath: tmpFile,
					Line:     4,
					Message:  "Unknown field 'invalid'",
					Severity: "error",
				},
			},
			expectEmpty: false,
			expectError: true,
		},
		{
			name: "multiple diagnostics",
			diagnostics: []Diagnostic{
				{
					FilePath: tmpFile,
					Line:     3,
					Message:  "Missing required field",
					Severity: "error",
				},
				{
					FilePath: tmpFile,
					Line:     4,
					Message:  "Unknown field",
					Severity: "error",
				},
			},
			expectEmpty: false,
			expectError: true,
		},
		{
			name: "filter out technical messages",
			diagnostics: []Diagnostic{
				{
					FilePath: tmpFile,
					Line:     3,
					Message:  "doesn't validate with https://mooncake.dev/schemas/v1",
					Severity: "error",
				},
			},
			expectEmpty: true,
		},
		{
			name: "warning and error",
			diagnostics: []Diagnostic{
				{
					FilePath: tmpFile,
					Line:     3,
					Message:  "Deprecated field",
					Severity: "warning",
				},
				{
					FilePath: tmpFile,
					Line:     4,
					Message:  "Invalid value",
					Severity: "error",
				},
			},
			expectEmpty: false,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FormatDiagnosticsWithContext(tt.diagnostics)
			if tt.expectEmpty {
				if result != "" {
					t.Errorf("FormatDiagnosticsWithContext() should return empty string, got %q", result)
				}
			} else {
				if result == "" {
					t.Error("FormatDiagnosticsWithContext() should return non-empty string")
				}
				if tt.expectError && !strings.Contains(result, "error") {
					t.Error("FormatDiagnosticsWithContext() should mention errors")
				}
			}
		})
	}
}
