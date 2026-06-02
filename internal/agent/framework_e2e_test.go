package agent

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// frameworkGreetHandler is a custom typed action with no dedicated config.Step
// field. It reads its params off the #111 carrier (step.With) and writes
// With["who"] to the file at With["out"].
type frameworkGreetHandler struct{}

func (frameworkGreetHandler) Metadata() actions.ActionMetadata {
	return actions.ActionMetadata{Name: "demo.greet", Description: "write a greeting (framework e2e)"}
}

func (frameworkGreetHandler) Validate(*config.Step) error { return nil }

func (frameworkGreetHandler) Run(_ actions.Context, step *config.Step) (actions.Result, error) {
	who, _ := step.With["who"].(string)
	out, _ := step.With["out"].(string)
	r := executor.NewResult()
	if err := os.WriteFile(out, []byte(who), 0o644); err != nil {
		r.Failed = true
		return r, err
	}
	r.Changed = true
	return r, nil
}

// TestRunLoop_Framework_CustomBackendAndAction is the end-to-end proof of the
// agent-framework path, fully offline: a consumer injects BOTH a custom
// reasoning backend (RunOptions.LLMClient, #106) AND a custom action registry
// (RunOptions.Registry, #105), and a generated plan that uses the generic
// action:/with: carrier (#111) dispatches to the consumer's handler and runs
// through the real RunLoop — no cloud, no provider env, no built-in field for
// the action. This is the whole framework thesis exercised in one test.
func TestRunLoop_Framework_CustomBackendAndAction(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	sentinel := filepath.Join(repo, "greeting.txt")

	// The injected backend "reasons" by emitting a carrier-form plan for the
	// custom action, then an empty plan to terminate the loop.
	plan := `[{"name":"greet","action":"demo.greet","with":{"who":"world","out":"` + sentinel + `"}}]`
	// One carrier plan that does the work, then empty plans so the loop's
	// no-change streak terminates it cleanly rather than exhausting the stub.
	backend := &stubLLMClient{plans: []string{plan, "[]\n", "[]\n", "[]\n", "[]\n"}}

	reg := actions.GlobalRegistry().Clone()
	if err := reg.Register(frameworkGreetHandler{}); err != nil {
		t.Fatalf("register custom action: %v", err)
	}

	result, err := RunLoop(context.Background(), RunOptions{
		Goal:          "greet the world via a custom action",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true, // skip the TTY gate
		LLMClient:     backend,
		Registry:      reg,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	got, readErr := os.ReadFile(sentinel)
	if readErr != nil {
		t.Fatalf("custom action did not execute through the loop — sentinel missing: %v (stop=%s, iters=%d)",
			readErr, result.StopReason, len(result.Iterations))
	}
	if string(got) != "world" {
		t.Errorf("custom action ran with wrong carrier params: got %q, want %q", got, "world")
	}
	// The backend must have been the injected one (no provider resolution).
	if backend.calls == 0 {
		t.Error("injected LLMClient was never called — backend injection not honored")
	}
}
