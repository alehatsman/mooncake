package executor

import (
	"errors"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
)

// fakeRawRunner is the minimal RawRunner shape needed to drive
// dispatchRunner: a Run method (mandated by actions.Runner) plus a
// RunRaw method so the executor takes the spec-69 RawRunner branch.
type fakeRawRunner struct {
	result *Result
	err    error
	calls  int
}

func (f *fakeRawRunner) Run(_ actions.Context, _ *config.Step) (actions.Result, error) {
	f.calls++
	return f.result, f.err
}

func (f *fakeRawRunner) RunRaw(ctx actions.Context, step *config.Step) (actions.Result, error) {
	return f.Run(ctx, step)
}

func dispatchTestContext() *ExecutionContext {
	return &ExecutionContext{
		Svc: &RunServices{
			Evaluator: expression.NewGovaluateEvaluator(),
			Template:  mustNewRenderer(),
			Logger:    logger.NewTestLogger(),
			Mode:      actions.ModeApply,
		},
		Scope: NewVariableScope(),
	}
}

// TestDispatchRunner_RawRunnerErrorPropagatesWithoutFailedWhen guards
// spec-69-followups B0. Before the fix, dispatchRunner's
// "failed_when masked the failure" branch fired whenever a RawRunner
// returned (non-nil *Result with Failed=false, non-nil err) — even
// with no failed_when set on the step. Every spec-69 phase-5
// migrated handler (os.user, os.cron, pkg.upgrade, …) returns errors
// that shape because they construct the Result up-front and don't
// flip result.Failed=true on error. The bug silently reported every
// such failure as ok=1 changed=0.
//
// The fix gates the err=nil clear on step.FailedWhen != "". This
// test pins the contract: a RawRunner err MUST propagate when no
// failed_when is set, regardless of the handler-set Result.Failed
// bit.
func TestDispatchRunner_RawRunnerErrorPropagatesWithoutFailedWhen(t *testing.T) {
	runner := &fakeRawRunner{
		result: NewResult(), // Failed=false by default — the shape that triggered B0
		err:    errors.New("handler-level failure"),
	}
	step := config.Step{}

	ec := dispatchTestContext()
	err := dispatchRunner(step, ec, runner)

	if err == nil {
		t.Fatal("dispatchRunner cleared err to nil despite no failed_when set; spec-69 B0 regression")
	}
	if !errors.Is(err, runner.err) {
		t.Errorf("dispatchRunner returned wrapped err = %v, want underlying %v", err, runner.err)
	}
	if runner.calls != 1 {
		t.Errorf("RunRaw called %d times, want 1 (no retry policy in step)", runner.calls)
	}
}

// TestDispatchRunner_RawRunnerErrorMaskedByFailedWhenFalse pins the
// MT-48 invariant in the executor pipeline: when step.FailedWhen
// evaluates false, the RawRunner err is suppressed and the step is
// reported as success. This is the path the B0 fix MUST NOT break.
func TestDispatchRunner_RawRunnerErrorMaskedByFailedWhenFalse(t *testing.T) {
	runner := &fakeRawRunner{
		result: NewResult(),
		err:    errors.New("handler-level failure"),
	}
	step := config.Step{FailedWhen: "false"}

	ec := dispatchTestContext()
	err := dispatchRunner(step, ec, runner)

	if err != nil {
		t.Fatalf("failed_when:false should mask handler err; got %v", err)
	}
}

// TestDispatchRunner_RawRunnerErrorPromotedByFailedWhenTrue covers
// the inverse: a clean handler outcome (result.Failed=false, err=nil)
// that the operator's failed_when expression marks as a failure
// post-hoc. The branch synthesizes "step failed (failed_when=true)"
// when err was nil. This wasn't directly broken by B0 but lives next
// door, so we pin it here too.
func TestDispatchRunner_RawRunnerErrorPromotedByFailedWhenTrue(t *testing.T) {
	runner := &fakeRawRunner{
		result: NewResult(),
		err:    nil,
	}
	step := config.Step{FailedWhen: "true"}

	ec := dispatchTestContext()
	err := dispatchRunner(step, ec, runner)

	if err == nil {
		t.Fatal("failed_when:true should promote a clean outcome to error; got nil")
	}
}

// TestDispatchRunner_RawRunnerNilResultPropagatesErr is the pkg
// handler shape — it returns (nil result, non-nil err) so the
// *Result type assertion fails and the override-clearing branch is
// skipped entirely. This worked before the B0 fix and must keep
// working: it's the reason pkg installs of nonexistent packages have
// always reported failure correctly (see spec-69-followups T8).
func TestDispatchRunner_RawRunnerNilResultPropagatesErr(t *testing.T) {
	runner := &fakeRawRunner{
		result: nil,
		err:    errors.New("nil-result handler failure"),
	}
	step := config.Step{}

	ec := dispatchTestContext()
	err := dispatchRunner(step, ec, runner)

	if err == nil {
		t.Fatal("nil-Result RawRunner err must propagate; got nil")
	}
	if !errors.Is(err, runner.err) {
		t.Errorf("dispatchRunner returned err = %v, want underlying %v", err, runner.err)
	}
}
