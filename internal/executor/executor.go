// Package executor implements the core execution engine for mooncake configuration plans.
//
// The executor is responsible for:
//   - Loading and validating configuration plans
//   - Expanding steps (loops, includes, presets)
//   - Evaluating conditions (when, unless, creates)
//   - Dispatching actions to handlers
//   - Managing execution context and variables
//   - Tracking results and statistics
//   - Emitting events for observability
//   - Handling dry-run mode
//   - Supporting privilege escalation (sudo/become)
//
// # Architecture
//
// The executor follows a pipeline architecture:
//
//	Plan Loading → Step Expansion → Condition Evaluation → Action Dispatch → Result Handling
//
// Each step goes through:
//  1. Pre-execution: Check when/unless/creates, apply tags filter
//  2. Variable processing: Merge step vars into context
//  3. Loop expansion: Expand with_items/with_filetree into multiple executions
//  4. Action execution: Dispatch to handler or legacy implementation
//  5. Post-execution: Evaluate changed_when/failed_when, register results
//  6. Event emission: Publish lifecycle events
//
// # Execution Context
//
// ExecutionContext carries all state needed during execution:
//   - Variables: Step vars, global vars, facts, registered results
//   - Template: Jinja2-like template renderer
//   - Evaluator: Expression evaluator for conditions
//   - Logger: Structured logging (TUI or text)
//   - PathUtil: Path resolution and expansion
//   - EventPublisher: Event emission for observability
//   - Stats: Execution statistics (total, success, failed, changed, skipped)
//
// # Action Dispatch
//
// All 28 actions are registered in actions.Registry. Each action is dispatched
// by looking up its handler via actions.Get(actionType) and calling handler.Execute().
//
// # Idempotency
//
// The executor enforces idempotency through:
//   - creates: Skip if path exists
//   - unless: Skip if command succeeds
//   - changed_when: Custom change detection
//   - Handler implementations: Built-in state checking
//
// # Dry-Run Mode
//
// When DryRun is true:
//   - No actual changes are made to the system
//   - Handlers log what would happen
//   - Template rendering still occurs (validates syntax)
//   - File existence checks are performed (read-only)
//   - Statistics track what would have changed
//
// # Error Handling
//
// Errors are wrapped with context using custom error types:
//   - RenderError: Template rendering failures (field + cause)
//   - EvaluationError: Expression evaluation failures (expression + cause)
//   - CommandError: Command execution failures (command + exit code)
//   - FileOperationError: File operation failures (path + operation + cause)
//   - StepValidationError: Configuration validation failures
//   - SetupError: Infrastructure/environment setup failures
//
// Use errors.Is() and errors.As() for programmatic error inspection.
//
// # Usage Example
//
//	// Load configuration
//	steps, err := config.ReadConfig("config.yml")
//	if err != nil {
//	    return err
//	}
//
//	// Create executor
//	log := logger.NewTextLogger()
//	exec := NewExecutor(log)
//
//	// Execute with options
//	result, err := exec.Execute(config.Plan{Steps: steps}, ExecuteOptions{
//	    DryRun: false,
//	    Tags: []string{"setup", "deploy"},
//	    Variables: map[string]interface{}{
//	        "environment": "production",
//	    },
//	})
//
//	// Check results
//	if !result.Success {
//	    log.Errorf("Execution failed: %d failed steps", result.FailedSteps)
//	}
//	log.Infof("Summary: %d changed, %d unchanged, %d failed",
//	    result.ChangedSteps, result.SuccessSteps-result.ChangedSteps, result.FailedSteps)
package executor

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/metrics"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/plan/filter"
	"github.com/alehatsman/mooncake/internal/secrets/resolver"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
	"github.com/alehatsman/mooncake/internal/utils"
)

// idempotencyUnlessTimeout bounds the `unless:` guard's shell-out
// in checkIdempotencyConditions. Guards by convention are cheap
// checks (`test -f`, `pgrep`, `kubectl get …`); 10s catches genuine
// misuse (a guard pointing at an unreachable host) without
// breaking the common case (sub-second probes on a healthy box).
// F055.
const idempotencyUnlessTimeout = 10 * time.Second

// generateStepID creates a unique identifier for a step
func generateStepID(step config.Step, ec *ExecutionContext) string {
	if step.ID != "" {
		return step.ID
	}
	return fmt.Sprintf("step-%d", readStat(ec.Svc.Stats.Global))
}

func markStepFailed(result *Result, step config.Step, ec *ExecutionContext) { //nolint:unused
	result.Failed = true
	result.Rc = 1
	captureResult(ec, step, result.ToRegisteredResult())
}

// AddGlobalVariables populates scope.Facts, scope.Metrics, and scope.Env
// from the system. Facts (capabilities, configuration) come from
// facts.Collect; metrics (live CPU/GPU/memory/load/network) come from
// metrics.Collect with per-metric TTL caching; env is a snapshot of
// the parent process environment exposed to templates as `env.*` so
// users can reference `{{ env.HOME }}`, `{{ env.MY_API_KEY }}`, etc.
// Keys across facts and metrics are disjoint by contract — see
// metrics.disjoint_test.go.
func AddGlobalVariables(scope *VariableScope) {
	scope.Facts = facts.Collect()
	if m, _, err := metrics.Collect(nil); err == nil {
		scope.Metrics = m
	}
	envSlice := os.Environ()
	scope.Env = make(map[string]string, len(envSlice))
	for _, kv := range envSlice {
		if i := strings.IndexByte(kv, '='); i >= 0 {
			scope.Env[kv[:i]] = kv[i+1:]
		}
	}
	// proposal-09: stamp the run's start time on the scope so
	// templates can build `{{ apply_started_at | strftime:... }}`
	// without each step re-reading the wall clock. Set once here so
	// both the apply.Runner path and the cmd/step single-step path
	// pick up the same field automatically.
	scope.ApplyStartedAt = time.Now()
}

func handleVars(step config.Step, ec *ExecutionContext) error { //nolint:unused
	ec.Svc.Logger.Debugf("Handling vars: %+v", step.Vars)

	if step.Vars == nil {
		return fmt.Errorf("vars is nil in step")
	}

	vars := step.Vars

	for k, v := range *vars {
		ec.Svc.Logger.Debugf("  %v: %v", k, v)
	}

	if ec.Mode() == actions.ModePlan {
		NewDryRunLogger(ec.Svc.Logger).LogVariableSet(len(*vars))
	}

	ec.MergeUserVars(*vars)

	// Emit variables.set event
	keys := make([]string, 0, len(*vars))
	for k := range *vars {
		keys = append(keys, k)
	}
	ec.EmitEvent(events.EventVarsSet, events.VarsSetData{
		Count:  len(*vars),
		Keys:   keys,
		DryRun: ec.Mode() == actions.ModePlan,
	})

	return nil
}

func handleWhenExpression(step config.Step, ec *ExecutionContext) (bool, error) {
	whenString := strings.Trim(step.When, " ")

	vars := ec.GetVariables()
	ec.Svc.Logger.Debugf("variables: %v", vars)

	whenExpression, err := ec.Svc.Template.Render(whenString, vars)
	if err != nil {
		return false, &RenderError{Field: "when", Cause: err}
	}

	ec.Svc.Logger.Debugf("whenExpression: %v", whenExpression)

	evalResult, err := ec.Svc.Evaluator.Evaluate(whenExpression, vars)
	if err != nil {
		return false, &EvaluationError{Expression: whenExpression, Cause: err}
	}

	ec.Svc.Logger.Debugf("evalResult: %v", evalResult)

	// Handle nil or non-bool results
	if evalResult == nil {
		return true, nil // Skip if expression evaluates to nil
	}

	boolResult, ok := evalResult.(bool)
	if !ok {
		return false, &EvaluationError{
			Expression: whenExpression,
			Cause:      fmt.Errorf("expression evaluated to %T, expected bool", evalResult),
		}
	}

	return !boolResult, nil
}

func shouldSkipByTags(step config.Step, ec *ExecutionContext) bool { //nolint:unused
	return !utils.MatchesTags(step.Tags, ec.Svc.Tags)
}

func checkIdempotencyConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
	vars := ec.GetVariables()

	// Collect (creates, unless) pairs from both step-level fields and the
	// shell-action-level guards. Shell-level guards mirror the universal
	// step-level fields but live on the action block so users can colocate
	// "what command to run" with "when to skip it".
	type guard struct{ creates, unless string }
	var guards []guard
	if step.UnlessExists != nil || step.UnlessCommand != nil {
		g := guard{}
		if step.UnlessExists != nil {
			g.creates = *step.UnlessExists
		}
		if step.UnlessCommand != nil {
			g.unless = *step.UnlessCommand
		}
		guards = append(guards, g)
	}
	// MT-15: `creates:` / `unless:` are friendly step-level aliases of
	// `unless_exists:` / `unless_command:`. Treated independently so
	// both forms can appear on the same step (rare, but harmless).
	if step.Creates != nil || step.Unless != nil {
		g := guard{}
		if step.Creates != nil {
			g.creates = *step.Creates
		}
		if step.Unless != nil {
			g.unless = *step.Unless
		}
		guards = append(guards, g)
	}
	if step.Shell != nil && (step.Shell.Creates != "" || step.Shell.Unless != "") {
		guards = append(guards, guard{creates: step.Shell.Creates, unless: step.Shell.Unless})
	}

	for _, g := range guards {
		if g.creates != "" {
			path, err := ec.Svc.Template.Render(g.creates, vars)
			if err != nil {
				return false, "", &RenderError{Field: "creates path", Cause: err}
			}
			expandedPath, err := ec.Svc.PathUtil.ExpandPath(path, ec.CurrentDir, vars)
			if err != nil {
				return false, "", &RenderError{Field: "creates path", Cause: err}
			}
			if _, err := os.Stat(expandedPath); err == nil {
				return true, fmt.Sprintf("creates: %s", expandedPath), nil
			}
		}
		if g.unless != "" {
			command, err := ec.Svc.Template.Render(g.unless, vars)
			if err != nil {
				return false, "", &RenderError{Field: "unless command", Cause: err}
			}
			// F055: run the guard via exec.CommandContext so Ctrl-C /
			// context cancel aborts the subprocess instead of waiting
			// for it to exit on its own. The guard runs BEFORE
			// step.started, so without ctx awareness an `unless:
			// kubectl get nodes` against an unreachable cluster
			// hangs mooncake with no visible event. A 10s hard cap
			// bounds well-behaved guards (typical `pgrep` / `test
			// -f` / `kubectl get` complete in <1s on a healthy host)
			// without breaking slow-but-legitimate probes; operators
			// who genuinely need a long guard should compose it via
			// `when:` against pre-computed facts, not a synchronous
			// shell-out. ec.Svc.Ctx is the run-wide cancel context
			// (set by Start / ExecutePlan); nil ctx falls back to
			// Background so legacy callers without a service ctx
			// still work.
			runCtx := context.Background()
			if ec.Svc != nil && ec.Svc.Ctx != nil {
				runCtx = ec.Svc.Ctx
			}
			unlessCtx, cancel := context.WithTimeout(runCtx, idempotencyUnlessTimeout)
			// #nosec G204 -- This is a provisioning tool designed to execute commands from user configs.
			cmd := exec.CommandContext(unlessCtx, "sh", "-c", command)
			runErr := cmd.Run()
			cancel()
			if runErr == nil {
				return true, fmt.Sprintf("unless: %s", command), nil
			}
		}
	}

	return false, "", nil
}

func checkSkipConditions(step config.Step, ec *ExecutionContext) (bool, string, error) {
	// Check if step was marked as skipped during planning (tag filtering)
	// The planner already evaluated tags and set step.Skipped - we trust that decision.
	// No need to recalculate at runtime (performance and single-source-of-truth).
	if step.Skipped {
		return true, "tags", nil
	}

	// spec-23 on_change: a triggered child runs iff its parent's step
	// reported changed=true. Lookup is by ID against the per-context
	// ChangedByStepID map populated as each prior step completes. A miss
	// (parent not yet recorded, or no map at all) treats as "didn't change"
	// — defensive and matches the user's intent that on_change children
	// only run on positive change signals.
	if step.TriggeredBy != "" {
		parentChanged := ec.ChangedByStepID[step.TriggeredBy]
		if !parentChanged {
			return true, "on_change: parent " + step.TriggeredBy + " did not change", nil
		}
	}

	// spec-30 transaction gating: body children after a failure skip
	// (the transaction has already rolled back; their sibling Reverse
	// path already ran). Rollback children skip when the transaction
	// committed. Implementation in transaction.go.
	if reason := ec.txnSkipReason(step); reason != "" {
		return true, reason, nil
	}

	// spec-23 §2 try-block gating: try children skip after a sibling
	// try failure; catch children skip when the try-block succeeded;
	// finally children never skip on this basis. Implementation in
	// trycatch.go.
	if reason := ec.trySkipReason(step); reason != "" {
		return true, reason, nil
	}

	// Check when condition
	if step.When != "" {
		shouldSkip, err := handleWhenExpression(step, ec)
		if err != nil {
			// In plan mode the variable set may not include results from
			// previous steps (nothing actually ran), so a when expression
			// that references e.g. `prior.changed` can be unevaluable.
			// Treat that as "skip in plan, can't predict" rather than
			// aborting the whole inspection. Real execution still
			// surfaces the error.
			if ec.Mode() == actions.ModePlan {
				return true, "when unevaluable in plan mode", nil
			}
			return false, "", err
		}
		if shouldSkip {
			return true, "when: " + strings.TrimSpace(step.When), nil
		}
	}

	// NOTE: Removed redundant ShouldSkipByTags() check.
	// Tag filtering is handled during planning phase (step.Skipped is set there).
	// The executor trusts the planner's decision for cleaner separation of concerns.

	return false, "", nil
}

func getStepDisplayName(step config.Step, ec *ExecutionContext) (string, bool) {
	vars := ec.GetVariables()
	// For with_filetree, show hierarchical structure
	if item, ok := vars["item"].(filetree.Item); ok {
		// For directories, show as headers with trailing slash
		if item.IsDir {
			if item.Path == "" {
				// Root directory
				return fmt.Sprintf("%s/", item.Name), true
			}
			// Subdirectory - show path without leading slash, with trailing slash
			dirPath := strings.TrimPrefix(item.Path, "/")
			return fmt.Sprintf("%s/", dirPath), true
		}

		// For files, show just the filename (not full destination path)
		// The directory context will be shown by the parent directory header
		if item.Name != "" {
			return item.Name, true
		}

		// Fallback to item path
		if item.Path != "" {
			return strings.TrimPrefix(item.Path, "/"), true
		}
	}

	// For with_items, append the item value to the step name so iterations
	// are distinguishable without losing the configured name. Falls back to
	// just the item value when the step has no name.
	if item, ok := vars["item"]; ok {
		itemStr := fmt.Sprintf("%v", item)
		if step.Name != "" {
			return fmt.Sprintf("%s: %s", step.Name, itemStr), true
		}
		return itemStr, true
	}

	// Use configured step name
	if step.Name != "" {
		return step.Name, true
	}

	// dx proposal-01: synthesize a label from the action type + key
	// field so unnamed steps don't render as a glyph with empty
	// body. The synthesized label flows through StepStartedData.Name
	// to both the human renderer and the JSON event channel.
	if synth := synthesizeStepName(step); synth != "" {
		return synth, true
	}

	return "", false
}

// DispatchStepAction executes the appropriate handler based on step type.
// All actions are now handled through the actions registry.
//
// INTERNAL: This function is exported for testing purposes only and is not part of
// the public API. It may change or be removed in future versions without notice.
func DispatchStepAction(step config.Step, ec *ExecutionContext) error {
	// Determine action type from step
	actionType := step.DetermineActionType()

	// Try to get handler from registry (new system)
	if handler, ok := actions.Get(actionType); ok {
		// Validate step configuration
		if err := handler.Validate(&step); err != nil {
			// Enhance error with step code context if available
			var errMsg string
			if step.Origin != nil && step.Origin.FilePath != "" {
				excerpt := config.FormatStepExcerpt(&step)
				if excerpt != "" {
					errMsg = fmt.Sprintf("validation failed for %s action: %v\n\nStep code (%s:%d):\n%s",
						actionType, err, step.Origin.FilePath, step.Origin.Line, excerpt)
				} else {
					errMsg = fmt.Sprintf("validation failed for %s action: %v", actionType, err)
				}
			} else {
				errMsg = fmt.Sprintf("validation failed for %s action: %v", actionType, err)
			}
			return fmt.Errorf("%s", errMsg)
		}

		// Spec 16: all handlers implement Runner. dispatchRunner is the
		// single dispatch path; it consults ctx.Mode() and emits the
		// appropriate events.
		dispatchErr := dispatchRunner(step, ec, handler)

		// proposal-11 vertical slice: if the step declared heal: and
		// the primary dispatch failed, run the heal children sequentially
		// and re-run the primary. On second-pass success the original
		// failure is suppressed and the run-wide Healed counter bumps.
		// Apply-mode only; plan mode falls through unchanged (heal
		// preview at plan time is a follow-up — see proposal-11).
		// Validate() restricts heal: to assert steps in this slice; the
		// actionType check here is defense-in-depth.
		if dispatchErr != nil &&
			len(step.Heal) > 0 &&
			actionType == "assert" &&
			ec.Mode() != actions.ModePlan {
			if healed := tryHeal(step, ec, handler, dispatchErr); healed {
				incStat(ec.Svc.Stats.Healed)
				return nil
			}
		}
		return dispatchErr
	}

	// If we get here, the action type is not registered
	return fmt.Errorf("no handler registered for action type: %s", actionType)
}

// tryHeal runs the heal children as a child plan, then re-dispatches
// the primary step. Returns true iff the re-dispatch succeeded —
// meaning the predicate now passes and the original failure can be
// suppressed. False on any failure path (heal-children failing, or
// re-dispatch still failing); the caller then surfaces the original
// dispatch error.
//
// proposal-11: heal children execute in the same ExecutionContext as
// the parent (same vars, same CurrentDir). They go through the full
// ExecuteSteps pipeline so loops / when / nested compounds all work.
//
// On a heal-child failure we deliberately do NOT retry the assert —
// if the remediation itself broke, the original predicate is unlikely
// to pass and surfacing the heal error would mask the real assert
// failure. The original error wins; the heal failure shows up in the
// child step's own event stream.
func tryHeal(step config.Step, ec *ExecutionContext, handler actions.Runner, origErr error) bool {
	logger := ec.GetLogger()
	if logger != nil {
		logger.Debugf("  assert failed, running heal (%d child step(s))", len(step.Heal))
	}

	// Run heal as a child plan. Errors from heal children mean the
	// remediation itself broke — bail without re-checking the assert.
	if err := ExecuteSteps(step.Heal, ec); err != nil {
		if logger != nil {
			logger.Debugf("  heal failed: %v (original assert error wins)", err)
		}
		_ = origErr // explicit: caller propagates origErr
		return false
	}

	// Re-evaluate the assertion. dispatchRunner re-emits the
	// EventAssertPassed/Failed pair, so the run log shows the post-heal
	// outcome alongside the original failure.
	if err := dispatchRunner(step, ec, handler); err != nil {
		if logger != nil {
			logger.Debugf("  assert still failing after heal: %v", err)
		}
		return false
	}
	if logger != nil {
		logger.Debugf("  assert healed successfully")
	}
	return true
}

func emitStepSkipped(step config.Step, ec *ExecutionContext, stepName, skipReason string) {
	incStat(ec.Svc.Stats.Skipped)
	stepID := generateStepID(step, ec)
	depth := 0
	if step.LoopContext != nil {
		depth = step.LoopContext.Depth
	}
	ec.EmitEvent(events.EventStepSkipped, events.StepSkippedData{
		StepID:      stepID,
		Name:        stepName,
		Level:       ec.Level,
		Reason:      skipReason,
		Depth:       depth,
		TriggeredBy: step.TriggeredBy,
		TryParent:   step.TryParent,
		TryRole:     step.TryRole,
	})
}

// syncResultEnvelope reconciles a handler's err return with the
// proposal-01 envelope fields on r so the JSON shape matches the
// run-level outcome. Called once per Run from dispatchRunner.
//
// Two stories live here:
//
//   - Proposal-06 / spec-69 finding B0: many handlers (os.user,
//     os.cron, pkg.upgrade, wait.http, os.mount, os.firewall) return
//     (Result with Failed=false, err with diagnostic). Pre-proposal
//     the envelope said success while the run said failure. Sync
//     Failed / Rc / Error here so handler authors don't have to
//     remember to set both.
//
//   - Proposal-02 / F4: when the run-wide context (runCtx) is
//     cancelled at the moment the handler errors, classify as
//     Cancelled rather than Failed. The exit-code aggregator and
//     recap counters treat them differently — cancelled→130,
//     failed→1. CancelledReason is derived by reading
//     context.Cause(runCtx) and matching the cancel-cause sentinels:
//     ErrCancelSignal → "sigint", ErrCancelFleet → "fleet_kill",
//     ErrCancelMCP → "mcp_shutdown", DeadlineExceeded → "timeout".
//     Any other cause (or a plain WithCancel with no cause attached)
//     maps to "cancelled" — the generic bucket that says "the run
//     was torn down but we don't know why specifically". Producers
//     that want accurate attribution must use context.WithCancelCause
//     with one of the sentinels.
//
// stats may be nil for callers that don't track counters; the
// Cancelled bump is then skipped. r must be non-nil; the typed-nil
// guard lives at the call site.
func syncResultEnvelope(runCtx context.Context, r *Result, err error, stats *ExecutionStats) {
	if err == nil {
		return
	}
	// Run-wide cancel takes precedence over plain failure — a step
	// that errored because the run was being torn down isn't a
	// handler-level fault.
	if runCtx != nil && runCtx.Err() != nil {
		if !r.Cancelled {
			r.Cancelled = true
			if stats != nil {
				incStat(stats.Cancelled)
			}
		}
		if r.CancelledReason == "" {
			r.CancelledReason = classifyCancelReason(runCtx)
		}
		if r.Error == "" {
			r.Error = err.Error()
		}
		return
	}
	if !r.Failed {
		r.Failed = true
		if r.Rc == 0 {
			r.Rc = 1
		}
	}
	if r.Error == "" {
		r.Error = err.Error()
	}
}

// classifyCancelReason maps the cancel cause attached to runCtx (via
// context.WithCancelCause) onto a CancelledReason enum value. The
// precedence is:
//
//  1. DeadlineExceeded — read off runCtx.Err() directly. A timer-fired
//     ctx exposes DeadlineExceeded as its Err, and context.Cause()
//     returns the same value when no explicit cause was attached. The
//     check has to come first so a producer that attaches a sentinel
//     to a parent ctx and then bounds it with WithTimeout still
//     reports "timeout" for the deadline path.
//  2. Sentinel match via errors.Is on context.Cause(runCtx) — covers
//     ErrCancelSignal / ErrCancelFleet / ErrCancelMCP.
//  3. Generic "cancelled" — any other cancel (plain WithCancel, or a
//     custom error nobody recognises). The producer didn't attribute
//     the cancel, so the envelope shouldn't pretend it knows.
//
// runCtx must already have runCtx.Err() != nil — callers are
// responsible for the gate; this helper does no recheck.
func classifyCancelReason(runCtx context.Context) string {
	if errors.Is(runCtx.Err(), context.DeadlineExceeded) {
		return CancelledReasonTimeout
	}
	cause := context.Cause(runCtx)
	switch {
	case errors.Is(cause, ErrCancelSignal):
		return CancelledReasonSigint
	case errors.Is(cause, ErrCancelFleet):
		return CancelledReasonFleetKill
	case errors.Is(cause, ErrCancelMCP):
		return CancelledReasonMCPShutdown
	case errors.Is(cause, context.DeadlineExceeded):
		return CancelledReasonTimeout
	default:
		return CancelledReasonCancelled
	}
}

func handleStepError(step config.Step, ec *ExecutionContext, stepErr error, stepID, stepName string, depth int, stepDuration time.Duration) error {
	incStat(ec.Svc.Stats.Failed)
	failedData := events.StepFailedData{
		StepID:       stepID,
		Name:         stepName,
		Action:       step.ActionType,
		Level:        ec.Level,
		ErrorMessage: stepErr.Error(),
		ExitCode:     -1,
		DurationMs:   stepDuration.Milliseconds(),
		Depth:        depth,
		DryRun:       ec.Mode() == actions.ModePlan,
	}
	var cmdErr *CommandError
	if errors.As(stepErr, &cmdErr) {
		failedData.ExitCode = cmdErr.ExitCode
	}
	if ec.CurrentResult != nil {
		// stdout/stderr on the failure event flow into the stderr JSON
		// blob (NewStderrErrorSubscriber) AND the runlog StepEntry, so
		// the cap balances three concerns: enough context for the
		// caller to actually diagnose the failure, not so much that
		// a verbose-but-truncated stream blows up the runlog, and a
		// known ceiling for SSE/fleet consumers that forward events
		// over the wire. 64 KiB covers ~800 lines at typical CLI
		// widths — fits the worst observed CI-gate failure (~40 KiB
		// of phase output before the failing line) with headroom.
		//
		// truncateTail keeps the LAST n bytes. For step failures the
		// useful information ("FAIL: ...", stack trace, exit code) is
		// always at the end of the stream; preserving the head leaves
		// the operator staring at the script's startup boilerplate.
		failedData.Stdout = truncateTail(ec.CurrentResult.Stdout, 65536)
		failedData.Stderr = truncateTail(ec.CurrentResult.Stderr, 65536)
	}
	ec.EmitEvent(events.EventStepFailed, failedData)

	// spec-30: a body child's failure triggers LIFO rollback of its
	// transaction's previously-completed children. Run BEFORE the
	// ContinueOnError branch — if the step is in a transaction, the
	// transaction's all-or-nothing semantics override per-step
	// continue_on_error. The rollback's own outcome is folded into the
	// step's reported failure; partial rollback is captured on the
	// TxnState for the run summary to surface.
	if rbErr := ec.handleTxnBodyFailure(step); rbErr != nil {
		ec.Svc.Logger.Errorf("rollback for transaction %s reported error: %v", step.TxnParent, rbErr)
	}

	if step.ContinueOnError {
		ec.Svc.Logger.Infof("  [WARNING] Ignoring error (ignore_errors: true): %v", stepErr)
		failedResult := NewResult()
		failedResult.Failed = true
		failedResult.Rc = 1
		captureResult(ec, step, failedResult.ToRegisteredResult())
		return nil
	}
	ec.Svc.Logger.Errorf("%v", stepErr)
	return stepErr
}

// dispatchPlanMode handles ModePlan dispatch for named non-import steps.
// Returns (true, err) when the step is fully handled; the caller should return err.
// Returns (false, nil) when the registered handler does not implement Runner — the
// caller falls through to the legacy DryRun path.
func dispatchPlanMode(step config.Step, ec *ExecutionContext, stepName string) (bool, error) {
	actionType := step.DetermineActionType()
	handler, ok := actions.Get(actionType)
	if ok {
		runner, isRunner := handler.(actions.Runner)
		if !isRunner {
			return false, nil
		}
		incStat(ec.Svc.Stats.Global)
		ec.CurrentStepID = generateStepID(step, ec)
		return true, dispatchRunner(step, ec, runner)
	}
	// Unknown action: emit a synthetic not-checkable event so the plan
	// formatter can still render the step rather than silently dropping it.
	incStat(ec.Svc.Stats.Global)
	stepID := generateStepID(step, ec)
	ec.CurrentStepID = stepID
	ec.EmitEvent(events.EventStepChecked, events.StepCheckedData{
		StepID:    stepID,
		Name:      stepName,
		Action:    actionType,
		Checkable: false,
		Reason:    "unknown action",
		Level:     ec.Level,
	})
	// F6: unknown-action steps in plan mode produce no diff, so they
	// count as OK to keep OK+Changed == Executed.
	incStat(ec.Svc.Stats.OK)
	incStat(ec.Svc.Stats.Executed)
	return true, nil
}

// postExecuteSuccess runs bookkeeping for a step that completed without error:
// stat counters, step.completed event, on_change tracking, txn snapshot, and
// RunCapture feed. Clears ec.CurrentResult before returning.
func postExecuteSuccess(step config.Step, ec *ExecutionContext, stepID, stepName string, depth int, stepDuration time.Duration) {
	incStat(ec.Svc.Stats.Executed)

	changed := false
	var resultData map[string]interface{}
	if ec.CurrentResult != nil {
		changed = ec.CurrentResult.Changed
		resultData = ec.CurrentResult.ToMap()
	}
	if changed {
		incStat(ec.Svc.Stats.Changed)
	} else {
		// F6: a successful no-change step bumps OK. Mutually exclusive
		// with Changed so the invariant OK+Changed == Executed holds
		// for every step that reaches postExecuteSuccess.
		incStat(ec.Svc.Stats.OK)
	}

	ec.EmitEvent(events.EventStepCompleted, events.StepCompletedData{
		StepID:      stepID,
		Name:        stepName,
		Level:       ec.Level,
		DurationMs:  stepDuration.Milliseconds(),
		Changed:     changed,
		Result:      resultData,
		Depth:       depth,
		DryRun:      ec.Mode() == actions.ModePlan,
		TriggeredBy: step.TriggeredBy,
		TryParent:   step.TryParent,
		TryRole:     step.TryRole,
	})

	if step.ID != "" {
		if ec.ChangedByStepID == nil {
			ec.ChangedByStepID = make(map[string]bool)
		}
		ec.ChangedByStepID[step.ID] = changed
	}

	ec.recordTxnBodyCompletion(step, ec.CurrentResult)
	ec.Svc.Capture.appendStep(step, ec.CurrentResult)
	ec.CurrentResult = nil
}

// ExecuteStep executes a single configuration step within the given execution context.
//
//nolint:gocyclo // Step dispatcher; complexity is the action-count fan-out.
func ExecuteStep(step config.Step, ec *ExecutionContext) error {
	// Validate step configuration
	if err := step.Validate(); err != nil {
		return err
	}

	// spec-30: a transaction-parent step (Transaction populated,
	// TxnRole empty — it's the parent, not a child) is a structural
	// marker, not an action to dispatch. The children expand as
	// siblings and carry the actions. The parent's presence in the
	// plan exists so plan-output renders the compound shape and the
	// run summary can attribute child outcomes to the right wrapper.
	if len(step.Transaction) > 0 && step.TxnRole == "" {
		// Treat as a no-op: nothing to dispatch. The executor's main
		// loop continues to the body children, which the txn state
		// machine in transaction.go drives.
		return nil
	}

	// spec-23 §2: a try-parent step (Try populated, TryRole empty) is
	// the same shape — structural wrapper, no leaf action. Its branches
	// expand as siblings and carry the real actions; this entry exists
	// purely so plan output renders the compound and the run summary
	// can attribute child outcomes to the right wrapper.
	if len(step.Try) > 0 && step.TryRole == "" {
		// Issue #23: capture continue_on_error from the compound so
		// the try-block resolution path (see ExecuteSteps below) can
		// honor it. The compound is a structural marker — without
		// this stash, the flag was being silently dropped.
		if step.ContinueOnError {
			ec.tryStateFor(step.ID).ContinueOnError = true
		}
		return nil
	}

	// proposal-11: heal-child siblings are preview-only at apply mode.
	// The planner expands them so `mooncake plan` surfaces their
	// diff/perms/risk alongside the parent assert; the actual apply
	// execution flows through DispatchStepAction's tryHeal seam on
	// the parent (which still reads the nested step.Heal field).
	// Silent skip — no step.skipped event, no global-counter bump.
	// Plan mode falls through so the per-step StepChecked event fires
	// with the handler's Differ/Permitter output.
	if step.HealParent != "" && ec.Mode() != actions.ModePlan {
		return nil
	}

	// Check if step should be skipped (when conditions, tags)
	shouldSkip, skipReason, err := checkSkipConditions(step, ec)
	if err != nil {
		return err
	}

	// Check idempotency conditions (creates, unless) for every action.
	// MT-15 (HIGH correctness): prior to this generalization the check
	// fired only for step.Shell / step.Cmd, so file.write / text.* / pkg
	// / file.copy / … silently re-ran on every apply even when the
	// operator wrote `unless_exists:` (or the friendly aliases
	// `creates:` / `unless:` introduced alongside this fix). The guards
	// are step-level metadata — they describe "should this step run at
	// all?" and predate the action dispatch. Gate-by-action-type was a
	// pre-spec-21 artifact.
	if !shouldSkip {
		idempotencySkip, idempotencyReason, err := checkIdempotencyConditions(step, ec)
		if err != nil {
			return err
		}
		if idempotencySkip {
			shouldSkip = true
			skipReason = idempotencyReason
		}
	}

	// Determine step display name
	stepName, hasStepName := getStepDisplayName(step, ec)

	// Handle skipped steps
	if shouldSkip {
		if hasStepName && step.Import == nil {
			emitStepSkipped(step, ec, stepName, skipReason)
		}
		return nil
	}

	// Plan-mode bypass for named non-import steps: Runner handlers emit
	// EventStepChecked (Spec 16); unknown actions get a synthetic
	// not-checkable event. Legacy (non-Runner) handlers fall through to
	// the normal started/completed lifecycle below.
	if ec.Mode() == actions.ModePlan && hasStepName && step.Import == nil {
		if handled, err := dispatchPlanMode(step, ec, stepName); handled || err != nil {
			return err
		}
	}

	// Debug: show tags for non-skipped steps
	if len(step.Tags) > 0 {
		ec.Svc.Logger.Debugf("  tags: [%s]", strings.Join(step.Tags, ", "))
	}

	// Debug: show action for unnamed steps
	if step.Name == "" {
		if step.Vars != nil {
			ec.Svc.Logger.Debugf("Setting variables")
		} else if step.VarsLoad != nil {
			ec.Svc.Logger.Debugf("Loading variables from %s", *step.VarsLoad)
		}
	}

	// Increment global step counter for non-skipped steps
	incStat(ec.Svc.Stats.Global)

	// Generate step ID and store in context for event correlation
	stepID := generateStepID(step, ec)
	ec.CurrentStepID = stepID

	// Get directory depth from loop context (for filetree items)
	depth := 0
	if step.LoopContext != nil {
		depth = step.LoopContext.Depth
	}

	// Emit step.started event
	ec.EmitEvent(events.EventStepStarted, events.StepStartedData{
		StepID:      stepID,
		Name:        stepName,
		Level:       ec.Level,
		GlobalStep:  readStat(ec.Svc.Stats.Global),
		Action:      step.ActionType,
		Tags:        step.Tags,
		When:        step.When,
		Depth:       depth,
		DryRun:      ec.Mode() == actions.ModePlan,
		TriggeredBy: step.TriggeredBy,
		TryParent:   step.TryParent,
		TryRole:     step.TryRole,
	})

	// Track start time for duration
	stepStartTime := time.Now()

	// Execute the appropriate handler
	stepErr := DispatchStepAction(step, ec)

	// Calculate duration
	stepDuration := time.Since(stepStartTime)

	// Handle errors
	if stepErr != nil {
		if err := handleStepError(step, ec, stepErr, stepID, stepName, depth, stepDuration); err != nil {
			return err
		}
		// F017: handleStepError returning nil means continue_on_error
		// swallowed the failure. It already emitted step.failed and
		// updated Stats.Failed; the success-path bookkeeping
		// (postExecuteSuccess → step.completed + Stats.Executed) must
		// not also run, or consumers see two terminal events for the
		// same StepID and the printed recap counts the step as both
		// failed and successfully executed.
		return nil
	}

	postExecuteSuccess(step, ec, stepID, stepName, depth, stepDuration)
	return nil
}

// ExecuteSteps executes a sequence of configuration steps within the given execution context.
//
// F016 stage-1(a): the loop checks ec.Svc.Ctx between steps and aborts
// with ctx.Err() if the context is cancelled. Handler-level
// cancellation (shell child interrupts, network step short-circuits)
// is the stage-3 audit and is not done here. nil ec.Svc.Ctx is treated
// as non-cancellable.
func ExecuteSteps(steps []config.Step, ec *ExecutionContext) error {
	ec.Svc.Logger.Debugf("Executing: %v", ec.CurrentFile)

	// Set total steps for this execution context
	ec.TotalSteps = len(steps)

	for i, step := range steps {
		if ctx := ec.Svc.Ctx; ctx != nil {
			if err := ctx.Err(); err != nil {
				// Proposal-02: the cancelled counter bump happens per-step
				// in syncResultEnvelope (dispatchRunner path) — that
				// attributes cancellation to the handler that actually
				// observed the cancelled runCtx, which is more accurate
				// than this loop-level guess about which step was in
				// flight. Between-step cancellation just terminates the
				// loop; no step gets counted as cancelled because no
				// step's mutation was interrupted.
				ec.Svc.Logger.Infof("execution cancelled by context after %d/%d steps: %v", i, len(steps), err)
				return err
			}
		}
		ec.CurrentIndex = i

		// If step has origin metadata (from planner), use its directory
		// This ensures relative paths work correctly for included files
		if step.Origin != nil && step.Origin.FilePath != "" {
			ec.CurrentDir = filepath.Dir(step.Origin.FilePath)
			ec.CurrentFile = step.Origin.FilePath
		}

		// If step has loop context (from planner), restore loop variables
		// This ensures when conditions can reference item, index, first, last
		if step.LoopContext != nil {
			ec.Scope.Loop = &LoopContext{
				Item:  step.LoopContext.Item,
				Index: step.LoopContext.Index,
				First: step.LoopContext.First,
				Last:  step.LoopContext.Last,
			}
		} else {
			ec.Scope.Loop = nil
		}

		if err := ExecuteStep(step, ec); err != nil {
			// spec-30: when the failing step is part of a transaction,
			// look ahead in the plan for same-transaction on_rollback
			// children and run them before propagating. The body of the
			// transaction is already toast (rollback ran inside
			// dispatchStepFailure); on_rollback exists for notification
			// and cleanup, and operators expect it to fire even when
			// the failure ultimately propagates upward.
			if step.TxnParent != "" && step.TxnRole == "body" {
				for j := i + 1; j < len(steps); j++ {
					next := steps[j]
					if next.TxnParent != step.TxnParent {
						break // left the transaction's contiguous expansion
					}
					if next.TxnRole != "rollback" {
						continue // a body child after the failure — txnSkipReason will skip it
					}
					// Best-effort: log but do not propagate rollback-child errors;
					// the original transaction failure is what bubbles up.
					if rbErr := ExecuteStep(next, ec); rbErr != nil {
						ec.Svc.Logger.Errorf("on_rollback step %q errored: %v", next.Name, rbErr)
					}
				}
			}
			// spec-23 §2: when the failing step is a try-block "try"
			// child, walk the remaining same-block siblings (catch +
			// finally) before propagating. recordTryBodyFailure marks
			// the TryState so the executor's trySkipReason will let
			// catch run and skip remaining try children. Catch can fail
			// too — when it does we replace the propagated error with
			// the catch error per spec ("the compound Step propagates
			// the later error"). Finally always runs; a finally error
			// is also propagated as the latest error.
			if step.TryParent != "" && step.TryRole == "try" {
				ec.recordTryBodyFailure(step, err)
				propagated := err
				lastConsumed := i
				for j := i + 1; j < len(steps); j++ {
					next := steps[j]
					if next.TryParent != step.TryParent {
						break // left the try-block's contiguous expansion
					}
					if cbErr := ExecuteStep(next, ec); cbErr != nil {
						// Catch / finally errored; surface as the propagated
						// error per spec. We deliberately do NOT short-circuit
						// the loop — finally still has to run after a failed
						// catch, and a later finally child would still get the
						// chance to run after an earlier finally failure (same
						// "best effort" shape as on_rollback above).
						if next.TryRole == "catch" {
							ec.recordTryCatchFailure(next)
						}
						propagated = cbErr
					}
					lastConsumed = j
				}
				// Issue #23: if the compound carried continue_on_error,
				// swallow the resolved error and resume the outer iteration
				// with steps AFTER the try-block. We can't advance the
				// for-range index directly, so recurse on the slice tail
				// — same plan, same context, just past the children we
				// already processed. Matches the leaf-action
				// ContinueOnError shape: warning logged + run continues.
				if t := ec.OpenTries[step.TryParent]; t != nil && t.ContinueOnError {
					ec.Svc.Logger.Infof("  [WARNING] Ignoring try-block failure (continue_on_error: true on the compound): %v", propagated)
					return ExecuteSteps(steps[lastConsumed+1:], ec)
				}
				return propagated
			}
			return err
		}
	}
	return nil
}

// StartConfig contains configuration for starting a mooncake execution.
type StartConfig struct {
	ConfigFilePath string
	// VarsFilePaths are loaded in order; later files override earlier on key
	// collision. Mirrors how `mooncake apply -v a.yml -v b.yml` reads on
	// the CLI and how `vars_files: ["a.yml", "b.yml"]` works in the agentd
	// /v1/runs payload.
	VarsFilePaths    []string
	SudoPass         string // Sudo password provided directly (use SudoPassFile for better security)
	SudoPassFile     string
	AskBecomePass    bool
	InsecureSudoPass bool
	Tags             []string
	// SkipTags excludes any step whose tags intersect this list
	// (MT-58, `--skip-tags`). Composes with Tags: a step runs only
	// when Tags admits it AND SkipTags doesn't exclude it.
	SkipTags []string
	// Names is the spec-50 step-name filter (`--step-filter name=<x>`).
	// AND'd with Tags at plan-build time: a step must pass both.
	Names []string

	// Artifact configuration
	ArtifactsDir      string
	CaptureFullOutput bool
	MaxOutputBytes    int
	MaxOutputLines    int

	// Capture, if non-nil, is populated by Start with the compiled
	// plan and per-step records. R1.1b's internal/apply.Runner uses
	// this to build its typed *KernelResult. Other callers leave nil.
	Capture *RunCapture
}

// Start begins execution of a mooncake configuration with the given settings.
// Always goes through the planner to expand loops, includes, and variables.
// Emits events through the provided publisher for all execution progress.
//
// ctx is checked between steps in the step loop (F016 stage-1(a)). A
// cancelled ctx causes the run to return early with ctx.Err(); the
// in-flight step (if any) continues to completion — handler-level
// cancellation is the stage-3 audit. nil ctx is treated as
// context.Background() — non-cancellable, never aborts.
func Start(ctx context.Context, startConfig StartConfig, log logger.Logger, publisher events.Publisher) error {
	log.Debugf("config: %v", startConfig)

	if startConfig.ConfigFilePath == "" {
		return &SetupError{Component: "config", Issue: "config file path is empty"}
	}

	// Resolve sudo password early (before plan building)
	passwordCfg := security.PasswordConfig{
		CLIPassword:    startConfig.SudoPass,
		AskInteractive: startConfig.AskBecomePass,
		PasswordFile:   startConfig.SudoPassFile,
		InsecureCLI:    startConfig.InsecureSudoPass,
	}

	sudoPassword, err := security.ResolvePassword(passwordCfg)
	if err != nil {
		return &SetupError{Component: "sudo password", Issue: "failed to resolve password", Cause: err}
	}

	// Create path expander for resolving paths
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return &SetupError{Component: "template renderer", Issue: "failed to create renderer", Cause: err}
	}
	pathExpander := pathutil.NewPathExpander(renderer)

	currentDir, err := os.Getwd()
	if err != nil {
		return err
	}

	// Load variables from each file in order; later files win on collision.
	variables := make(map[string]interface{})
	for _, path := range startConfig.VarsFilePaths {
		if path == "" {
			continue
		}
		expandedPath, expandErr := pathExpander.ExpandPath(path, currentDir, nil)
		if expandErr != nil {
			return &RenderError{Field: "vars file path", Cause: expandErr}
		}

		log.Debugf("Reading variables from file: %v", expandedPath)
		vars, err := config.ReadVariables(expandedPath)
		if err != nil {
			// Surface vars-file read failures at WARNING (was Debugf
			// pre-cleanup). The operator explicitly passed `-v X` or
			// `vars_files: [X]` in their agentd payload; a silent
			// Debug-level skip on missing/perm-denied/parse-broken
			// files meant the run proceeded as if the file wasn't
			// there. UX risk if the missing file carried critical
			// secrets. `continue` semantics preserved — failing the
			// run hard would be too aggressive for the agentd payload
			// race-condition case where a mid-deploy worker might
			// not see a freshly-published file yet.
			log.Infof("[WARNING] failed to read variables from %s: %v (skipping this file)", expandedPath, err)
			continue
		}
		log.Debugf("Read variables: %v", vars)
		for k, v := range vars {
			variables[k] = v
		}
	}

	// Expand config file path
	configFilePath, err := pathExpander.ExpandPath(startConfig.ConfigFilePath, currentDir, nil)
	if err != nil {
		return err
	}

	log.Debugf("Building plan from configuration: %v", configFilePath)

	// ALWAYS build plan first (expands loops, includes, vars)
	planner, err := plan.NewPlanner()
	if err != nil {
		return &SetupError{Component: "planner", Issue: "failed to create planner", Cause: err}
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configFilePath,
		Variables:  variables,
		Tags:       startConfig.Tags,
		SkipTags:   startConfig.SkipTags,
		Names:      startConfig.Names,
	})
	if err != nil {
		return &SetupError{Component: "planner", Issue: "failed to build plan", Cause: err}
	}

	log.Debugf("Plan built with %d steps", len(planData.Steps))

	// Fail fast on a tag filter that matches no tagged step. Without
	// this check `--tags deplly` (typo of `deploy`) silently passes —
	// the filter rejects all tagged steps and only untagged
	// "scaffolding" steps run; the recap shows green (`failed=0`) so
	// the user thinks their deploy ran when it didn't.
	if msg := filter.UnmatchedTagsError(startConfig.Tags, planData); msg != "" {
		return &SetupError{Component: "tags", Issue: msg}
	}

	// Setup artifact writer if artifacts-dir is specified
	if startConfig.ArtifactsDir != "" {
		// Gather system facts for artifact generation
		systemFacts := facts.Collect()

		// Create artifact writer
		artifactWriter, err := artifacts.NewWriter(
			artifacts.Config{
				BaseDir:        startConfig.ArtifactsDir,
				CaptureStdout:  startConfig.CaptureFullOutput,
				CaptureStderr:  startConfig.CaptureFullOutput,
				MaxOutputBytes: startConfig.MaxOutputBytes,
				MaxOutputLines: startConfig.MaxOutputLines,
			},
			planData,
			systemFacts,
		)
		if err != nil {
			return &SetupError{Component: "artifacts", Issue: "failed to create artifact writer", Cause: err}
		}
		// MT-53 (events-drop-on-close): the writer subscribes to an
		// async channel publisher. EventRunCompleted is queued just
		// before ExecutePlan returns; if we close the writer before
		// the publisher's forwarding goroutine has drained its channel,
		// the writer's `closed` flag is set and the queued
		// run-completed event is silently dropped — including the call
		// that writes results.json / SUMMARY.md / changed_files.json.
		// Drain pending events FIRST (LIFO defer order: Flush below
		// runs *before* the Close above).
		defer artifactWriter.Close()
		defer publisher.Flush()

		// Subscribe artifact writer to events
		publisher.Subscribe(artifactWriter)

		log.Debugf("Artifacts will be written to: %s/runs/%s", startConfig.ArtifactsDir, "...")
	}

	// R1.1b: feed Capture.Plan up-front so the Runner sees the compiled
	// plan even if execution fails partway. No-op when Capture is nil.
	startConfig.Capture.setPlan(planData)

	// Execute the plan with event publisher
	return executePlanWithCapture(ctx, planData, sudoPassword, actions.ModeApply, log, publisher, startConfig.Capture)
}

// ExecutePlan executes a pre-compiled plan.
// Emits events through the provided publisher for all execution progress.
//
// Callers that need the typed *KernelResult substrate (R1.1b) should
// use ExecutePlanWithCapture or go through executor.Start with
// StartConfig.Capture set; this entry point does not surface the
// per-step records.
//
// ctx is checked between steps — see Start for the cancellation
// contract.
func ExecutePlan(ctx context.Context, p *plan.Plan, sudoPass string, mode actions.Mode, log logger.Logger, publisher events.Publisher) error {
	return executePlanWithCapture(ctx, p, sudoPass, mode, log, publisher, nil)
}

// ExecutePlanWithCapture runs a pre-compiled plan and fills the
// caller-supplied *RunCapture with the plan + per-step results, so
// the result can be lifted into an *apply.KernelResult.
//
// This is the from-saved-plan analog of executor.Start with Capture
// set. Used by apply.NewRunnerFromPlan (R1.1c) so the saved-plan
// path produces the same typed kernel result as the
// compiled-from-config path. capture may be nil — equivalent to
// calling ExecutePlan directly.
//
// ctx is checked between steps — see Start for the cancellation
// contract.
func ExecutePlanWithCapture(ctx context.Context, p *plan.Plan, sudoPass string, mode actions.Mode, log logger.Logger, publisher events.Publisher, capture *RunCapture) error {
	return executePlanWithCapture(ctx, p, sudoPass, mode, log, publisher, capture)
}

// executePlanWithCapture is the shared implementation behind
// ExecutePlan and Start. Pass capture=nil to disable the
// kernel-result substrate (legacy callers).
func executePlanWithCapture(ctx context.Context, p *plan.Plan, sudoPass string, mode actions.Mode, log logger.Logger, publisher events.Publisher, capture *RunCapture) error {
	steps := p.Steps
	variables := p.InitialVars

	// Start timing
	startTime := time.Now()

	// Emit run.started event
	publisher.Publish(events.Event{
		Type:      events.EventRunStarted,
		Timestamp: time.Now(),
		Data: events.RunStartedData{
			RootFile:   p.RootFile,
			Tags:       p.Tags,
			DryRun:     mode == actions.ModePlan,
			TotalSteps: len(p.Steps),
		},
	})

	// Emit plan.loaded event
	publisher.Publish(events.Event{
		Type:      events.EventPlanLoaded,
		Timestamp: time.Now(),
		Data: events.PlanLoadedData{
			RootFile:   p.RootFile,
			TotalSteps: len(p.Steps),
			Tags:       p.Tags,
		},
	})

	// Create dependencies
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		return &SetupError{Component: "template renderer", Issue: "failed to create renderer", Cause: err}
	}
	evaluator := expression.NewGovaluateEvaluator()
	pathExpander := pathutil.NewPathExpander(renderer)
	fileTreeWalker := filetree.NewWalker(pathExpander)

	// Create redactor and add sudo password if present
	redactor := security.NewRedactor()
	if sudoPass != "" {
		redactor.AddSensitive(sudoPass)
	}

	// Set redactor on logger for automatic redaction
	log.SetRedactor(redactor)

	// Use the directory of the root config file, not the current working directory
	// This ensures relative paths in the config (like ./template.j2) are resolved correctly
	configDir := filepath.Dir(p.RootFile)

	// Initialize variables from plan (system facts already injected by planner)
	if variables == nil {
		variables = make(map[string]interface{})
	}

	// Initialize global step counter and statistics
	globalExecuted := 0
	statsExecuted := 0
	statsSkipped := 0
	statsFailed := 0
	statsChanged := 0
	statsOK := 0
	statsReverted := 0
	statsCancelled := 0
	statsHealed := 0

	// spec-72 §1: probe escalation once per run. NOPASSWD / NNP /
	// sudo availability are stable per host (operator-configured,
	// not per-run state), so caching the report for the run is
	// correct and saves a sudo invocation per privileged step. The
	// PasswordlessSudo bool below is the phase-1 backward-compat
	// shim: it stays true for exactly the cases the old
	// detectPasswordlessSudo returned true (NOPASSWD probe
	// succeeded), so existing call sites that read the bool see no
	// behavior change.
	escalation := security.ProbeEscalation(ctx, sudoPass)
	svc := &RunServices{
		Logger:           log.WithPadLevel(0),
		SudoPass:         sudoPass,
		Escalation:       escalation,
		PasswordlessSudo: escalation.Reason == security.EscalationAvailablePasswordless,
		Tags:             []string{}, // tag filtering done by planner (step.Skipped)
		Mode:             mode,
		Stats: &ExecutionStats{
			Global:    &globalExecuted,
			Executed:  &statsExecuted,
			Skipped:   &statsSkipped,
			Failed:    &statsFailed,
			Changed:   &statsChanged,
			OK:        &statsOK,
			Reverted:  &statsReverted,
			Cancelled: &statsCancelled,
			Healed:    &statsHealed,
		},
		Template:       renderer,
		Evaluator:      evaluator,
		PathUtil:       pathExpander,
		FileTree:       fileTreeWalker,
		Redactor:       redactor,
		EventPublisher: publisher,
		Capture:        capture,
		Ctx:            ctx,
		Modules:        p.Modules,
	}
	// R1.1b: Capture.Plan was already set by Start; for the direct
	// ExecutePlan entry point (where capture is nil) this is a no-op.
	// We also call setPlan here so callers that ever wire Capture
	// directly to ExecutePlan in the future get correct behavior.
	capture.setPlan(p)
	scope := &VariableScope{
		User:          variables,
		Results:       make(map[string]RegisteredResult),
		ResultOrigins: make(map[string]resultOrigin),
	}

	// Populate Facts + Metrics on the typed scope so templates and `when:`
	// expressions can read live values (load_avg_1m, cpu_usage_pct, …) the
	// way LLM_GUIDE.md and the metrics docs promise. The planner had been
	// injecting facts into User for compat, but no one was populating the
	// Metrics section — `{{ load_avg_1m }}` rendered empty for every apply
	// path.
	AddGlobalVariables(scope)
	executionContext := ExecutionContext{
		Svc:          svc,
		Scope:        scope,
		CurrentDir:   configDir,
		CurrentFile:  "",
		Level:        0,
		CurrentIndex: 0,
		TotalSteps:   len(steps),
	}

	// Execute pre-expanded steps
	execErr := ExecuteSteps(steps, &executionContext)

	// Calculate duration
	duration := time.Since(startTime)

	publisher.Publish(events.Event{
		Type:      events.EventRunCompleted,
		Timestamp: time.Now(),
		Data: events.RunCompletedData{
			TotalSteps:     len(steps),
			SuccessSteps:   statsExecuted,
			OkSteps:        statsOK,
			FailedSteps:    statsFailed,
			SkippedSteps:   statsSkipped,
			ChangedSteps:   statsChanged,
			RevertedSteps:  statsReverted,
			CancelledSteps: statsCancelled,
			HealedSteps:    statsHealed,
			DurationMs:     duration.Milliseconds(),
			Success:        execErr == nil,
			CheckMode:      mode == actions.ModePlan,
			ErrorMessage: func() string {
				if execErr != nil {
					return execErr.Error()
				}
				return ""
			}(),
		},
	})

	return execErr
}

// formatMode formats an os.FileMode as an octal string (e.g., "0644").
func formatMode(mode os.FileMode) string {
	return fmt.Sprintf("%#o", mode)
}

// truncateTail returns the last maxLen bytes of s, prefixed with an
// ellipsis line so callers see that earlier output was dropped. For
// step-failure stdout/stderr where the diagnostic ("FAIL: ...", stack
// trace, exit-code printout) is invariably at the END of the stream;
// using the head-truncating truncate() would leave operators staring
// at startup boilerplate while the actual error scrolled off.
//
// The "... <N bytes truncated> ...\n" marker is intentionally
// human-parseable (no JSON, no structured fields) — this output lands
// in a free-text stdout/stderr field that downstream consumers (runlog
// reader, MCP, the stderr error subscriber) display verbatim.
func truncateTail(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	dropped := len(s) - maxLen
	return fmt.Sprintf("... <%d bytes truncated> ...\n%s", dropped, s[dropped:])
}

// dispatchRunner invokes a Spec 16 Runner handler. The handler's Run
// method decides what to do based on ec.Mode(); this function handles
// the result plumbing (CurrentResult, register, events, stats) that
// previously lived in the Execute / DryRun / Check branches of
// DispatchStepAction.
//
// In ModePlan it emits EventStepChecked (matching the legacy Check
// flow) so existing subscribers continue to work; in ModeApply it
// relies on the surrounding ExecuteStep to emit started/completed.
//
// Spec-22 phase 3 preflight: when the underlying handler implements
// actions.Permitter, check declared permissions BEFORE calling Run.
// Handlers without Permitter skip the check (legacy behavior). The
// type-assertion on actions.Permitter is intentional rather than
// going through Handler — runner is typed as actions.Runner here and
// the Permitter interface stands alone, so we can introspect without
// up-casting.
func dispatchRunner(step config.Step, ec *ExecutionContext, runner actions.Runner) error {
	if p, ok := runner.(actions.Permitter); ok {
		sudoAvailable := ec.Svc != nil && (ec.Svc.SudoPass != "" || ec.Svc.PasswordlessSudo)
		if err := preflightPermissions(p.Permissions(&step), &step, sudoAvailable); err != nil {
			return err
		}
	}

	// Spec-72 Layer C: bind the step's AsUser to ec for the duration
	// of this dispatch. ctx.Privileged() reads CurrentAsUser, so
	// handlers don't have to thread step.AsUser themselves. Cleared
	// after dispatch returns so a follow-up step that lacks AsUser
	// doesn't inherit this step's binding.
	prevAsUser := ec.CurrentAsUser
	ec.CurrentAsUser = step.AsUser
	defer func() { ec.CurrentAsUser = prevAsUser }()
	// spec-23 §3: resolve any `!secret env:FOO` markers to their actual
	// values just before the handler sees them. No-op in plan mode —
	// markers stay so plan output redaction can rewrite them as
	// `"!secret env:FOO"` rather than leaking the value. The pure walk
	// lives in internal/secrets/resolver so frontends (MCP check_plan,
	// agent loop pre-submit) can call it without dragging in executor.
	if ec.Mode() != actions.ModePlan {
		var redactor *security.Redactor
		if ec.Svc != nil {
			redactor = ec.Svc.Redactor
		}
		if err := resolver.Resolve(&step, redactor); err != nil {
			return err
		}
	}
	// Spec-68 wave 2.5: in apply mode, capture the typed Diff before
	// the handler runs so we can attach it to the runlog StepEntry
	// (and from there to `mooncake explain`'s typed-payload output).
	// Mirrors the plan-mode pattern below at the StepChecked emit but
	// stores the value on Result instead of routing through an event —
	// no per-step event fires for apply-mode mutations today, and
	// adding one is out of scope.
	//
	// Errors are swallowed for the same best-effort reason the
	// plan-mode call swallows them: Diff is auxiliary metadata; the
	// handler's actual error path is what gates the apply.
	var preAppliedDiff any
	if ec.Mode() != actions.ModePlan {
		if differ, ok := runner.(actions.Differ); ok {
			if d, dErr := differ.Diff(ec, &step); dErr == nil {
				preAppliedDiff = &d
			}
		}
	}

	// Spec-69 phase 2-3: if the handler opts into RawRunner, the
	// executor owns the retry loop AND post-loop override application.
	// Legacy Runner.Run handlers stay on the path below; their own
	// in-handler retry+override logic still runs.
	var (
		result actions.Result
		err    error
	)
	if rr, ok := runner.(actions.RawRunner); ok && ec.Mode() != actions.ModePlan {
		isRetryable := func(actions.Result, error) bool { return true }
		if rd, ok := runner.(actions.Retryable); ok {
			isRetryable = func(res actions.Result, e error) bool { return rd.IsRetryable(res, e, &step) }
		}
		// Track the final attempt count so handlers that surface
		// attempts as a registered fact (http.request's response.attempts)
		// see the cross-attempt count rather than a per-RunRaw "1".
		// Single-attempt RunRaw can't observe its own retries.
		lastAttempt := 0
		// F053: pass the run-wide ctx so the inter-attempt delay
		// respects Ctrl-C / cancel instead of blocking the whole sleep
		// window. ec.Svc.Ctx was already plumbed onto RunServices at
		// ExecutePlan entry (line ~1212); no new plumbing needed.
		retryCtx := context.Background()
		if ec.Svc != nil && ec.Svc.Ctx != nil {
			retryCtx = ec.Svc.Ctx
		}
		result, err = runWithRetry(retryCtx, &step, ec.GetLogger(), func(attempt int) (actions.Result, error) {
			lastAttempt = attempt
			return rr.RunRaw(ec, &step)
		}, isRetryable)
		// Only overwrite when the handler already populated Data["attempts"];
		// we don't invent the key for handlers that didn't ask for it.
		if r, rok := result.(*Result); rok && r != nil && r.Data != nil {
			if _, has := r.Data["attempts"]; has {
				r.Data["attempts"] = lastAttempt
			}
		}
		// Override application: applies once post-retry. MT-48 holds
		// because the retry loop above branched on raw err, not the
		// post-override verdict. failed_when:false may mask the final
		// failure entirely; changed_when overrides Changed.
		//
		// The `r != nil` guard handles the typed-nil case: a handler
		// returning (*Result)(nil) alongside an err satisfies the
		// *Result type assertion (rok==true) but would nil-deref on
		// r.Failed. Treat that the same as a non-*Result return — the
		// err carries the outcome; there's nothing to apply overrides
		// to.
		if r, rok := result.(*Result); rok && r != nil {
			if oErr := applyResultOverrides(ec, &step, r); oErr != nil {
				err = oErr
			} else if r.Failed {
				// Either failed_when set it, or it was set by the
				// raw outcome; surface a real error so callers see
				// the same Failed=true + err combination they got
				// pre-spec-69. When err is already non-nil (the
				// retry loop returned the "after N attempts" wrap),
				// keep it — it has the attempt count.
				if err == nil {
					err = fmt.Errorf("step failed (failed_when=true)")
				}
			} else if err != nil && step.FailedWhen != "" {
				// failed_when masked the failure: clear err so the
				// step reports success. This is the documented
				// "retry N times, then don't fail the run no matter
				// what" pattern (MT-48). Gated on FailedWhen being
				// set — otherwise a RawRunner returning (non-nil
				// *Result with Failed=false, non-nil err) silently
				// reports the step as ok=1 changed=0. Most spec-69-
				// migrated handlers (os.user, os.cron, pkg.upgrade,
				// …) return errors that way: they construct the
				// Result up-front and don't set result.Failed=true on
				// error. See docs-working/specs/spec-69-followups.md
				// finding B0.
				err = nil
			}
		}
	} else {
		result, err = runner.Run(ec, &step)
	}

	// Capture the result on the context whether or not Run errored,
	// matching the existing Execute behavior so callers can read
	// stdout/stderr from failed shell-like steps.
	//
	// Typed-nil guard: handlers like pkg install of a missing package
	// return (nil *Result, err) which satisfies the `(*Result)`
	// assertion with r==nil; dereferencing would panic. T8 /
	// RawRunnerNilResultPropagatesErr pins that path.
	if r, ok := result.(*Result); ok && r != nil {
		ec.CurrentResult = r
		if preAppliedDiff != nil {
			r.AppliedDiff = preAppliedDiff
		}
		syncResultEnvelope(ec.Svc.Ctx, r, err, ec.Svc.Stats)
	} else {
		ec.CurrentResult = NewResult()
	}
	if err != nil {
		return err
	}

	if ec.Mode() == actions.ModePlan {
		actionType := step.DetermineActionType()
		name := step.Name
		if name == "" {
			name = actionType
		}
		// Read Spec-16 fields off the concrete Result; legacy callers
		// may pass a *Result with WouldChange/Reason/Checkable set.
		var wouldChange, checkable bool
		var reason string
		var detail any
		if r, ok := result.(*Result); ok {
			wouldChange = r.WouldChange
			checkable = r.Checkable
			reason = r.Reason
			detail = r.Detail
		}
		// Spec-22 phase 4-followup: when the handler natively implements
		// Differ, compute a structural Diff and carry it through the
		// StepChecked event so `mooncake plan --format json` exposes it
		// as the per-step `diff:` field. Direct type-assert (not
		// ResolveDiffer) so we don't fill JSON output with default-
		// Differ stubs for handlers that haven't opted in.
		//
		// Errors from Diff are intentionally swallowed: Diff is best-
		// effort planning information, the Reason / Result.Detail
		// already carries the actionable error message for the
		// underlying problem (template render fail etc.), and
		// surfacing a nil Diff is the right signal for "couldn't
		// predict structurally."
		var diffPayload any
		if differ, ok := runner.(actions.Differ); ok {
			if d, err := differ.Diff(ec, &step); err == nil {
				diffPayload = &d
			}
		}
		// Spec-22 phase 6: when the handler natively implements Coster,
		// surface its CostEstimate through the same StepChecked event
		// path. Same direct type-assert as Diff above — skipping
		// ResolveCoster intentionally so non-Coster handlers don't
		// emit a misleading default (Resources=-1, Bytes=-1, Risk=5)
		// every time. Coster errors are swallowed for the same
		// best-effort reasons as Diff.
		var costPayload any
		if coster, ok := runner.(actions.Coster); ok {
			if c, err := coster.Cost(ec, &step); err == nil {
				costPayload = &c
			}
		}
		ec.EmitEvent(events.EventStepChecked, events.StepCheckedData{
			StepID:      ec.CurrentStepID,
			Name:        name,
			Action:      actionType,
			WouldChange: wouldChange,
			Checkable:   checkable,
			Reason:      reason,
			Detail:      detail,
			Diff:        diffPayload,
			Cost:        costPayload,
			Level:       ec.Level,
		})
		if wouldChange {
			incStat(ec.Svc.Stats.Changed)
		} else {
			// F6: plan-mode parity with apply — a step that predicts
			// no change counts as OK in the recap, keeping OK+Changed
			// == Executed across modes so plan→apply numerical
			// comparisons stay honest.
			incStat(ec.Svc.Stats.OK)
		}
		incStat(ec.Svc.Stats.Executed)
		// spec-37: in plan mode the bind happens only when the handler
		// declares CaptureInPlan; captureResult enforces that internally.
		if ec.CurrentResult != nil {
			captureResult(ec, step, ec.CurrentResult.ToRegisteredResult())
		}
		return nil
	}

	// ModeApply: register result if requested.
	if ec.CurrentResult != nil {
		captureResult(ec, step, ec.CurrentResult.ToRegisteredResult())
	}
	return nil
}

func parseFileMode(modeStr string, defaultMode os.FileMode) os.FileMode { //nolint:unused
	if modeStr == "" {
		return defaultMode
	}

	// Parse as octal
	mode, err := strconv.ParseUint(modeStr, 8, 32)
	if err != nil {
		return defaultMode
	}

	return os.FileMode(mode)
}
