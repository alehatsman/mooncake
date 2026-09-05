package executor_test

// --keep-going: a failing step records itself and the run carries on,
// so one unavailable upstream package can't strand the rest of a
// first-provisioning run. The failures are re-raised together at the
// end, so the exit code is unchanged — only how much work happens
// before you hear about it.

import (
	"errors"
	"strings"
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

func keepGoingContext(t *testing.T, keepGoing bool) *executor.ExecutionContext {
	t.Helper()
	renderer, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:         logger.NewTestLogger(),
			Template:       renderer,
			Evaluator:      expression.NewGovaluateEvaluator(),
			PathUtil:       pathutil.NewPathExpander(renderer),
			Stats:          executor.NewExecutionStats(),
			Redactor:       security.NewRedactor(),
			EventPublisher: events.NewSyncPublisher(),
			KeepGoing:      keepGoing,
		},
		Scope: executor.NewVariableScope(),
	}
}

func TestKeepGoing_RecordsFailureAndContinues(t *testing.T) {
	ec := keepGoingContext(t, true)

	step := config.Step{Name: "broken package", Shell: &config.ShellAction{Cmd: "exit 3"}}
	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep returned %v; keep-going must not abort the run", err)
	}

	if got := len(ec.Svc.DeferredFailures); got != 1 {
		t.Fatalf("DeferredFailures has %d entries, want 1", got)
	}
	if name := ec.Svc.DeferredFailures[0].StepName; name != "broken package" {
		t.Errorf("StepName = %q, want %q", name, "broken package")
	}
	// The failure is still a failure in the recap.
	if *ec.Svc.Stats.Failed != 1 {
		t.Errorf("Stats.Failed = %d, want 1", *ec.Svc.Stats.Failed)
	}
}

func TestKeepGoing_OffStillAborts(t *testing.T) {
	ec := keepGoingContext(t, false)

	step := config.Step{Name: "broken package", Shell: &config.ShellAction{Cmd: "exit 3"}}
	if err := executor.ExecuteStep(step, ec); err == nil {
		t.Fatal("ExecuteStep returned nil; without keep-going a failure must abort")
	}
	if got := len(ec.Svc.DeferredFailures); got != 0 {
		t.Errorf("DeferredFailures has %d entries, want 0 when keep-going is off", got)
	}
}

// A transaction is all-or-nothing by construction; letting the run
// continue past a rolled-back transaction would leave exactly the
// partial state the transaction exists to prevent.
func TestKeepGoing_DoesNotOverrideTransactionSemantics(t *testing.T) {
	ec := keepGoingContext(t, true)

	step := config.Step{
		Name:      "inside a transaction",
		Shell:     &config.ShellAction{Cmd: "exit 3"},
		TxnParent: "txn-1",
	}
	if err := executor.ExecuteStep(step, ec); err == nil {
		t.Fatal("ExecuteStep returned nil; a transaction child must still abort under keep-going")
	}
	if got := len(ec.Svc.DeferredFailures); got != 0 {
		t.Errorf("DeferredFailures has %d entries, want 0 for a transaction child", got)
	}
}

func TestDeferredFailuresError_Message(t *testing.T) {
	one := &executor.DeferredFailuresError{
		Failures: []executor.DeferredFailure{{StepName: "a", Err: errors.New("boom")}},
	}
	if got := one.Error(); !strings.Contains(got, "1 step failed") || !strings.Contains(got, "a: boom") {
		t.Errorf("single-failure message = %q", got)
	}

	two := &executor.DeferredFailuresError{
		Failures: []executor.DeferredFailure{
			{StepName: "a", Err: errors.New("boom")},
			{StepName: "b", Err: errors.New("bang")},
		},
	}
	got := two.Error()
	for _, want := range []string{"2 steps failed", "• a: boom", "• b: bang"} {
		if !strings.Contains(got, want) {
			t.Errorf("multi-failure message %q missing %q", got, want)
		}
	}

	// errors.Is still reaches the underlying cause.
	sentinel := errors.New("sentinel")
	wrapped := &executor.DeferredFailuresError{
		Failures: []executor.DeferredFailure{{StepName: "a", Err: sentinel}},
	}
	if !errors.Is(wrapped, sentinel) {
		t.Error("errors.Is could not reach the first failure through Unwrap")
	}
}
