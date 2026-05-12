package package_handler

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
		Variables:  map[string]interface{}{},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		CurrentMode: planMode(plan),
		Stats:      executor.NewExecutionStats(),
	}
}

// TestRun_UpgradeAlwaysWouldChange: upgrade isn't idempotent at the
// package-list level — we can't know without running whether anything
// would update. Plan always reports would-change.
func TestRun_UpgradeAlwaysWouldChange(t *testing.T) {
	step := &config.Step{
		Package: &config.Package{
			Manager: "pacman", // explicit so plan doesn't have to auto-detect
			Upgrade: true,
		},
	}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("upgrade plan should report WouldChange; got %+v", r)
	}
	if !strings.Contains(r.Reason, "upgrade") {
		t.Errorf("reason should mention upgrade; got %q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeExecute
}
