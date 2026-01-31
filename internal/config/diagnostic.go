package config

import (
	"fmt"
	"strings"
)

// Diagnostic represents a validation error or warning with source location
type Diagnostic struct {
	FilePath string
	Line     int
	Column   int
	Message  string
	Severity string // "error" or "warning"
}

// String formats the diagnostic as "path/to/file.yml:line:col: message"
func (d *Diagnostic) String() string {
	severity := ""
	if d.Severity != "" && d.Severity != "error" {
		severity = d.Severity + ": "
	}
	return fmt.Sprintf("%s:%d:%d: %s%s", d.FilePath, d.Line, d.Column, severity, d.Message)
}

// FormatDiagnostics formats multiple diagnostics as a newline-separated string
func FormatDiagnostics(diagnostics []Diagnostic) string {
	if len(diagnostics) == 0 {
		return ""
	}

	var builder strings.Builder
	for i, diag := range diagnostics {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(diag.String())
	}
	return builder.String()
}

// ValidationError wraps multiple diagnostics into a single error
type ValidationError struct {
	Diagnostics []Diagnostic
}

// Error implements the error interface
func (e *ValidationError) Error() string {
	if len(e.Diagnostics) == 0 {
		return "validation failed with no specific errors"
	}
	if len(e.Diagnostics) == 1 {
		return e.Diagnostics[0].String()
	}
	return fmt.Sprintf("validation failed with %d error(s):\n%s",
		len(e.Diagnostics),
		FormatDiagnostics(e.Diagnostics))
}

// HasErrors returns true if any diagnostic has severity "error" or unspecified severity
func HasErrors(diagnostics []Diagnostic) bool {
	for _, diag := range diagnostics {
		if diag.Severity == "" || diag.Severity == "error" {
			return true
		}
	}
	return false
}
