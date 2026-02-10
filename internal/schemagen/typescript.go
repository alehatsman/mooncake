package schemagen

import (
	"fmt"
	"sort"
	"strings"
)

// TypeScriptGenerator generates TypeScript definitions from schema.
type TypeScriptGenerator struct {
	schema *Schema
}

// GenerateTypeScript generates TypeScript definitions from a schema.
func (s *Schema) GenerateTypeScript() string {
	gen := &TypeScriptGenerator{schema: s}
	return gen.generate()
}

// generate creates the complete TypeScript definition file.
func (g *TypeScriptGenerator) generate() string {
	var b strings.Builder

	// File header
	b.WriteString("/**\n")
	b.WriteString(" * TypeScript definitions for Mooncake configuration\n")
	b.WriteString(" * \n")
	b.WriteString(" * Auto-generated from action metadata.\n")
	b.WriteString(" * Do not edit manually - regenerate with: mooncake schema generate --format typescript\n")
	b.WriteString(" */\n\n")

	// Generate interfaces for each action
	actionNames := make([]string, 0, len(g.schema.Definitions))
	for name := range g.schema.Definitions {
		if name != "step" { // Skip step definition, handle separately
			actionNames = append(actionNames, name)
		}
	}
	sort.Strings(actionNames)

	for _, name := range actionNames {
		def := g.schema.Definitions[name]
		g.generateInterface(&b, name, def)
		b.WriteString("\n")
	}

	// Generate Step interface
	if stepDef, ok := g.schema.Definitions["step"]; ok {
		g.generateStepInterface(&b, stepDef)
	}

	// Generate Config type
	b.WriteString("/**\n")
	b.WriteString(" * Complete mooncake configuration\n")
	b.WriteString(" */\n")
	b.WriteString("export type MooncakeConfig = Step[];\n\n")

	// Export default
	b.WriteString("export default MooncakeConfig;\n")

	return b.String()
}

// generateInterface creates a TypeScript interface for an action.
func (g *TypeScriptGenerator) generateInterface(b *strings.Builder, name string, def *Definition) {
	// Convert name to PascalCase for interface name
	interfaceName := toPascalCase(name) + "Action"

	// JSDoc comment
	b.WriteString("/**\n")
	if def.Description != "" {
		b.WriteString(" * " + def.Description + "\n")
	}
	if len(def.XPlatforms) > 0 {
		b.WriteString(" * \n")
		b.WriteString(" * @platforms " + strings.Join(def.XPlatforms, ", ") + "\n")
	}
	if def.XRequiresSudo {
		b.WriteString(" * @requiresSudo true\n")
	}
	if def.XCategory != "" {
		b.WriteString(" * @category " + def.XCategory + "\n")
	}
	b.WriteString(" */\n")

	// Interface declaration
	fmt.Fprintf(b, "export interface %s {\n", interfaceName)

	// Sort properties for consistent output
	propNames := make([]string, 0, len(def.Properties))
	for propName := range def.Properties {
		propNames = append(propNames, propName)
	}
	sort.Strings(propNames)

	// Required fields map
	requiredMap := make(map[string]bool)
	for _, req := range def.Required {
		requiredMap[req] = true
	}

	// Generate properties
	for _, propName := range propNames {
		prop := def.Properties[propName]
		g.generateProperty(b, propName, prop, requiredMap[propName])
	}

	b.WriteString("}\n")
}

// generateProperty creates a TypeScript property definition.
func (g *TypeScriptGenerator) generateProperty(b *strings.Builder, name string, prop *Property, required bool) {
	// JSDoc comment for property
	if prop.Description != "" || len(prop.Enum) > 0 || prop.Default != nil {
		b.WriteString("  /**\n")
		if prop.Description != "" {
			// Wrap long descriptions
			desc := wrapText(prop.Description, 70)
			for _, line := range strings.Split(desc, "\n") {
				b.WriteString("   * " + line + "\n")
			}
		}
		if len(prop.Enum) > 0 {
			enumStrs := make([]string, len(prop.Enum))
			for i, v := range prop.Enum {
				enumStrs[i] = fmt.Sprintf("%v", v)
			}
			b.WriteString("   * \n")
			b.WriteString("   * @values " + strings.Join(enumStrs, " | ") + "\n")
		}
		if prop.Default != nil {
			fmt.Fprintf(b, "   * @default %v\n", prop.Default)
		}
		b.WriteString("   */\n")
	}

	// Property declaration
	optional := ""
	if !required {
		optional = "?"
	}

	tsType := g.propertyToTypeScript(prop)
	fmt.Fprintf(b, "  %s%s: %s;\n", name, optional, tsType)
}

// propertyToTypeScript converts a Property to TypeScript type string.
func (g *TypeScriptGenerator) propertyToTypeScript(prop *Property) string {
	// Handle $ref
	if prop.Ref != "" {
		// Extract type name from ref
		parts := strings.Split(prop.Ref, "/")
		if len(parts) > 0 {
			typeName := parts[len(parts)-1]
			return toPascalCase(typeName) + "Action"
		}
	}

	// Handle oneOf (union types)
	if len(prop.OneOf) > 0 {
		types := make([]string, len(prop.OneOf))
		for i, oneOfProp := range prop.OneOf {
			types[i] = g.propertyToTypeScript(oneOfProp)
		}
		return strings.Join(types, " | ")
	}

	// Handle enum
	if len(prop.Enum) > 0 {
		enumValues := make([]string, len(prop.Enum))
		for i, v := range prop.Enum {
			switch v := v.(type) {
			case string:
				enumValues[i] = fmt.Sprintf(`"%s"`, v)
			case bool:
				enumValues[i] = fmt.Sprintf("%t", v)
			case float64, int:
				enumValues[i] = fmt.Sprintf("%v", v)
			default:
				enumValues[i] = fmt.Sprintf(`"%v"`, v)
			}
		}
		return strings.Join(enumValues, " | ")
	}

	// Handle basic types
	switch prop.Type { //nolint:goconst // JSON Schema type constants
	case "string":
		return "string"
	case "number", "integer":
		return "number"
	case "boolean":
		return "boolean"
	case "array":
		if prop.Items != nil {
			itemType := g.propertyToTypeScript(prop.Items)
			return itemType + "[]"
		}
		return "any[]"
	case "object":
		if len(prop.Properties) > 0 {
			// Inline object type
			var objType strings.Builder
			objType.WriteString("{\n")
			propNames := make([]string, 0, len(prop.Properties))
			for propName := range prop.Properties {
				propNames = append(propNames, propName)
			}
			sort.Strings(propNames)
			for _, propName := range propNames {
				nestedProp := prop.Properties[propName]
				nestedType := g.propertyToTypeScript(nestedProp)
				objType.WriteString(fmt.Sprintf("    %s: %s;\n", propName, nestedType))
			}
			objType.WriteString("  }")
			return objType.String()
		}
		// Generic object
		if prop.AdditionalProps != nil && *prop.AdditionalProps {
			return "Record<string, any>"
		}
		return "object"
	default:
		return "any"
	}
}

// generateStepInterface creates the Step interface with union type for actions.
func (g *TypeScriptGenerator) generateStepInterface(b *strings.Builder, def *Definition) {
	b.WriteString("/**\n")
	b.WriteString(" * A single configuration step\n")
	b.WriteString(" * \n")
	b.WriteString(" * Each step must contain exactly one action (shell, file, service, etc.)\n")
	b.WriteString(" * plus optional universal fields (name, when, register, etc.)\n")
	b.WriteString(" */\n")
	b.WriteString("export interface Step {\n")

	// Universal fields (from step definition)
	universalFields := []string{
		"name", "when", "creates", "unless", "become", "tags",
		"register", "with_filetree", "with_items", "env", "cwd",
		"timeout", "retries", "retry_delay", "changed_when",
		"failed_when", "become_user", "include",
	}

	// Required fields map
	requiredMap := make(map[string]bool)
	for _, req := range def.Required {
		requiredMap[req] = true
	}

	// Generate universal fields first
	for _, fieldName := range universalFields {
		if prop, ok := def.Properties[fieldName]; ok {
			g.generateProperty(b, fieldName, prop, requiredMap[fieldName])
		}
	}

	b.WriteString("\n  // Action fields (exactly one must be specified)\n")

	// Get all action names
	actionNames := make([]string, 0)
	for propName, prop := range def.Properties {
		// Skip universal fields
		isUniversal := false
		for _, uf := range universalFields {
			if propName == uf {
				isUniversal = true
				break
			}
		}
		// Include action fields (those with Ref or OneOf)
		if !isUniversal && (prop.Ref != "" || len(prop.OneOf) > 0) {
			actionNames = append(actionNames, propName)
		}
	}
	sort.Strings(actionNames)

	// Generate action fields
	for _, actionName := range actionNames {
		prop := def.Properties[actionName]
		g.generateProperty(b, actionName, prop, false)
	}

	b.WriteString("}\n\n")
}

// toPascalCase converts a string to PascalCase.
func toPascalCase(s string) string {
	parts := strings.Split(s, "_")
	for i, part := range parts {
		if len(part) > 0 {
			parts[i] = strings.ToUpper(part[:1]) + part[1:]
		}
	}
	result := strings.Join(parts, "")
	// Handle special cases
	if len(result) > 0 {
		return strings.ToUpper(result[:1]) + result[1:]
	}
	return result
}

// wrapText wraps text at the specified width.
func wrapText(text string, width int) string {
	if len(text) <= width {
		return text
	}

	var result strings.Builder
	words := strings.Fields(text)
	lineLen := 0

	for i, word := range words {
		wordLen := len(word)
		if lineLen+wordLen+1 > width {
			result.WriteString("\n")
			lineLen = 0
		} else if i > 0 {
			result.WriteString(" ")
			lineLen++
		}
		result.WriteString(word)
		lineLen += wordLen
	}

	return result.String()
}
