package docgen

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// ActionProperties represents parsed action properties from schema.
type ActionProperties struct {
	Name        string
	Description string
	Properties  []PropertyDef
	Category    string
	Version     string
}

// PropertyDef represents a single property definition.
type PropertyDef struct {
	Name        string
	Type        string
	Description string
	Required    bool
	Default     string
	Enum        []string
}

// SchemaDefinition represents the JSON schema structure.
type SchemaDefinition struct {
	Definitions map[string]ActionDefinition `json:"definitions"`
}

// ActionDefinition represents an action in the schema.
type ActionDefinition struct {
	Type                 string                       `json:"type"`
	Description          string                       `json:"description"`
	Properties           map[string]PropertySchema    `json:"properties"`
	Required             []string                     `json:"required"`
	AdditionalProperties interface{}                  `json:"additionalProperties"`
	Category             string                       `json:"x-category"`
	Version              string                       `json:"x-version"`
}

// PropertySchema represents a property in the schema.
type PropertySchema struct {
	Type        interface{} `json:"type"` // Can be string or array
	Description string      `json:"description"`
	Default     interface{} `json:"default"`
	Enum        []interface{} `json:"enum"`
	Ref         string      `json:"$ref"`
	Properties  map[string]PropertySchema `json:"properties"`
	Items       *PropertySchema `json:"items"`
}

// generateActionProperties generates properties tables from schema.json.
func (g *Generator) generateActionProperties(w io.Writer) error {
	// Load and parse schema
	schema, err := loadSchema()
	if err != nil {
		return fmt.Errorf("failed to load schema: %w", err)
	}

	// Extract action properties
	actions := extractActionProperties(schema)

	// Sort actions by name
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].Name < actions[j].Name
	})

	// Generate header with title and description
	write(w, "# Action Properties Reference\n\n")
	g.writeMetadataHeader(w)
	write(w, "This document is auto-generated from `internal/config/schema.json`.\n")
	write(w, "Properties are guaranteed to match the schema definition.\n\n")

	// Generate properties table for each action
	for i, action := range actions {
		if i > 0 {
			write(w, "\n\n---\n\n")
		}
		g.writeActionProperties(w, action)
	}

	return nil
}

// loadSchema loads the JSON schema from embedded data (via config package).
func loadSchema() (*SchemaDefinition, error) {
	var schema SchemaDefinition
	if err := json.Unmarshal(config.SchemaJSON(), &schema); err != nil {
		return nil, fmt.Errorf("failed to parse embedded schema: %w", err)
	}
	return &schema, nil
}

// extractActionProperties extracts action properties from schema definitions.
func extractActionProperties(schema *SchemaDefinition) []ActionProperties {
	// Dynamically extract action names from schema definitions
	actionNames := make([]string, 0, len(schema.Definitions))
	for name, def := range schema.Definitions {
		// Only include action definitions (type: object with properties or category)
		if def.Type == "object" && (len(def.Properties) > 0 || def.Category != "") {
			actionNames = append(actionNames, name)
		}
	}

	// Sort for consistent output
	sort.Strings(actionNames)

	// Preallocate actions slice
	actions := make([]ActionProperties, 0, len(actionNames))

	for _, name := range actionNames {
		def := schema.Definitions[name]

		action := ActionProperties{
			Name:        name,
			Description: def.Description,
			Category:    def.Category,
			Version:     def.Version,
		}

		// Extract properties
		props := make([]PropertyDef, 0, len(def.Properties))
		for propName, propSchema := range def.Properties {
			prop := PropertyDef{
				Name:        propName,
				Type:        formatType(propSchema.Type),
				Description: propSchema.Description,
				Required:    contains(def.Required, propName),
			}

			// Format default value
			if propSchema.Default != nil {
				prop.Default = fmt.Sprintf("%v", propSchema.Default)
			}

			// Format enum values
			if len(propSchema.Enum) > 0 {
				for _, e := range propSchema.Enum {
					prop.Enum = append(prop.Enum, fmt.Sprintf("%v", e))
				}
			}

			props = append(props, prop)
		}

		// Sort properties by name
		sort.Slice(props, func(i, j int) bool {
			return props[i].Name < props[j].Name
		})

		action.Properties = props
		actions = append(actions, action)
	}

	return actions
}

// formatType converts JSON schema type to readable format.
func formatType(t interface{}) string {
	switch v := t.(type) {
	case string:
		return v
	case []interface{}:
		var types []string
		for _, item := range v {
			if s, ok := item.(string); ok {
				types = append(types, s)
			}
		}
		return strings.Join(types, " or ")
	default:
		return "any"
	}
}

// contains checks if a slice contains a string.
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// writeActionProperties writes properties table for a single action.
func (g *Generator) writeActionProperties(w io.Writer, action ActionProperties) {
	// Capitalize first letter only (simple title case for action names)
	title := action.Name
	if len(title) > 0 {
		title = strings.ToUpper(title[:1]) + title[1:]
	}
	write(w, "## %s\n\n", title)

	if action.Description != "" {
		write(w, "%s\n\n", action.Description)
	}

	if len(action.Properties) == 0 {
		write(w, "*No properties defined in schema.*\n")
		return
	}

	// Properties table
	write(w, "| Property | Type | Required | Description |\n")
	write(w, "|----------|------|----------|-------------|\n")

	for _, prop := range action.Properties {
		required := "No"
		if prop.Required {
			required = "**Yes**"
		}

		description := prop.Description
		if description == "" {
			description = "-"
		}

		// Add default value if present
		if prop.Default != "" {
			description += fmt.Sprintf(" (default: `%s`)", prop.Default)
		}

		// Add enum values if present
		if len(prop.Enum) > 0 {
			enumStr := strings.Join(prop.Enum, ", ")
			description += fmt.Sprintf(" (allowed: `%s`)", enumStr)
		}

		write(w, "| `%s` | %s | %s | %s |\n",
			prop.Name,
			prop.Type,
			required,
			description)
	}

	// Add metadata if available
	if action.Category != "" || action.Version != "" {
		write(w, "\n**Metadata:**\n")
		if action.Category != "" {
			write(w, "- Category: `%s`\n", action.Category)
		}
		if action.Version != "" {
			write(w, "- Version: `%s`\n", action.Version)
		}
	}
}
