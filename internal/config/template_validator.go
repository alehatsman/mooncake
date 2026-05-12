package config

import (
	"fmt"
	"strings"

	"github.com/flosch/pongo2/v6"
)

// TemplateValidator validates pongo2 template syntax in configuration fields
type TemplateValidator struct{}

// NewTemplateValidator creates a new template validator
func NewTemplateValidator() *TemplateValidator {
	return &TemplateValidator{}
}

// ValidateSyntax checks if a template string has valid pongo2 syntax
// Returns an error if the syntax is invalid
func (v *TemplateValidator) ValidateSyntax(template string) error {
	if template == "" {
		return nil
	}

	// Try to parse the template to check syntax
	_, err := pongo2.FromString(template)
	return err
}

// ValidateSteps validates template syntax in all templatable fields across steps
// Returns diagnostics for any syntax errors found
func (v *TemplateValidator) ValidateSteps(steps []Step, locationMap *LocationMap, filePath string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, len(steps)*5) // Preallocate with reasonable estimate

	for i, step := range steps {
		// Validate templatable fields
		stepDiagnostics := v.validateStepTemplates(step, i, locationMap, filePath)
		diagnostics = append(diagnostics, stepDiagnostics...)
	}

	return diagnostics
}

// validateField validates a single field and appends a diagnostic if there's an error
func (v *TemplateValidator) validateField(fieldValue, fieldPath, fieldDescription, filePath string, locationMap *LocationMap, diagnostics []Diagnostic) []Diagnostic {
	if fieldValue == "" {
		return diagnostics
	}

	err := v.ValidateSyntax(fieldValue)
	if err != nil {
		pos := locationMap.GetOrDefault(fieldPath, Position{Line: 1, Column: 1})
		diagnostics = append(diagnostics, Diagnostic{
			FilePath: filePath,
			Line:     pos.Line,
			Column:   pos.Column,
			Message:  fmt.Sprintf("Invalid template syntax in %s: %s", fieldDescription, formatTemplateError(err)),
			Severity: "error",
		})
	}
	return diagnostics
}

// validateStepTemplates validates all templatable fields in a single step
func (v *TemplateValidator) validateStepTemplates(step Step, stepIndex int, locationMap *LocationMap, filePath string) []Diagnostic {
	diagnostics := make([]Diagnostic, 0, 10) // Preallocate with reasonable capacity

	// Define fields that may contain template expressions
	templateFields := []struct {
		name  string
		value string
		path  string // JSON pointer path
	}{
		{"when", step.When, fmt.Sprintf("/%d/when", stepIndex)},
		{"changed_when", step.ChangedWhen, fmt.Sprintf("/%d/changed_when", stepIndex)},
		{"failed_when", step.FailedWhen, fmt.Sprintf("/%d/failed_when", stepIndex)},
		{"cwd", step.Cwd, fmt.Sprintf("/%d/cwd", stepIndex)},
		{"timeout", step.Timeout, fmt.Sprintf("/%d/timeout", stepIndex)},
		{"as_user", step.AsUser, fmt.Sprintf("/%d/as_user", stepIndex)},
	}
	if step.Retry != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"retry.delay", step.Retry.Delay, fmt.Sprintf("/%d/retry/delay", stepIndex)})
	}

	// Validate pointer fields
	if step.Shell != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"shell", step.Shell.Cmd, fmt.Sprintf("/%d/shell", stepIndex)})
	}

	if step.ForEach != nil && step.ForEach.Expr != "" {
		// Only the scalar form contains template syntax; list form is literal.
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"for_each", step.ForEach.Expr, fmt.Sprintf("/%d/for_each", stepIndex)})
	}

	if step.ForEachFile != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"for_each_file", *step.ForEachFile, fmt.Sprintf("/%d/for_each_file", stepIndex)})
	}

	if step.Import != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"import", *step.Import, fmt.Sprintf("/%d/import", stepIndex)})
	}

	if step.VarsLoad != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"vars.load", *step.VarsLoad, fmt.Sprintf("/%d/vars.load", stepIndex)})
	}

	// Validate environment variables
	for key, value := range step.Env {
		err := v.ValidateSyntax(value)
		if err != nil {
			pos := locationMap.GetOrDefault(fmt.Sprintf("/%d/env/%s", stepIndex, key), Position{Line: 1, Column: 1})
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     pos.Line,
				Column:   pos.Column,
				Message:  fmt.Sprintf("Invalid template syntax in env.%s: %s", key, formatTemplateError(err)),
				Severity: "error",
			})
		}
	}

	// Validate template action fields
	if step.FileTemplate != nil {
		diagnostics = v.validateField(step.FileTemplate.Src, fmt.Sprintf("/%d/file.template/src", stepIndex), "file.template.src", filePath, locationMap, diagnostics)
		diagnostics = v.validateField(step.FileTemplate.Dest, fmt.Sprintf("/%d/file.template/dest", stepIndex), "file.template.dest", filePath, locationMap, diagnostics)
	}

	// Validate file.write action fields
	if step.FileWrite != nil {
		diagnostics = v.validateField(step.FileWrite.Path, fmt.Sprintf("/%d/file.write/path", stepIndex), "file.write.path", filePath, locationMap, diagnostics)
		diagnostics = v.validateField(step.FileWrite.Content, fmt.Sprintf("/%d/file.write/content", stepIndex), "file.write.content", filePath, locationMap, diagnostics)
	}

	// Validate string fields
	for _, field := range templateFields {
		if field.value == "" {
			continue
		}

		err := v.ValidateSyntax(field.value)
		if err != nil {
			pos := locationMap.GetOrDefault(field.path, Position{Line: 1, Column: 1})
			diagnostics = append(diagnostics, Diagnostic{
				FilePath: filePath,
				Line:     pos.Line,
				Column:   pos.Column,
				Message:  fmt.Sprintf("Invalid template syntax in %s: %s", field.name, formatTemplateError(err)),
				Severity: "error",
			})
		}
	}

	return diagnostics
}

// formatTemplateError extracts a user-friendly message from pongo2 errors
func formatTemplateError(err error) string {
	if err == nil {
		return ""
	}

	errStr := err.Error()

	// Extract the meaningful part of the error message
	// pongo2 errors often contain "Error in ..." prefix
	if strings.Contains(errStr, "Error in") {
		parts := strings.SplitN(errStr, ":", 2)
		if len(parts) > 1 {
			return strings.TrimSpace(parts[1])
		}
	}

	return errStr
}
