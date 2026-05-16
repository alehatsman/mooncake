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

import "time"

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
	Kind     Kind             `json:"kind"                 yaml:"kind"`
	Action   *ActionPayload   `json:"action,omitempty"     yaml:"action,omitempty"`
	Run      *RunPayload      `json:"run,omitempty"        yaml:"run,omitempty"`
	Resource *ResourcePayload `json:"resource,omitempty"   yaml:"resource,omitempty"`
	Op       *OpPayload       `json:"op,omitempty"         yaml:"op,omitempty"`
	NotFound *NotFoundPayload `json:"not_found,omitempty"  yaml:"not_found,omitempty"`
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

// RunPayload is the kind:run wire shape — what the operator wants
// from "what did this run actually do?"
//
// Mirrors spec-68 §"The noun set" §2. Per-step typed Diff is omitted
// in wave 2 because executor.Result carries plan-time Detail only,
// not apply-time Diff; the Steps array carries enough metadata
// (action verb + resource handle + result + reversibility) to drive
// agent follow-ups without it.
type RunPayload struct {
	RunID      string     `json:"run_id"                 yaml:"run_id"`
	OpID       string     `json:"op_id,omitempty"        yaml:"op_id,omitempty"`
	TS         time.Time  `json:"ts"                     yaml:"ts"`
	Config     string     `json:"config,omitempty"       yaml:"config,omitempty"`
	DurationMs int64      `json:"duration_ms,omitempty"  yaml:"duration_ms,omitempty"`
	Totals     RunTotals  `json:"totals"                 yaml:"totals"`
	Steps      []RunStep  `json:"steps,omitempty"        yaml:"steps,omitempty"`
	Caveats    RunCaveats `json:"caveats"                yaml:"caveats"`
}

// RunTotals mirrors the runlog totals row.
type RunTotals struct {
	Changed int `json:"changed" yaml:"changed"`
	Ok      int `json:"ok"      yaml:"ok"`
	Skipped int `json:"skipped" yaml:"skipped"`
	Failed  int `json:"failed"  yaml:"failed"`
}

// RunStep is a single row in RunPayload.Steps. Sourced from
// runlog.StepEntry — see internal/runlog for the writer side.
type RunStep struct {
	Index      int    `json:"index"                yaml:"index"`
	Action     string `json:"action"               yaml:"action"`
	Resource   string `json:"resource,omitempty"   yaml:"resource,omitempty"`
	Result     string `json:"result"               yaml:"result"`
	DurationMs int64  `json:"duration_ms,omitempty" yaml:"duration_ms,omitempty"`
	Reversible bool   `json:"reversible,omitempty" yaml:"reversible,omitempty"`
}

// RunCaveats surfaces the run-level metadata an operator wants
// up-front: how many steps are irreversible (no Reverser), whether
// the run failed, etc.
type RunCaveats struct {
	IrreversibleStepCount int `json:"irreversible_step_count" yaml:"irreversible_step_count"`
}

// ResourcePayload is the kind:resource wire shape — newest-first
// history of every step that touched a given resource handle.
//
// Mirrors spec-68 §"The noun set" §3.
type ResourcePayload struct {
	Resource string          `json:"resource"            yaml:"resource"`
	History  []ResourceEvent `json:"history,omitempty"   yaml:"history,omitempty"`
}

// ResourceEvent is one row in a resource's history.
type ResourceEvent struct {
	RunID      string    `json:"run_id"                yaml:"run_id"`
	OpID       string    `json:"op_id,omitempty"       yaml:"op_id,omitempty"`
	TS         time.Time `json:"ts"                    yaml:"ts"`
	Action     string    `json:"action"                yaml:"action"`
	Result     string    `json:"result"                yaml:"result"`
	Reversible bool      `json:"reversible,omitempty"  yaml:"reversible,omitempty"`
}

// OpPayload is the kind:op wire shape — "what command was this and
// what did it produce?"
//
// Mirrors spec-68 §"The noun set" §4. Parent chains for replay-of-
// replay are exposed via the Parent field; consumers recurse on
// Parent to walk the chain.
type OpPayload struct {
	OpID     string    `json:"op_id"               yaml:"op_id"`
	TS       time.Time `json:"ts"                  yaml:"ts"`
	Command  string    `json:"command"             yaml:"command"`
	Args     []string  `json:"args,omitempty"      yaml:"args,omitempty"`
	Actor    string    `json:"actor,omitempty"     yaml:"actor,omitempty"`
	Parent   string    `json:"parent,omitempty"    yaml:"parent,omitempty"`
	Config   string    `json:"config,omitempty"    yaml:"config,omitempty"`
	Runs     []string  `json:"runs,omitempty"      yaml:"runs,omitempty"`
	PlanOnly bool      `json:"plan_only,omitempty" yaml:"plan_only,omitempty"`
}
