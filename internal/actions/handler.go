// Package actions provides the handler interface and registry for mooncake actions.
//
// The actions package defines a standard interface that all action handlers must implement,
// along with a registry system for discovering and dispatching to handlers at runtime.
//
// To create a new action handler:
//  1. Create a new package under internal/actions (e.g., internal/actions/notify)
//  2. Implement the Handler interface
//  3. Register your handler in an init() function
//  4. The handler will be automatically available for use
//
// Example:
//
//	package notify
//
//	import "github.com/alehatsman/mooncake/internal/actions"
//
//	type Handler struct{}
//
//	func init() {
//	    actions.Register(&Handler{})
//	}
//
//	func (h *Handler) Metadata() actions.ActionMetadata {
//	    return actions.ActionMetadata{
//	        Name:        "notify",
//	        Description: "Send notifications",
//	        Category:    actions.CategorySystem,
//	    }
//	}
//
//	// ... implement other interface methods
package actions

import (
	"github.com/alehatsman/mooncake/internal/config"
)

// ActionCategory groups related actions by their primary function.
type ActionCategory string

const (
	// CategoryCommand represents actions that execute commands (shell, command)
	CategoryCommand ActionCategory = "command"

	// CategoryFile represents actions that manipulate files (file, template, copy, download)
	CategoryFile ActionCategory = "file"

	// CategorySystem represents system-level actions (service, assert, preset)
	CategorySystem ActionCategory = "system"

	// CategoryData represents data manipulation actions (vars, include_vars)
	CategoryData ActionCategory = "data"

	// CategoryNetwork represents network-related actions (download, http requests)
	CategoryNetwork ActionCategory = "network"

	// CategoryOutput represents output/display actions (print)
	CategoryOutput ActionCategory = "output"
)

// ActionMetadata describes an action type and its capabilities.
type ActionMetadata struct {
	// Name is the action name as it appears in YAML (e.g., "shell", "file", "notify")
	Name string

	// Description is a human-readable description of what this action does
	Description string

	// Category groups related actions (command, file, system, etc.)
	Category ActionCategory

	// SupportsDryRun indicates whether this action can be executed in dry-run mode
	SupportsDryRun bool

	// SupportsBecome indicates whether this action supports privilege escalation (sudo)
	SupportsBecome bool

	// EmitsEvents lists the event types this action emits (e.g., "file.created", "notify.sent")
	EmitsEvents []string

	// Version is the action implementation version (semantic versioning)
	Version string
}

// Handler defines the interface that all action handlers must implement.
//
// A handler is responsible for:
//   - Validating action configuration
//   - Executing the action
//   - Handling dry-run mode
//   - Emitting appropriate events
//   - Returning results
//
// Handlers should be stateless - all execution state is passed via ExecutionContext.
type Handler interface {
	// Metadata returns metadata describing this action type.
	Metadata() ActionMetadata

	// Validate checks if the step configuration is valid for this action.
	// This is called before Execute to fail fast on configuration errors.
	// Returns an error if validation fails.
	Validate(step *config.Step) error

	// Execute runs the action and returns a result.
	// The result includes whether the action made changes, output data,
	// and any error information.
	//
	// Handlers should:
	//   - Emit appropriate events via ctx.GetEventPublisher()
	//   - Handle template rendering via ctx.GetTemplate()
	//   - Use ctx.GetLogger() for logging
	//   - Return a Result with Changed=true if the action modified state
	//
	// If an error occurs, return it - the executor will handle result registration.
	Execute(ctx Context, step *config.Step) (Result, error)

	// DryRun logs what would happen if Execute were called, without making changes.
	// This is called when the executor is in dry-run mode.
	//
	// Handlers should:
	//   - Use ctx.GetLogger() to describe what would happen
	//   - Attempt to render templates (but catch errors gracefully)
	//   - NOT make any actual changes to the system
	//   - NOT emit action-specific events (step lifecycle events are handled by executor)
	//
	// Returns an error only if dry-run simulation fails catastrophically.
	DryRun(ctx Context, step *config.Step) error
}

// HandlerFunc is a function type that implements Handler for simple actions.
// This allows creating handlers without defining a new type.
type HandlerFunc struct {
	metadata ActionMetadata
	validate func(*config.Step) error
	execute  func(Context, *config.Step) (Result, error)
	dryRun   func(Context, *config.Step) error
}

// NewHandlerFunc creates a Handler from function implementations.
func NewHandlerFunc(
	metadata ActionMetadata,
	validate func(*config.Step) error,
	execute func(Context, *config.Step) (Result, error),
	dryRun func(Context, *config.Step) error,
) Handler {
	return &HandlerFunc{
		metadata: metadata,
		validate: validate,
		execute:  execute,
		dryRun:   dryRun,
	}
}

func (h *HandlerFunc) Metadata() ActionMetadata {
	return h.metadata
}

func (h *HandlerFunc) Validate(step *config.Step) error {
	if h.validate != nil {
		return h.validate(step)
	}
	return nil
}

func (h *HandlerFunc) Execute(ctx Context, step *config.Step) (Result, error) {
	return h.execute(ctx, step)
}

func (h *HandlerFunc) DryRun(ctx Context, step *config.Step) error {
	if h.dryRun != nil {
		return h.dryRun(ctx, step)
	}
	// Default: log that action would execute
	ctx.GetLogger().Infof("  [DRY-RUN] Would execute %s action", h.metadata.Name)
	return nil
}
