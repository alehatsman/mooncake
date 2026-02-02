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
		{"retry_delay", step.RetryDelay, fmt.Sprintf("/%d/retry_delay", stepIndex)},
		{"become_user", step.BecomeUser, fmt.Sprintf("/%d/become_user", stepIndex)},
	}

	// Validate pointer fields
	if step.Shell != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"shell", step.Shell.Cmd, fmt.Sprintf("/%d/shell", stepIndex)})
	}

	if step.WithItems != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"with_items", *step.WithItems, fmt.Sprintf("/%d/with_items", stepIndex)})
	}

	if step.WithFileTree != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"with_filetree", *step.WithFileTree, fmt.Sprintf("/%d/with_filetree", stepIndex)})
	}

	if step.Include != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"include", *step.Include, fmt.Sprintf("/%d/include", stepIndex)})
	}

	if step.IncludeVars != nil {
		templateFields = append(templateFields, struct {
			name  string
			value string
			path  string
		}{"include_vars", *step.IncludeVars, fmt.Sprintf("/%d/include_vars", stepIndex)})
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
	if step.Template != nil {
		diagnostics = v.validateField(step.Template.Src, fmt.Sprintf("/%d/template/src", stepIndex), "template.src", filePath, locationMap, diagnostics)
		diagnostics = v.validateField(step.Template.Dest, fmt.Sprintf("/%d/template/dest", stepIndex), "template.dest", filePath, locationMap, diagnostics)
	}

	// Validate file action fields
	if step.File != nil {
		diagnostics = v.validateField(step.File.Path, fmt.Sprintf("/%d/file/path", stepIndex), "file.path", filePath, locationMap, diagnostics)
		diagnostics = v.validateField(step.File.Content, fmt.Sprintf("/%d/file/content", stepIndex), "file.content", filePath, locationMap, diagnostics)
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
