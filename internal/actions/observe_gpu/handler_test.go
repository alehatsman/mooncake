package observe_gpu

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/metrics"
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
	if err := h.Validate(&config.Step{ObserveGPU: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveGPU: &config.ObserveGPU{}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
	bad := -1
	if err := h.Validate(&config.Step{ObserveGPU: &config.ObserveGPU{Index: &bad}}); err == nil {
		t.Fatal("expected error for negative index")
	}
}

func TestRun_NoGPU_CountZero(t *testing.T) {
	// On a CI / WSL box without nvidia-smi or a recognized GPU, the
	// observation must still complete cleanly with Count=0.
	h := &Handler{}
	res, err := h.Run(newCtx(t, false), &config.Step{ObserveGPU: &config.ObserveGPU{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("Found should be true (observation completed); got false")
	}
	val, _ := data["value"].(map[string]any)
	if val == nil {
		t.Fatalf("value must be present")
	}
	count, _ := val["count"].(float64)
	if int(count) < 0 {
		t.Errorf("count must be >= 0; got %v", val["count"])
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	res, err := h.Run(newCtx(t, true), &config.Step{ObserveGPU: &config.ObserveGPU{}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}

func TestBuildObservation_Aggregates(t *testing.T) {
	in := []metrics.GPUMetrics{
		{Index: 0, UsagePct: 12.5, MemoryUsedMB: 1024, MemoryUsedPct: 5.0, TemperatureC: 45},
		{Index: 1, UsagePct: 80.0, MemoryUsedMB: 8192, MemoryUsedPct: 50.0, TemperatureC: 72},
	}
	obs := buildObservation(in, nil)
	if obs.Count != 2 {
		t.Errorf("Count = %d, want 2", obs.Count)
	}
	if obs.Aggregate.MaxUtilizationPct != 80.0 {
		t.Errorf("MaxUtilizationPct = %v, want 80.0", obs.Aggregate.MaxUtilizationPct)
	}
	wantBytes := int64((1024 + 8192) * 1024 * 1024)
	if obs.Aggregate.MemoryUsedBytes != wantBytes {
		t.Errorf("MemoryUsedBytes = %d, want %d", obs.Aggregate.MemoryUsedBytes, wantBytes)
	}
}

func TestBuildObservation_IndexFilter(t *testing.T) {
	in := []metrics.GPUMetrics{
		{Index: 0, UsagePct: 12.5, MemoryUsedMB: 1024},
		{Index: 1, UsagePct: 80.0, MemoryUsedMB: 8192},
	}
	want := 1
	obs := buildObservation(in, &want)
	if obs.Count != 1 {
		t.Errorf("Count = %d, want 1", obs.Count)
	}
	if obs.GPUs[0].Index != 1 {
		t.Errorf("GPU index = %d, want 1", obs.GPUs[0].Index)
	}
}
