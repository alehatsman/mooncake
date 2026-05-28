package actions

import (
	"context"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
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
//	    path, err := ctx.Template().RenderString(step.FileWrite.Path, ctx.Variables())
//
//	    // Log progress
//	    ctx.Logger().Infof("Creating file at %s", path)
//
//	    // Emit events for observability
//	    ctx.EventPublisher().Publish(events.Event{
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
	// Template returns the template renderer for processing Jinja2-like templates.
	//
	// Use this to render:
	//   - Path strings with variables: "{{ home }}/{{ item }}"
	//   - Content with logic: "{% if os == 'linux' %}...{% endif %}"
	//   - Filters: "{{ path | expanduser }}"
	//
	// The renderer has access to all variables in scope (step vars, globals, facts).
	Template() template.Renderer

	// Evaluator returns the expression evaluator for conditions.
	//
	// Use this to evaluate:
	//   - when: "os == 'linux' && arch == 'amd64'"
	//   - changed_when: "result.rc == 0 and 'changed' in result.stdout"
	//   - failed_when: "result.rc != 0 and result.rc != 5"
	//
	// Returns interface{} which should be cast to bool for conditions.
	Evaluator() expression.Evaluator

	// Logger returns the logger for handler output.
	//
	// Use levels appropriately:
	//   - Infof: User-visible progress ("Installing package nginx")
	//   - Debugf: Detailed info ("Command: apt install nginx")
	//   - Warnf: Non-fatal issues ("File already exists, skipping")
	//   - Errorf: Failures ("Failed to create directory: permission denied")
	//
	// Output is formatted for TUI or text mode automatically.
	Logger() logger.Logger

	// Variables returns all variables in the current scope.
	//
	// Includes:
	//   - Step-level vars (defined in step.Vars)
	//   - Global vars (from vars actions)
	//   - System facts (os, arch, cpu_cores, memory_total_mb, etc.)
	//   - Registered results (from register: field on previous steps)
	//   - Loop context (item, item_index when in with_items/with_filetree)
	//
	// Keys are strings, values are interface{} (string, int, bool, []interface{}, map[string]interface{}).
	Variables() map[string]interface{}

	// EventPublisher returns the event publisher for observability.
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
	EventPublisher() events.Publisher

	// Mode reports the dispatch mode for this context. Handlers
	// implementing Runner consult this to decide whether to perform side
	// effects (ModeApply) or only inspect state (ModePlan).
	Mode() Mode

	// Effects returns a Performer wired to this context. Handlers
	// implementing Runner should call Performer methods instead of os.*
	// directly so that ModePlan vs ModeApply is decided in one place.
	// The returned Performer is cheap to construct and may be called
	// multiple times.
	Effects() Performer

	// Privileged returns the spec-72 Layer C escalation primitive,
	// pre-bound by dispatchRunner to the current step's AsUser.
	// Handlers should call ctx.Privileged().Run(...) for any shell-out
	// that needs escalation; the primitive decides the sudo wrap
	// (none / sudo / sudo -u <name>) from the bound AsUser. Handlers
	// must NOT read step.AsUser or step.ShouldBecome() for execution
	// decisions — the primitive sees them transparently. See
	// security.Privileged and spec-72 for the rationale.
	Privileged() *security.Privileged

	// StepID returns the unique ID of the currently executing step.
	//
	// Format: "step-{global_step_number}"
	//
	// Use this when:
	//   - Emitting events (so they're associated with the step)
	//   - Creating temporary files (include step ID to avoid conflicts)
	//   - Logging (though step ID is usually added automatically)
	StepID() string

	// MergeUserVars merges the provided key-value pairs into the user variable scope.
	// Use this instead of mutating the map returned by Variables() directly,
	// so that the write goes to the correct typed bucket (Scope.User when available).
	MergeUserVars(vars map[string]interface{})

	// Ctx returns the run-wide context driving this apply. Handlers
	// MUST plumb this into any external call (exec.CommandContext,
	// http.NewRequestWithContext, net.Dialer{...}.DialContext, …) so
	// the apply observes SIGINT / fleet kill / MCP shutdown / caller
	// cancel. Per-step timeouts compose on top via
	// context.WithTimeout(ctx.Ctx(), step.Timeout).
	//
	// Producers attach typed cancel causes via context.WithCancelCause
	// (see executor.ErrCancelSignal / ErrCancelFleet / ErrCancelMCP);
	// syncResultEnvelope then classifies the resulting Cancelled
	// step's CancelledReason. F2 (handler-ctx threading) is what
	// makes that classification observable end-to-end — without it,
	// handlers that ignore ctx complete normally during a cancel and
	// the recap never says cancelled=N.
	//
	// May return context.Background() in detached contexts (Reverse
	// plans, certain test setups) — handlers should treat it as
	// non-nil but always-cancellable in principle.
	Ctx() context.Context
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

// Runner is the unified handler entry point introduced by Spec 16. The
// Handler interface now embeds Run as a required method; Runner remains
// as a named type alias for clarity in call sites that want to express
// "the Run capability" specifically.
type Runner interface {
	Run(ctx Context, step *config.Step) (Result, error)
}

// RawRunner is the spec-69 phase 2-3 opt-in alternative to Runner.
// Handlers that implement RawRunner delegate retry-loop and result-
// override (changed_when / failed_when) responsibility to the
// executor. RunRaw must:
//
//   - Execute exactly one attempt of the action's work.
//   - Return (Result, error) reflecting the raw outcome; never apply
//     failed_when or changed_when overrides itself.
//   - Be safe to call multiple times in a row when retries are
//     configured.
//
// The executor wraps RunRaw in its retry loop (honoring step.Retry
// fields uniformly across all RawRunner implementations) and then
// applies overrides once, post-loop. This preserves MT-48 (the retry
// decision is on the raw exit code, never on the post-failed_when
// verdict) and MT-62 (backoff strategies honored uniformly).
//
// Handlers that implement both Runner.Run and RawRunner.RunRaw are
// dispatched via RunRaw — the executor prefers the RawRunner path.
// Handlers without RawRunner go through the legacy Runner.Run path
// unchanged; their own internal retry+override logic still applies.
type RawRunner interface {
	RunRaw(ctx Context, step *config.Step) (Result, error)
}

// Retryable is an optional companion to RawRunner. Handlers that
// implement it can decide per-attempt whether a retry should be
// tried — useful for actions like http.request where 5xx/429/
// timeout errors are retryable but 4xx errors aren't.
//
// The decision input is (result, err, step):
//   - result: whatever RunRaw returned. May be nil on pre-exec
//     failures. Carries Data which handlers like http.request
//     populate with status_code so retry policy can branch on it
//     without inventing a typed-error wrapper.
//   - err:    the raw outcome of the most recent attempt.
//   - step:   the YAML step (retry policy fields, action config).
//
// When absent, the executor's retry loop treats every non-nil err
// as retryable up to step.RetryAttempts().
type Retryable interface {
	IsRetryable(result Result, err error, step *config.Step) bool
}
