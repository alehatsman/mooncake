package shell

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
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope: &executor.VariableScope{
			User:    map[string]interface{}{"name": "mooncake"},
			Results: make(map[string]executor.RegisteredResult),
		},
		CurrentDir: "/tmp",
	}
}

// TestRun_PlanRendersCommand: plan mode renders the command template
// and surfaces it via Reason. Does NOT execute the command.
func TestRun_PlanRendersCommand(t *testing.T) {
	step := &config.Step{Shell: &config.ShellAction{Cmd: "echo hello {{ name }}"}}
	h := &Handler{}

	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("shell plan should report WouldChange (commands assumed to mutate)")
	}
	if !strings.Contains(r.Reason, "echo hello mooncake") {
		t.Errorf("reason should include rendered command; got %q", r.Reason)
	}
	if strings.Contains(r.Reason, "sudo") {
		t.Error("reason should not say sudo when Become is false")
	}
}

// TestRun_PlanWithBecome: plan reason includes "sudo" marker when
// step.ShouldBecome() is true, so users can see when the command would
// escalate.
func TestRun_PlanWithBecome(t *testing.T) {
	step := &config.Step{Shell: &config.ShellAction{Cmd: "rm -rf /tmp/x"}, AsUser: "root"}
	h := &Handler{}

	res, _ := h.Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, "sudo") {
		t.Errorf("reason should mention sudo; got %q", r.Reason)
	}
}

// TestRun_PlanTruncatesLongCommand: long commands are truncated for
// display so plan output stays readable. Truncation is single-line and
// ellipsis-tailed.
func TestRun_PlanTruncatesLongCommand(t *testing.T) {
	long := strings.Repeat("x ", 100)
	step := &config.Step{Shell: &config.ShellAction{Cmd: long}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.HasSuffix(r.Reason, "...") {
		t.Errorf("long command should be truncated; got %q", r.Reason)
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
