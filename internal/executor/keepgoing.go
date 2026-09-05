package executor

import (
	"fmt"
	"strings"
)

// DeferredFailure is a step that failed under --keep-going: the run
// carried on past it, and the failure is reported when the run ends.
type DeferredFailure struct {
	StepName string
	Err      error
}

// DeferredFailuresError is the run's error when --keep-going let it
// finish past one or more failures. The run still exits non-zero — the
// flag changes *when* you hear about a failure, never *whether*.
type DeferredFailuresError struct {
	Failures []DeferredFailure
}

func (e *DeferredFailuresError) Error() string {
	if len(e.Failures) == 1 {
		return fmt.Sprintf("1 step failed (run continued because --keep-going): %s: %v",
			e.Failures[0].StepName, e.Failures[0].Err)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%d steps failed (run continued because --keep-going):", len(e.Failures))
	for _, f := range e.Failures {
		fmt.Fprintf(&b, "\n  • %s: %v", f.StepName, f.Err)
	}
	return b.String()
}

// Unwrap exposes the first failure so errors.Is/As against a specific
// cause still works on a multi-failure run.
func (e *DeferredFailuresError) Unwrap() error {
	if len(e.Failures) == 0 {
		return nil
	}
	return e.Failures[0].Err
}
