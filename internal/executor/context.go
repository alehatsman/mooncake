// Package executor provides the execution engine for mooncake configuration steps.
package executor

import (
	"time"

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
}

// NewExecutionStats creates a new ExecutionStats with all counters initialized to zero
func NewExecutionStats() *ExecutionStats {
	return &ExecutionStats{
		Global:   new(int),
		Executed: new(int),
		Skipped:  new(int),
		Failed:   new(int),
	}
}

// ExecutionContext holds all state needed to execute a step or sequence of steps.
//
// The context is designed to be copied when entering nested execution scopes (includes, loops).
// Most fields are copied by value, but certain fields use pointers to maintain shared state
// across the entire execution tree.
//
// Field categories:
//   - Configuration: Variables, CurrentDir, CurrentFile (copied on nested contexts)
//   - Display state: Level, CurrentIndex, TotalSteps (modified for each scope)
//   - Execution settings: Logger, SudoPass, Tags, DryRun (shared across contexts)
//   - Global counters: Pointers that accumulate across all contexts
//   - Dependencies: Shared service instances
type ExecutionContext struct {
	// Variables contains template variables available to steps.
	// Copied on context copy so nested contexts can have their own variables (e.g., loop items).
	Variables map[string]interface{}

	// CurrentDir is the directory containing the current config file.
	// Used for resolving relative paths in include, template src, etc.
	CurrentDir string

	// CurrentFile is the absolute path to the current config file being executed.
	// Used for error messages and debugging.
	CurrentFile string

	// Level tracks nesting depth for display indentation.
	// 0 = root config, increments by 1 for each include or loop level.
	Level int

	// CurrentIndex is the 0-based index of the current step within the current scope.
	// Resets to 0 when entering includes or loops.
	CurrentIndex int

	// TotalSteps is the number of steps in the current execution scope.
	// Updated when entering includes or loops to reflect the new scope size.
	TotalSteps int

	// Logger handles all output, configured with padding based on Level.
	Logger logger.Logger

	// SudoPass is the password used for steps with become: true.
	// Empty string if not provided via --sudo-pass flag.
	SudoPass string

	// Tags filters which steps execute (empty = all steps execute).
	// Steps without matching tags are skipped when this is non-empty.
	Tags []string

	// DryRun when true prevents any system changes (preview mode).
	// Commands are not executed, files are not created, templates are not rendered.
	DryRun bool

	// Stats holds shared execution statistics counters.
	// SHARED via pointer - all contexts update the same counters.
	Stats *ExecutionStats

	// Template renders template strings with variable substitution.
	// SHARED across all contexts - same instance used everywhere.
	Template template.Renderer

	// Evaluator evaluates when condition expressions.
	// SHARED across all contexts - same instance used everywhere.
	Evaluator expression.Evaluator

	// PathUtil expands paths with tilde and variable substitution.
	// SHARED across all contexts - same instance used everywhere.
	PathUtil *pathutil.PathExpander

	// FileTree walks directory trees for with_filetree.
	// SHARED across all contexts - same instance used everywhere.
	FileTree *filetree.Walker

	// Redactor redacts sensitive values (passwords) from log output.
	// SHARED across all contexts - same instance used everywhere.
	Redactor *security.Redactor

	// EventPublisher publishes execution events to subscribers.
	// SHARED across all contexts - same instance used everywhere.
	EventPublisher events.Publisher

	// CurrentStepID is the unique identifier for the currently executing step.
	// Used for correlating events from the same step execution.
	CurrentStepID string

	// CurrentResult holds the result of the currently executing step.
	// Handlers should set this to provide result data to event emission.
	CurrentResult *Result
}

// Copy creates a new ExecutionContext for a nested execution scope (include or loop).
// Variables map is shallow copied, display fields are copied by value, and pointer fields remain shared across all contexts.
func (ec *ExecutionContext) Copy() ExecutionContext {
	newVariables := make(map[string]interface{})
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	return ExecutionContext{
		Variables:    newVariables,
		CurrentDir:   ec.CurrentDir,
		CurrentFile:  ec.CurrentFile,
		Level:        ec.Level,
		CurrentIndex: ec.CurrentIndex,
		TotalSteps:   ec.TotalSteps,
		Logger:       ec.Logger,
		SudoPass:     ec.SudoPass,
		Tags:         ec.Tags,
		DryRun:       ec.DryRun,

		// Share the same statistics pointer
		Stats: ec.Stats,

		// Share the same dependency instances
		Template:  ec.Template,
		Evaluator: ec.Evaluator,
		PathUtil:  ec.PathUtil,
		FileTree:  ec.FileTree,
		Redactor:  ec.Redactor,

		// Share the same event publisher
		EventPublisher: ec.EventPublisher,
		CurrentStepID:  ec.CurrentStepID,
	}
}

// EmitEvent publishes an event to all subscribers
func (ec *ExecutionContext) EmitEvent(eventType events.EventType, data interface{}) {
	if ec.EventPublisher != nil {
		ec.EventPublisher.Publish(events.Event{
			Type:      eventType,
			Timestamp: time.Now(),
			Data:      data,
		})
	}
}

// HandleDryRun executes dry-run logging if in dry-run mode.
// Returns true if in dry-run mode (caller should return early).
// The logFn is called with a dryRunLogger to perform logging.
func (ec *ExecutionContext) HandleDryRun(logFn func(*dryRunLogger)) bool {
	if !ec.DryRun {
		return false
	}
	dryRun := newDryRunLogger(ec.Logger)
	logFn(dryRun)
	return true
}
