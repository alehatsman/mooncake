package schemagen

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestConvertToOpenAPI(t *testing.T) {
	// Create a simple schema
	schema := &Schema{
		Title:       "Test Schema",
		Description: "Test description",
		Type:        "array",
		Items: &SchemaRef{
			Ref: "#/definitions/step",
		},
		Definitions: map[string]*Definition{
			"step": {
				Type:        "object",
				Description: "A step",
				Properties: map[string]*Property{
					"name": {
						Type:        "string",
						Description: "Step name",
					},
				},
			},
		},
	}

	// Convert to OpenAPI
	spec := schema.ConvertToOpenAPI()

	// Validate structure
	if spec.OpenAPI != "3.0.3" {
		t.Errorf("expected OpenAPI 3.0.3, got %s", spec.OpenAPI)
	}

	if spec.Info.Title != schema.Title {
		t.Errorf("expected title %s, got %s", schema.Title, spec.Info.Title)
	}

	if spec.Info.Description != schema.Description {
		t.Errorf("expected description %s, got %s", schema.Description, spec.Info.Description)
	}

	if spec.Info.Version != "0.3.0" {
		t.Errorf("expected version 0.3.0, got %s", spec.Info.Version)
	}

	if spec.Info.License.Name != "MIT" {
		t.Errorf("expected MIT license, got %s", spec.Info.License.Name)
	}

	// Check schemas were converted
	if len(spec.Components.Schemas) == 0 {
		t.Error("expected schemas to be converted")
	}

	if _, ok := spec.Components.Schemas["step"]; !ok {
		t.Error("expected step schema to exist")
	}
}

func TestConvertRef(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"#/definitions/step", "#/components/schemas/step"},
		{"#/definitions/shell", "#/components/schemas/shell"},
		{"", ""},
		{"#/other/path", "#/other/path"}, // Not a definitions ref
	}

	for _, tt := range tests {
		result := convertRef(tt.input)
		if result != tt.expected {
			t.Errorf("convertRef(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestConvertDefinitionToOpenAPISchema(t *testing.T) {
	def := &Definition{
		Type:        "object",
		Description: "Test definition",
		Properties: map[string]*Property{
			"field1": {
				Type:        "string",
				Description: "Field 1",
			},
			"field2": {
				Type:        "integer",
				Minimum:     ptrFloat64(0),
				Maximum:     ptrFloat64(100),
			},
		},
		Required: []string{"field1"},
		XPlatforms: []string{"linux", "darwin"},
		XCategory: "test",
	}

	schema := convertDefinitionToOpenAPISchema(def)

	if schema.Type != def.Type {
		t.Errorf("expected type %s, got %s", def.Type, schema.Type)
	}

	if schema.Description != def.Description {
		t.Errorf("expected description %s, got %s", def.Description, schema.Description)
	}

	if len(schema.Properties) != len(def.Properties) {
		t.Errorf("expected %d properties, got %d", len(def.Properties), len(schema.Properties))
	}

	if len(schema.Required) != len(def.Required) {
		t.Errorf("expected %d required fields, got %d", len(def.Required), len(schema.Required))
	}

	if len(schema.XPlatforms) != len(def.XPlatforms) {
		t.Errorf("expected %d platforms, got %d", len(def.XPlatforms), len(schema.XPlatforms))
	}

	if schema.XCategory != def.XCategory {
		t.Errorf("expected category %s, got %s", def.XCategory, schema.XCategory)
	}
}

func TestConvertPropertyToOpenAPISchema(t *testing.T) {
	prop := &Property{
		Type:        "string",
		Description: "Test property",
		Enum:        []interface{}{"a", "b", "c"},
		Default:     "a",
		Pattern:     "^[a-z]+$",
		MinLength:   ptrInt(1),
		MaxLength:   ptrInt(100),
	}

	schema := convertPropertyToOpenAPISchema(prop)

	if schema.Type != prop.Type {
		t.Errorf("expected type %s, got %s", prop.Type, schema.Type)
	}

	if schema.Description != prop.Description {
		t.Errorf("expected description %s, got %s", prop.Description, schema.Description)
	}

	if len(schema.Enum) != len(prop.Enum) {
		t.Errorf("expected %d enum values, got %d", len(prop.Enum), len(schema.Enum))
	}

	if schema.Default != prop.Default {
		t.Errorf("expected default %v, got %v", prop.Default, schema.Default)
	}

	if schema.Pattern != prop.Pattern {
		t.Errorf("expected pattern %s, got %s", prop.Pattern, schema.Pattern)
	}
}

func TestGenerateOpenAPI(t *testing.T) {
	// Use real generator
	opts := GeneratorOptions{
		IncludeExamples:   false,
		IncludeExtensions: true,
		StrictValidation:  true,
		OutputFormat:      "openapi",
	}
	generator := NewGenerator(opts)

	spec, err := generator.GenerateOpenAPI()
	if err != nil {
		t.Fatalf("failed to generate OpenAPI: %v", err)
	}

	// Validate structure
	if spec.OpenAPI != "3.0.3" {
		t.Errorf("expected OpenAPI 3.0.3, got %s", spec.OpenAPI)
	}

	if spec.Info.Title == "" {
		t.Error("expected non-empty title")
	}

	if spec.Info.Version == "" {
		t.Error("expected non-empty version")
	}

	if spec.Info.Contact == nil {
		t.Error("expected contact information")
	}

	if spec.Info.License == nil {
		t.Error("expected license information")
	}

	if len(spec.Components.Schemas) == 0 {
		t.Error("expected schemas to be present")
	}

	// Check for key schemas
	requiredSchemas := []string{"step", "shell", "file", "service"}
	for _, name := range requiredSchemas {
		if _, ok := spec.Components.Schemas[name]; !ok {
			t.Errorf("expected schema %s to exist", name)
		}
	}
}

func TestOpenAPIJSONSerialization(t *testing.T) {
	// Create a simple spec
	spec := &OpenAPISpec{
		OpenAPI: "3.0.3",
		Info: OpenAPIInfo{
			Title:   "Test API",
			Version: "1.0.0",
		},
		Components: OpenAPIComponents{
			Schemas: map[string]*OpenAPISchema{
				"test": {
					Type:        "object",
					Description: "Test schema",
					Properties: map[string]*OpenAPISchema{
						"name": {
							Type: "string",
						},
					},
					Required: []string{"name"},
				},
			},
		},
	}

	// Serialize to JSON
	data, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("failed to marshal OpenAPI spec: %v", err)
	}

	// Check it's valid JSON
	if !json.Valid(data) {
		t.Error("generated JSON is invalid")
	}

	// Check it contains expected fields
	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"openapi":"3.0.3"`) {
		t.Error("expected openapi field in JSON")
	}

	if !strings.Contains(jsonStr, `"Test API"`) {
		t.Error("expected title in JSON")
	}

	// Deserialize back
	var deserialized OpenAPISpec
	if err := json.Unmarshal(data, &deserialized); err != nil {
		t.Fatalf("failed to unmarshal OpenAPI spec: %v", err)
	}

	if deserialized.OpenAPI != spec.OpenAPI {
		t.Errorf("expected OpenAPI %s, got %s", spec.OpenAPI, deserialized.OpenAPI)
	}
}

// Helper functions for creating pointers
func ptrFloat64(v float64) *float64 {
	return &v
}

func ptrInt(v int) *int {
	return &v
}
