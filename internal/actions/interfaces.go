package actions

import (
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// Context defines the interface for action execution context.
// This avoids circular imports with the executor package.
type Context interface {
	// Template provides template rendering
	GetTemplate() template.Renderer

	// Evaluator provides expression evaluation
	GetEvaluator() expression.Evaluator

	// Logger provides logging
	GetLogger() logger.Logger

	// Variables provides access to execution variables
	GetVariables() map[string]interface{}

	// EventPublisher provides event publishing
	GetEventPublisher() events.Publisher

	// DryRun indicates if this is a dry-run execution
	IsDryRun() bool

	// CurrentStepID returns the current step ID
	GetCurrentStepID() string
}

// Result defines the interface for action results.
// This avoids circular imports with the executor package.
type Result interface {
	// SetChanged marks whether the action made changes
	SetChanged(changed bool)

	// SetStdout sets the stdout output
	SetStdout(stdout string)

	// SetStderr sets the stderr output
	SetStderr(stderr string)

	// SetFailed marks the result as failed
	SetFailed(failed bool)

	// SetData sets custom result data
	SetData(data map[string]interface{})

	// RegisterTo registers the result to variables
	RegisterTo(variables map[string]interface{}, name string)
}
