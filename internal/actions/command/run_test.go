package command

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
		Variables:  map[string]interface{}{"name": "alice"},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		DryRun:     plan,
		Stats:      executor.NewExecutionStats(),
	}
}

// TestRun_PlanRendersArgv: plan mode renders each argv element and
// joins them for display. Does NOT execute.
func TestRun_PlanRendersArgv(t *testing.T) {
	step := &config.Step{Command: &config.CommandAction{Argv: []string{"useradd", "-m", "{{ name }}"}}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("command plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "useradd -m alice") {
		t.Errorf("reason should include rendered argv; got %q", r.Reason)
	}
}

func TestRun_PlanWithBecome(t *testing.T) {
	step := &config.Step{Command: &config.CommandAction{Argv: []string{"shutdown", "now"}}, Become: true}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "sudo") {
		t.Errorf("reason should mention sudo; got %q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}
