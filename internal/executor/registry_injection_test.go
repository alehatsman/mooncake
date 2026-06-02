package executor_test

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

// recordingHandler is a minimal typed Handler that records whether its Run
// was invoked. It registers under a caller-chosen action name so a test can
// shadow a built-in discriminator and prove which registry the executor
// dispatched against.
type recordingHandler struct {
	name string
	ran  *bool
}

func (h recordingHandler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{
		Name:        h.name,
		Description: "recording test handler",
		Category:    actions.CategoryOutput,
	}
}

func (h recordingHandler) Validate(*config.Step) error { return nil }

func (h recordingHandler) Run(_ actions.Context, _ *config.Step) (actions.Result, error) {
	*h.ran = true
	return executor.NewResult(), nil
}

// TestDispatchStepAction_HonorsInjectedRegistry is the executor half of the
// #105 registry-as-dependency unlock: the executor must resolve handlers
// against the registry injected on RunServices, not the process-wide global.
//
// We register a custom handler under the "shell" discriminator into a FRESH
// registry that does NOT contain the real shell handler. If dispatch routed
// through the global, the real shell handler would run (and our recorder
// would stay false). Because the executor reads ec.ActionRegistry(), our
// injected handler runs instead.
func TestDispatchStepAction_HonorsInjectedRegistry(t *testing.T) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	ran := false
	reg := actions.NewRegistry()
	if err := reg.Register(recordingHandler{name: "shell", ran: &ran}); err != nil {
		t.Fatalf("register: %v", err)
	}

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:   logger.NewTestLogger(),
			Template: renderer,
			Redactor: security.NewRedactor(),
			Registry: reg,
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: t.TempDir(),
	}

	// step.Shell sets the discriminator to "shell"; the injected registry
	// resolves that to our recorder rather than the real shell handler.
	step := config.Step{Shell: &config.ShellAction{Cmd: "echo should-not-run"}}
	if err := executor.DispatchStepAction(step, ec); err != nil {
		t.Fatalf("DispatchStepAction: %v", err)
	}

	if !ran {
		t.Fatal("executor dispatched to the global registry, not the injected one")
	}
}

// TestActionRegistry_FallsBackToGlobal documents the nil-fallback contract:
// an ExecutionContext with no injected registry resolves against the global,
// keeping every existing (CLI/MCP/fleet) caller unchanged.
func TestActionRegistry_FallsBackToGlobal(t *testing.T) {
	ec := &executor.ExecutionContext{Svc: &executor.RunServices{}}
	if ec.ActionRegistry() != actions.GlobalRegistry() {
		t.Fatal("nil RunServices.Registry must fall back to the global registry")
	}

	reg := actions.NewRegistry()
	ec.Svc.Registry = reg
	if ec.ActionRegistry() != reg {
		t.Fatal("injected RunServices.Registry must be returned verbatim")
	}
}
