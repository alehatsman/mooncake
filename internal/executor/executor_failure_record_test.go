package executor_test

// A failed step has to reach the run log. postExecuteSuccess owns the
// success path's capture feed and never runs for a failure, so before this
// was wired runs.jsonl said failed=1 in its header while every step it
// listed read ok or changed — the one fact a post-mortem needs, missing.

import (
	"testing"

	_ "github.com/alehatsman/mooncake/internal/actions/shell"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

func captureContext(t *testing.T, keepGoing bool) (*executor.ExecutionContext, *executor.RunCapture) {
	t.Helper()
	ec := keepGoingContext(t, keepGoing)
	capture := &executor.RunCapture{}
	ec.Svc.Capture = capture
	return ec, capture
}

// TestFailedStep_IsCaptured covers the three ways a step failure exits
// handleStepError. All of them bypass postExecuteSuccess, so each one needs
// its own record — and exactly one, since a double-record would show the
// step as both failed and successful.
func TestFailedStep_IsCaptured(t *testing.T) {
	tests := []struct {
		name      string
		keepGoing bool
		step      config.Step
		wantErr   bool
	}{
		{
			name:    "fatal failure",
			step:    config.Step{Name: "boom", Shell: &config.ShellAction{Cmd: "exit 3"}},
			wantErr: true,
		},
		{
			name:      "keep-going failure",
			keepGoing: true,
			step:      config.Step{Name: "boom", Shell: &config.ShellAction{Cmd: "exit 3"}},
		},
		{
			name: "continue_on_error failure",
			step: config.Step{
				Name:            "boom",
				Shell:           &config.ShellAction{Cmd: "exit 3"},
				ContinueOnError: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec, capture := captureContext(t, tt.keepGoing)

			err := executor.ExecuteStep(tt.step, ec)
			if gotErr := err != nil; gotErr != tt.wantErr {
				t.Fatalf("ExecuteStep error = %v, want error: %v", err, tt.wantErr)
			}

			steps := capture.Steps()
			if len(steps) != 1 {
				t.Fatalf("captured %d step records, want exactly 1", len(steps))
			}
			rec := steps[0]
			if rec.Result == nil {
				t.Fatal("captured record has no Result")
			}
			if got := rec.Result.Status(); got != "failed" {
				t.Errorf("Status() = %q, want %q", got, "failed")
			}
			if rec.Result.Rc != 3 {
				t.Errorf("Rc = %d, want 3 (the shell's real exit code)", rec.Result.Rc)
			}
			if rec.Result.Error == "" {
				t.Error("Error is empty; the record says which step failed but not why")
			}
		})
	}
}

// TestSucceededStep_IsCapturedOnce guards the other side of the same wire:
// the success path must keep producing exactly one record, not two.
func TestSucceededStep_IsCapturedOnce(t *testing.T) {
	ec, capture := captureContext(t, false)

	step := config.Step{Name: "fine", Shell: &config.ShellAction{Cmd: "echo fine"}}
	if err := executor.ExecuteStep(step, ec); err != nil {
		t.Fatalf("ExecuteStep: %v", err)
	}

	steps := capture.Steps()
	if len(steps) != 1 {
		t.Fatalf("captured %d step records, want exactly 1", len(steps))
	}
	if got := steps[0].Result.Status(); got == "failed" {
		t.Errorf("Status() = %q for a step that succeeded", got)
	}
	if e := steps[0].Result.Error; e != "" {
		t.Errorf("unexpected Error on a successful step: %q", e)
	}
}
