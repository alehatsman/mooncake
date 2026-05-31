// Package executor provides the execution engine for mooncake configuration steps.
package executor

import (
	"context"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/control"
	"github.com/alehatsman/mooncake/internal/effects"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/filetree"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// ExecutionStats holds shared statistics counters for execution tracking.
// All fields are pointers to enable shared state across nested execution contexts.
type ExecutionStats struct {
	// Global tracks total non-skipped steps across the entire execution tree
	Global *int
	// Executed counts successfully completed steps
	Executed *int
	// Skipped counts steps skipped due to when conditions or tag filtering
	Skipped *int
	// Failed counts steps that failed with errors
	Failed *int
	// Changed counts steps that resulted in a system change
	Changed *int
	// OK counts steps that completed successfully without changing the
	// system (F6 / proposal-02). Mutually exclusive with Changed at each
	// step decision site — a step is either "ran and did nothing" (OK)
	// or "ran and mutated" (Changed). Invariant: OK + Changed == Executed
	// for every successful step. Lifts the renderer-side derivation
	// (`ok := SuccessSteps - ChangedSteps`) onto a first-class field so
	// downstream consumers (agentd, MCP, SDK) read it directly.
	OK *int
	// Reverted counts steps whose changes were undone by a transaction's
	// LIFO Reverse() pass (MT-45). Reverted steps are subtracted from
	// Changed at rollback time so the recap reflects net effect, not
	// gross writes-then-undos.
	Reverted *int
	// Cancelled counts steps interrupted mid-execution per proposal-02
	// (SIGINT, fleet kill, timeout). Distinct from Failed: a cancelled
	// step didn't fail on its own merits; the exit-code aggregator
	// maps cancelled>0 to 130, failed>0 to 1.
	Cancelled *int
	// Healed counts assert steps that failed on first dispatch, ran
	// their declared heal: child plan, and passed the re-check
	// (proposal-11). Distinct from Failed (assert never passed) and
	// Changed (heal children's own changes are counted separately).
	// A non-zero Healed means the system drifted and was restored
	// in-band — the kernel-level self-healing signal.
	Healed *int
}

// NewExecutionStats creates a new ExecutionStats with all counters initialized to zero
func NewExecutionStats() *ExecutionStats {
	return &ExecutionStats{
		Global:    new(int),
		Executed:  new(int),
		Skipped:   new(int),
		Failed:    new(int),
		Changed:   new(int),
		OK:        new(int),
		Reverted:  new(int),
		Cancelled: new(int),
		Healed:    new(int),
	}
}

// incStat bumps the counter at p by one. nil-safe so callers don't
// have to guard every increment site; the F053 cold-read pass
// surfaced 10 unguarded `*ec.Svc.Stats.X++` sites alongside the 2
// already-guarded ones in postExecuteSuccess / handleTxnBodyFailure
// — the inconsistency was a panic latent in any future caller that
// builds an `&ExecutionStats{}` literal instead of going through
// NewExecutionStats. Centralising the guard here keeps the call
// sites short while making the safety uniform.
func incStat(p *int) {
	if p != nil {
		*p++
	}
}

// decStat is the inverse of incStat, used by transaction.go's
// MT-45 rollback bookkeeping (rolled-back steps subtract from the
// run-wide Changed counter so the recap reflects net effect, not
// gross writes-then-undos). Stays at zero rather than going
// negative — a roll-back of a step that didn't bump Changed
// shouldn't make the counter negative.
func decStat(p *int) {
	if p != nil && *p > 0 {
		*p--
	}
}

// readStat returns the counter at p, or zero if p is nil. Companion
// to incStat for the two non-increment sites (generateStepID's
// step-N label and StepStartedData.GlobalStep).
func readStat(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// Mode and its constants live in the actions package; re-exported here
// for backward source compatibility during the Spec 16 migration.
type Mode = actions.Mode

const (
	ModeApply = actions.ModeApply
	ModePlan  = actions.ModePlan
)

// RunServices holds the shared, immutable-after-construction services and
// configuration for a mooncake run. One instance is created per run and
// referenced by all nested ExecutionContexts via pointer.
type RunServices struct {
	Template       template.Renderer
	Evaluator      expression.Evaluator
	PathUtil       *pathutil.PathExpander
	FileTree       *filetree.Walker
	Redactor       *security.Redactor
	EventPublisher events.Publisher
	Logger         logger.Logger
	Stats          *ExecutionStats
	Mode           actions.Mode
	Tags           []string
	SudoPass       string
	// PasswordlessSudo is set at run construction by probing
	// `sudo -n true`. When true, Sudo+AsUser steps don't need a
	// password flag — preflight passes and BecomeRunner uses
	// `sudo -n <cmd>` instead of `sudo -S`.
	//
	// spec-72 phase 1: derived from Escalation.Reason ==
	// EscalationAvailablePasswordless. Kept on RunServices as the
	// backward-compat shim while preflight/BecomeRunner/effects still
	// read the bool; phase 5 drops it in favor of Escalation.
	PasswordlessSudo bool

	// Escalation is the unified, once-per-run answer to "can this
	// process escalate to root, and if not, why not?". Populated by
	// security.ProbeEscalation at RunServices construction
	// (spec-72 §1). Consumed by *security.Privileged for the actual
	// sudo wrap and by preflight for diagnostic messages.
	Escalation security.EscalationReport
	// Capture, if non-nil, records the compiled plan and per-step
	// outcomes for callers that want the typed *KernelResult shape
	// (internal/apply.Runner for R1.1b). nil for the legacy
	// executor.Start callers that only care about the error return.
	Capture *RunCapture

	// Ctx carries cancellation/deadline state for the run. The step
	// loop in ExecuteSteps checks Ctx.Err() before dispatching each
	// step and aborts cleanly if the context is cancelled (F016
	// stage-1(a) — handler-level cancellation is the stage-3 audit).
	// May be nil in test contexts that construct RunServices directly;
	// callers that want cancellation set this to a context that the
	// embedding shell (daemon Shutdown / CLI signal handler) cancels.
	Ctx context.Context

	// Modules is the playbook's `modules:` alias map (spec-67). Read by the
	// `use:` action handler so alias references like `use: postgres` resolve
	// to a cached module. Empty when the playbook declares no modules.
	Modules map[string]string
}

// LoopContext holds the current loop iteration state for a step executing
// inside a with_items or with_filetree loop. It is stored in VariableScope.Loop
// so ToMap() can inject item/index/first/last without polluting the User map.
type LoopContext struct {
	Item  interface{}
	Index int
	First bool
	Last  bool
}

// ExecutionContext holds per-scope state for a step sequence.
// Cloned when entering nested scopes (includes, loops); Svc is shared.
//
// Field categories:
//   - Svc: shared services and run configuration — pointer, never copied
//   - Scope, CurrentDir, *: per-scope state — copied on Clone
//   - CurrentStepID, CurrentResult: per-step state — not copied on Clone
type ExecutionContext struct {
	// Svc holds all shared services and run-level configuration.
	// All nested contexts share the same *RunServices pointer.
	Svc *RunServices

	// Scope holds all variable categories in typed sections.
	// It is the authoritative source for variables.
	// Use NewVariableScope() to create a ready scope.
	Scope *VariableScope

	// CurrentDir is the directory containing the current config file.
	// Set per-file by the planner (each include / component flips it to
	// the included file's own dir). All relative paths in step fields
	// resolve against this Node-style: `./foo` is relative to the YAML
	// file that declares the step.
	CurrentDir string

	// CurrentFile is the absolute path to the current config file being executed.
	CurrentFile string

	// Level tracks nesting depth for display indentation.
	Level int

	// CurrentIndex is the 0-based index of the current step within the current scope.
	CurrentIndex int

	// TotalSteps is the number of steps in the current execution scope.
	TotalSteps int

	// CurrentStepID is the unique identifier for the currently executing step.
	// Not copied on Clone — resets per step.
	CurrentStepID string

	// CurrentResult holds the result of the currently executing step.
	// Not copied on Clone — resets per step.
	CurrentResult *Result

	// CurrentAsUser is the step's declared AsUser, bound by
	// dispatchRunner before calling runner.Run. Consumed by
	// ec.Privileged() and ec.Effects() so handlers don't read
	// step.AsUser for execution decisions — the primitive sees it
	// transparently. Spec-72 Layer C.
	//
	// Not copied on Clone — each step gets its own binding; nested
	// scopes (loops, includes) inherit through the per-step
	// dispatchRunner re-binding rather than through structural
	// copying. Empty for steps that didn't declare as_user.
	CurrentAsUser string

	// ChangedByStepID records the .Changed outcome of each step that has
	// completed in this execution context, keyed by step.ID. Read by
	// on_change child execution (spec-23 §1): a child runs iff
	// ChangedByStepID[step.TriggeredBy] is true. Survives across steps so
	// triggered children can look back at their parents; never copied to
	// nested Clone() scopes (each scope tracks its own changes).
	ChangedByStepID map[string]bool

	// OpenTxns tracks per-transaction state for spec-30 transaction:
	// blocks. Keyed by the transaction-parent step ID (which children
	// carry as TxnParent). Created lazily when the first body child of
	// a given TxnParent completes. The state type lives in
	// internal/control — see kernel.md for the kernel-sub-system
	// rationale (R0.1).
	OpenTxns map[string]*control.TxnState

	// CompletedByTxn tracks the in-order list of body children that
	// ran to completion (no error) for each transaction, keyed by the
	// transaction-parent step ID. Each entry carries the step and the
	// concrete *Result the handler produced — the Reverser needs the
	// Result to know what to undo. This slot lives in executor (not
	// in control alongside TxnState) because *Result is an
	// executor-package type and moving it would create a circular
	// import.
	CompletedByTxn map[string][]TxnCompletedChild

	// OpenTries tracks per-try-block state for spec-23 §2 try / catch /
	// finally. Keyed by the compound-parent step ID (which children
	// carry as TryParent). Created lazily by the executor's trycatch.go
	// when a try child's failure has to be recorded. The state type
	// lives in internal/control.
	OpenTries map[string]*control.TryState
}

// TxnCompletedChild captures one body child's step + result for later
// Reverse() consumption. Stored in ExecutionContext.CompletedByTxn —
// the *Result field keeps this type out of internal/control.
type TxnCompletedChild struct {
	Step   config.Step
	Result *Result
}

// Clone creates a new ExecutionContext for a nested execution scope (include or loop).
// Svc is shared by pointer; Scope is deep-cloned (User+Results); per-step fields are reset.
func (ec *ExecutionContext) Clone() ExecutionContext {
	cloned := ExecutionContext{
		Svc:          ec.Svc,
		CurrentDir:   ec.CurrentDir,
		CurrentFile:  ec.CurrentFile,
		Level:        ec.Level,
		CurrentIndex: ec.CurrentIndex,
		TotalSteps:   ec.TotalSteps,
		// CurrentStepID and CurrentResult intentionally omitted — per-step state
	}
	cloned.Scope = ec.Scope.Clone()
	return cloned
}

// EmitEvent publishes an event to all subscribers
func (ec *ExecutionContext) EmitEvent(eventType events.Type, data interface{}) {
	if ec.Svc.EventPublisher != nil {
		ec.Svc.EventPublisher.Publish(events.Event{
			Type:      eventType,
			Timestamp: time.Now(),
			Data:      data,
		})
	}
}

// Mode returns the current dispatch mode (ModeApply or ModePlan).
func (ec *ExecutionContext) Mode() Mode {
	return ec.Svc.Mode
}

// Effects returns a Performer pre-bound to the current step's
// AsUser. Like ec.Privileged(), the per-step binding means handlers
// don't have to thread step.AsUser through PerformerOpts — the
// Performer consults its bound state to decide sudo wrap and
// post-write chown.
func (ec *ExecutionContext) Effects() actions.Performer {
	return effects.NewPerformer(ec.Mode, ec.Svc.SudoPass, ec.Svc.PasswordlessSudo, ec.CurrentAsUser)
}

// Privileged returns the spec-72 Layer C escalation primitive,
// pre-bound to the current step's AsUser. Handlers should call
// ctx.Privileged().Run(...) / .Command(...) for shell-outs and let
// the primitive decide the sudo wrap from the bound AsUser. No
// per-call `become bool` plumbing; no per-handler `step.ShouldBecome`
// reads. dispatchRunner sets ec.CurrentAsUser from step.AsUser
// before calling Run, so each step sees a primitive bound to its
// own declared identity.
func (ec *ExecutionContext) Privileged() *security.Privileged {
	return &security.Privileged{
		SudoPass:   ec.Svc.SudoPass,
		Escalation: ec.Svc.Escalation,
		AsUser:     ec.CurrentAsUser,
	}
}

// --- actions.Context interface implementation ---

// Template returns the template renderer.
func (ec *ExecutionContext) Template() template.Renderer {
	return ec.Svc.Template
}

// Evaluator returns the expression evaluator.
func (ec *ExecutionContext) Evaluator() expression.Evaluator {
	return ec.Svc.Evaluator
}

// Logger returns the logger.
func (ec *ExecutionContext) Logger() logger.Logger {
	return ec.Svc.Logger
}

// Variables returns all variables merged into a flat map for template/expression engines.
func (ec *ExecutionContext) Variables() map[string]interface{} {
	m := ec.Scope.ToMap()
	// Overlay component_dir per step: the dir of the file/component that
	// declares the running step (ec.CurrentDir; the module-cache dir for a
	// `use:`d component). Plan-time rendering resolves this for action
	// fields, but execute-time renders (when/creates/unless and path
	// expansion) read it from here. invocation_dir is a global constant
	// carried in the User scope from the plan's InitialVars.
	if ec.CurrentDir != "" {
		m["component_dir"] = ec.CurrentDir
	}
	return m
}

// MergeUserVars merges the provided key-value pairs into the user variable scope.
// Logs a warning for any key that shadows a system fact or metric.
//
// Drops the `if ec.Svc != nil` guard the pre-cleanup version carried —
// every other accessor on ExecutionContext (EmitEvent, Mode, Effects,
// Privileged, Template / Evaluator / Logger / EventPublisher) derefs
// ec.Svc unconditionally. Svc is always non-nil in production paths
// (Start / executePlanWithCapture sets it on every constructed context);
// a future test that builds an EC without Svc panics here exactly the
// same way it would in any of the peer accessors. Convention drift
// closed.
func (ec *ExecutionContext) MergeUserVars(vars map[string]interface{}) {
	for _, k := range ec.Scope.shadowedKeys(vars) {
		ec.Svc.Logger.Infof("[WARNING] variable %q shadows a system fact or metric and will override it", k)
	}
	for k, v := range vars {
		ec.Scope.User[k] = v
	}
}

// RegisterResult registers a Result under the given name for use in subsequent steps.
func (ec *ExecutionContext) RegisterResult(r *Result, name string) {
	ec.Scope.Results[name] = r.ToRegisteredResult()
}

// EventPublisher returns the event publisher.
func (ec *ExecutionContext) EventPublisher() events.Publisher {
	return ec.Svc.EventPublisher
}

// StepID returns the current step ID.
func (ec *ExecutionContext) StepID() string {
	return ec.CurrentStepID
}

// Ctx returns the run-wide context (ec.Svc.Ctx). Handlers reach through
// this to plumb cancellation into shell-outs and HTTP calls so SIGINT /
// fleet kill / MCP shutdown propagates end-to-end (F2).
//
// Returns context.Background() when Svc or Svc.Ctx is nil — production
// paths always populate both, but the guard keeps test-built contexts
// that skip RunServices construction from panicking. Returning a live
// (non-nil, non-cancellable) ctx is safer than nil for handlers that
// chain WithTimeout / WithCancel onto it.
func (ec *ExecutionContext) Ctx() context.Context {
	if ec.Svc == nil || ec.Svc.Ctx == nil {
		return context.Background()
	}
	return ec.Svc.Ctx
}
