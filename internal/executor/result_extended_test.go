package executor

import (
	"testing"
	"time"
)

// TestResult_Status_AllStates tests all possible status states
func TestResult_Status_AllStates(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected string
	}{
		{
			"default ok",
			Result{},
			"ok",
		},
		{
			"changed only",
			Result{Changed: true},
			"changed",
		},
		{
			"failed only",
			Result{Failed: true},
			"failed",
		},
		{
			"skipped only",
			Result{Skipped: true},
			"skipped",
		},
		{
			"failed with changed",
			Result{Failed: true, Changed: true},
			"failed",
		},
		{
			"failed with skipped",
			Result{Failed: true, Skipped: true},
			"failed",
		},
		{
			"skipped with changed",
			Result{Skipped: true, Changed: true},
			"skipped",
		},
		{
			"all flags",
			Result{Failed: true, Skipped: true, Changed: true},
			"failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status := tt.result.Status(); status != tt.expected {
				t.Errorf("Status() = %v, want %v", status, tt.expected)
			}
		})
	}
}

// TestResult_ToMap_CompleteFields tests ToMap with all fields populated
func TestResult_ToMap_CompleteFields(t *testing.T) {
	startTime := time.Now()
	duration := 250 * time.Millisecond

	result := Result{
		Stdout:    "standard output",
		Stderr:    "error output",
		Rc:        127,
		Failed:    true,
		Changed:   false,
		Skipped:   false,
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
	}

	m := result.ToMap()

	// Verify all fields
	if m["stdout"] != "standard output" {
		t.Errorf("ToMap() stdout = %v, want 'standard output'", m["stdout"])
	}
	if m["stderr"] != "error output" {
		t.Errorf("ToMap() stderr = %v, want 'error output'", m["stderr"])
	}
	if m["rc"] != 127 {
		t.Errorf("ToMap() rc = %v, want 127", m["rc"])
	}
	if m["failed"] != true {
		t.Errorf("ToMap() failed = %v, want true", m["failed"])
	}
	if m["changed"] != false {
		t.Errorf("ToMap() changed = %v, want false", m["changed"])
	}
	if m["skipped"] != false {
		t.Errorf("ToMap() skipped = %v, want false", m["skipped"])
	}
	if m["status"] != "failed" {
		t.Errorf("ToMap() status = %v, want 'failed'", m["status"])
	}
	if m["duration_ms"] != int64(250) {
		t.Errorf("ToMap() duration_ms = %v, want 250", m["duration_ms"])
	}
}

// TestResult_RegisterTo_MultipleVars tests registering multiple results
func TestResult_RegisterTo_MultipleVars(t *testing.T) {
	variables := make(map[string]interface{})

	result1 := Result{
		Stdout:  "output1",
		Changed: true,
	}
	result1.RegisterTo(variables, "result1")

	result2 := Result{
		Stdout:  "output2",
		Changed: false,
	}
	result2.RegisterTo(variables, "result2")

	// Both should be registered
	if _, ok := variables["result1"]; !ok {
		t.Error("result1 should be registered")
	}
	if _, ok := variables["result2"]; !ok {
		t.Error("result2 should be registered")
	}

	// Verify they're different
	r1 := variables["result1"].(map[string]interface{})
	r2 := variables["result2"].(map[string]interface{})

	if r1["stdout"] == r2["stdout"] {
		t.Error("Results should have different stdout values")
	}
}

// TestResult_RegisterTo_Overwrite tests overwriting registered results
func TestResult_RegisterTo_Overwrite(t *testing.T) {
	variables := make(map[string]interface{})

	result1 := Result{
		Stdout: "first",
	}
	result1.RegisterTo(variables, "myresult")

	result2 := Result{
		Stdout: "second",
	}
	result2.RegisterTo(variables, "myresult")

	// Should have the second result
	resultMap := variables["myresult"].(map[string]interface{})
	if resultMap["stdout"] != "second" {
		t.Errorf("stdout = %v, want 'second' (overwritten)", resultMap["stdout"])
	}
}

// TestResult_SetFailed_RcBehavior tests Rc field behavior with SetFailed
func TestResult_SetFailed_RcBehavior(t *testing.T) {
	// Setting failed to true sets Rc to 1
	result := NewResult()
	result.SetFailed(true)

	if result.Rc != 1 {
		t.Errorf("SetFailed(true) should set Rc to 1, got %d", result.Rc)
	}

	// Setting failed to false doesn't change Rc
	result.SetFailed(false)
	if result.Rc != 1 {
		t.Errorf("SetFailed(false) should not change Rc, got %d", result.Rc)
	}
}

// TestResult_SetFailed_MultipleCalls tests multiple SetFailed calls
func TestResult_SetFailed_MultipleCalls(t *testing.T) {
	result := NewResult()

	result.SetFailed(true)
	if !result.Failed {
		t.Error("First SetFailed(true) should set Failed")
	}

	result.SetFailed(false)
	if result.Failed {
		t.Error("SetFailed(false) should unset Failed")
	}

	result.SetFailed(true)
	if !result.Failed {
		t.Error("Second SetFailed(true) should set Failed again")
	}
}

// TestResult_SetStdout_Empty tests SetStdout with empty string
func TestResult_SetStdout_Empty(t *testing.T) {
	result := NewResult()
	result.SetStdout("initial")
	result.SetStdout("")

	if result.Stdout != "" {
		t.Errorf("SetStdout(\"\") should clear stdout, got %q", result.Stdout)
	}
}

// TestResult_SetStderr_Empty tests SetStderr with empty string
func TestResult_SetStderr_Empty(t *testing.T) {
	result := NewResult()
	result.SetStderr("initial error")
	result.SetStderr("")

	if result.Stderr != "" {
		t.Errorf("SetStderr(\"\") should clear stderr, got %q", result.Stderr)
	}
}

// TestResult_SetChanged_Toggle tests toggling Changed flag
func TestResult_SetChanged_Toggle(t *testing.T) {
	result := NewResult()

	result.SetChanged(true)
	if !result.Changed {
		t.Error("SetChanged(true) should set Changed")
	}

	result.SetChanged(false)
	if result.Changed {
		t.Error("SetChanged(false) should unset Changed")
	}

	result.SetChanged(true)
	if !result.Changed {
		t.Error("SetChanged(true) should set Changed again")
	}
}

// TestResult_SetData_NoOp tests that SetData is a no-op
func TestResult_SetData_NoOp(t *testing.T) {
	result := NewResult()

	// Should not panic
	result.SetData(map[string]interface{}{
		"test": "data",
	})

	result.SetData(nil)

	// No assertions - just ensuring no panic
}

// TestNewResult_Idempotent tests that NewResult returns fresh instances
func TestNewResult_Idempotent(t *testing.T) {
	result1 := NewResult()
	result2 := NewResult()

	// Should be different instances
	result1.Stdout = "test1"
	result2.Stdout = "test2"

	if result1.Stdout == result2.Stdout {
		t.Error("NewResult() should return independent instances")
	}
}

// TestResult_ToMap_ZeroTime tests ToMap with zero time values
func TestResult_ToMap_ZeroTime(t *testing.T) {
	result := NewResult()
	// Don't set StartTime/EndTime/Duration

	m := result.ToMap()

	// Should have duration_ms as 0
	if m["duration_ms"] != int64(0) {
		t.Errorf("ToMap() duration_ms = %v, want 0 for zero duration", m["duration_ms"])
	}
}
