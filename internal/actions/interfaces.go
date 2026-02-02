package actions

import (
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/template"
)

// Context provides the execution environment for action handlers.
//
// Context is the primary interface through which handlers interact with the mooncake
// runtime. It provides access to:
//   - Template rendering (Jinja2-like syntax with variables and filters)
//   - Expression evaluation (when/changed_when/failed_when conditions)
//   - Logging (structured output to TUI or text)
//   - Variables (step vars, global vars, facts, registered results)
//   - Event publishing (for observability and artifacts)
//   - Execution mode (dry-run vs actual execution)
//
// This interface avoids circular imports between actions and executor packages.
//
// Example usage in a handler:
//
//	func (h *Handler) Execute(ctx actions.Context, step *config.Step) (actions.Result, error) {
//	    // Render template strings
//	    path, err := ctx.GetTemplate().RenderString(step.File.Path, ctx.GetVariables())
//
//	    // Log progress
//	    ctx.GetLogger().Infof("Creating file at %s", path)
//
//	    // Emit events for observability
//	    ctx.GetEventPublisher().Publish(events.Event{
//	        Type: events.EventFileCreated,
//	        Data: events.FileOperationData{Path: path},
//	    })
//
//	    // Return result
//	    result := executor.NewResult()
//	    result.SetChanged(true)
//	    return result, nil
//	}
type Context interface {
	// GetTemplate returns the template renderer for processing Jinja2-like templates.
	//
	// Use this to render:
	//   - Path strings with variables: "{{ home }}/{{ item }}"
	//   - Content with logic: "{% if os == 'linux' %}...{% endif %}"
	//   - Filters: "{{ path | expanduser }}"
	//
	// The renderer has access to all variables in scope (step vars, globals, facts).
	GetTemplate() template.Renderer

	// GetEvaluator returns the expression evaluator for conditions.
	//
	// Use this to evaluate:
	//   - when: "os == 'linux' && arch == 'amd64'"
	//   - changed_when: "result.rc == 0 and 'changed' in result.stdout"
	//   - failed_when: "result.rc != 0 and result.rc != 5"
	//
	// Returns interface{} which should be cast to bool for conditions.
	GetEvaluator() expression.Evaluator

	// GetLogger returns the logger for handler output.
	//
	// Use levels appropriately:
	//   - Infof: User-visible progress ("Installing package nginx")
	//   - Debugf: Detailed info ("Command: apt install nginx")
	//   - Warnf: Non-fatal issues ("File already exists, skipping")
	//   - Errorf: Failures ("Failed to create directory: permission denied")
	//
	// Output is formatted for TUI or text mode automatically.
	GetLogger() logger.Logger

	// GetVariables returns all variables in the current scope.
	//
	// Includes:
	//   - Step-level vars (defined in step.Vars)
	//   - Global vars (from vars actions)
	//   - System facts (os, arch, cpu_cores, memory_total_mb, etc.)
	//   - Registered results (from register: field on previous steps)
	//   - Loop context (item, item_index when in with_items/with_filetree)
	//
	// Keys are strings, values are interface{} (string, int, bool, []interface{}, map[string]interface{}).
	GetVariables() map[string]interface{}

	// GetEventPublisher returns the event publisher for observability.
	//
	// Emit events for:
	//   - State changes (EventFileCreated, EventServiceStarted)
	//   - Progress tracking (custom events for long operations)
	//   - Artifact generation (paths to created files)
	//
	// Events are consumed by:
	//   - Artifact collector (for rollback support)
	//   - External observers (CI/CD integrations)
	//   - Audit logs
	GetEventPublisher() events.Publisher

	// IsDryRun returns true if this is a dry-run execution.
	//
	// In dry-run mode:
	//   - Handlers MUST NOT make actual changes
	//   - Handlers SHOULD log what would happen
	//   - Template rendering should still work (to validate syntax)
	//   - File existence checks are OK (read-only operations)
	//   - Writing/deleting/executing is NOT OK
	//
	// The DryRun() method handles this automatically, but Execute() can also check.
	IsDryRun() bool

	// GetCurrentStepID returns the unique ID of the currently executing step.
	//
	// Format: "step-{global_step_number}"
	//
	// Use this when:
	//   - Emitting events (so they're associated with the step)
	//   - Creating temporary files (include step ID to avoid conflicts)
	//   - Logging (though step ID is usually added automatically)
	GetCurrentStepID() string
}

// Result represents the outcome of an action execution.
//
// Results track:
//   - Whether changes were made (for idempotency reporting)
//   - Output data (stdout/stderr from commands)
//   - Success/failure status
//   - Custom data (for result registration)
//
// Results can be registered to variables for use in subsequent steps via the
// register: field.
//
// Example:
//
//	result := executor.NewResult()
//	result.SetChanged(true)  // File was created/modified
//	result.SetData(map[string]interface{}{
//	    "path": "/etc/myapp/config.yml",
//	    "size": 1024,
//	    "checksum": "sha256:abc123...",
//	})
//
//	// If step has register: myfile, data is available as:
//	// {{ myfile.changed }} = true
//	// {{ myfile.path }} = "/etc/myapp/config.yml"
//
// This interface avoids circular imports between actions and executor packages.
type Result interface {
	// SetChanged marks whether this action modified system state.
	//
	// Set to true if the action:
	//   - Created/modified/deleted files or directories
	//   - Started/stopped/restarted services
	//   - Installed/removed packages
	//   - Executed commands that changed state
	//
	// Set to false if the action:
	//   - Found state already as desired (idempotent)
	//   - Only read/queried information
	//   - Failed before making changes
	//
	// Changed count is reported in run summary and used for idempotency tracking.
	SetChanged(changed bool)

	// SetStdout captures standard output from the action.
	//
	// Used primarily by shell/command actions. Output is:
	//   - Available in registered results as {{ result.stdout }}
	//   - Shown in TUI output view
	//   - Logged to artifacts
	//   - Used in changed_when/failed_when expressions
	SetStdout(stdout string)

	// SetStderr captures standard error from the action.
	//
	// Used primarily by shell/command actions. Error output is:
	//   - Available in registered results as {{ result.stderr }}
	//   - Shown in TUI output view (usually in red)
	//   - Logged to artifacts
	//   - Used in changed_when/failed_when expressions
	SetStderr(stderr string)

	// SetFailed marks the result as failed.
	//
	// Usually you should return an error instead of calling this. Use this when:
	//   - The action completed but didn't achieve desired state
	//   - failed_when expression evaluated to true
	//   - Assertion failed (assert action)
	//
	// Failed steps:
	//   - Increment failure count in run summary
	//   - Stop execution (unless ignore_errors: true)
	//   - Are highlighted in TUI
	SetFailed(failed bool)

	// SetData attaches custom data to the result.
	//
	// Data becomes available when the result is registered via register: field.
	//
	// Example:
	//
	//	result.SetData(map[string]interface{}{
	//	    "checksum": "sha256:abc123",
	//	    "size_bytes": 1024,
	//	    "format": "json",
	//	})
	//
	// Then in subsequent steps:
	//	  when: myfile.checksum == "sha256:abc123"
	//	  shell: echo "File size: {{ myfile.size_bytes }}"
	//
	// Keys should be snake_case. Values should be JSON-serializable.
	SetData(data map[string]interface{})

	// RegisterTo registers this result to the variables map.
	//
	// Called automatically by the executor when a step has register: field.
	// Creates a map in variables with:
	//   - changed: bool
	//   - failed: bool
	//   - stdout: string (if set)
	//   - stderr: string (if set)
	//   - rc: int (if applicable)
	//   - ...custom data from SetData()
	//
	// Handlers typically don't call this directly.
	RegisterTo(variables map[string]interface{}, name string)
}
