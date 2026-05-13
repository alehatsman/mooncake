package service

import (
	"runtime"
	"strings"
	"testing"

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
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger: logger.NewLogger(logger.ErrorLevel),
			Mode: planMode(plan),
			Stats: executor.NewExecutionStats(),
		},
		Scope: executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

// TestRun_RestartAlwaysWouldChange (Linux): restart/reload are
// non-idempotent operations, so plan always reports would-change for
// these states.
func TestRun_RestartAlwaysWouldChange(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only inspection path")
	}
	step := &config.Step{
		OsService: &config.ServiceAction{
			Name:  "definitely-not-a-real-service-xyz",
			State: "restarted",
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("restart plan should report WouldChange; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "restart") {
		t.Errorf("reason should mention restart; got %q", r.Reason)
	}
}

// TestRun_NonLinux_NotCheckable: on non-Linux platforms, plan is
// honest about not being able to inspect (no launchd/Windows
// implementation yet).
func TestRun_NonLinux_NotCheckable(t *testing.T) {
	if runtime.GOOS == "linux" {
		t.Skip("non-Linux path")
	}
	step := &config.Step{OsService: &config.ServiceAction{Name: "x", State: "started"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.Checkable {
		t.Error("non-Linux should report not-checkable")
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}
