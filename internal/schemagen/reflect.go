package schemagen

import (
	"reflect"
	"strings"
)

// extractStructProperties extracts schema properties from a Go struct type.
// Returns the properties map and a list of required field names.
func extractStructProperties(t reflect.Type) (map[string]*Property, []string) {
	if t.Kind() != reflect.Struct {
		return nil, nil
	}

	properties := make(map[string]*Property)
	var required []string

	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)

		// Skip unexported fields
		if !field.IsExported() {
			continue
		}

		// Get JSON tag (used for YAML field names)
		jsonTag := field.Tag.Get("json")
		if jsonTag == "" || jsonTag == "-" {
			continue
		}

		// Parse JSON tag: "name,omitempty"
		parts := strings.Split(jsonTag, ",")
		fieldName := parts[0]
		isOptional := false
		for _, part := range parts[1:] {
			if part == "omitempty" {
				isOptional = true
				break
			}
		}

		// Extract property metadata
		prop := &Property{}

		// Get description from YAML tag or comment
		yamlTag := field.Tag.Get("yaml")
		if yamlTag != "" {
			// Parse yaml tag for additional metadata
			yamlParts := strings.Split(yamlTag, ",")
			if len(yamlParts) > 0 && yamlParts[0] != fieldName && yamlParts[0] != "-" {
				fieldName = yamlParts[0]
			}
		}

		// Get schema hints from custom tag
		schemaTag := field.Tag.Get("schema")
		if schemaTag != "" {
			parseSchemaTag(prop, schemaTag)
		}

		// Determine type from Go field type
		fieldType := field.Type
		if fieldType.Kind() == reflect.Ptr {
			fieldType = fieldType.Elem()
			isOptional = true // Pointer fields are optional
		}

		setPropertyType(prop, fieldType)

		// Add to properties
		properties[fieldName] = prop

		// Track required fields
		if !isOptional && !strings.Contains(schemaTag, "optional") {
			required = append(required, fieldName)
		}
	}

	return properties, required
}

// setPropertyType sets the JSON Schema type based on Go type.
func setPropertyType(prop *Property, t reflect.Type) {
	switch t.Kind() {
	case reflect.String:
		prop.Type = "string"
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		prop.Type = "integer"
	case reflect.Float32, reflect.Float64:
		prop.Type = "number" //nolint:goconst // JSON Schema type
	case reflect.Bool:
		prop.Type = "boolean" //nolint:goconst // JSON Schema type
	case reflect.Slice, reflect.Array:
		prop.Type = "array"
		// Extract element type
		elemType := t.Elem()
		prop.Items = &Property{}
		setPropertyType(prop.Items, elemType)
	case reflect.Map:
		prop.Type = "object"
		// Maps are typically string -> interface{} in config
		// Allow additional properties for maps
		trueVal := true
		prop.AdditionalProps = &trueVal
	case reflect.Struct:
		prop.Type = "object"
		// For nested structs, extract properties recursively
		nestedProps, nestedRequired := extractStructProperties(t)
		prop.Properties = nestedProps
		if len(nestedRequired) > 0 {
			prop.Required = nestedRequired
		}
		// Set additionalProperties to false for nested objects
		falseVal := false
		prop.AdditionalProps = &falseVal

		// Apply known validation to nested properties
		// (Will be called with action name context from caller)
		for nestedField, nestedProp := range nestedProps {
			if pattern, ok := KnownPatterns[nestedField]; ok {
				nestedProp.Pattern = pattern
			}
		}
	case reflect.Interface:
		// interface{} can be anything
		// Don't set type to allow any value
	case reflect.Ptr:
		// Dereference and recurse
		setPropertyType(prop, t.Elem())
	}
}

// parseSchemaTag extracts metadata from custom schema tags.
// Format: schema:"required,description=Service name,enum=val1|val2|val3"
func parseSchemaTag(prop *Property, tag string) {
	parts := strings.Split(tag, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)

		if part == "required" {
			// Handled by caller
			continue
		}

		if strings.HasPrefix(part, "description=") {
			prop.Description = strings.TrimPrefix(part, "description=")
			continue
		}

		if strings.HasPrefix(part, "enum=") {
			enumStr := strings.TrimPrefix(part, "enum=")
			values := strings.Split(enumStr, "|")
			prop.Enum = make([]interface{}, len(values))
			for i, v := range values {
				prop.Enum[i] = v
			}
			continue
		}

		if strings.HasPrefix(part, "default=") {
			prop.Default = strings.TrimPrefix(part, "default=")
			continue
		}

		if strings.HasPrefix(part, "pattern=") {
			prop.Pattern = strings.TrimPrefix(part, "pattern=")
			continue
		}

		if strings.HasPrefix(part, "format=") {
			prop.Format = strings.TrimPrefix(part, "format=")
			continue
		}

		// minLength and maxLength parsing could be added here if needed
	}
}

// inferDescription attempts to infer a description from field name.
