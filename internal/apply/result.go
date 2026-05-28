package apply

// result.go materializes the kernel-surface contract from
// docs-working/vision/kernel.md: a typed *KernelResult carrying
// the plan that ran, per-step outcomes, the audit event tail, and
// summary counters. KernelResult.Reverse() exposes the
// transaction-walker pattern (LIFO over completed reversible steps;
// call Reverser per step; assemble an inverse plan) as a public
// kernel operation for the first time.
//
// Downstream consumers (cross-run rewind, MCP rollback tools, the
// agent loop's "undo your last action" affordance) build on
// KernelResult.Reverse() without re-deriving the algorithm.

import (
	"fmt"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/plan"
)

// KernelResult is the kernel's typed "what just happened" shape.
// Returned by Runner.Run and consumable directly by Reverse(),
// Explain (un-shipped), MCP rollback tools, and SDK callers
// without further re-derivation. See vision/kernel.md.
type KernelResult struct {
	// Plan is the typed plan that was executed. nil if Run failed
	// before plan compilation (e.g. validation error).
	Plan *plan.Plan

	// Steps lists per-step outcomes in execution order.
	// One entry per step.completed event observed during the run.
	Steps []StepResult

	// Events is the audit substrate — the run's full event tail
	// in observed order. Stable for serialisation to JSONL.
	Events []events.Event

	// Summary aggregates ok/changed/skipped/failed/reverted
	// counters plus wall-clock duration. Derived from the
	// run.completed event.
	Summary RunSummary
}

// StepResult bundles a step's input + outcome. Reverse() walks
// these in LIFO order. Held by value rather than pointer because
// the slice may outlive the executor's per-run state.
type StepResult struct {
	// Step is the typed step the executor dispatched. Carries the
	// step's action, name, ID, tags, transaction membership, and
	// the ReverseData snapshot the handler captured pre-mutation
	// (spec-22 phase 5).
	Step config.Step

	// Result is the executor's typed outcome — stdout/stderr/rc/
	// changed/failed/skipped flags plus any action-specific Data
	// or Detail. nil only if the step never ran (currently never;
	// included for forward compatibility).
	Result *executor.Result
}

// RunSummary aggregates run-wide counters. Field shapes match
// events.RunCompletedData so MCP / agent / SDK callers can map
// 1-to-1.
type RunSummary struct {
	TotalSteps int
	// Ok is the legacy "successful steps" count — equal to SuccessSteps
	// (total executed-not-failed). MCP / agentd / history consumers have
	// read it under this name since proposal-02 shipped; F6 keeps the
	// name and semantics for backward compatibility, and adds OkSteps
	// below as the actual "ran-without-changes" bucket the recap line
	// reports as `ok=`.
	Ok int
	// OkSteps counts steps that completed successfully without changing
	// the system (F6). OkSteps + Changed == Ok (i.e. SuccessSteps) for
	// the typical case. Distinct from the legacy Ok field above so
	// existing consumers keep their meaning.
	OkSteps  int
	Changed  int
	Skipped  int
	Failed   int
	Reverted int
	// Cancelled counts steps interrupted mid-execution per proposal-02
	// (SIGINT, fleet kill, timeout).
	Cancelled int
	// Healed counts assert steps that failed, ran their heal: child
	// plan, and passed the re-check (proposal-11).
	Healed       int
	DurationMs   int64
	Success      bool
	ErrorMessage string
	CheckMode    bool
}

// Reverse builds the inverse plan from this run's reversible
// subset. Returns the empty plan (no steps) if nothing was
// reversible.
//
// Walks Steps in LIFO order. For each step where Result.Changed
// is true, consults the action's registered handler:
//
//   - If the handler implements actions.Reverser AND its Reverse
//     call returns a non-nil inverse step, the inverse step is
//     appended to the returned plan.
//   - If the handler does not implement Reverser, the step is
//     silently skipped (no error). Same semantics as the existing
//     transaction walker.
//   - If Reverse returns (nil, nil) the handler is signalling
//     "no-op reverse" (e.g. a step that produced no change in
//     practice). The step is skipped.
//   - If Reverse returns (nil, err) the handler declares itself
//     irreversible. The error is wrapped with the step name and
//     returned; the inverse plan up to that point is still
//     returned (caller can inspect it).
//
// The inverse plan inherits the original plan's RootFile and
// metadata so callers can serialise it via plan.Save. Step order
// in the returned plan reflects the LIFO walk — running it
// undoes the original from last-completed back to first.
//
// This is the same algorithm used by transaction:'s LIFO
// rollback; lifted here so cross-run rewind / MCP rollback /
// agent-loop undo can build on it without depending on
// transaction state.
func (r *KernelResult) Reverse() (*plan.Plan, error) {
	out := &plan.Plan{
		Version: "1.0",
	}
	if r == nil {
		return out, nil
	}
	if r.Plan != nil {
		out.RootFile = r.Plan.RootFile
		out.GeneratedOn = r.Plan.GeneratedOn
	}
	out.GeneratedAt = time.Now().UTC()

	ctx := newReverseContext()
	for i := len(r.Steps) - 1; i >= 0; i-- {
		sr := r.Steps[i]
		if sr.Result == nil || !sr.Result.Changed {
			continue
		}
		actionType := sr.Step.ActionType
		if actionType == "" {
			actionType = sr.Step.DetermineActionType()
		}
		handler, ok := actions.Get(actionType)
		if !ok {
			// Unknown action — silently skip; nothing to reverse.
			continue
		}
		if !actions.IsReverser(handler) {
			// Handler doesn't natively implement Reverser.
			// Default reverser refuses; skip silently.
			continue
		}
		reverser := actions.ResolveReverser(handler)
		stepCopy := sr.Step
		var resultVal executor.Result
		if sr.Result != nil {
			resultVal = *sr.Result
		}
		inverse, err := reverser.Reverse(ctx, &stepCopy, &resultVal)
		if err != nil {
			return out, fmt.Errorf("reverse %q: %w", sr.Step.Name, err)
		}
		if inverse == nil {
			// "No-op reverse" — handler signalled nothing to undo.
			continue
		}
		// Strip transaction membership so the inverse step can run
		// as a one-off; matches existing runReverse semantics in
		// internal/executor/transaction.go.
		inverse.ID = ""
		inverse.TxnParent = ""
		inverse.TxnRole = ""
		out.Steps = append(out.Steps, *inverse)
	}
	return out, nil
}
