package kernel

import (
	"errors"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/apply"
)

// TestMapCancelExit_CancelledOnly_Maps130 pins the proposal-02 exit
// code aggregation rule: cancelled>0 AND failed=0 → 130 regardless
// of which sentinel the Runner returned. Covers the non-SIGINT cancel
// paths (timeout, fleet kill, programmatic ctx.Cancel) that
// runWithSignalCtx's signal handler doesn't catch.
func TestMapCancelExit_CancelledOnly_Maps130(t *testing.T) {
	kr := &apply.KernelResult{
		Summary: apply.RunSummary{
			Cancelled: 1,
			Failed:    0,
		},
	}
	got := mapCancelExit(kr, errors.New("context canceled"))
	coder, ok := got.(cli.ExitCoder)
	if !ok {
		t.Fatalf("expected cli.ExitCoder; got %T (%v)", got, got)
	}
	if coder.ExitCode() != 130 {
		t.Errorf("exit code = %d, want 130", coder.ExitCode())
	}
}

// TestMapCancelExit_CancelledPlusFailed_PassesThrough guards the
// "real failure happened mid-cancel" case — if any step actually
// failed, the failure outranks cancellation and the err return wins
// (CLI exits 1). Proposal-02 §Exit code aggregation: failed>0 → 1
// takes precedence.
func TestMapCancelExit_CancelledPlusFailed_PassesThrough(t *testing.T) {
	kr := &apply.KernelResult{
		Summary: apply.RunSummary{
			Cancelled: 1,
			Failed:    2,
		},
	}
	runErr := errors.New("step exploded")
	got := mapCancelExit(kr, runErr)
	if got != runErr {
		t.Errorf("got %v, want passthrough of runErr", got)
	}
}

// TestMapCancelExit_NilResult_PassesThrough covers the catastrophic-
// setup-error path: apply.Runner.Run can return (nil, err) when it
// fails before the executor even spins up (plan compile error, vars
// file unreadable). No envelope to inspect; err must propagate
// untouched.
func TestMapCancelExit_NilResult_PassesThrough(t *testing.T) {
	runErr := errors.New("config invalid")
	got := mapCancelExit(nil, runErr)
	if got != runErr {
		t.Errorf("got %v, want passthrough of runErr on nil KernelResult", got)
	}
}

// TestMapCancelExit_CleanRun_PassesNil pins the happy path: no
// cancellation, no failure, runErr is nil. mapCancelExit must not
// fabricate an exit code.
func TestMapCancelExit_CleanRun_PassesNil(t *testing.T) {
	kr := &apply.KernelResult{Summary: apply.RunSummary{}}
	if got := mapCancelExit(kr, nil); got != nil {
		t.Errorf("got %v, want nil on clean run", got)
	}
}
