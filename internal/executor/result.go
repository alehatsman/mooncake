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

	// WouldChange indicates that a plan-mode (non-mutating) inspection
	// predicts the step would change the system if executed.
	// Set by handlers running in ModePlan (Spec 16). Mirrors today's
	// CheckResult.WouldChange and is the eventual replacement.
	WouldChange bool `json:"would_change,omitempty"`

	// Reason is a short human-readable description of the result, e.g.
	// "would create directory", "already matches", "content differs".
	// Populated alongside WouldChange and Changed.
	Reason string `json:"reason,omitempty"`

	// Checkable indicates whether the action supports plan-mode inspection.
	// False for actions like shell where no prediction is possible (the
	// command must run to know its effect). Set by handlers in ModePlan.
	Checkable bool `json:"checkable,omitempty"`

	// Skipped is reserved for future use to indicate skipped steps.
	// Currently not set by any step type.
	Skipped bool `json:"skipped"`

	// Data holds custom result data set by actions via SetData.
	// This allows actions to provide additional structured information
	// that can be accessed in templates and registered results.
	Data map[string]interface{} `json:"data,omitempty"`

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
	m := map[string]interface{}{
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
		"rc":          r.Rc,
		"failed":      r.Failed,
		"changed":     r.Changed,
		"skipped":     r.Skipped,
		"duration_ms": r.Duration.Milliseconds(),
		"status":      r.Status(),
	}

	// Merge custom data fields into the map
	if r.Data != nil {
		for k, v := range r.Data {
			m[k] = v
		}
	}

	return m
}

// RegisterTo registers this result to the variables map under the given name.
// The result can be accessed using nested field syntax (e.g., "result.stdout", "result.rc") in templates and when conditions.
func (r *Result) RegisterTo(variables map[string]interface{}, name string) {
	variables[name] = r.ToMap()
}

// RegisteredResult is a snapshot of a Result stored in VariableScope.Results.
// It is a flat copy — no pointer aliasing — so the scope can be safely cloned.
type RegisteredResult struct {
	Stdout     string
	Stderr     string
	Rc         int
	Failed     bool
	Changed    bool
	Skipped    bool
	DurationMs int64
	Data       map[string]interface{}
}

// ToRegisteredResult converts a *Result into a RegisteredResult snapshot.
func (r *Result) ToRegisteredResult() RegisteredResult {
	var data map[string]interface{}
	if r.Data != nil {
		data = make(map[string]interface{}, len(r.Data))
		for k, v := range r.Data {
			data[k] = v
		}
	}
	return RegisteredResult{
		Stdout:     r.Stdout,
		Stderr:     r.Stderr,
		Rc:         r.Rc,
		Failed:     r.Failed,
		Changed:    r.Changed,
		Skipped:    r.Skipped,
		DurationMs: r.Duration.Milliseconds(),
		Data:       data,
	}
}

// ToMap converts a RegisteredResult to map[string]interface{} for template engines.
func (r RegisteredResult) ToMap() map[string]interface{} {
	m := map[string]interface{}{
		"stdout":      r.Stdout,
		"stderr":      r.Stderr,
		"rc":          r.Rc,
		"failed":      r.Failed,
		"changed":     r.Changed,
		"skipped":     r.Skipped,
		"duration_ms": r.DurationMs,
	}
	for k, v := range r.Data {
		m[k] = v
	}
	return m
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
// This merges the provided data into the result's ToMap output,
// allowing actions to provide additional structured information.
func (r *Result) SetData(data map[string]interface{}) {
	if r.Data == nil {
		r.Data = make(map[string]interface{})
	}
	for k, v := range data {
		r.Data[k] = v
	}
}
