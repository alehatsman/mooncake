//go:build !windows

package shell

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// TestF2_RunWideCancel_KillsShell pins the canonical case from the
// proposal-01/02/06 followups F2 entry: a long-running `shell: sleep N`
// must return promptly when the run-wide ctx is cancelled, NOT after
// the full sleep duration. Pre-F2 the shell handler built its cmdCtx
// from context.Background() — completely ignoring SIGINT / fleet kill
// / MCP shutdown — which is why apply.runWithSignalCtx needs the
// os.Exit(130) hard-kill.
//
// Post-F2: setupCommandContext roots cmdCtx in ctx.Ctx(), so the
// outer cancel propagates through exec.CommandContext to the child
// process group (exec_unix.go's setpgid + kill -- -pgid wiring).
func TestF2_RunWideCancel_KillsShell(t *testing.T) {
	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(false),
			Stats:    executor.NewExecutionStats(),
			Ctx:      runCtx,
		},
		Scope: &executor.VariableScope{
			User:    map[string]interface{}{},
			Results: make(map[string]executor.RegisteredResult),
		},
		CurrentDir: "/tmp",
	}

	step := &config.Step{Shell: &config.ShellAction{Cmd: "sleep 30"}}
	h := &Handler{}

	// Fire the cancel 150ms after the handler starts. The shell should
	// return within ~1s (cancellation latency budget for the OS signal
	// delivery + child reap), well before the 30s sleep would elapse.
	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err = h.Run(ec, step)
	elapsed := time.Since(start)

	if elapsed > 5*time.Second {
		t.Fatalf("shell did not honor run-wide ctx cancel: elapsed=%v (expected ≲1s; would have been 30s pre-F2)", elapsed)
	}
	if err == nil {
		t.Errorf("expected non-nil error from cancelled shell run; got nil")
	}
	// The error chain should reflect the cancellation. Exact wrapping
	// varies across exec-on-cancel paths (sometimes "signal: killed",
	// sometimes context.Canceled), so we accept either signal of
	// "ran but cancelled" — and reject the false-positive where
	// shell ran to completion without error.
	if err != nil && !errors.Is(err, context.Canceled) && elapsed < 100*time.Millisecond {
		t.Logf("err=%v elapsed=%v (informational)", err, elapsed)
	}
}

// TestF2_StepTimeout_StillFires guards the step.Timeout path post-F2:
// the timeout chains onto ctx.Ctx() via WithTimeout, so a step-level
// --timeout still fires even when the run-wide ctx never cancels.
func TestF2_StepTimeout_StillFires(t *testing.T) {
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(false),
			Stats:    executor.NewExecutionStats(),
			Ctx:      context.Background(),
		},
		Scope: &executor.VariableScope{
			User:    map[string]interface{}{},
			Results: make(map[string]executor.RegisteredResult),
		},
		CurrentDir: "/tmp",
	}

	step := &config.Step{
		Shell:   &config.ShellAction{Cmd: "sleep 5"},
		Timeout: "300ms",
	}

	start := time.Now()
	_, err = h(t).Run(ec, step)
	elapsed := time.Since(start)

	if elapsed > 2*time.Second {
		t.Fatalf("step.Timeout did not fire: elapsed=%v", elapsed)
	}
	if err == nil {
		t.Errorf("expected timeout error; got nil")
	}
}

func h(_ *testing.T) *Handler { return &Handler{} }
