// Package executor provides the execution engine for mooncake configuration steps.
package executor

import (
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
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

// ExecutionContext holds per-scope state for a step sequence.
// Cloned when entering nested scopes (includes, loops); Svc is shared.
//
// Field categories:
//   - Svc: shared services and run configuration — pointer, never copied
//   - Variables, CurrentDir, *: per-scope state — copied on Clone
//   - CurrentStepID, CurrentResult: per-step state — not copied on Clone
type ExecutionContext struct {
	// Svc holds all shared services and run-level configuration.
	// All nested contexts share the same *RunServices pointer.
	Svc *RunServices

	// Variables contains template variables available to steps.
	// Shallow-copied on Clone so nested contexts have their own variable scope.
	Variables map[string]interface{}

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
}

// Clone creates a new ExecutionContext for a nested execution scope (include or loop).
// Svc is shared by pointer; Variables is shallow-copied; per-step fields are reset.
func (ec *ExecutionContext) Clone() ExecutionContext {
	newVariables := make(map[string]interface{}, len(ec.Variables))
	for k, v := range ec.Variables {
		newVariables[k] = v
	}

	return ExecutionContext{
		Svc:           ec.Svc,
		Variables:     newVariables,
		CurrentDir:    ec.CurrentDir,
		PresetBaseDir: ec.PresetBaseDir,
		CurrentFile:   ec.CurrentFile,
		Level:         ec.Level,
		CurrentIndex:  ec.CurrentIndex,
		TotalSteps:    ec.TotalSteps,
		// CurrentStepID and CurrentResult intentionally omitted — per-step state
	}
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

// GetVariables returns the execution variables.
func (ec *ExecutionContext) GetVariables() map[string]interface{} {
	return ec.Variables
}

// GetEventPublisher returns the event publisher.
func (ec *ExecutionContext) GetEventPublisher() events.Publisher {
	return ec.Svc.EventPublisher
}

// GetCurrentStepID returns the current step ID.
func (ec *ExecutionContext) GetCurrentStepID() string {
	return ec.CurrentStepID
}
