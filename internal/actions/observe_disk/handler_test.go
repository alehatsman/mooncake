package observe_disk

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
	if err := h.Validate(&config.Step{ObserveDisk: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveDisk: &config.ObserveDisk{}}); err != nil {
		t.Fatalf("expected no error for empty path (defaults to /): %v", err)
	}
}

func TestRun_RootPath(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveDisk: &config.ObserveDisk{Path: "/tmp"}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true for /tmp; data=%v", data)
	}
	val, _ := data["value"].(map[string]any)
	if tot, _ := val["total_bytes"].(float64); tot <= 0 {
		t.Errorf("total_bytes must be > 0; got %v", val["total_bytes"])
	}
}

func TestRun_BadPath(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveDisk: &config.ObserveDisk{Path: "/no-such-path-xyz-12345"}}
	res, err := h.Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for nonexistent path")
	}
	if errStr, _ := data["error"].(string); errStr == "" {
		t.Errorf("expected error message for bad path")
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	res, err := h.Run(newCtx(t, true), &config.Step{ObserveDisk: &config.ObserveDisk{Path: "/"}})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}
