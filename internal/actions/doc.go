// Package actions provides the action handler system for mooncake.
//
// # Overview
//
// The actions package defines a standard interface (Handler) that all action
// implementations must follow, along with a registry for discovering handlers
// at runtime.
//
// # Architecture
//
// Actions are implemented as packages under internal/actions/. Each action
// package provides a Handler implementation that is registered globally on
// import via an init() function.
//
// The executor looks up handlers from the registry based on the action type
// determined from the step configuration.
//
// # Backward Compatibility
//
// This new system is designed to work alongside the existing action implementations.
// The Step struct retains all existing action fields (Shell, File, Template, etc.),
// and actions are migrated incrementally to the new Handler interface.
//
// # Creating a New Action
//
// To create a new action handler:
//
// 1. Create a package under internal/actions/ (e.g., internal/actions/notify)
//
// 2. Implement the Handler interface:
//
//	type Handler struct{}
//
//	func (h *Handler) Metadata() actions.ActionMetadata {
//	    return actions.ActionMetadata{
//	        Name:           "notify",
//	        Description:    "Send notifications",
//	        Category:       actions.CategorySystem,
//	        SupportsDryRun: true,
//	    }
//	}
//
//	func (h *Handler) Validate(step *config.Step) error {
//	    // Validate step.Notify config
//	    return nil
//	}
//
//	func (h *Handler) Execute(ctx *executor.ExecutionContext, step *config.Step) (*executor.Result, error) {
//	    // Implement action logic
//	    return &executor.Result{Changed: true}, nil
//	}
//
//	func (h *Handler) DryRun(ctx *executor.ExecutionContext, step *config.Step) error {
//	    ctx.Logger.Infof("  [DRY-RUN] Would send notification")
//	    return nil
//	}
//
// 3. Register the handler in init():
//
//	func init() {
//	    actions.Register(&Handler{})
//	}
//
// 4. Import the package in the executor to ensure registration:
//
//	import _ "github.com/alehatsman/mooncake/internal/actions/notify"
//
// # Migration Strategy
//
// Existing actions are being migrated incrementally:
//
//   - Phase 1: Create Handler implementations for simple actions (print, vars)
//   - Phase 2: Migrate complex actions (shell, file, template)
//   - Phase 3: Migrate specialized actions (service, assert, preset)
//   - Phase 4: Remove legacy code paths
//
// During migration, both old and new implementations coexist. The executor
// checks if a handler is registered and prefers it, falling back to legacy
// implementations for non-migrated actions.
package actions
