package schemagen

import (
	"encoding/json"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	_ "github.com/alehatsman/mooncake/internal/register" // Register action handlers
)

func TestGenerate(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: true,
		IncludeExamples:   false,
		StrictValidation:  false,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Verify basic schema structure
	if schema.SchemaURI != "http://json-schema.org/draft-07/schema#" {
		t.Errorf("Expected schema URI, got %s", schema.SchemaURI)
	}

	// Schema should have oneOf for both array and RunConfig formats
	if len(schema.OneOf) != 2 {
		t.Fatalf("Expected 2 oneOf alternatives, got %d", len(schema.OneOf))
	}

	// First alternative should be array of steps
	if schema.OneOf[0].Type != "array" {
		t.Errorf("Expected first oneOf type 'array', got %s", schema.OneOf[0].Type)
	}
	if schema.OneOf[0].Items == nil || schema.OneOf[0].Items.Ref != "#/definitions/step" {
		t.Error("Expected first oneOf items to reference #/definitions/step")
	}

	// Second alternative should be RunConfig
	if schema.OneOf[1].Ref != "#/definitions/runConfig" {
		t.Errorf("Expected second oneOf to reference runConfig, got %s", schema.OneOf[1].Ref)
	}

	// Verify step definition exists
	stepDef, ok := schema.Definitions["step"]
	if !ok {
		t.Fatal("Step definition not found")
	}

	if stepDef.Type != "object" {
		t.Errorf("Expected step type 'object', got %s", stepDef.Type)
	}

	// Verify universal fields exist in step. MT-77: `creates` and
	// `unless` are MT-15 step-level aliases of `unless_exists` /
	// `unless_command`; the schema generator must list them so
	// `validate` doesn't reject them and bottom out at the
	// empty-vocab oneOf error path.
	universalFields := []string{
		"name", "when", "as", "tags", "as_user",
		"creates", "unless", "unless_exists", "unless_command",
	}
	for _, field := range universalFields {
		if _, ok := stepDef.Properties[field]; !ok {
			t.Errorf("Universal field %s not found in step definition", field)
		}
	}
}

func TestGenerateActionDefinitions(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Get all registered actions
	actionMetas := actions.List()
	if len(actionMetas) == 0 {
		t.Fatal("No actions registered")
	}

	// Verify each action has a definition
	for _, meta := range actionMetas {
		def, ok := schema.Definitions[meta.Name]
		if !ok {
			t.Errorf("Definition for action %s not found", meta.Name)
			continue
		}

		// Verify description
		if def.Description == "" {
			t.Errorf("Action %s has no description", meta.Name)
		}

		// Verify custom extensions when enabled
		if opts.IncludeExtensions {
			if def.XCategory != string(meta.Category) {
				t.Errorf("Action %s category mismatch: expected %s, got %s",
					meta.Name, meta.Category, def.XCategory)
			}

			if def.XSupportsDryRun != meta.SupportsDryRun {
				t.Errorf("Action %s SupportsDryRun mismatch", meta.Name)
			}

			if len(meta.SupportedPlatforms) > 0 {
				if len(def.XPlatforms) != len(meta.SupportedPlatforms) {
					t.Errorf("Action %s platforms count mismatch", meta.Name)
				}
			}

			// proposal-05: the four spec-22 ABI capability bools are
			// emitted as x-implements-* extensions, sourced from the
			// metadata Registry.List() now populates.
			if def.XImplementsDiff != meta.ImplementsDiff {
				t.Errorf("Action %s XImplementsDiff = %v, want %v", meta.Name, def.XImplementsDiff, meta.ImplementsDiff)
			}
			if def.XImplementsCost != meta.ImplementsCost {
				t.Errorf("Action %s XImplementsCost = %v, want %v", meta.Name, def.XImplementsCost, meta.ImplementsCost)
			}
			if def.XImplementsReverse != meta.ImplementsReverse {
				t.Errorf("Action %s XImplementsReverse = %v, want %v", meta.Name, def.XImplementsReverse, meta.ImplementsReverse)
			}
			if def.XImplementsPermissions != meta.ImplementsPermissions {
				t.Errorf("Action %s XImplementsPermissions = %v, want %v", meta.Name, def.XImplementsPermissions, meta.ImplementsPermissions)
			}
		}
	}
}

// TestGenerateActionDefinitions_AtLeastOneHandlerImplementsEachABI is a
// sanity check that the new x-implements-* fields are *actually* set on
// at least one real shipping handler — guards against a regression where
// the central population in Registry.List() gets bypassed and every
// flag silently goes false. spec-22 phases shipped real Diff/Reverse/
// Cost/Permissions across the standard library, so each must surface
// somewhere in the registered set.
func TestGenerateActionDefinitions_AtLeastOneHandlerImplementsEachABI(t *testing.T) {
	gen := NewGenerator(GeneratorOptions{IncludeExtensions: true, OutputFormat: "json"})
	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	var anyDiff, anyCost, anyReverse, anyPerms bool
	for _, def := range schema.Definitions {
		if def.XImplementsDiff {
			anyDiff = true
		}
		if def.XImplementsCost {
			anyCost = true
		}
		if def.XImplementsReverse {
			anyReverse = true
		}
		if def.XImplementsPermissions {
			anyPerms = true
		}
	}
	if !anyDiff || !anyCost || !anyReverse || !anyPerms {
		t.Errorf("expected at least one shipping handler per ABI flag; got diff=%v cost=%v reverse=%v perms=%v",
			anyDiff, anyCost, anyReverse, anyPerms)
	}
}

func TestGenerateWithoutExtensions(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: false,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Pick a random action and verify no extensions
	for name, def := range schema.Definitions {
		if name == "step" {
			continue
		}

		if def.XCategory != "" {
			t.Errorf("Action %s has X-Category when extensions disabled", name)
		}
		if len(def.XPlatforms) > 0 {
			t.Errorf("Action %s has X-Platforms when extensions disabled", name)
		}
		break
	}
}

func TestMarshalJSON(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Test MarshalJSON
	data, err := schema.MarshalJSON()
	if err != nil {
		t.Fatalf("MarshalJSON() failed: %v", err)
	}

	// Verify it's valid JSON
	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Generated JSON is invalid: %v", err)
	}

	// Verify basic structure
	if result["$schema"] != "http://json-schema.org/draft-07/schema#" {
		t.Error("Schema URI not found in marshaled JSON")
	}

	// Should have oneOf instead of type
	if _, ok := result["oneOf"]; !ok {
		t.Error("oneOf not found in marshaled JSON")
	}
}

func TestMarshalPrettyJSON(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: false,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Test MarshalPrettyJSON
	data, err := schema.MarshalPrettyJSON()
	if err != nil {
		t.Fatalf("MarshalPrettyJSON() failed: %v", err)
	}

	// Verify it's valid JSON with indentation
	if len(data) == 0 {
		t.Error("MarshalPrettyJSON produced empty output")
	}

	var result map[string]interface{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("Pretty JSON is invalid: %v", err)
	}
}

func TestSpecificActionStructures(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	tests := []struct {
		action         string
		requiredFields []string
		propertyTypes  map[string]string
	}{
		{
			action: "shell",
			// Note: cmd has omitempty in JSON tag, so it's not marked as required
			// by reflection, even though it's logically required
			requiredFields: []string{},
			propertyTypes:  map[string]string{"cmd": "string"},
		},
		{
			action:         "os.service",
			requiredFields: []string{"name"},
			propertyTypes:  map[string]string{"name": "string", "state": "string"},
		},
		{
			action:         "file.write",
			requiredFields: []string{"path"},
			propertyTypes:  map[string]string{"path": "string", "state": "string"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.action, func(t *testing.T) {
			def, ok := schema.Definitions[tt.action]
			if !ok {
				t.Fatalf("Definition for %s not found", tt.action)
			}

			// Check required fields
			for _, required := range tt.requiredFields {
				found := false
				for _, req := range def.Required {
					if req == required {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Required field %s not found in %s", required, tt.action)
				}
			}

			// Check property types
			for propName, expectedType := range tt.propertyTypes {
				prop, ok := def.Properties[propName]
				if !ok {
					t.Errorf("Property %s not found in %s", propName, tt.action)
					continue
				}
				if prop.Type != expectedType {
					t.Errorf("Property %s in %s has type %s, expected %s",
						propName, tt.action, prop.Type, expectedType)
				}
			}
		})
	}
}

func TestVarsAndIncludeVarsSpecialCases(t *testing.T) {
	opts := GeneratorOptions{
		IncludeExtensions: true,
		OutputFormat:      "json",
	}
	gen := NewGenerator(opts)

	schema, err := gen.Generate()
	if err != nil {
		t.Fatalf("Generate() failed: %v", err)
	}

	// Test vars action (should be object type)
	varsDef, ok := schema.Definitions["vars"]
	if !ok {
		t.Fatal("vars definition not found")
	}
	if varsDef.Type != "object" {
		t.Errorf("vars should be object type, got %s", varsDef.Type)
	}

	// Test vars.load action (should be string type)
	varsLoadDef, ok := schema.Definitions["vars.load"]
	if !ok {
		t.Fatal("vars.load definition not found")
	}
	if varsLoadDef.Type != "string" {
		t.Errorf("vars.load should be string type, got %s", varsLoadDef.Type)
	}
}
