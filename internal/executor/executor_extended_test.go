package executor

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

// TestResult_SetChanged tests SetChanged method
func TestResult_SetChanged(t *testing.T) {
	result := NewResult()

	if result.Changed {
		t.Error("New result should not be changed")
	}

	result.SetChanged(true)
	if !result.Changed {
		t.Error("SetChanged(true) should set Changed to true")
	}

	result.SetChanged(false)
	if result.Changed {
		t.Error("SetChanged(false) should set Changed to false")
	}
}

// TestResult_SetStdout tests SetStdout method
func TestResult_SetStdout(t *testing.T) {
	result := NewResult()

	result.SetStdout("test output")
	if result.Stdout != "test output" {
		t.Errorf("SetStdout() Stdout = %q, want 'test output'", result.Stdout)
	}

	result.SetStdout("different output")
	if result.Stdout != "different output" {
		t.Errorf("SetStdout() Stdout = %q, want 'different output'", result.Stdout)
	}
}

// TestResult_SetStderr tests SetStderr method
func TestResult_SetStderr(t *testing.T) {
	result := NewResult()

	result.SetStderr("error output")
	if result.Stderr != "error output" {
		t.Errorf("SetStderr() Stderr = %q, want 'error output'", result.Stderr)
	}

	result.SetStderr("different error")
	if result.Stderr != "different error" {
		t.Errorf("SetStderr() Stderr = %q, want 'different error'", result.Stderr)
	}
}

// TestResult_SetFailed tests SetFailed method
func TestResult_SetFailed(t *testing.T) {
	result := NewResult()

	if result.Failed {
		t.Error("New result should not be failed")
	}
	if result.Rc != 0 {
		t.Errorf("New result Rc = %d, want 0", result.Rc)
	}

	result.SetFailed(true)
	if !result.Failed {
		t.Error("SetFailed(true) should set Failed to true")
	}
	if result.Rc != 1 {
		t.Errorf("SetFailed(true) should set Rc to 1, got %d", result.Rc)
	}

	result.SetFailed(false)
	if result.Failed {
		t.Error("SetFailed(false) should set Failed to false")
	}
	// Rc should remain 1 (not reset)
	if result.Rc != 1 {
		t.Errorf("After SetFailed(false) Rc = %d, want 1 (not reset)", result.Rc)
	}
}

// TestResult_SetData tests SetData method (currently a no-op)
func TestResult_SetData(t *testing.T) {
	result := NewResult()

	// SetData is currently a no-op, just ensure it doesn't panic
	result.SetData(map[string]interface{}{
		"key": "value",
		"num": 42,
	})

	// No assertions - this is a TODO/placeholder method
}

// TestExecutionContext_ModeRoundtrip verifies Mode is reported
// by Mode() unchanged.
func TestExecutionContext_ModeRoundtrip(t *testing.T) {
	ctx := &ExecutionContext{Svc: &RunServices{Mode: actions.ModePlan}}
	if ctx.Mode() != actions.ModePlan {
		t.Error("Mode() should return ModePlan when Mode is ModePlan")
	}
	ctx.Svc.Mode = actions.ModeApply
	if ctx.Mode() != actions.ModeApply {
		t.Error("Mode() should return ModeApply when Mode is ModeApply")
	}
}

