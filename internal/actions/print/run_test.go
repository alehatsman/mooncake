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
		Variables:  map[string]interface{}{"who": "world"},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		DryRun:     plan,
		Stats:      executor.NewExecutionStats(),
	}
}

// TestRun_PlanRendersMessage: plan renders the print message template
// and surfaces it in Reason. Does not actually print.
func TestRun_PlanRendersMessage(t *testing.T) {
	step := &config.Step{Print: &config.PrintAction{Msg: "hello {{ who }}"}}
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
	step := &config.Step{Print: &config.PrintAction{Msg: "first\nsecond\n"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "first") || strings.Contains(r.Reason, "second") {
		t.Errorf("preview should be first line only; got %q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}
