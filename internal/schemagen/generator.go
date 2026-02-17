package schemagen

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// Generator generates JSON Schema from action metadata and struct definitions.
type Generator struct {
	opts GeneratorOptions
}

// NewGenerator creates a new schema generator with options.
func NewGenerator(opts GeneratorOptions) *Generator {
	return &Generator{opts: opts}
}

// Generate creates a complete JSON Schema from the action registry.
func (g *Generator) Generate() (*Schema, error) {
	schema := &Schema{
		SchemaURI:   "http://json-schema.org/draft-07/schema#",
		ID:          "https://mooncake.dev/schemas/config.json",
		Title:       "Mooncake Configuration Schema",
		Description: "JSON Schema for Mooncake configuration files",
		Definitions: make(map[string]*Definition),
	}

	// Add step definition (universal fields + action fields)
	stepDef, err := g.generateStepDefinition()
	if err != nil {
		return nil, fmt.Errorf("failed to generate step definition: %w", err)
	}
	schema.Definitions["step"] = stepDef

	// Add RunConfig definition (new structured format with version/vars/steps)
	runConfigDef := g.generateRunConfigDefinition()
	schema.Definitions["runConfig"] = runConfigDef

	// Generate definitions for each action
	for _, meta := range actions.List() {
		def, err := g.generateActionDefinition(meta)
		if err != nil {
			return nil, fmt.Errorf("failed to generate definition for action %s: %w", meta.Name, err)
		}

		// Store as both the action name and action_action for consistency
		schema.Definitions[meta.Name] = def
		if meta.Name == "shell" || meta.Name == "command" { //nolint:goconst // Action name checks
			// Special case: shell can be string or object
			schema.Definitions[meta.Name+"_action"] = def
		}
	}

	// Support both formats at root level using oneOf:
	// 1. Array of steps (old format for backward compatibility)
	// 2. RunConfig object (new format with version/vars/steps)
	schema.OneOf = []*OneOfConstraint{
		{
			Type: "array",
			Items: &SchemaRef{
				Ref: "#/definitions/step",
			},
		},
		{
			Ref: "#/definitions/runConfig",
		},
	}

	return schema, nil
}

// generateStepDefinition creates the step definition with universal fields.
//nolint:unparam // Error return kept for future error handling
func (g *Generator) generateStepDefinition() (*Definition, error) {
	def := &Definition{
		Type:       "object",
		Properties: make(map[string]*Property),
	}

	// Set additionalProperties to false if strict validation is enabled
	if g.opts.StrictValidation {
		falseVal := false
		def.AdditionalProperties = &falseVal
	}

	// Helper for bool pointers
	trueVal := true

	// Add universal fields
	universalFields := map[string]*Property{
		"name": {
			Type:        "string",
			Description: "Name of the step (universal)",
		},
		"when": {
			Type:        "string",
			Description: "Conditional expression for step execution (universal)",
		},
		"creates": {
			Type:        "string",
			Description: "Skip step if this file path exists. Useful for idempotency (universal)",
		},
		"unless": {
			Type:        "string",
			Description: "Skip step if this command succeeds (exit code 0). Useful for idempotency (universal)",
		},
		"become": {
			Type:        "boolean",
			Description: "Execute with sudo privileges. Works with: shell, command, file, template",
		},
		"tags": {
			Type: "array",
			Items: &Property{
				Type: "string",
			},
			Description: "Tags for filtering step execution (universal)",
		},
		"register": {
			Type:        "string",
			Description: "Variable name to store step execution result (universal)",
		},
		"with_filetree": {
			Type:        "string",
			Description: "Directory path for iterating over files (universal)",
		},
		"with_items": {
			Type:        "string",
			Description: "Variable expression for iterating over items (universal)",
		},
		"env": {
			Type:            "object",
			Properties:      map[string]*Property{},
			AdditionalProps: &trueVal,
			Description:     "Environment variables for the step",
		},
		"cwd": {
			Type:        "string",
			Description: "Working directory for the step",
		},
		"timeout": {
			Type:        "string",
			Description: "⚠️ SHELL/COMMAND ONLY: Maximum execution time (e.g., '30s', '5m', '1h'). Works with 'shell' and 'command' actions. Ignored for file/template/include.",
		},
		"retries": {
			Type:        "integer",
			Description: "⚠️ SHELL/COMMAND ONLY: Number of retry attempts on failure. Works with 'shell' and 'command' actions. Ignored for file/template/include.",
		},
		"retry_delay": {
			Type:        "string",
			Description: "⚠️ SHELL/COMMAND ONLY: Delay between retry attempts (e.g., '1s', '5s'). Works with 'shell' and 'command' actions. Ignored for file/template/include.",
		},
		"changed_when": {
			Type:        "string",
			Description: "Expression to override changed result",
		},
		"failed_when": {
			Type:        "string",
			Description: "Expression to override failure condition",
		},
		"become_user": {
			Type:        "string",
			Description: "⚠️ SHELL/COMMAND ONLY: User to become via sudo (e.g., 'root', 'postgres'). Works with 'shell' and 'command' actions. Ignored for file/template/include.",
		},
		"include": {
			Type:        "string",
			Description: "Path to YAML file with steps to include",
		},
	}

	for name, prop := range universalFields {
		// Apply known patterns and ranges to universal fields
		if g.opts.StrictValidation {
			applyKnownValidation("step", name, prop)
		}
		def.Properties[name] = prop
	}

	// Get all action names for oneOf generation
	actionMetas := actions.List()
	var actionNames []string //nolint:prealloc // Size unknown at compile time

	// Add action fields (will be populated by generateActionDefinition)
	// These are added as properties that reference action definitions
	for _, meta := range actionMetas {
		actionNames = append(actionNames, meta.Name)
		actionProp := &Property{
			Description: meta.Description,
		}

		// Handle special cases where actions support multiple forms
		if meta.Name == "shell" || meta.Name == "preset" {
			actionProp.OneOf = []*Property{
				{
					Type:        "string",
					Description: fmt.Sprintf("%s (simple string format)", meta.Name),
				},
				{
					Ref:         fmt.Sprintf("#/definitions/%s", meta.Name),
					Description: fmt.Sprintf("%s (structured object format)", meta.Name),
				},
			}
		} else {
			actionProp.Ref = fmt.Sprintf("#/definitions/%s", meta.Name)
		}

		def.Properties[meta.Name] = actionProp
	}

	// Add "include" to action names for oneOf generation
	// (include is a special string field, not a registered action)
	actionNames = append(actionNames, "include")

	// Sort action names for deterministic schema generation
	sort.Strings(actionNames)

	// Generate oneOf constraints to ensure only one action per step
	if g.opts.StrictValidation {
		def.OneOf = g.generateOneOfConstraints(actionNames)
	}

	return def, nil
}

// generateRunConfigDefinition creates the RunConfig definition for structured configs.
func (g *Generator) generateRunConfigDefinition() *Definition {
	trueVal := true

	def := &Definition{
		Type:        "object", //nolint:goconst // JSON Schema type
		Description: "Structured configuration with version, global variables, and steps",
		Properties: map[string]*Property{
			"version": {
				Type:        "string", //nolint:goconst // JSON Schema type
				Description: "Configuration schema version (e.g., '1.0')",
			},
			"vars": {
				Type:            "object", //nolint:goconst // JSON Schema type
				Description:     "Global variables available to all steps",
				Properties:      map[string]*Property{},
				AdditionalProps: &trueVal,
			},
			"steps": {
				Type: "array",
				Items: &Property{
					Ref: "#/definitions/step",
				},
				Description: "Configuration steps to execute",
			},
		},
		Required: []string{"steps"},
	}

	// Set additionalProperties to false if strict validation is enabled
	if g.opts.StrictValidation {
		falseVal := false
		def.AdditionalProperties = &falseVal
	}

	return def
}

// generateOneOfConstraints creates oneOf constraints to enforce mutual exclusion of actions.
// Each constraint says: "if this action is required, then none of the other actions can be present"
func (g *Generator) generateOneOfConstraints(actionNames []string) []*OneOfConstraint {
	var constraints []*OneOfConstraint //nolint:prealloc // Size depends on actionNames length

	for i, actionName := range actionNames {
		// Create a constraint for this action
		constraint := &OneOfConstraint{
			Required: []string{actionName},
			Not: &NotConstraint{
				AnyOf: make([]*RequiredConstraint, 0),
			},
		}

		// Add all other actions to the "not.anyOf" list
		for j, otherAction := range actionNames {
			if i != j {
				constraint.Not.AnyOf = append(constraint.Not.AnyOf, &RequiredConstraint{
					Required: []string{otherAction},
				})
			}
		}

		constraints = append(constraints, constraint)
	}

	return constraints
}

// generateActionDefinition creates a definition for a specific action.
//nolint:unparam // Error return kept for future error handling
func (g *Generator) generateActionDefinition(meta actions.ActionMetadata) (*Definition, error) {
	def := &Definition{
		Type:        "object",
		Description: meta.Description,
		Properties:  make(map[string]*Property),
	}

	// Set additionalProperties to false if strict validation is enabled
	if g.opts.StrictValidation {
		falseVal := false
		def.AdditionalProperties = &falseVal
	}

	// Add custom extensions if enabled
	if g.opts.IncludeExtensions {
		if len(meta.SupportedPlatforms) > 0 {
			def.XPlatforms = meta.SupportedPlatforms
		}
		def.XRequiresSudo = meta.RequiresSudo
		def.XImplementsCheck = meta.ImplementsCheck
		def.XCategory = string(meta.Category)
		def.XSupportsDryRun = meta.SupportsDryRun
		def.XSupportsBecome = meta.SupportsBecome
		def.XVersion = meta.Version
		def.XEmitsEvents = meta.EmitsEvents
	}

	// Handle special cases for actions that don't have dedicated structs
	if meta.Name == "vars" {
		// vars is a map[string]interface{}
		def.Type = "object" //nolint:goconst // JSON Schema type
		def.Description = "Define or update variables"
		// Properties can be any key-value pairs - allow additional properties
		trueVal := true
		def.AdditionalProperties = &trueVal
		return def, nil
	}

	if meta.Name == "include_vars" {
		// include_vars is a string (file path)
		def.Type = "string" //nolint:goconst // JSON Schema type
		def.Description = "Load variables from a YAML file"
		return def, nil
	}

	// Reflect on the corresponding config struct to extract properties
	structType, err := getActionStruct(meta.Name)
	if err != nil {
		// For actions without reflection data, return basic definition
		return def, nil
	}

	// Extract properties from struct fields
	props, required := extractStructProperties(structType)
	def.Properties = props
	def.Required = required

	// Apply known enums, patterns, and descriptions if enabled
	if g.opts.StrictValidation {
		for fieldName, prop := range props {
			applyKnownValidation(meta.Name, fieldName, prop)
			applyEnhancedDescription(meta.Name, fieldName, prop)
		}
	}

	return def, nil
}

// getActionStruct returns the reflect.Type for an action's config struct.
func getActionStruct(actionName string) (reflect.Type, error) {
	var actionStruct interface{}

	switch actionName {
	case "shell":
		actionStruct = &config.ShellAction{}
	case "command":
		actionStruct = &config.CommandAction{}
	case "file":
		actionStruct = &config.File{}
	case "template":
		actionStruct = &config.Template{}
	case "copy":
		actionStruct = &config.Copy{}
	case "download":
		actionStruct = &config.Download{}
	case "unarchive":
		actionStruct = &config.Unarchive{}
	case "package":
		actionStruct = &config.Package{}
	case "service":
		actionStruct = &config.ServiceAction{}
	case "assert":
		actionStruct = &config.Assert{}
	case "preset":
		actionStruct = &config.PresetInvocation{}
	case "print":
		actionStruct = &config.PrintAction{}
	case "file_replace":
		actionStruct = &config.FileReplace{}
	case "repo_search":
		actionStruct = &config.RepoSearch{}
	case "repo_tree":
		actionStruct = &config.RepoTree{}
	case "wait":
		actionStruct = &config.WaitAction{}
	case "vars":
		// vars is a map[string]interface{} directly in Step
		return nil, fmt.Errorf("vars action uses inline map definition")
	case "include_vars":
		// include_vars is a string directly in Step
		return nil, fmt.Errorf("include_vars action uses inline string definition")
	default:
		return nil, fmt.Errorf("unknown action: %s", actionName)
	}

	t := reflect.TypeOf(actionStruct)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t, nil
}

// MarshalJSON converts the schema to JSON.
func (s *Schema) MarshalJSON() ([]byte, error) {
	// Create an alias type to avoid infinite recursion
	type SchemaAlias Schema
	return json.Marshal((*SchemaAlias)(s))
}

// MarshalPrettyJSON converts the schema to pretty-printed JSON.
func (s *Schema) MarshalPrettyJSON() ([]byte, error) {
	// Create an alias type to avoid infinite recursion
	type SchemaAlias Schema
	return json.MarshalIndent((*SchemaAlias)(s), "", "  ")
}

// GenerateOpenAPI generates an OpenAPI 3.0 specification.
func (g *Generator) GenerateOpenAPI() (*OpenAPISpec, error) {
	// First generate JSON Schema
	schema, err := g.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate base schema: %w", err)
	}

	// Convert to OpenAPI format
	spec := schema.ConvertToOpenAPI()

	return spec, nil
}

// GenerateTypeScript generates TypeScript definitions.
func (g *Generator) GenerateTypeScript() (string, error) {
	// First generate JSON Schema
	schema, err := g.Generate()
	if err != nil {
		return "", fmt.Errorf("failed to generate base schema: %w", err)
	}

	// Convert to TypeScript
	ts := schema.GenerateTypeScript()

	return ts, nil
}
