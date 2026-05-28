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

	// SupportedPlatforms lists the operating systems this action supports.
	// Valid values: "linux", "darwin", "windows", "freebsd", "openbsd", "netbsd", "dragonfly", "solaris", "aix"
	// Empty list means all platforms are supported
	SupportedPlatforms []string

	// RequiresSudo indicates whether this action typically requires elevated privileges.
	// This is informational - actual privilege requirements may vary based on the operation.
	RequiresSudo bool

	// ImplementsCheck indicates whether this action implements idempotency checks.
	// Actions with idempotency checks verify current state before making changes.
	ImplementsCheck bool

	// ImplementsDiff / ImplementsCost / ImplementsReverse /
	// ImplementsPermissions report whether the handler natively
	// implements the spec-22 four-method ABI sub-interfaces. Populated
	// centrally in Registry.List() from the IsDiffer/IsCoster/
	// IsReverser/IsPermitter helpers (registry_abi.go) — per-handler
	// authors do not set these by hand; the registry derives them
	// from the live interface satisfaction so the columns stay
	// honest as new methods land. Drives proposal-05 capability
	// columns in `mooncake actions list` and the x-implements-*
	// extensions emitted by `mooncake schema generate`.
	ImplementsDiff        bool
	ImplementsCost        bool
	ImplementsReverse     bool
	ImplementsPermissions bool

	// CaptureInPlan declares that this action's result is safe to bind into
	// Scope.Results during plan mode. Reserved for side-effect-free /
	// observation-only actions whose result is informative (e.g. read.json,
	// read.yaml). Default false: mutation actions must not affect vars during
	// plan. See spec-37.
	CaptureInPlan bool

	// Examples is an ordered list of hand-written YAML snippets shown by
	// `mooncake actions show <verb>`. Each entry should be a complete
	// step (one or more `- verb:` blocks) the user can copy-paste into a
	// playbook. When non-empty, the renderer prints these instead of the
	// synthetic schema-derived minimum example. Multi-line entries are
	// printed verbatim so authors control wrapping and field order.
	Examples []string
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
//
// Spec 16 collapsed the previous Execute / DryRun / Check trio into a
// single Run(ctx, step) method (the Runner interface). The legacy
// methods may still exist on concrete handler types — they are no
// longer part of the contract.
type Handler interface {
	// Metadata returns metadata describing this action type.
	Metadata() ActionMetadata

	// Validate checks if the step configuration is valid for this action.
	// Called before Run to fail fast on configuration errors.
	Validate(step *config.Step) error

	// Run executes the action when ctx.Mode() is ModeApply, or
	// inspects state and returns a prediction when ctx.Mode() is
	// ModePlan. Implementations:
	//
	//   - emit appropriate events via ctx.EventPublisher() (execute mode)
	//   - render templates via ctx.Template()
	//   - use ctx.Logger() for logging
	//   - return Result with Changed=true (execute) or
	//     WouldChange=true (plan) when state would change
	//   - route filesystem mutations through ctx.Effects() so that
	//     plan and execute modes share the same predicates
	//
	// Returns an error only on unrecoverable failure.
	Run(ctx Context, step *config.Step) (Result, error)
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
	ctx.Logger().Infof("  [DRY-RUN] Would execute %s action", h.metadata.Name)
	return nil
}

// Run satisfies the Spec 16 Handler contract. In ModePlan it reports
// "not checkable" by default; in ModeApply it delegates to the
// underlying execute function. HandlerFunc users wanting accurate
// plan-mode behavior should construct a typed Handler with its own
// Run method instead.
func (h *HandlerFunc) Run(ctx Context, step *config.Step) (Result, error) {
	if ctx.Mode() == ModePlan {
		return nil, nil
	}
	return h.execute(ctx, step)
}
