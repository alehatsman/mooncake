package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/logger"
)

// makeECWithMode builds a minimal ExecutionContext with a TestLogger and
// the given run mode, sufficient for captureResult.
func makeECWithMode(mode actions.Mode) (*ExecutionContext, *logger.TestLogger) {
	tl := logger.NewTestLogger()
	return &ExecutionContext{
		Svc: &RunServices{
			Logger: tl,
			Mode:   mode,
			Stats:  NewExecutionStats(),
		},
		Scope: NewVariableScope(),
	}, tl
}

// makeRegisteredResult is a tiny helper that wraps a marker value into a
// RegisteredResult, so tests can verify the bind happened.
func makeRegisteredResult(marker string) RegisteredResult {
	r := NewResult()
	r.Stdout = marker
	return r.ToRegisteredResult()
}

// withCaptureInPlan swaps resolveCaptureInPlan for the test's lifetime so
// tests can drive the plan-mode gate without registering fixture actions.
func withCaptureInPlan(t *testing.T, f func(actionType string) bool) {
	t.Helper()
	prev := resolveCaptureInPlan
	resolveCaptureInPlan = f
	t.Cleanup(func() { resolveCaptureInPlan = prev })
}

// TestCaptureResult_CollisionWarnsOnDistinctSteps covers spec-37 test #1.
func TestCaptureResult_CollisionWarnsOnDistinctSteps(t *testing.T) {
	ec, tl := makeECWithMode(actions.ModeApply)

	first := config.Step{ID: "step-0001", Name: "alpha", As: "x",
		Origin: &config.Origin{FilePath: "a.yml", Line: 1, Column: 1}}
	second := config.Step{ID: "step-0002", Name: "beta", As: "x",
		Origin: &config.Origin{FilePath: "a.yml", Line: 9, Column: 1}}

	captureResult(ec, first, makeRegisteredResult("first"))
	captureResult(ec, second, makeRegisteredResult("second"))

	if !tl.Contains(`name "x" overwritten by step step-0002`) {
		t.Errorf("expected collision warning, got logs: %+v", tl.GetLogs())
	}
	if _, ok := ec.Scope.Results["x"]; !ok {
		t.Fatal("expected x to be bound after overwrite")
	}
}

// TestCaptureResult_LoopSiblingsDoNotWarn covers spec-37 test #2.
// Two iterations of the same for_each step share an Origin and have
// LoopContext set — no warning.
func TestCaptureResult_LoopSiblingsDoNotWarn(t *testing.T) {
	ec, tl := makeECWithMode(actions.ModeApply)

	origin := &config.Origin{FilePath: "a.yml", Line: 5, Column: 3}
	iter0 := config.Step{ID: "step-0001", Name: "loop", As: "x",
		Origin: origin, LoopContext: &config.LoopContext{Type: "for_each", Index: 0}}
	iter1 := config.Step{ID: "step-0002", Name: "loop", As: "x",
		Origin: origin, LoopContext: &config.LoopContext{Type: "for_each", Index: 1}}

	captureResult(ec, iter0, makeRegisteredResult("a"))
	captureResult(ec, iter1, makeRegisteredResult("b"))

	if tl.Contains("overwritten") {
		t.Errorf("loop siblings should not warn, got logs: %+v", tl.GetLogs())
	}
}

// TestCaptureResult_MixedLoopThenNonLoopWarns covers spec-37 test #3.
// A for_each iteration writes name x; a subsequent non-loop step overwrites
// x — warning must fire on the non-loop step.
func TestCaptureResult_MixedLoopThenNonLoopWarns(t *testing.T) {
	ec, tl := makeECWithMode(actions.ModeApply)

	loopOrigin := &config.Origin{FilePath: "a.yml", Line: 5, Column: 3}
	standaloneOrigin := &config.Origin{FilePath: "a.yml", Line: 20, Column: 3}

	loopStep := config.Step{ID: "step-0001", Name: "loop", As: "x",
		Origin: loopOrigin, LoopContext: &config.LoopContext{Type: "for_each", Index: 0}}
	bareStep := config.Step{ID: "step-0002", Name: "bare", As: "x",
		Origin: standaloneOrigin}

	captureResult(ec, loopStep, makeRegisteredResult("loop"))
	captureResult(ec, bareStep, makeRegisteredResult("bare"))

	if !tl.Contains(`overwritten by step step-0002`) {
		t.Errorf("expected warning on non-loop overwrite, got logs: %+v", tl.GetLogs())
	}
}

// TestCaptureResult_PlanModeDefaultSkipsBind covers spec-37 test #4.
func TestCaptureResult_PlanModeDefaultSkipsBind(t *testing.T) {
	ec, _ := makeECWithMode(actions.ModePlan)
	withCaptureInPlan(t, func(string) bool { return false })

	step := config.Step{ID: "step-0001", Name: "n", As: "x"}
	captureResult(ec, step, makeRegisteredResult("planned"))

	if _, ok := ec.Scope.Results["x"]; ok {
		t.Error("plan-mode default action must not bind into Scope.Results")
	}
	if _, ok := ec.Scope.ResultOrigins["x"]; ok {
		t.Error("plan-mode default action must not populate ResultOrigins either")
	}
}

// TestCaptureResult_PlanModeCaptureInPlanBinds covers spec-37 test #5.
func TestCaptureResult_PlanModeCaptureInPlanBinds(t *testing.T) {
	ec, _ := makeECWithMode(actions.ModePlan)
	withCaptureInPlan(t, func(string) bool { return true })

	step := config.Step{ID: "step-0001", Name: "n", As: "x"}
	captureResult(ec, step, makeRegisteredResult("planned"))

	if _, ok := ec.Scope.Results["x"]; !ok {
		t.Error("plan-mode opt-in action must bind into Scope.Results")
	}
}

// TestCaptureResult_ApplyModeAlwaysBinds covers spec-37 test #6 (regression).
func TestCaptureResult_ApplyModeAlwaysBinds(t *testing.T) {
	ec, _ := makeECWithMode(actions.ModeApply)
	withCaptureInPlan(t, func(string) bool { return false })

	step := config.Step{ID: "step-0001", Name: "n", As: "x"}
	captureResult(ec, step, makeRegisteredResult("applied"))

	if _, ok := ec.Scope.Results["x"]; !ok {
		t.Error("apply-mode bind must happen regardless of CaptureInPlan")
	}
}

// TestCaptureResult_FailedStepBindsInApplyMode covers spec-37 test #7.
// markStepFailed → captureResult must still publish in apply mode.
func TestCaptureResult_FailedStepBindsInApplyMode(t *testing.T) {
	ec, _ := makeECWithMode(actions.ModeApply)
	withCaptureInPlan(t, func(string) bool { return false })

	step := config.Step{ID: "step-0001", Name: "n", As: "x"}
	result := NewResult()
	markStepFailed(result, step, ec)

	if _, ok := ec.Scope.Results["x"]; !ok {
		t.Error("failed-step write must still happen in apply mode")
	}
	if !result.Failed {
		t.Error("markStepFailed must mark Failed=true")
	}
}

// TestCaptureResult_FailedStepRespectsGateInPlanMode covers spec-37 test #8.
func TestCaptureResult_FailedStepRespectsGateInPlanMode(t *testing.T) {
	t.Run("default off skips", func(t *testing.T) {
		ec, _ := makeECWithMode(actions.ModePlan)
		withCaptureInPlan(t, func(string) bool { return false })

		step := config.Step{ID: "step-0001", As: "x"}
		markStepFailed(NewResult(), step, ec)
		if _, ok := ec.Scope.Results["x"]; ok {
			t.Error("plan-mode failed-step write must be gated out when CaptureInPlan=false")
		}
	})
	t.Run("opt-in binds", func(t *testing.T) {
		ec, _ := makeECWithMode(actions.ModePlan)
		withCaptureInPlan(t, func(string) bool { return true })

		step := config.Step{ID: "step-0002", As: "x"}
		markStepFailed(NewResult(), step, ec)
		if _, ok := ec.Scope.Results["x"]; !ok {
			t.Error("plan-mode failed-step write must happen when CaptureInPlan=true")
		}
	})
}

// TestDryRunLogger_LogRegisterUsesCaptureWording covers spec-37 test #9.
func TestDryRunLogger_LogRegisterUsesCaptureWording(t *testing.T) {
	tl := logger.NewTestLogger()
	d := NewDryRunLogger(tl)
	d.LogRegister(config.Step{As: "pkg_version"})

	if !tl.Contains("Would capture result as: pkg_version") {
		t.Errorf("expected new wording, got logs: %+v", tl.GetLogs())
	}
	if tl.Contains("Would register result as:") {
		t.Errorf("old wording must not appear, got logs: %+v", tl.GetLogs())
	}
}

// TestCaptureResult_EmptyAsIsNoOp covers the empty-As fast path.
func TestCaptureResult_EmptyAsIsNoOp(t *testing.T) {
	ec, _ := makeECWithMode(actions.ModeApply)
	step := config.Step{ID: "step-0001"}
	captureResult(ec, step, makeRegisteredResult("x"))
	if len(ec.Scope.Results) != 0 {
		t.Errorf("expected no bind for empty As, got %d entries", len(ec.Scope.Results))
	}
}

// TestCaptureResult_TwoDistinctLoopsAtDifferentOriginsWarn ensures the
// for_each-sibling exemption is keyed on Origin, not just LoopContext.
// Two loops in different files (or at different lines) writing to the
// same `as:` name must still warn.
func TestCaptureResult_TwoDistinctLoopsAtDifferentOriginsWarn(t *testing.T) {
	ec, tl := makeECWithMode(actions.ModeApply)

	a := config.Step{ID: "step-0001", Name: "loop_a", As: "x",
		Origin:      &config.Origin{FilePath: "a.yml", Line: 5, Column: 3},
		LoopContext: &config.LoopContext{Type: "for_each", Index: 0}}
	b := config.Step{ID: "step-0002", Name: "loop_b", As: "x",
		Origin:      &config.Origin{FilePath: "b.yml", Line: 5, Column: 3},
		LoopContext: &config.LoopContext{Type: "for_each", Index: 0}}

	captureResult(ec, a, makeRegisteredResult("a"))
	captureResult(ec, b, makeRegisteredResult("b"))

	if !tl.Contains(`overwritten by step step-0002`) {
		t.Errorf("distinct loops at different origins must warn, got logs: %+v", tl.GetLogs())
	}
}
