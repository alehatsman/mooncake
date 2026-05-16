package executor_test

// F017 regression: a leaf step with continue_on_error: true that fails
// at dispatch must produce exactly one terminal event (step.failed) and
// must not double-count itself in Stats.Executed / Stats.Failed. Before
// the fix, ExecuteStep called handleStepError (emitting step.failed +
// incrementing Stats.Failed) and then fell through to
// postExecuteSuccess (emitting step.completed + incrementing
// Stats.Executed), so consumers of the event stream saw the step flip
// failed→completed for the same StepID and the recap reported the step
// as both failed and executed.

import (
	"testing"

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

func TestContinueOnError_EmitsSingleTerminalEvent(t *testing.T) {
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}

	publisher := events.NewSyncPublisher()
	var failed, completed int
	publisher.Subscribe(&capturingSubscriber{
		onEvent: func(e events.Event) {
			switch e.Type {
			case events.EventStepFailed:
				failed++
			case events.EventStepCompleted:
				completed++
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

	step := config.Step{
		Name:            "expected to fail",
		Shell:           &config.ShellAction{Cmd: "exit 1"},
		ContinueOnError: true,
	}

	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep returned error; continue_on_error should swallow it: %v", err)
	}

	if failed != 1 {
		t.Errorf("EventStepFailed fired %d times; want 1", failed)
	}
	if completed != 0 {
		t.Errorf("EventStepCompleted fired %d times; want 0 (continue_on_error swallow must not look like success)", completed)
	}

	// Stats.Failed counts the step (the action errored); Stats.Executed
	// must NOT (per its docstring: "Executed counts successfully
	// completed steps"). Pre-fix, both counters incremented.
	if *ec.Svc.Stats.Failed != 1 {
		t.Errorf("Stats.Failed = %d, want 1", *ec.Svc.Stats.Failed)
	}
	if *ec.Svc.Stats.Executed != 0 {
		t.Errorf("Stats.Executed = %d, want 0 (failed step is not 'successfully completed')", *ec.Svc.Stats.Executed)
	}
}
