package config

import (
	"strings"
	"testing"
)

func TestDiagnostic_String(t *testing.T) {
	tests := []struct {
		name       string
		diagnostic Diagnostic
		want       string
	}{
		{
			name: "error severity",
			diagnostic: Diagnostic{
				FilePath: "/path/to/config.yml",
				Line:     10,
				Column:   5,
				Message:  "missing required field 'src'",
				Severity: "error",
			},
			want: "/path/to/config.yml:10:5: missing required field 'src'",
		},
		{
			name: "warning severity",
			diagnostic: Diagnostic{
				FilePath: "/path/to/config.yml",
				Line:     15,
				Column:   3,
				Message:  "schema validator initialization failed",
				Severity: "warning",
			},
			want: "/path/to/config.yml:15:3: warning: schema validator initialization failed",
		},
		{
			name: "no severity defaults to error",
			diagnostic: Diagnostic{
				FilePath: "/path/to/config.yml",
				Line:     20,
				Column:   1,
				Message:  "invalid syntax",
				Severity: "",
			},
			want: "/path/to/config.yml:20:1: invalid syntax",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.diagnostic.String()
			if got != tt.want {
				t.Errorf("Diagnostic.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFormatDiagnostics(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics []Diagnostic
		want        string
	}{
		{
			name:        "empty diagnostics",
			diagnostics: []Diagnostic{},
			want:        "",
		},
		{
			name: "single diagnostic",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "error message",
					Severity: "error",
				},
			},
			want: "/path/to/config.yml:10:5: error message",
		},
		{
			name: "multiple diagnostics",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "first error",
					Severity: "error",
				},
				{
					FilePath: "/path/to/config.yml",
					Line:     20,
					Column:   3,
					Message:  "second error",
					Severity: "error",
				},
			},
			want: "/path/to/config.yml:10:5: first error\n/path/to/config.yml:20:3: second error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatDiagnostics(tt.diagnostics)
			if got != tt.want {
				t.Errorf("FormatDiagnostics() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  *ValidationError
		want string
	}{
		{
			name: "no diagnostics",
			err: &ValidationError{
				Diagnostics: []Diagnostic{},
			},
			want: "validation failed with no specific errors",
		},
		{
			name: "single diagnostic",
			err: &ValidationError{
				Diagnostics: []Diagnostic{
					{
						FilePath: "/path/to/config.yml",
						Line:     10,
						Column:   5,
						Message:  "error message",
						Severity: "error",
					},
				},
			},
			want: "/path/to/config.yml:10:5: error message",
		},
		{
			name: "multiple diagnostics",
			err: &ValidationError{
				Diagnostics: []Diagnostic{
					{
						FilePath: "/path/to/config.yml",
						Line:     10,
						Column:   5,
						Message:  "first error",
						Severity: "error",
					},
					{
						FilePath: "/path/to/config.yml",
						Line:     20,
						Column:   3,
						Message:  "second error",
						Severity: "error",
					},
				},
			},
			want: "validation failed with 2 error(s):\n/path/to/config.yml:10:5: first error\n/path/to/config.yml:20:3: second error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("ValidationError.Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHasErrors(t *testing.T) {
	tests := []struct {
		name        string
		diagnostics []Diagnostic
		want        bool
	}{
		{
			name:        "empty diagnostics",
			diagnostics: []Diagnostic{},
			want:        false,
		},
		{
			name: "only warnings",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "warning message",
					Severity: "warning",
				},
			},
			want: false,
		},
		{
			name: "has errors",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "error message",
					Severity: "error",
				},
			},
			want: true,
		},
		{
			name: "unspecified severity counts as error",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "error message",
					Severity: "",
				},
			},
			want: true,
		},
		{
			name: "mixed warnings and errors",
			diagnostics: []Diagnostic{
				{
					FilePath: "/path/to/config.yml",
					Line:     10,
					Column:   5,
					Message:  "warning message",
					Severity: "warning",
				},
				{
					FilePath: "/path/to/config.yml",
					Line:     20,
					Column:   3,
					Message:  "error message",
					Severity: "error",
				},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasErrors(tt.diagnostics)
			if got != tt.want {
				t.Errorf("HasErrors() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestDiagnosticFormat verifies the exact format matches the requirement
func TestDiagnosticFormat(t *testing.T) {
	diag := Diagnostic{
		FilePath: "/Users/user/mooncake.yml",
		Line:     15,
		Column:   3,
		Message:  "Step has more than one action",
		Severity: "error",
	}

	expected := "/Users/user/mooncake.yml:15:3: Step has more than one action"
	got := diag.String()

	if got != expected {
		t.Errorf("Diagnostic format mismatch.\nGot:  %s\nWant: %s", got, expected)
	}

	// Verify format pattern: path:line:col: message
	parts := strings.Split(got, ":")
	if len(parts) < 4 {
		t.Errorf("Diagnostic format should have at least 4 parts separated by ':', got %d parts", len(parts))
	}
}
