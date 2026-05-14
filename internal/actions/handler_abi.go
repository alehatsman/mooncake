package actions

import "github.com/alehatsman/mooncake/internal/config"

// Spec-22 extended Handler ABI.
//
// Beyond the minimal Handler interface (Metadata/Validate/Run), handlers
// may opt into any of four extension capabilities by implementing the
// corresponding sub-interface:
//
//   - Differ     — return a structured per-step diff (machine-readable).
//   - Reverser   — return a Step that undoes this step's side effects.
//   - Coster     — return a coarse blast-radius estimate.
//   - Permitter  — declare what privileges/binaries the step needs.
//
// Each is OPT-IN. Handlers that don't implement a given sub-interface
// receive safe defaults from the registry helpers (ResolveDiffer /
// ResolveReverser / ResolveCoster / ResolvePermitter). The Handler
// interface itself is unchanged; this file adds new types only.
//
// See docs-working/specs/action-surface/spec-22-extended-handler-abi.md
// for design rationale and the per-handler implementation matrix.

// ----- Diff -----------------------------------------------------------------

// Diff is a machine-readable structural delta of what a step would change.
// Returned by handlers implementing the Differ interface; consumed by
// mooncake plan --format json, the agent SDK, and (later) the policy /
// transaction layers.
//
// Diff is intentionally distinct from snapshot.SnapshotDiff (which
// compares two full system snapshots). A Diff describes ONE step's
// predicted change to ONE resource; SnapshotDiff describes the
// before/after of an entire box.
type Diff struct {
	// Resource identifies the thing being changed: a file path, a
	// package name, a service unit, an external object reference.
	Resource ResourceRef `json:"resource"`

	// Operation classifies the change at a coarse level. Combined
	// with Before / After it tells a consumer "what kind of change"
	// without needing to inspect the typed payload.
	Operation Operation `json:"operation"`

	// Before is the pre-change state, action-defined shape. nil for
	// OpCreate (nothing was there). For file.write: a FileSnapshot
	// (path + size + sha256 + mode + ...). For pkg: a PkgSnapshot
	// (name + installed-version-before).
	Before any `json:"before,omitempty"`

	// After is the post-change state. nil for OpDelete. Same typed
	// shape as Before per action; consumers learn the type from
	// Resource.Kind.
	After any `json:"after,omitempty"`

	// Lines, when populated, is a unified-diff-style breakdown for
	// textual content. Mostly used by file.write / file.template /
	// text.* handlers. Empty for non-textual actions.
	Lines []DiffLine `json:"lines,omitempty"`
}

// ResourceRef identifies the target of a step's change. Kind selects
// the schema of any typed metadata the consumer might need; Identifier
// is the human-readable handle (path / package name / service unit).
// Attributes carries optional extras without forcing a per-kind struct.
type ResourceRef struct {
	Kind       ResourceKind      `json:"kind"`
	Identifier string            `json:"identifier"`
	Attributes map[string]string `json:"attributes,omitempty"`
}

// ResourceKind tags a ResourceRef so consumers can dispatch on the
// expected shape of Before / After. Kept open-ended as a string type so
// future actions (k8s objects, cloud resources) can add their own kinds
// without breaking consumers that don't recognize them.
type ResourceKind string

const (
	ResourceFile    ResourceKind = "file"
	ResourcePackage ResourceKind = "package"
	ResourceService ResourceKind = "service"
	ResourceText    ResourceKind = "text"
	ResourceShell   ResourceKind = "shell"
	ResourceVar     ResourceKind = "var"
	ResourceOther   ResourceKind = "other"
)

// Operation is a coarse classifier shared by every action's Diff. Lets
// a UI sort/group changes ("3 creates, 1 update, 0 deletes") without
// looking at the typed Before/After.
type Operation string

const (
	OpCreate Operation = "create"
	OpUpdate Operation = "update"
	OpDelete Operation = "delete"
	OpNoop   Operation = "noop"
)

// DiffLine is one entry in a unified-diff-style breakdown of text
// content. Op uses single-character markers matching `diff -u` output:
// "+" added, "-" removed, " " context. LineNumber refers to the
// post-change file when Op != "-", to the pre-change file when "-",
// and to either when " ".
type DiffLine struct {
	Op         DiffOp `json:"op"`
	Text       string `json:"text"`
	LineNumber int    `json:"line_number,omitempty"`
}

// DiffOp is the one-character marker for a DiffLine.
type DiffOp string

const (
	DiffOpAdd     DiffOp = "+"
	DiffOpRemove  DiffOp = "-"
	DiffOpContext DiffOp = " "
)

// ----- Cost -----------------------------------------------------------------

// CostEstimate is a coarse, pre-execution signal of a step's blast
// radius. Consumed by run-recap (averaged risk + summed resources),
// JSON plan output (per-step), and future policy layers. Not a hard
// gate — informational unless something downstream chooses to enforce.
type CostEstimate struct {
	// Resources is a lower-bound count of distinct things this step
	// touches (files, packages, service units, ...). Lower bound:
	// dynamic actions may touch more. -1 = unknown.
	Resources int `json:"resources"`

	// Bytes is an order-of-magnitude estimate of bytes written or
	// mutated by this step. -1 = unknown / not applicable.
	Bytes int64 `json:"bytes"`

	// Reversible reports whether the handler implements Reverser
	// (and would therefore return a non-nil Step from Reverse).
	// Mirrors what `(h, ok := h.(Reverser)); ok` would report.
	Reversible bool `json:"reversible"`

	// Risk is a 1..10 informational band:
	//   1–3   safe (read-only, idempotent writes to scratch)
	//   4–6   routine (config writes, package installs)
	//   7–9   high impact (service restarts, kernel params)
	//   10    destructive (deletes, drops, rm -rf)
	// Default fallback for handlers that don't implement Coster is 5.
	Risk int `json:"risk"`
}

// ----- Permissions ----------------------------------------------------------

// PermissionSet declares the privileges and external dependencies a
// step needs to run. Consumed by executor preflight (fail-fast if
// a required binary is missing or Sudo is required and we're not
// elevated), plan output (surface `requires:` lines per step), and the
// future policy DSL.
type PermissionSet struct {
	// Sudo: this step requires elevated privileges. Executor checks
	// effective UID (or that as_user resolves to root) before run.
	Sudo bool `json:"sudo,omitempty"`

	// Network: this step makes outbound network calls. Informational
	// today; a later policy layer may gate on this.
	Network bool `json:"network,omitempty"`

	// RequiredBinaries: programs that must resolve via exec.LookPath
	// before this step can run. Executor fails preflight with a
	// clear "missing binary X for action Y" message.
	RequiredBinaries []string `json:"required_binaries,omitempty"`

	// FilesystemWrite: declared write paths or globs. "*" = anywhere.
	// Used by policy / UI surfaces — NOT enforced today.
	FilesystemWrite []string `json:"filesystem_write,omitempty"`

	// Notes: free-form human-readable extras for UI display.
	Notes []string `json:"notes,omitempty"`
}

// ----- Sub-interfaces -------------------------------------------------------

// Differ is the optional interface handlers implement to produce a
// structured per-step Diff. Called in plan mode by the planner; consumed
// by JSON plan output, the agent SDK, and any UI past `mooncake plan`.
//
// Handlers without a Differ implementation get a coarse default
// (Operation-only, derived from Run's plan-mode Result) via
// ResolveDiffer.
type Differ interface {
	Diff(ctx Context, step *config.Step) (Diff, error)
}

// Reverser is the optional interface handlers implement to declare how
// their effect is undone. Spec-30 (`transaction:` blocks) is the
// primary consumer: on transaction failure the executor walks completed
// steps in reverse order and applies the Step each Reverser returns.
//
// Result-arg note: Reverse takes the apply-time Result so the handler
// can use any data captured during Run (e.g. the path of a backup
// file, the previous package version). For purely structural reverses
// the arg may be ignored.
//
// Return semantics:
//   - (step, nil)    — apply this Step to undo
//   - (nil, nil)     — no reverse needed (e.g. step was a noop)
//   - (nil, error)   — handler declares itself irreversible; rollback
//     requires manual intervention. Transaction will surface this.
type Reverser interface {
	Reverse(ctx Context, step *config.Step, result Result) (*config.Step, error)
}

// Coster is the optional interface for pre-execution blast-radius
// signal. Handlers that don't implement it get a neutral default of
// Risk=5 with Reversible inferred from whether Reverser is implemented.
type Coster interface {
	Cost(ctx Context, step *config.Step) (CostEstimate, error)
}

// Permitter is the optional interface for declaring required privileges.
// Cheap to implement (often a static return) and high-leverage: surfaces
// permission requirements at plan time instead of as runtime failures.
type Permitter interface {
	Permissions(step *config.Step) PermissionSet
}
