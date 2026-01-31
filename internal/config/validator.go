package config

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/santhosh-tekuri/jsonschema/v5"
)

//go:embed schema.json
var schemaData string

// SchemaValidator validates configuration against JSON Schema
type SchemaValidator struct {
	schema *jsonschema.Schema
}

// NewSchemaValidator creates a new SchemaValidator with the embedded schema
func NewSchemaValidator() (*SchemaValidator, error) {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft7

	// Add the schema to the compiler
	if err := compiler.AddResource("https://mooncake.dev/schemas/config.json", strings.NewReader(schemaData)); err != nil {
		return nil, fmt.Errorf("failed to add schema resource: %w", err)
	}

	// Compile the schema
	schema, err := compiler.Compile("https://mooncake.dev/schemas/config.json")
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}

	return &SchemaValidator{
		schema: schema,
	}, nil
}

// Validate validates steps against the JSON Schema and returns diagnostics
func (v *SchemaValidator) Validate(steps []Step, locationMap *LocationMap, filePath string) []Diagnostic {
	// Convert steps to JSON for validation
	data, err := json.Marshal(steps)
	if err != nil {
		return []Diagnostic{
			{
				FilePath: filePath,
				Line:     1,
				Column:   1,
				Message:  fmt.Sprintf("failed to marshal config for validation: %v", err),
				Severity: "error",
			},
		}
	}

	// Parse JSON back to interface{} for validation
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err != nil {
		return []Diagnostic{
			{
				FilePath: filePath,
				Line:     1,
				Column:   1,
				Message:  fmt.Sprintf("failed to unmarshal config for validation: %v", err),
				Severity: "error",
			},
		}
	}

	// Validate against schema
	if err := v.schema.Validate(jsonData); err != nil {
		return mapValidationErrors(err, locationMap, filePath)
	}

	return nil
}

// mapValidationErrors converts jsonschema validation errors to diagnostics with source locations
func mapValidationErrors(err error, locationMap *LocationMap, filePath string) []Diagnostic {
	var diagnostics []Diagnostic

	validationErr, ok := err.(*jsonschema.ValidationError)
	if !ok {
		// Non-validation error
		return []Diagnostic{
			{
				FilePath: filePath,
				Line:     1,
				Column:   1,
				Message:  err.Error(),
				Severity: "error",
			},
		}
	}

	// Process the validation error and its causes recursively
	collectDiagnostics(validationErr, locationMap, filePath, &diagnostics)

	return diagnostics
}

// collectDiagnostics recursively collects diagnostics from validation errors
func collectDiagnostics(validationErr *jsonschema.ValidationError, locationMap *LocationMap, filePath string, diagnostics *[]Diagnostic) {
	// Get the instance location (JSON pointer to the problematic data)
	instancePath := validationErr.InstanceLocation

	// Look up source position
	pos := locationMap.Get(instancePath)
	if pos.Line == 0 {
		// Fallback to line 1 if position not found
		pos.Line = 1
		pos.Column = 1
	}

	// Format the error message
	message := formatValidationErrorMessage(validationErr)

	// Skip adding diagnostic if this is just a parent error with detailed causes
	// (we'll add the causes below)
	shouldAddDiagnostic := true
	if len(validationErr.Causes) > 0 {
		// Check if this is a oneOf error (which we want to report specially)
		if validationErr.KeywordLocation != "" && strings.Contains(validationErr.KeywordLocation, "/oneOf") {
			shouldAddDiagnostic = true
			message = formatOneOfError(validationErr)
		} else {
			// For other errors with causes, only report if the message adds value
			shouldAddDiagnostic = message != ""
		}
	}

	if shouldAddDiagnostic && message != "" {
		*diagnostics = append(*diagnostics, Diagnostic{
			FilePath: filePath,
			Line:     pos.Line,
			Column:   pos.Column,
			Message:  message,
			Severity: "error",
		})
	}

	// Process causes recursively (unless we already formatted them above)
	if !strings.Contains(validationErr.KeywordLocation, "/oneOf") {
		for _, cause := range validationErr.Causes {
			collectDiagnostics(cause, locationMap, filePath, diagnostics)
		}
	}
}
