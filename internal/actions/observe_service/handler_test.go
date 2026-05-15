package observe_service

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

func TestValidate_RequiresName(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObserveService: nil}); err == nil {
		t.Fatal("expected error for nil")
	}
	if err := h.Validate(&config.Step{ObserveService: &config.ObserveService{}}); err == nil {
		t.Fatal("expected error for empty name")
	}
	if err := h.Validate(&config.Step{ObserveService: &config.ObserveService{Name: "nginx", Manager: "magic"}}); err == nil {
		t.Fatal("expected error for unknown manager")
	}
	if err := h.Validate(&config.Step{ObserveService: &config.ObserveService{Name: "nginx"}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_NonexistentService_NotFound(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveService: &config.ObserveService{Name: "definitely-not-a-real-service-xyz.service"}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for nonexistent service")
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveService: &config.ObserveService{Name: "anything"}}
	ctx := newCtx(t, true)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
}

func TestParseProps(t *testing.T) {
	in := "LoadState=loaded\nActiveState=active\nSubState=running\nUnitFileState=enabled\n"
	got := parseProps(in)
	if got["LoadState"] != "loaded" || got["ActiveState"] != "active" || got["SubState"] != "running" || got["UnitFileState"] != "enabled" {
		t.Errorf("parseProps misparsed: %v", got)
	}
}
