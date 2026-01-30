package executor

// Result represents the result of executing a step
type Result struct {
	// Stdout contains the standard output from the command
	Stdout string `json:"stdout"`

	// Stderr contains the standard error from the command
	Stderr string `json:"stderr"`

	// Rc is the return code (exit status) of the command
	Rc int `json:"rc"`

	// Failed indicates whether the step failed
	Failed bool `json:"failed"`

	// Changed indicates whether the step made changes (always true for now)
	Changed bool `json:"changed"`

	// Skipped indicates whether the step was skipped
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
