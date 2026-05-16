// Package explain resolves nouns about the mooncake kernel into typed
// answers. See spec-68 for the full design.
//
// Wave 1 (this file): kind: action only.
//
//	mooncake explain pkg.install
//	mooncake explain shell
//
// Wave 2 will add run / resource / op resolution; wave 3 wires the
// MCP `explain` tool. The discriminated-union payload shape below
// is shared across waves so downstream code (renderers, MCP serde,
// JSON-schema validation) does not change between waves.
//
// Source of truth for wave 1:
//   - actions.Get(name).Metadata() for action metadata
//   - the matching node in internal/config/schema.json for the schema
//   - Differ / Reverser type assertions for diff_shape / reverse_shape
//   - examples/*.yml for usage excerpts
//
// No filesystem reads outside the embedded schema, the registered
// action set, and the in-tree examples/ directory. No git. No RAG.
// No third-party services.
package explain

// Kind is the discriminator on Result.
type Kind string

const (
	KindAction   Kind = "action"
	KindRun      Kind = "run"      // implemented in wave 2
	KindResource Kind = "resource" // implemented in wave 2
	KindOp       Kind = "op"       // implemented in wave 2
	KindNotFound Kind = "not_found"
)

// Result is the discriminated-union payload returned by Resolve.
// Exactly one of the kind-named pointer fields is non-nil; Kind
// names which.
//
// JSON / YAML / MCP serialization MUST emit Kind as a top-level
// "kind" field and inline the populated payload — see render.go and
// the spec-68 outputSchema for the wire shape.
type Result struct {
	Kind     Kind             `json:"kind"               yaml:"kind"`
	Action   *ActionPayload   `json:"action,omitempty"   yaml:"action,omitempty"`
	NotFound *NotFoundPayload `json:"not_found,omitempty" yaml:"not_found,omitempty"`
	// Run / Resource / Op land in wave 2.
}

// ActionPayload is the kind:action wire shape.
//
// Mirrors spec-68 §"The noun set" §1 verbatim.
type ActionPayload struct {
	Name         string         `json:"name"                   yaml:"name"`
	Metadata     ActionMetadata `json:"metadata"               yaml:"metadata"`
	Schema       *SchemaSlice   `json:"schema,omitempty"       yaml:"schema,omitempty"`
	DiffShape    DiffShape      `json:"diff_shape"             yaml:"diff_shape"`
	ReverseShape ReverseShape   `json:"reverse_shape"          yaml:"reverse_shape"`
	Examples     []ExampleHit   `json:"examples,omitempty"     yaml:"examples,omitempty"`
	SpecOrigin   []SpecRef      `json:"spec_origin,omitempty"  yaml:"spec_origin,omitempty"`
}

// ActionMetadata is the displayable subset of internal/actions.ActionMetadata.
// We re-declare it here so JSON / YAML field names stay stable
// independent of the internal struct (which is named for Go conventions,
// not wire format).
type ActionMetadata struct {
	Description        string   `json:"description"                   yaml:"description"`
	Category           string   `json:"category"                      yaml:"category"`
	Version            string   `json:"version"                       yaml:"version"`
	SupportedPlatforms []string `json:"supported_platforms,omitempty" yaml:"supported_platforms,omitempty"`
	SupportsDryRun     bool     `json:"supports_dry_run"              yaml:"supports_dry_run"`
	SupportsBecome     bool     `json:"supports_become"               yaml:"supports_become"`
	RequiresSudo       bool     `json:"requires_sudo"                 yaml:"requires_sudo"`
	ImplementsCheck    bool     `json:"implements_check"              yaml:"implements_check"`
	EmitsEvents        []string `json:"emits_events,omitempty"        yaml:"emits_events,omitempty"`
}

// SchemaSlice carries the per-action node out of schema.json, with
// the required-and-property subset that's useful for a tool selector.
// The full JSON Schema node is preserved in Raw for callers that want
// to validate input against it without re-reading schema.json.
type SchemaSlice struct {
	Required   []string                  `json:"required,omitempty"   yaml:"required,omitempty"`
	Properties map[string]SchemaProperty `json:"properties,omitempty" yaml:"properties,omitempty"`
	Raw        map[string]any            `json:"-"                    yaml:"-"`
}

// SchemaProperty is a flattened summary of one property node from
// the action's schema definition.
type SchemaProperty struct {
	Type        string   `json:"type,omitempty"        yaml:"type,omitempty"`
	Enum        []any    `json:"enum,omitempty"        yaml:"enum,omitempty"`
	Description string   `json:"description,omitempty" yaml:"description,omitempty"`
	Default     any      `json:"default,omitempty"     yaml:"default,omitempty"`
	Required    bool     `json:"required,omitempty"    yaml:"required,omitempty"`
	OneOf       []string `json:"one_of,omitempty"      yaml:"one_of,omitempty"`
}

// DiffShape is "what kind of typed Diff does this action emit, if any."
// Wave 1 reports declared vs not-declared; richer shape inspection
// lands in wave 2 when we have a sample step to feed Differ.Diff.
type DiffShape struct {
	Declared bool   `json:"declared"          yaml:"declared"`
	Note     string `json:"note,omitempty"    yaml:"note,omitempty"`
}

// ReverseShape is "does this action ship Reverse, and what does the
// reverse step look like."
type ReverseShape struct {
	Declared     bool         `json:"declared"                yaml:"declared"`
	ProducesStep *ReverseStep `json:"produces_step,omitempty" yaml:"produces_step,omitempty"`
	Caveat       string       `json:"caveat,omitempty"        yaml:"caveat,omitempty"`
}

// ReverseStep is a sketch of the step a Reverser would emit. Wave 1
// leaves this nil; wave 2 may synthesize it from a sample.
type ReverseStep struct {
	Action string         `json:"action"          yaml:"action"`
	Args   map[string]any `json:"args,omitempty"  yaml:"args,omitempty"`
}

// ExampleHit is one excerpt from the examples/ tree showing this
// action in use.
type ExampleHit struct {
	Path    string `json:"path"    yaml:"path"`
	Excerpt string `json:"excerpt" yaml:"excerpt"`
}

// SpecRef points at the spec a behavior is grounded in. Empty on
// wave 1 — populated in a follow-up that wires spec frontmatter.
type SpecRef struct {
	Spec string `json:"spec" yaml:"spec"`
	File string `json:"file" yaml:"file"`
}

// NotFoundPayload is the typed not_found response. The candidates
// list is the agent's hook to recover: "you said X; you might have
// meant one of these."
type NotFoundPayload struct {
	Noun       string          `json:"noun"                 yaml:"noun"`
	Candidates []NotFoundMatch `json:"candidates,omitempty" yaml:"candidates,omitempty"`
	Reason     string          `json:"reason,omitempty"     yaml:"reason,omitempty"`
}

// NotFoundMatch is one candidate for a misresolved noun.
type NotFoundMatch struct {
	Kind Kind   `json:"kind" yaml:"kind"`
	ID   string `json:"id"   yaml:"id"`
}
