package artifact_capture

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
		Variables:   map[string]interface{}{},
		Template:    r,
		PathUtil:    pathutil.NewPathExpander(r),
		Logger:      logger.NewLogger(logger.ErrorLevel),
		CurrentDir:  "/tmp",
		CurrentMode: planMode(plan),
		Stats:       executor.NewExecutionStats(),
	}
}

// TestRun_AlwaysWouldChange: artifact_capture is non-idempotent (it
// re-runs its inner steps and re-captures), so plan mode always
// reports would-change with the inner-step count for context.
func TestRun_AlwaysWouldChange(t *testing.T) {
	step := &config.Step{
		ArtifactCapture: &config.ArtifactCapture{
			Name:  "demo",
			Steps: []config.Step{{}, {}, {}},
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("artifact_capture plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, "demo") {
		t.Errorf("reason should include artifact name; got %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "3") {
		t.Errorf("reason should include inner step count; got %q", r.Reason)
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
