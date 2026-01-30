package executor

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
}

// NewResult creates a new Result with default values
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

// ToMap converts Result to a map for use in template variables
func (r *Result) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"stdout":  r.Stdout,
		"stderr":  r.Stderr,
		"rc":      r.Rc,
		"failed":  r.Failed,
		"changed": r.Changed,
		"skipped": r.Skipped,
	}
}

// RegisterTo registers this result to the variables map under the given name.
// The result can be accessed using nested field syntax (e.g., "result.stdout", "result.rc")
// in templates and when conditions.
func (r *Result) RegisterTo(variables map[string]interface{}, name string) {
	variables[name] = r.ToMap()
}
