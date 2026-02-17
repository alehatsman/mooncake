package actions

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// Schema represents the JSON schema structure
type Schema struct {
	Definitions map[string]ActionDefinition `json:"definitions"`
}

// ActionDefinition represents an action's schema definition
type ActionDefinition struct {
	Type        string                     `json:"type"`
	Description string                     `json:"description"`
	Properties  map[string]PropertySchema  `json:"properties"`
	Required    []string                   `json:"required"`
}

// PropertySchema represents a property's schema
type PropertySchema struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Pattern     string `json:"pattern,omitempty"`
}

var cachedSchema *Schema

// loadSchema loads and caches the JSON schema
func loadSchema() (*Schema, error) {
	if cachedSchema != nil {
		return cachedSchema, nil
	}

	var schema Schema
	if err := json.Unmarshal(config.SchemaJSON(), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse schema: %w", err)
	}

	cachedSchema = &schema
	return &schema, nil
}

// GetActionHint generates a helpful hint for an action based on the schema.
// It includes:
// - Action description
// - Required parameters with descriptions
// - Optional parameters with descriptions
// - Examples where available
func GetActionHint(actionName string, missingField string) string {
	schema, err := loadSchema()
	if err != nil {
		// Fallback to basic message if schema can't be loaded
		return ""
	}

	actionDef, exists := schema.Definitions[actionName]
	if !exists {
		return ""
	}

	var hint strings.Builder
	hint.WriteString("\n\n")

	// Add action description if available
	if actionDef.Description != "" {
		hint.WriteString(fmt.Sprintf("The '%s' action: %s\n\n", actionName, actionDef.Description))
	}

	// Build lists of required and optional fields
	requiredFields := make(map[string]bool)
	for _, req := range actionDef.Required {
		requiredFields[req] = true
	}

	var required, optional []string
	for fieldName := range actionDef.Properties {
		if requiredFields[fieldName] {
			required = append(required, fieldName)
		} else {
			optional = append(optional, fieldName)
		}
	}

	// Sort for consistent output
	sort.Strings(required)
	sort.Strings(optional)

	// Show required parameters
	if len(required) > 0 {
		hint.WriteString("Required parameters:\n")
		for _, field := range required {
			prop := actionDef.Properties[field]
			desc := prop.Description
			if desc == "" {
				desc = prop.Type
			}

			// Add missing indicator if this is the missing field
			missing := ""
			if field == missingField {
				missing = " ← MISSING"
			}

			hint.WriteString(fmt.Sprintf("  - %s: %s%s\n", field, desc, missing))

			// Add pattern hint for special cases
			if prop.Pattern != "" {
				hint.WriteString(fmt.Sprintf("    Pattern: %s\n", prop.Pattern))
			}
		}
	}

	// Show optional parameters
	if len(optional) > 0 {
		hint.WriteString("\nOptional parameters:\n")
		for _, field := range optional {
			prop := actionDef.Properties[field]
			desc := prop.Description
			if desc == "" {
				desc = prop.Type
			}
			hint.WriteString(fmt.Sprintf("  - %s: %s\n", field, desc))
		}
	}

	return hint.String()
}

// GetFieldExample returns an example value for a field based on schema
func GetFieldExample(actionName, fieldName string) string {
	schema, err := loadSchema()
	if err != nil {
		return ""
	}

	actionDef, exists := schema.Definitions[actionName]
	if !exists {
		return ""
	}

	prop, exists := actionDef.Properties[fieldName]
	if !exists {
		return ""
	}

	// Generate example based on field name and type
	switch fieldName {
	case "path":
		return `path: "/tmp/myfile"`
	case "src":
		return `src: "./source/file"`
	case "dest":
		return `dest: "/destination/file"`
	case "url":
		return `url: "https://example.com/file"`
	case "mode":
		return `mode: "0644"`
	case "state":
		return `state: directory`
	case "cmd", "command":
		return `cmd: "echo hello"`
	default:
		// Generic example based on type
		switch prop.Type {
		case "string":
			return fmt.Sprintf(`%s: "value"`, fieldName)
		case "boolean":
			return fmt.Sprintf(`%s: true`, fieldName)
		case "integer":
			return fmt.Sprintf(`%s: 123`, fieldName)
		default:
			return ""
		}
	}
}
