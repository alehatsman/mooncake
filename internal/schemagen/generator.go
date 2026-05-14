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
		if meta.Name == "shell" || meta.Name == "cmd" { //nolint:goconst // Action name checks
			// Special case: shell can be string or object
			schema.Definitions[meta.Name+"_action"] = def
		}
	}

	// Add import definition (not a registered action, but needs a definition for schema)
	schema.Definitions["import"] = &Definition{
		Type:        "string", //nolint:goconst // JSON Schema type
		Description: "Import steps from another file",
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

	// Add universal fields (spec-21 modernized names)
	universalFields := map[string]*Property{
		"name": {
			Type:        "string",
			Description: "Name of the step (universal)",
		},
		"when": {
			Type:        "string",
			Description: "Conditional expression for step execution (universal)",
		},
		"unless_exists": {
			Type:        "string",
			Description: "Skip step if this file path exists. Useful for idempotency (universal)",
		},
		"unless_command": {
			Type:        "string",
			Description: "Skip step if this command succeeds (exit code 0). Useful for idempotency (universal)",
		},
		"as_user": {
			Type:        "string",
			Description: "Run as this user (empty = current user, 'root' = sudo to root, '<name>' = sudo to user). Works with: shell, cmd, file.write, file.template",
		},
		"tags": {
			Type: "array",
			Items: &Property{
				Type: "string",
			},
			Description: "Tags for filtering step execution (universal)",
		},
		"as": {
			Type:        "string",
			Description: "Variable name to store step execution outputs (universal)",
		},
		"for_each_file": {
			Type:        "string",
			Description: "Directory path for iterating over files (universal)",
		},
		"for_each": {
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
			Description: "⚠️ shell/cmd ONLY: Maximum execution time (e.g., '30s', '5m', '1h')",
		},
		"retry": {
			Type:        "object",
			Description: "Retry policy: { attempts: int, delay: string, backoff: string }",
		},
		"changed_when": {
			Type:        "string",
			Description: "Expression to override changed result",
		},
		"failed_when": {
			Type:        "string",
			Description: "Expression to override failure condition",
		},
		"import": {
			Type:        "string",
			Description: "Path to YAML file with steps to import",
		},
		"continue_on_error": {
			Type:        "boolean",
			Description: "Continue execution even if this step fails (universal)",
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
		if meta.Name == "shell" || meta.Name == "use" {
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

	// Add "import" to action names for oneOf generation
	// (import is a special string field, not a registered action)
	actionNames = append(actionNames, "import")

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
		// Build the property constraint for this action. Most actions are a
		// strict object reference. shell and use accept both string and
		// object forms — preserve that here, otherwise the oneOf branch
		// rejects valid `shell: echo hello` (scalar) input.
		var actionProp *Property
		if actionName == "shell" || actionName == "use" {
			actionProp = &Property{
				OneOf: []*Property{
					{Type: "string"},
					{Ref: fmt.Sprintf("#/definitions/%s", actionName)},
				},
			}
		} else {
			actionProp = &Property{
				Ref: fmt.Sprintf("#/definitions/%s", actionName),
			}
		}

		constraint := &OneOfConstraint{
			Required: []string{actionName},
			Properties: map[string]*Property{
				actionName: actionProp,
			},
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

	if meta.Name == "vars.load" {
		// vars.load is a string (file path)
		def.Type = "string" //nolint:goconst // JSON Schema type
		def.Description = "Load variables from a YAML file"
		return def, nil
	}

	if meta.Name == "import" {
		// import is a string (file path)
		def.Type = "string" //nolint:goconst // JSON Schema type
		def.Description = "Import steps from another file"
		return def, nil
	}

	// Handle artifact_capture specially (has circular reference via Steps)
	if meta.Name == "artifact.capture" {
		def.Properties = map[string]*Property{
			"name":              {Type: "string", Description: "Name of the artifact (used for output directory)"},
			"output_dir":        {Type: "string", Description: "Base directory for artifacts (default: './artifacts')"},
			"format":            {Type: "string", Enum: []interface{}{"json", "markdown", "both"}, Description: "Output format"},
			"capture_content":   {Type: "boolean", Description: "Capture full file content before/after"},
			"max_diff_size":     {Type: "integer", Description: "Maximum diff size in bytes per file"},
			"include_checksums": {Type: "boolean", Description: "Include SHA256 checksums"},
			"embed_plan":        {Type: "boolean", Description: "Embed full plan in artifact for LLM context"},
			"max_plan_steps":    {Type: "integer", Description: "Don't embed plan if exceeds this many steps"},
			"steps": {
				Type: "array",
				Items: &Property{
					Ref: "#/definitions/step",
				},
				Description: "Steps to execute while capturing changes",
			},
		}
		def.Required = []string{"name", "steps"}
		return def, nil
	}

	// Handle artifact_validate specially
	if meta.Name == "artifact.validate" {
		def.Properties = map[string]*Property{
			"artifact_file":     {Type: "string", Description: "Path to artifact metadata JSON file"},
			"max_files":         {Type: "integer", Description: "Maximum number of files allowed to change"},
			"max_lines_changed": {Type: "integer", Description: "Maximum total lines changed"},
			"max_file_size":     {Type: "integer", Description: "Maximum file size in bytes after changes"},
			"require_tests":     {Type: "boolean", Description: "Require test file changes when code files change"},
			"allowed_paths": {
				Type:        "array",
				Items:       &Property{Type: "string"},
				Description: "Glob patterns for allowed file paths",
			},
			"forbidden_paths": {
				Type:        "array",
				Items:       &Property{Type: "string"},
				Description: "Glob patterns for forbidden file paths",
			},
		}
		def.Required = []string{"artifact_file"}
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

	// Apply structural field overrides (e.g. union types). These are
	// independent of validation strictness.
	for fieldName, prop := range props {
		applyFieldOverride(meta.Name, fieldName, prop)
	}

	// Apply known enums, patterns, and descriptions if enabled
	if g.opts.StrictValidation {
		for fieldName, prop := range props {
			applyKnownValidation(meta.Name, fieldName, prop)
			applyEnhancedDescription(meta.Name, fieldName, prop)
		}
	}

	return def, nil
}

// actionStructByName maps action YAML keys to a zero value of their
// underlying config struct. Used by getActionStruct to reflect each
// action's properties when generating JSON Schema.
var actionStructByName = map[string]any{
	"shell":             &config.ShellAction{},
	"cmd":               &config.CommandAction{},
	"file.write":        &config.File{},
	"file.template":     &config.Template{},
	"file.copy":         &config.Copy{},
	"file.download":     &config.Download{},
	"tool":              &config.Tool{},
	"file.unarchive":    &config.Unarchive{},
	"pkg":               &config.Package{},
	"pkg.repo":          &config.PkgRepo{},
	"os.service":        &config.ServiceAction{},
	"os.user":           &config.OsUser{},
	"os.ssh_key":        &config.OsSSHKey{},
	"container.image":   &config.ContainerImage{},
	"container":         &config.Container{},
	"assert":            &config.Assert{},
	"use":               &config.PresetInvocation{},
	"log":               &config.PrintAction{},
	"text.line":         &config.TextLine{},
	"text.replace":      &config.FileReplace{},
	"text.insert":       &config.FileInsert{},
	"text.delete_range": &config.FileDeleteRange{},
	"text.patch":        &config.FilePatchApply{},
	"repo.search":       &config.RepoSearch{},
	"repo.tree":         &config.RepoTree{},
	"repo.patch":        &config.RepoApplyPatchset{},
	"git.clone":         &config.GitClone{},
	"git.checkout":      &config.GitCheckout{},
	"git.config":        &config.GitConfig{},
	"wait.port":         &config.WaitPort{},
	"wait.http":         &config.WaitHTTP{},
	"wait.file":         &config.WaitFile{},
	"wait.command":      &config.WaitCommand{},
}

// getActionStruct returns the reflect.Type for an action's config struct.
func getActionStruct(actionName string) (reflect.Type, error) {
	if v, ok := actionStructByName[actionName]; ok {
		t := reflect.TypeOf(v)
		if t.Kind() == reflect.Pointer {
			t = t.Elem()
		}
		return t, nil
	}

	switch actionName {
	case "artifact.capture":
		return nil, fmt.Errorf("artifact.capture uses custom definition (circular reference)")
	case "artifact.validate":
		return nil, fmt.Errorf("artifact.validate uses custom definition")
	case "vars":
		return nil, fmt.Errorf("vars action uses inline map definition")
	case "vars.load":
		return nil, fmt.Errorf("vars.load action uses inline string definition")
	case "import":
		return nil, fmt.Errorf("import action uses inline string definition")
	default:
		return nil, fmt.Errorf("unknown action: %s", actionName)
	}
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
