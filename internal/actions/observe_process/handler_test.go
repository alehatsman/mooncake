package observe_process

import (
	"os"
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

func TestValidate_RequiresSelector(t *testing.T) {
	h := &Handler{}
	if err := h.Validate(&config.Step{ObserveProcess: nil}); err == nil {
		t.Fatal("expected error for nil config")
	}
	if err := h.Validate(&config.Step{ObserveProcess: &config.ObserveProcess{}}); err == nil {
		t.Fatal("expected error for empty selector")
	}
	if err := h.Validate(&config.Step{ObserveProcess: &config.ObserveProcess{Pattern: "[invalid"}}); err == nil {
		t.Fatal("expected error for bad regex")
	}
	if err := h.Validate(&config.Step{ObserveProcess: &config.ObserveProcess{Name: "go"}}); err != nil {
		t.Fatalf("expected no error: %v", err)
	}
}

func TestRun_SelfProcess_Found(t *testing.T) {
	// `go test` runs as a process whose argv[0] basename is the test
	// binary. Find it by reading /proc/self/comm or argv0; on macOS
	// fall through to ps. Either way, the *current* process exists.
	exe, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable unavailable: %v", err)
	}
	// Use the binary basename as a name selector — guaranteed to match.
	basename := exe
	for i := len(exe) - 1; i >= 0; i-- {
		if exe[i] == '/' {
			basename = exe[i+1:]
			break
		}
	}

	h := &Handler{}
	step := &config.Step{ObserveProcess: &config.ObserveProcess{Name: basename}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); !found {
		t.Errorf("expected found=true for self process %q; got data=%v", basename, data)
	}
}

func TestRun_NoMatch_NotFound(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveProcess: &config.ObserveProcess{Name: "definitely-not-a-real-process-name-xyz-12345"}}
	ctx := newCtx(t, false)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("expected found=false for nonexistent process")
	}
}

func TestRun_PlanMode_Defers(t *testing.T) {
	h := &Handler{}
	step := &config.Step{ObserveProcess: &config.ObserveProcess{Name: "anything"}}
	ctx := newCtx(t, true)
	res, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	data := res.(*executor.Result).Data
	if found, _ := data["found"].(bool); found {
		t.Errorf("plan-mode Found must be false")
	}
	if errStr, _ := data["error"].(string); errStr == "" {
		t.Errorf("plan-mode Error must explain the defer")
	}
}

func TestPermissions_ReadOnly(t *testing.T) {
	h := &Handler{}
	p := h.Permissions(&config.Step{})
	if p.Sudo {
		t.Errorf("observe.process should not require Sudo")
	}
	if p.Network {
		t.Errorf("observe.process should not flag Network")
	}
}
