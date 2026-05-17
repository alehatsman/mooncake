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
//
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
		// MT-15 added these as friendly step-level aliases of
		// unless_exists / unless_command on the Step struct, but the
		// schema generator never grew the corresponding properties — so
		// `creates:` and `unless:` at step level were silently rejected
		// by validate, and the validator's oneOf failure mode bottomed
		// out at the empty-vocab error path ("Step must have exactly
		// one action ()") because no action variant matched. MT-77.
		"creates": {
			Type:        "string",
			Description: "Skip step if this file path exists (universal alias of unless_exists)",
		},
		"unless": {
			Type:        "string",
			Description: "Skip step if this command succeeds (universal alias of unless_command)",
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
			// for_each accepts three shapes, all handled by the
			// ForEachField custom UnmarshalYAML in internal/config/config.go:
			//   for_each: items_var          # template variable
			//   for_each: [a, b, c]          # inline literal list
			//   for_each:                    # block literal list
			//     - a
			//     - b
			// The schema must accept the same union; otherwise the JSON-
			// schema validator runs before yaml-decode and rejects the
			// list forms (issue #20).
			OneOf: []*Property{
				{Type: "string", Description: "Variable expression rendered through templates and resolved to a list"},
				{Type: "array", Items: &Property{}, Description: "Inline / block literal list of items"},
			},
			Description: "Iterate over items (universal). Accepts a template-variable string or an inline list.",
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
		"on_change": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Reactive triggers: child steps that run only if this step reports changed=true (spec-23)",
		},
		"triggered_by": {
			Type:        "string",
			Description: "(plan metadata) Parent step ID when expanded from an on_change child",
		},
		"transaction": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Sequence of steps that apply all-or-nothing; failure runs Reverse() on prior steps in LIFO order (spec-30)",
		},
		"on_rollback": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Steps that run after a transaction's rollback finishes (spec-30)",
		},
		"allow_irreversible": {
			Type:        "boolean",
			Description: "Allow a transaction to include steps without Reverser support (spec-30)",
		},
		"txn_parent": {
			Type:        "string",
			Description: "(plan metadata) Parent transaction step ID when expanded from a transaction child",
		},
		"txn_role": {
			Type:        "string",
			Description: "(plan metadata) Role of this step inside its parent transaction: 'body' or 'rollback'",
		},
		"try": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Sequence of steps run with structured error recovery; on first failure runs catch (if set) then finally (spec-23 §2)",
		},
		"catch": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Steps that run when any try child errors. Requires try: on the same step (spec-23 §2)",
		},
		"finally": {
			Type: "array",
			Items: &Property{
				Ref: "#/definitions/step",
			},
			Description: "Steps that always run after try (and catch, if it ran). Requires try: on the same step (spec-23 §2)",
		},
		"try_parent": {
			Type:        "string",
			Description: "(plan metadata) Parent compound step ID when expanded from a try/catch/finally branch",
		},
		"try_role": {
			Type:        "string",
			Description: "(plan metadata) Role of this step inside its parent compound: 'try', 'catch', or 'finally'",
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

	// Add "import", "transaction", and "try" to action names for oneOf
	// generation. "import" is a special string field (not a registered
	// action). "transaction" (spec-30) and "try" (spec-23 §2) are
	// compound nodes, not leaf actions. All three are special-cased
	// rather than registered as actions.
	actionNames = append(actionNames, "import", "transaction", "try")

	// Sort action names for deterministic schema generation
	sort.Strings(actionNames)

	// Generate oneOf constraints to ensure only one action per step
	def.OneOf = g.generateOneOfConstraints(actionNames)

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
		switch actionName {
		case "shell", "use":
			actionProp = &Property{
				OneOf: []*Property{
					{Type: "string"},
					{Ref: fmt.Sprintf("#/definitions/%s", actionName)},
				},
			}
		case "transaction":
			// spec-30: transaction: is a top-level compound-step shape,
			// not an action. The property's full shape is declared in
			// universalFields above (array of #/definitions/step). The
			// oneOf branch here just asserts the property exists; the
			// universalFields entry constrains its shape.
			actionProp = &Property{}
		case "try":
			// spec-23 §2: same shape as transaction — a compound-step
			// wrapper, not an action. The full property shape is
			// declared in universalFields above. catch/finally are NOT
			// in the oneOf list because they aren't standalone wrappers
			// (they require try: to be present, enforced by Step.Validate).
			actionProp = &Property{}
		default:
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
//
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
		def.XImplementsDiff = meta.ImplementsDiff
		def.XImplementsCost = meta.ImplementsCost
		def.XImplementsReverse = meta.ImplementsReverse
		def.XImplementsPermissions = meta.ImplementsPermissions
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
	"pkg.hold":          &config.PkgHold{},
	"pkg.upgrade":       &config.PkgUpgrade{},
	"pkg.list":          &config.PkgList{},
	"os.service":        &config.ServiceAction{},
	"os.user":           &config.OsUser{},
	"os.ssh_key":        &config.OsSSHKey{},
	"os.cron":           &config.OsCron{},
	"os.sysctl":         &config.OsSysctl{},
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
	"text.patch.ini":    &config.TextPatchINI{},
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
	"read.json":         &config.ReadFile{},
	"read.yaml":         &config.ReadFile{},

	// Newly-wired up; previously missing from the table which meant
	// the schema generator emitted an empty `{type: object,
	// additionalProperties: false}` for them — and any YAML plan
	// using these actions failed JSON-schema validation at apply time.
	"os.firewall":            &config.OsFirewall{},
	"os.group":               &config.OsGroup{},
	"os.mount":               &config.OsMount{},
	"os.systemd":             &config.OsSystemd{},
	"text.patch.json":        &config.TextPatchJSON{},
	"text.patch.yaml":        &config.TextPatchYAML{},
	"windows.firewall_rule":  &config.WindowsFirewallRule{},
	"windows.scheduled_task": &config.WindowsScheduledTask{},
	"http.request":           &config.HTTPRequest{},

	"observe.port":    &config.ObservePort{},
	"observe.process": &config.ObserveProcess{},
	"observe.http":    &config.ObserveHTTP{},
	"observe.service": &config.ObserveService{},
	"observe.cpu":     &config.ObserveCPU{},
	"observe.memory":  &config.ObserveMemory{},
	"observe.disk":    &config.ObserveDisk{},
	"observe.gpu":     &config.ObserveGPU{},
	"observe.logs":    &config.ObserveLogs{},
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
