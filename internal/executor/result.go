package executor

import (
	"time"
)

// Result represents the outcome of executing a step and can be registered
// to variables for use in subsequent steps via the "register" field.
//
// Field usage varies by step type:
//
// Shell steps:
//   - Stdout: captured standard output from the command
//   - Stderr: captured standard error from the command
//   - Rc: exit code (0 for success, non-zero for failure)
//   - Failed: true if Rc != 0
//   - Changed: always true (commands are assumed to make changes)
//
// File steps (file with state: file or directory):
//   - Rc: 0 for success, 1 for failure
//   - Failed: true if file/directory operation failed
//   - Changed: true if file/directory was created or content modified
//
// Template steps:
//   - Rc: 0 for success, 1 for failure
//   - Failed: true if template rendering or file write failed
//   - Changed: true if output file was created or content changed
//
// Variable steps (vars, include_vars):
//   - All fields remain at default values (not currently used)
//
// The Skipped field is reserved for future use but not currently set by any step type.
type Result struct {
	// Stdout contains the standard output from shell commands.
	// Only populated by shell steps.
	Stdout string `json:"stdout"`

	// Stderr contains the standard error from shell commands.
	// Only populated by shell steps.
	Stderr string `json:"stderr"`

	// Rc is the return/exit code.
	// For shell steps: the command's exit code (0 = success).
	// For file/template steps: 0 for success, 1 for failure.
	Rc int `json:"rc"`

	// Failed indicates whether the step execution failed.
	// Set to true when shell commands exit non-zero or when file/template operations error.
	Failed bool `json:"failed"`

	// Changed indicates whether the step made modifications to the system.
	// Shell steps: always true (commands assumed to make changes).
	// File steps: true if file/directory was created or modified.
	// Template steps: true if output file was created or content changed.
	Changed bool `json:"changed"`

	// Skipped is reserved for future use to indicate skipped steps.
	// Currently not set by any step type.
	Skipped bool `json:"skipped"`

	// Timing information
	StartTime time.Time     `json:"start_time,omitempty"`
	EndTime   time.Time     `json:"end_time,omitempty"`
	Duration  time.Duration `json:"duration_ms,omitempty"` // Duration in time.Duration format
}

// NewResult creates a new Result with default values.
func NewResult() *Result {
	return &Result{
		Stdout:  "",
		Stderr:  "",
		Rc:      0,
		Failed:  false,
		Changed: false,
		Skipped: false,
	}
}

// Status returns a string representation of the result status.
func (r *Result) Status() string {
	if r.Failed {
		return "failed"
	}
	if r.Skipped {
		return "skipped"
	}
	if r.Changed {
		return "changed"
	}
	return "ok"
}

// ToMap converts Result to a map for use in template variables.
func (r *Result) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
		"rc":          r.Rc,
		"failed":      r.Failed,
		"changed":     r.Changed,
		"skipped":     r.Skipped,
		"duration_ms": r.Duration.Milliseconds(),
		"status":      r.Status(),
	}
}

// RegisterTo registers this result to the variables map under the given name.
// The result can be accessed using nested field syntax (e.g., "result.stdout", "result.rc") in templates and when conditions.
func (r *Result) RegisterTo(variables map[string]interface{}, name string) {
	variables[name] = r.ToMap()
}

// --- actions.Result interface implementation ---
// These methods allow Result to be used as an actions.Result,
// avoiding circular import dependencies between executor and actions packages.

// SetChanged marks whether the action made changes.
func (r *Result) SetChanged(changed bool) {
	r.Changed = changed
}

// SetStdout sets the stdout output.
func (r *Result) SetStdout(stdout string) {
	r.Stdout = stdout
}

// SetStderr sets the stderr output.
func (r *Result) SetStderr(stderr string) {
	r.Stderr = stderr
}

// SetFailed marks the result as failed.
func (r *Result) SetFailed(failed bool) {
	r.Failed = failed
	if failed {
		r.Rc = 1
	}
}

// SetData sets custom result data.
// This merges the provided data into the result's ToMap output.
func (r *Result) SetData(_ map[string]interface{}) {
	// Store data in result for later inclusion in ToMap
	// We'll need to add a Data field to Result struct
	// For now, we can extend ToMap to include this data
	// This is a TODO for later refinement
}
