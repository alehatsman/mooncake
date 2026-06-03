// Package mooncake is the public, stable-facing entry point for building
// agents and tools on top of the mooncake kernel.
//
// It is a thin facade over the implementation packages under internal/.
// Everything here is a type alias or a constructor wrapper, so an external
// module can import this package alone — it never has to name an internal/
// path (which Go forbids cross-module). The implementation stays in
// internal/ until the API settles; the Phase-2 repo split (issue #110) is a
// mechanical move once it does.
//
// # Building a custom-action agent
//
// A consumer compiles its own agent binary by seeding a registry with the
// built-ins, registering its own typed Handler implementations, and pointing
// the loop at that registry:
//
//	reg := mooncake.DefaultRegistry()        // built-ins (file/http/shell/…)
//	_ = reg.Register(myteam.IssueHandler{})  // a custom typed action
//	res, err := mooncake.RunLoop(ctx, mooncake.RunOptions{
//	    Goal:     "open a tracking issue for the failing build",
//	    RepoRoot: ".",
//	    Registry: reg,                        // ← custom vocabulary
//	})
//
// The registered action surfaces in the planner's vocabulary automatically
// (see BuildSchemaChunkForRegistry) and resolves at execution time — no
// schema.json edit, no prompt change. The moat is preserved: a custom action
// is admitted only by implementing the typed Handler ABI (Metadata / Validate
// / Run, plus the optional Reverser / Coster / Permitter / Differ capability
// interfaces). There is no untyped extension boundary. See
// docs-working/vision/agent_framework.md.
package mooncake

import (
	"context"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/agent"
	"github.com/alehatsman/mooncake/internal/agent/llm"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"

	// Registering this import populates the global registry with every
	// built-in handler via their init() functions, so DefaultRegistry can
	// hand out a clone seeded with the built-ins.
	_ "github.com/alehatsman/mooncake/internal/register"
)

// ----------------------------------------------------------------------------
// Action registry + the typed Handler ABI
// ----------------------------------------------------------------------------

// Registry holds the action handlers a run resolves against. Build one with
// NewRegistry (empty) or DefaultRegistry (seeded with the built-ins), then
// Register custom typed handlers into it.
type Registry = actions.Registry

// Handler is the typed action ABI. A custom action implements Metadata,
// Validate, and Run; optionally one or more of the capability interfaces
// below to declare reversibility, cost, permissions, or a diff.
type Handler = actions.Handler

// Context is the execution context passed to Handler.Run — it exposes the
// template renderer, variables, event publisher, logger, and run mode.
type Context = actions.Context

// Result is the typed outcome interface a Handler.Run returns. Build a
// concrete one with NewResult (which returns a *ResultData implementing this
// interface).
type Result = actions.Result

// ResultData is the concrete result value a handler populates — set Changed,
// Stdout, Failed, Reason, etc. on it, then return it as a Result. Construct
// with NewResult.
type ResultData = executor.Result

// ActionMetadata describes an action: its name, description, category, and
// capability flags. Returned by Handler.Metadata.
type ActionMetadata = actions.ActionMetadata

// Mode is the run mode passed to a handler: ModeApply performs real work,
// ModePlan predicts changes without side effects.
type Mode = actions.Mode

// Run modes. See Mode.
const (
	ModeApply = actions.ModeApply
	ModePlan  = actions.ModePlan
)

// The optional capability interfaces a Handler may also implement. Each one
// a custom action satisfies spreads a typed kernel guarantee to that action.
type (
	// Reverser declares a compensating action (honest SAGA rollback).
	Reverser = actions.Reverser
	// Coster declares an estimated cost for plan-time budgeting.
	Coster = actions.Coster
	// Permitter declares the permissions an action requires for policy gating.
	Permitter = actions.Permitter
	// Differ declares a typed pre-execution diff.
	Differ = actions.Differ
	// Runner is the Run capability, named for call sites that want it explicitly.
	Runner = actions.Runner
)

// Capability payload types. A consumer needs these to *implement* the
// capability interfaces above against the facade alone: Permitter returns a
// PermissionSet, Coster returns a CostEstimate, Differ returns a Diff.
type (
	// PermissionSet declares the privileges/network/binaries a step requires,
	// surfaced at plan time. Returned by Permitter.Permissions.
	PermissionSet = actions.PermissionSet
	// CostEstimate is a coarse pre-execution blast-radius signal (resources,
	// bytes, risk band, reversibility). Returned by Coster.Cost.
	CostEstimate = actions.CostEstimate
)

// The Differ payload cluster (#120). A consumer that implements Differ must be
// able to name every type its return value reaches through, against the facade
// alone — Diff carries a ResourceRef (tagged by ResourceKind), an Operation,
// and an optional []DiffLine (each tagged by a DiffOp).
type (
	// Diff is a machine-readable structural delta of what one step would
	// change to one resource. Returned by Differ.Diff. Before/After are
	// action-defined typed payloads (e.g. a file snapshot); Lines carries
	// an optional unified-diff-style breakdown for textual content.
	Diff = actions.Diff
	// ResourceRef identifies the target of a step's change: a Kind, a
	// human-readable Identifier (path / package / unit), and optional
	// Attributes.
	ResourceRef = actions.ResourceRef
	// ResourceKind tags a ResourceRef so consumers can dispatch on the shape
	// of Before/After (file, package, service, ...).
	ResourceKind = actions.ResourceKind
	// Operation is the coarse change classifier shared by every Diff
	// (create / update / delete / noop).
	Operation = actions.Operation
	// DiffLine is one entry in a unified-diff-style breakdown of text content.
	DiffLine = actions.DiffLine
	// DiffOp is the one-character marker for a DiffLine ("+" / "-" / " ").
	DiffOp = actions.DiffOp
)

// ResourceKind values. See ResourceRef.
const (
	ResourceFile    = actions.ResourceFile
	ResourcePackage = actions.ResourcePackage
	ResourceService = actions.ResourceService
	ResourceText    = actions.ResourceText
	ResourceShell   = actions.ResourceShell
	ResourceVar     = actions.ResourceVar
	ResourceGit     = actions.ResourceGit
	ResourceOther   = actions.ResourceOther
)

// Operation values. See Diff.Operation.
const (
	OpCreate = actions.OpCreate
	OpUpdate = actions.OpUpdate
	OpDelete = actions.OpDelete
	OpNoop   = actions.OpNoop
)

// DiffOp markers. See DiffLine.
const (
	DiffOpAdd     = actions.DiffOpAdd
	DiffOpRemove  = actions.DiffOpRemove
	DiffOpContext = actions.DiffOpContext
)

// Policy is the per-run permissions-as-contract gate (#11) applied to every
// plan an agent run executes: an action allow/deny list, a network switch, and
// a risk cap. A nil *Policy (and the zero value) enforces nothing. Set it on
// RunOptions.Policy to drop the shell escape hatch from an unattended run
// (Policy{DeniedActions: []string{"shell", "cmd"}}) — the planner may still
// propose a denied step, but the executor refuses it before any side effect.
type Policy = executor.Policy

// Step is a single plan step. Handler.Validate and Handler.Run receive a
// *Step; a custom handler reads its own configuration off it.
type Step = config.Step

// NewRegistry returns an empty registry. Use DefaultRegistry to start from
// the built-ins instead.
func NewRegistry() *Registry {
	return actions.NewRegistry()
}

// GlobalRegistry returns the process-wide default registry the built-ins
// register into. Prefer DefaultRegistry (a clone) over mutating this.
func GlobalRegistry() *Registry {
	return actions.GlobalRegistry()
}

// DefaultRegistry returns a fresh registry pre-populated with all built-in
// handlers (a clone of the global). Register custom handlers into the result
// without affecting the global or other consumers.
func DefaultRegistry() *Registry {
	return actions.GlobalRegistry().Clone()
}

// RegisterBuiltins registers every built-in handler into reg — the explicit
// alternative to DefaultRegistry for a consumer that starts from NewRegistry
// and wants the built-ins alongside its own handlers. A built-in whose name
// is already registered in reg is skipped, so pre-registering an override is
// safe.
//
//	reg := mooncake.NewRegistry()
//	_ = mooncake.RegisterBuiltins(reg)
//	_ = reg.Register(myCustomHandler)
func RegisterBuiltins(reg *Registry) error {
	return actions.RegisterBuiltins(reg)
}

// NewResult constructs an empty result for a Handler.Run implementation to
// populate (Changed / Stdout / Failed / …) and return as a Result.
func NewResult() *ResultData {
	return executor.NewResult()
}

// ----------------------------------------------------------------------------
// Agent loop
// ----------------------------------------------------------------------------

// RunOptions configures an agent run. Set Registry to plan and execute
// against a custom action vocabulary; leave it nil to use the built-ins.
type RunOptions = agent.RunOptions

// LoopResult is the outcome of a RunLoop: iterations run, why it stopped,
// and the final iteration log.
type LoopResult = agent.LoopResult

// IterationLog records one iteration of the loop (or a single Run).
type IterationLog = agent.IterationLog

// DiffStat is the file/insertion/deletion count an iteration changed,
// carried on IterationLog.DiffStat and AgentCompletedData.DiffStat.
type DiffStat = agent.DiffStat

// AgentCompletedData is the payload of the terminal agent.completed event
// emitted in JSON output mode — the run outcome (iterations, stop reason,
// status, diff stat, changed files) a programmatic consumer keys on without
// parsing prose.
type AgentCompletedData = agent.AgentCompletedData

// StopReason explains why RunLoop stopped.
type StopReason = agent.StopReason

// StopReason values. See LoopResult.StopReason.
const (
	StopSuccess    = agent.StopSuccess
	StopNoProgress = agent.StopNoProgress
	StopNoChange   = agent.StopNoChange
	StopFailed     = agent.StopFailed
	StopMaxReached = agent.StopMaxReached
	StopAborted    = agent.StopAborted
	StopStepDone   = agent.StopStepDone
	StopCanceled   = agent.StopCanceled
)

// Output format values for RunOptions.OutputFormat: "" / OutputFormatText is
// the human-readable rendering; OutputFormatJSON emits the NDJSON event stream
// (one Event per line) a programmatic consumer parses.
const (
	OutputFormatText = agent.OutputFormatText
	OutputFormatJSON = agent.OutputFormatJSON
)

// Style selects the planning style.
type Style = agent.Style

// Planning styles. StylePlan emits one complete plan per turn; StyleStep
// emits one action per turn and feeds results back.
const (
	StylePlan = agent.StylePlan
	StyleStep = agent.StyleStep
)

// ConfirmResult is what an Approver returns: the outcome plus optional edited
// plan bytes.
type ConfirmResult = agent.ConfirmResult

// ConfirmOutcome is the decision an Approver renders on a generated plan.
type ConfirmOutcome = agent.ConfirmOutcome

// Approver outcomes.
const (
	OutcomeApply  = agent.OutcomeApply
	OutcomeReject = agent.OutcomeReject
	OutcomeAbort  = agent.OutcomeAbort
)

// ----------------------------------------------------------------------------
// Reasoning backend (the L4 layer — swappable)
// ----------------------------------------------------------------------------

// LLMClient is the reasoning backend the loop generates plans against. Set
// RunOptions.LLMClient to inject a fully custom or offline backend (a local
// ollama / vLLM client, or a deterministic test double) directly, bypassing
// the Provider/Endpoint/Model resolution. Implement this single method to
// supply your own.
type LLMClient = llm.Client

// LLMClientOptions selects a built-in backend via the resolution chain
// (provider flag → MOONCAKE_AGENT_PROVIDER → claude binary → CLAUDE_API_KEY →
// MOONCAKE_AGENT_ENDPOINT openai-shape). Use with NewLLMClient when you want a
// resolved client to inspect or reuse; for the common case, set the
// Provider/Endpoint/Model fields on RunOptions instead.
type LLMClientOptions = llm.ClientOptions

// NewLLMClient resolves a built-in reasoning backend from opts. A fully
// offline run points Endpoint at a local OpenAI-shape server (ollama / vLLM);
// no cloud is required.
func NewLLMClient(opts LLMClientOptions) (LLMClient, error) {
	return llm.NewClientWithOptions(opts)
}

// RunLoop runs the iterate-until-done agent loop: generate a plan, gate it,
// execute it, feed results back, and repeat until the goal is met, the user
// stops it, or a limit is hit. Honors RunOptions.Registry.
func RunLoop(ctx context.Context, opts RunOptions) (*LoopResult, error) {
	return agent.RunLoop(ctx, opts)
}

// Run executes a single agent turn (generate one plan, gate it, execute it)
// and returns its iteration log. Honors RunOptions.Registry.
func Run(ctx context.Context, opts RunOptions) (*IterationLog, error) {
	return agent.Run(ctx, opts)
}

// BuildSchemaChunkForRegistry renders the action-vocabulary section of the
// planner's system prompt from a live registry, so a consumer can inspect
// exactly which actions (built-in + custom) the model will be told about.
func BuildSchemaChunkForRegistry(reg *Registry) (string, error) {
	return agent.BuildSchemaChunkForRegistry(reg)
}
