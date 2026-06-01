package executor_test

// Issue #73: step.failed must carry the real process exit code for
// shell and cmd actions. Both actions return a plain fmt.Errorf (not a
// *CommandError), and applyResultOverrides / runWithRetry replace the
// handler's error before handleStepError sees it — so the exit code can
// only be recovered from the Result's Rc, which survives that path.
//
// Pre-fix both `shell: exit 7` and `cmd: sh -c "exit 7"` reported
// exit_code -1; the equivalent failed_when-on-clean-exit case must still
// report -1 (issue #21: don't fabricate a code for a command that
// exited 0).

import (
	"testing"

	_ "github.com/alehatsman/mooncake/internal/actions/command"
	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/expression"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/security"
	"github.com/alehatsman/mooncake/internal/template"
)

func captureStepFailedExitCode(t *testing.T, step config.Step) (int, bool) {
	t.Helper()

	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	publisher := events.NewSyncPublisher()
	var exitCode int
	var seen bool
	publisher.Subscribe(&capturingSubscriber{
		onEvent: func(e events.Event) {
			if e.Type != events.EventStepFailed {
				return
			}
			if d, ok := e.Data.(events.StepFailedData); ok {
				exitCode = d.ExitCode
				seen = true
			}
		},
	})

	ec := &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:         logger.NewTestLogger(),
			Template:       renderer,
			Evaluator:      expression.NewGovaluateEvaluator(),
			PathUtil:       pathutil.NewPathExpander(renderer),
			Stats:          executor.NewExecutionStats(),
			Redactor:       security.NewRedactor(),
			EventPublisher: publisher,
		},
		Scope: executor.NewVariableScope(),
	}

	// ExecuteStep returns the step error for a non-continue_on_error
	// failure; that's expected here — we only care about the emitted
	// step.failed event.
	_ = executor.ExecuteStep(step, ec)
	return exitCode, seen
}

func TestStepFailed_ShellExitCode(t *testing.T) {
	got, seen := captureStepFailedExitCode(t, config.Step{
		Name:  "boom",
		Shell: &config.ShellAction{Cmd: "exit 7"},
	})
	if !seen {
		t.Fatal("no step.failed event emitted")
	}
	if got != 7 {
		t.Errorf("shell exit 7: step.failed exit_code = %d, want 7", got)
	}
}

func TestStepFailed_CmdExitCode(t *testing.T) {
	got, seen := captureStepFailedExitCode(t, config.Step{
		Name: "boom",
		Cmd:  &config.CommandAction{Argv: []string{"sh", "-c", "exit 7"}},
	})
	if !seen {
		t.Fatal("no step.failed event emitted")
	}
	if got != 7 {
		t.Errorf("cmd exit 7: step.failed exit_code = %d, want 7", got)
	}
}

func TestStepFailed_FailedWhenCleanExitKeepsSentinel(t *testing.T) {
	// failed_when fires on a command that exited 0: there is no real
	// failing exit code, so the -1 "N/A" sentinel must be preserved
	// rather than fabricating a code (issue #21).
	got, seen := captureStepFailedExitCode(t, config.Step{
		Name:       "clean-but-failed",
		Shell:      &config.ShellAction{Cmd: "exit 0"},
		FailedWhen: "true",
	})
	if !seen {
		t.Fatal("no step.failed event emitted")
	}
	if got != -1 {
		t.Errorf("failed_when on clean exit: step.failed exit_code = %d, want -1 (sentinel)", got)
	}
}
