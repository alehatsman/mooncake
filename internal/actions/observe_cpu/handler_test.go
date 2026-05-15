package observe_cpu

import (
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
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func TestValidate(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObserveCPU: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveCPU: &config.ObserveCPU{}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_PopulatesCoreCount(t *testing.T) {
	h := &Handler{}
	res, err := h.Run(newCtx(t, false), &config.Step{ObserveCPU: &config.ObserveCPU{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	val, _ := data["value"].(map[string]any)
	cores, _ := val["cores"].(float64)
	if int(cores) <= 0 {
		t.Errorf("expected cores > 0; got %v", val["cores"])
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	res, err := h.Run(newCtx(t, true), &config.Step{ObserveCPU: &config.ObserveCPU{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}
