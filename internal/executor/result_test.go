package executor

import (
	"testing"
	"time"
)

func TestResult_Status(t *testing.T) {
	tests := []struct {
		name     string
		result   Result
		expected string
	}{
		{
			name:     "ok",
			result:   Result{},
			expected: "ok",
		},
		{
			name:     "changed",
			result:   Result{Changed: true},
			expected: "changed",
		},
		{
			name:     "failed",
			result:   Result{Failed: true},
			expected: "failed",
		},
		{
			name:     "skipped",
			result:   Result{Skipped: true},
			expected: "skipped",
		},
		{
			name:     "failed takes precedence over changed",
			result:   Result{Failed: true, Changed: true},
			expected: "failed",
		},
		{
			name:     "failed takes precedence over skipped",
			result:   Result{Failed: true, Skipped: true},
			expected: "failed",
		},
		{
			name:     "skipped takes precedence over changed",
			result:   Result{Skipped: true, Changed: true},
			expected: "skipped",
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

func TestResult_ToMap_IncludesTiming(t *testing.T) {
	startTime := time.Now()
	duration := 100 * time.Millisecond

	result := Result{
		Stdout:    "output",
		Stderr:    "error",
		Rc:        0,
		Failed:    false,
		Changed:   true,
		Skipped:   false,
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
	}

	m := result.ToMap()

	// Check basic fields
	if m["stdout"] != "output" {
		t.Errorf("ToMap() stdout = %v, want 'output'", m["stdout"])
	}
	if m["stderr"] != "error" {
		t.Errorf("ToMap() stderr = %v, want 'error'", m["stderr"])
	}
	if m["rc"] != 0 {
		t.Errorf("ToMap() rc = %v, want 0", m["rc"])
	}
	if m["failed"] != false {
		t.Errorf("ToMap() failed = %v, want false", m["failed"])
	}
	if m["changed"] != true {
		t.Errorf("ToMap() changed = %v, want true", m["changed"])
	}
	if m["skipped"] != false {
		t.Errorf("ToMap() skipped = %v, want false", m["skipped"])
	}

	// Check timing fields
	if _, ok := m["duration_ms"]; !ok {
		t.Error("ToMap() should include duration_ms")
	}
	if durationMs, ok := m["duration_ms"].(int64); !ok || durationMs != 100 {
		t.Errorf("ToMap() duration_ms = %v, want 100", m["duration_ms"])
	}

	// Check status field
	if _, ok := m["status"]; !ok {
		t.Error("ToMap() should include status")
	}
	if status, ok := m["status"].(string); !ok || status != "changed" {
		t.Errorf("ToMap() status = %v, want 'changed'", m["status"])
	}
}

func TestResult_RegisterTo_WithTiming(t *testing.T) {
	startTime := time.Now()
	duration := 50 * time.Millisecond

	result := Result{
		Stdout:    "test output",
		Changed:   true,
		StartTime: startTime,
		EndTime:   startTime.Add(duration),
		Duration:  duration,
	}

	variables := make(map[string]interface{})
	result.RegisterTo(variables, "test_result")

	// Check that result is registered
	registered, ok := variables["test_result"]
	if !ok {
		t.Fatal("Result was not registered to variables")
	}

	// Check that it's a map
	resultMap, ok := registered.(map[string]interface{})
	if !ok {
		t.Fatal("Registered result is not a map")
	}

	// Verify timing is accessible
	if _, ok := resultMap["duration_ms"]; !ok {
		t.Error("Registered result should include duration_ms")
	}

	if _, ok := resultMap["status"]; !ok {
		t.Error("Registered result should include status")
	}

	if status := resultMap["status"]; status != "changed" {
		t.Errorf("Registered result status = %v, want 'changed'", status)
	}
}

func TestNewResult_DefaultValues(t *testing.T) {
	result := NewResult()

	if result.Stdout != "" {
		t.Errorf("NewResult() Stdout = %v, want empty string", result.Stdout)
	}
	if result.Stderr != "" {
		t.Errorf("NewResult() Stderr = %v, want empty string", result.Stderr)
	}
	if result.Rc != 0 {
		t.Errorf("NewResult() Rc = %v, want 0", result.Rc)
	}
	if result.Failed {
		t.Error("NewResult() Failed should be false")
	}
	if result.Changed {
		t.Error("NewResult() Changed should be false")
	}
	if result.Skipped {
		t.Error("NewResult() Skipped should be false")
	}
	if !result.StartTime.IsZero() {
		t.Error("NewResult() StartTime should be zero")
	}
	if !result.EndTime.IsZero() {
		t.Error("NewResult() EndTime should be zero")
	}
	if result.Duration != 0 {
		t.Error("NewResult() Duration should be zero")
	}
}

func TestResult_Status_Priority(t *testing.T) {
	// Test priority: Failed > Skipped > Changed > Ok

	// All flags set - Failed should win
	result := Result{
		Failed:  true,
		Skipped: true,
		Changed: true,
	}
	if status := result.Status(); status != "failed" {
		t.Errorf("With all flags, Status() = %v, want 'failed'", status)
	}

	// Skipped and Changed - Skipped should win
	result = Result{
		Skipped: true,
		Changed: true,
	}
	if status := result.Status(); status != "skipped" {
		t.Errorf("With skipped and changed, Status() = %v, want 'skipped'", status)
	}
}

