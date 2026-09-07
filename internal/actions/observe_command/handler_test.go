package observe_command

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
			Ctx:      context.Background(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: t.TempDir(),
	}
}

func run(t *testing.T, ctx *executor.ExecutionContext, o *config.ObserveCommand) (actions.Result, error) {
	t.Helper()
	h := &Handler{}
	step := &config.Step{ObserveCommand: o}
	if err := h.Validate(step); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return h.Run(ctx, step)
}

// PublishObservation flattens the envelope into Data, so the universal fields
// live at the top level.
func found(t *testing.T, res actions.Result) bool {
	t.Helper()
	r, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	f, ok := r.Data["found"].(bool)
	if !ok {
		t.Fatalf("no `found` flag in result data: %+v", r.Data)
	}
	return f
}

func value(t *testing.T, res actions.Result) map[string]any {
	t.Helper()
	r := res.(*executor.Result)
	v, ok := r.Data["value"].(map[string]any)
	if !ok {
		t.Fatalf("no typed value in result data: %+v", r.Data)
	}
	return v
}

func TestRun_ZeroExitIsFound(t *testing.T) {
	res, err := run(t, newCtx(t, false), &config.ObserveCommand{Cmd: "true"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !found(t, res) {
		t.Error("exit 0 with the default expect_exit should be Found")
	}
}

// The exit status IS the data. A non-zero exit must not fail the step, or the
// action is useless for "is this service healthy yet?".
func TestRun_NonZeroExitIsDataNotFailure(t *testing.T) {
	res, err := run(t, newCtx(t, false), &config.ObserveCommand{Cmd: "exit 3"})
	if err != nil {
		t.Fatalf("a non-zero exit must not fail the step: %v", err)
	}
	if found(t, res) {
		t.Error("exit 3 should not be Found when expect_exit is 0")
	}
	if got := value(t, res)["exit_code"]; got != float64(3) && got != 3 {
		t.Errorf("exit_code = %v, want 3", got)
	}
}

func TestRun_ExpectExit(t *testing.T) {
	res, err := run(t, newCtx(t, false), &config.ObserveCommand{Cmd: "exit 42", ExpectExit: 42})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !found(t, res) {
		t.Error("exit 42 with expect_exit: 42 should be Found")
	}
}

func TestRun_CapturesOutput(t *testing.T) {
	res, err := run(t, newCtx(t, false),
		&config.ObserveCommand{Cmd: "echo out; echo err >&2"})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	v := value(t, res)
	if v["stdout"] != "out\n" {
		t.Errorf("stdout = %q, want %q", v["stdout"], "out\n")
	}
	if v["stderr"] != "err\n" {
		t.Errorf("stderr = %q, want %q", v["stderr"], "err\n")
	}
}

// A command that cannot start at all IS a probe failure, so it sets Error —
// unlike a command that runs and exits non-zero.
func TestRun_UnstartableCommandSetsError(t *testing.T) {
	res, err := run(t, newCtx(t, false), &config.ObserveCommand{
		Cmd: "definitely-not-a-real-binary-xyz", ExpectExit: 0,
	})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	// bash itself starts fine and exits 127, so this is still an observation.
	if found(t, res) {
		t.Error("a missing binary should not be Found")
	}
	if got := value(t, res)["exit_code"]; got != float64(127) && got != 127 {
		t.Errorf("exit_code = %v, want 127 (bash's not-found code)", got)
	}
}

func TestRun_WaitSucceedsWhenCommandStartsPassing(t *testing.T) {
	ctx := newCtx(t, false)
	flag := filepath.Join(t.TempDir(), "ready")

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(flag, []byte("x"), 0o644)
	}()

	if _, err := run(t, ctx, &config.ObserveCommand{
		Cmd:  "test -f " + flag,
		Wait: &config.WaitSpec{For: "5s", Interval: "5ms"},
	}); err != nil {
		t.Fatalf("wait should have succeeded once the command started passing: %v", err)
	}
}

func TestRun_WaitTimeoutFailsTheStep(t *testing.T) {
	res, err := run(t, newCtx(t, false), &config.ObserveCommand{
		Cmd:  "false",
		Wait: &config.WaitSpec{For: "20ms", Interval: "5ms"},
	})
	if err == nil {
		t.Fatal("a wait that never satisfies must fail the step")
	}
	if res == nil {
		t.Fatal("a timed-out wait should still return its last observation")
	}
	if found(t, res) {
		t.Error("timed-out wait should report Found=false")
	}
}

// This handler shells out, so plan mode must not run the command at all —
// deferring is not an optimization here, it is the difference between a plan
// and a side effect.
func TestRun_PlanModeDoesNotRunTheCommand(t *testing.T) {
	ctx := newCtx(t, true)
	marker := filepath.Join(t.TempDir(), "should-not-exist")

	res, err := run(t, ctx, &config.ObserveCommand{Cmd: "touch " + marker})
	if err != nil {
		t.Fatalf("plan mode should not fail: %v", err)
	}
	if _, statErr := os.Stat(marker); statErr == nil {
		t.Error("plan mode ran the command; it must defer")
	}
	if r := res.(*executor.Result); r.Reason == "" {
		t.Error("plan mode should explain what it would observe")
	}
}

func TestValidate(t *testing.T) {
	h := &Handler{}
	t.Run("missing config", func(t *testing.T) {
		if err := h.Validate(&config.Step{}); err == nil {
			t.Error("expected an error with no config")
		}
	})
	t.Run("blank cmd", func(t *testing.T) {
		step := &config.Step{ObserveCommand: &config.ObserveCommand{Cmd: "   "}}
		if err := h.Validate(step); err == nil {
			t.Error("expected an error for a whitespace-only cmd")
		}
	})
	t.Run("bad wait condition", func(t *testing.T) {
		step := &config.Step{ObserveCommand: &config.ObserveCommand{
			Cmd: "true", Wait: &config.WaitSpec{Until: "eventually"},
		}}
		if err := h.Validate(step); err == nil {
			t.Error("expected an error for an unknown wait condition")
		}
	})
}

func TestHandlerIsNotAReverser(t *testing.T) {
	var h any = &Handler{}
	if _, ok := h.(actions.Reverser); ok {
		t.Error("observe.command implements actions.Reverser; an observation has nothing to undo")
	}
}

// Every other observe handler reports Risk 1. This one runs an arbitrary
// command, and the ABI should say so rather than flatten it into "pure read".
func TestCost_RisksMoreThanAPureRead(t *testing.T) {
	h := &Handler{}
	est, err := h.Cost(nil, &config.Step{})
	if err != nil {
		t.Fatalf("Cost: %v", err)
	}
	if est.Risk <= 1 {
		t.Errorf("Risk = %d; running an arbitrary command is riskier than a pure read", est.Risk)
	}
	if est.Reversible {
		t.Error("Reversible should be false")
	}
}
