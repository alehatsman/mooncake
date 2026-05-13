package print

import (
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
		Scope: &executor.VariableScope{
			User:    map[string]interface{}{"who": "world"},
			Results: make(map[string]executor.RegisteredResult),
		},
		CurrentDir: "/tmp",
	}
}

// TestRun_PlanRendersMessage: plan renders the print message template
// and surfaces it in Reason. Does not actually print.
func TestRun_PlanRendersMessage(t *testing.T) {
	step := &config.Step{Log: &config.PrintAction{Msg: "hello {{ who }}"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Error("print should not WouldChange (no state mutation)")
	}
	if !strings.Contains(r.Reason, "hello world") {
		t.Errorf("reason should contain rendered message; got %q", r.Reason)
	}
}

// TestRun_PlanMultilinePreview: multi-line message is condensed to
// the first non-empty line for the preview.
func TestRun_PlanMultilinePreview(t *testing.T) {
	step := &config.Step{Log: &config.PrintAction{Msg: "first\nsecond\n"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "first") || strings.Contains(r.Reason, "second") {
		t.Errorf("preview should be first line only; got %q", r.Reason)
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
