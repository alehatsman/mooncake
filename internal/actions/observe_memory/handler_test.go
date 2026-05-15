package observe_memory

import (
	"runtime"
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
	if err := h.Validate(&config.Step{ObserveMemory: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveMemory: &config.ObserveMemory{}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_HostMemory(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skipf("observe.memory not implemented on %s", runtime.GOOS)
	}
	h := &Handler{}
	res, err := h.Run(newCtx(t, false), &config.Step{ObserveMemory: &config.ObserveMemory{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true on host with /proc or sysctl; data=%v", data)
	}
	val, _ := data["value"].(map[string]any)
	if tot, _ := val["total_bytes"].(float64); tot <= 0 {
		t.Errorf("total_bytes must be > 0; got %v", val["total_bytes"])
	}
	if used, _ := val["used_bytes"].(float64); used < 0 {
		t.Errorf("used_bytes must be >= 0; got %v", val["used_bytes"])
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	res, err := h.Run(newCtx(t, true), &config.Step{ObserveMemory: &config.ObserveMemory{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}
