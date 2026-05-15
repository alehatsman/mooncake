// Package executor provides the execution engine for mooncake configuration steps.
package executor

import (
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
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
}

// NewExecutionStats creates a new ExecutionStats with all counters initialized to zero
func NewExecutionStats() *ExecutionStats {
	return &ExecutionStats{
		Global:   new(int),
		Executed: new(int),
		Skipped:  new(int),
		Failed:   new(int),
		Changed:  new(int),
	}
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
	CurrentDir string

	// PresetBaseDir is the root directory of the currently executing preset.
	PresetBaseDir string

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
	// a given TxnParent completes.
	OpenTxns map[string]*TxnState

	// OpenTries tracks per-try-block state for spec-23 §2 try / catch /
	// finally. Keyed by the compound-parent step ID (which children
	// carry as TryParent). Created lazily by the executor's trycatch.go
	// when a try child's failure has to be recorded.
	OpenTries map[string]*TryState
}

// TxnState is the per-transaction state the executor accumulates as it
// walks a transaction's body children. When a body child fails, the
// executor walks Completed in reverse and calls Reverse() on each.
type TxnState struct {
	// Completed is the in-order list of body children that ran to
	// completion (no error). Each entry carries the step and the
	// concrete *Result the handler produced — the Reverser needs the
	// Result to know what to undo.
	Completed []TxnCompletedChild

	// Failed is true once any body child of this transaction has
	// errored. Subsequent body children skip; rollback gets triggered
	// on the failing child's path.
	Failed bool

	// RolledBack is true once rollback has been attempted. Set whether
	// rollback fully succeeded or only partially reverted — it's the
	// signal to fire on_rollback children regardless.
	RolledBack bool

	// PartialRollback is true if any Reverse() in the LIFO walk
	// returned an error. Used to surface ROLLBACK INCOMPLETE in the
	// final run output.
	PartialRollback bool
}

// TxnCompletedChild captures one body child's step + result for later
// Reverse() consumption.
type TxnCompletedChild struct {
	Step   config.Step
	Result *Result
}

// Clone creates a new ExecutionContext for a nested execution scope (include or loop).
// Svc is shared by pointer; Scope is deep-cloned (User+Results); per-step fields are reset.
func (ec *ExecutionContext) Clone() ExecutionContext {
	cloned := ExecutionContext{
		Svc:           ec.Svc,
		CurrentDir:    ec.CurrentDir,
		PresetBaseDir: ec.PresetBaseDir,
		CurrentFile:   ec.CurrentFile,
		Level:         ec.Level,
		CurrentIndex:  ec.CurrentIndex,
		TotalSteps:    ec.TotalSteps,
		// CurrentStepID and CurrentResult intentionally omitted — per-step state
	}
	cloned.Scope = ec.Scope.Clone()
	return cloned
}

// EmitEvent publishes an event to all subscribers
func (ec *ExecutionContext) EmitEvent(eventType events.EventType, data interface{}) {
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

// Effects returns a Performer that routes filesystem and command
// primitives by the current Mode.
func (ec *ExecutionContext) Effects() actions.Performer {
	return effects.NewPerformer(ec.Mode, ec.Svc.SudoPass)
}

// --- actions.Context interface implementation ---

// GetTemplate returns the template renderer.
func (ec *ExecutionContext) GetTemplate() template.Renderer {
	return ec.Svc.Template
}

// GetEvaluator returns the expression evaluator.
func (ec *ExecutionContext) GetEvaluator() expression.Evaluator {
	return ec.Svc.Evaluator
}

// GetLogger returns the logger.
func (ec *ExecutionContext) GetLogger() logger.Logger {
	return ec.Svc.Logger
}

// GetVariables returns all variables merged into a flat map for template/expression engines.
func (ec *ExecutionContext) GetVariables() map[string]interface{} {
	return ec.Scope.ToMap()
}

// MergeUserVars merges the provided key-value pairs into the user variable scope.
// Logs a warning for any key that shadows a system fact or metric.
func (ec *ExecutionContext) MergeUserVars(vars map[string]interface{}) {
	if ec.Svc != nil {
		for _, k := range ec.Scope.shadowedKeys(vars) {
			ec.Svc.Logger.Infof("[WARNING] variable %q shadows a system fact or metric and will override it", k)
		}
	}
	for k, v := range vars {
		ec.Scope.User[k] = v
	}
}

// RegisterResult registers a Result under the given name for use in subsequent steps.
func (ec *ExecutionContext) RegisterResult(r *Result, name string) {
	ec.Scope.Results[name] = r.ToRegisteredResult()
}

// GetEventPublisher returns the event publisher.
func (ec *ExecutionContext) GetEventPublisher() events.Publisher {
	return ec.Svc.EventPublisher
}

// GetCurrentStepID returns the current step ID.
func (ec *ExecutionContext) GetCurrentStepID() string {
	return ec.CurrentStepID
}
