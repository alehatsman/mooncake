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

// The reason Found tracks Active rather than Exists: an installed unit that
// is not running is a real, common state, and it is exactly what a
// `wait: { until: running }` has to keep polling through. If Found meant
// Exists, that wait would return immediately and report success for a
// service that never came up.
func TestSystemdObservation_InstalledButStopped(t *testing.T) {
	const stopped = "LoadState=loaded\n" +
		"ActiveState=inactive\n" +
		"SubState=dead\n" +
		"UnitFileState=enabled\n"
	obs := systemdObservation(stopped)
	if !obs.Exists {
		t.Error("a loaded unit exists")
	}
	if obs.Active {
		t.Error("an inactive unit is not active")
	}
	if !obs.Enabled {
		t.Error("UnitFileState=enabled means enabled")
	}
	if obs.SubState != "dead" {
		t.Errorf("sub_state = %q, want dead", obs.SubState)
	}
}

func TestSystemdObservation_Running(t *testing.T) {
	const running = "LoadState=loaded\n" +
		"ActiveState=active\n" +
		"SubState=running\n" +
		"UnitFileState=enabled\n"
	obs := systemdObservation(running)
	if !obs.Exists || !obs.Active {
		t.Errorf("expected exists+active, got %+v", obs)
	}
}

func TestSystemdObservation_NotFound(t *testing.T) {
	obs := systemdObservation("LoadState=not-found\nActiveState=inactive\nSubState=dead\n")
	if obs.Exists {
		t.Error("LoadState=not-found means the unit does not exist")
	}
	if obs.Active {
		t.Error("a missing unit is not active")
	}
}

func TestValidate_RejectsBadWait(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveService: &config.ObserveService{
		Name: "nginx",
		Wait: &config.WaitSpec{Until: "purple"},
	}}
	if err := h.Validate(step); err == nil {
		t.Fatal("expected an unknown wait condition to fail at validate time")
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
