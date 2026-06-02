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

// Result is the typed outcome a Handler.Run returns. Construct one with
// NewResult.
type Result = actions.Result

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

// NewResult constructs an empty Result for a Handler.Run implementation to
// populate (Changed / Stdout / Failed / …).
func NewResult() Result {
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

// StopReason explains why RunLoop stopped.
type StopReason = agent.StopReason

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
