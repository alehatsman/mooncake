package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

// mockRedactor is a test redactor that replaces "secret" with "REDACTED"
type mockRedactor struct{}

func (m *mockRedactor) Redact(s string) string {
	return strings.ReplaceAll(s, "secret", "REDACTED")
}

// captureStdout captures stdout during function execution
func captureStdout(f func()) string {
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	f()

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	io.Copy(&buf, r)
	return buf.String()
}

func TestNewConsoleSubscriber(t *testing.T) {
	tests := []struct {
		name      string
		logLevel  int
		logFormat string
	}{
		{"text format", 1, "text"},
		{"json format", 2, "json"},
		{"debug level", 0, "text"},
		{"info level", 1, "text"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sub := NewConsoleSubscriber(tt.logLevel, tt.logFormat, false)
			if sub == nil {
				t.Fatal("NewConsoleSubscriber returned nil")
			}
			if sub.logLevel != tt.logLevel {
				t.Errorf("logLevel = %d, want %d", sub.logLevel, tt.logLevel)
			}
			if sub.logFormat != tt.logFormat {
				t.Errorf("logFormat = %s, want %s", sub.logFormat, tt.logFormat)
			}
		})
	}
}

func TestConsoleSubscriber_SetRedactor(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)
	if sub.redactor != nil {
		t.Error("expected nil redactor initially")
	}

	redactor := &mockRedactor{}
	sub.SetRedactor(redactor)

	if sub.redactor == nil {
		t.Error("redactor not set")
	}
}

func TestConsoleSubscriber_Close(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)
	// Should not panic
	sub.Close()
}

func TestConsoleSubscriber_OnEvent_JSON(t *testing.T) {
	sub := NewConsoleSubscriber(1, "json", false)

	event := events.Event{
		Type:      events.EventStepStarted,
		Timestamp: time.Now(),
		Data: events.StepStartedData{
			StepID:     "step-1",
			Name:       "Test step",
			Level:      0,
			GlobalStep: 1,
			Action:     "shell",
		},
	}

	output := captureStdout(func() {
		sub.OnEvent(event)
	})

	// Verify it's valid JSON
	var decoded events.Event
	if err := json.Unmarshal([]byte(output), &decoded); err != nil {
		t.Fatalf("output is not valid JSON: %v\nOutput: %s", err, output)
	}

	// Verify event type
	if decoded.Type != events.EventStepStarted {
		t.Errorf("decoded event type = %v, want %v", decoded.Type, events.EventStepStarted)
	}
}

func TestConsoleSubscriber_OnEvent_Text_StepStarted(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name       string
		data       events.StepStartedData
		wantOutput string
		wantEmpty  bool
	}{
		{
			name: "regular step",
			data: events.StepStartedData{
				StepID:     "step-1",
				Name:       "Install nginx",
				Level:      0,
				GlobalStep: 1,
				Action:     "shell",
			},
			wantOutput: "Install nginx",
		},
		{
			name: "nested step",
			data: events.StepStartedData{
				StepID:     "step-2",
				Name:       "Configure service",
				Level:      1,
				GlobalStep: 2,
				Action:     "file",
			},
			wantOutput: "Configure service",
		},
		{
			name: "directory step",
			data: events.StepStartedData{
				StepID:     "step-3",
				Name:       "templates/",
				Level:      0,
				GlobalStep: 3,
				Action:     "filetree",
			},
			wantEmpty: true, // Directories don't show started event
		},
		{
			name: "step with depth",
			data: events.StepStartedData{
				StepID:     "step-4",
				Name:       "config.yml",
				Level:      0,
				GlobalStep: 4,
				Action:     "file",
				Depth:      2,
			},
			wantOutput: "config.yml",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      events.EventStepStarted,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			if tt.wantEmpty {
				if output != "" {
					t.Errorf("expected no output, got: %s", output)
				}
				return
			}

			if !strings.Contains(output, tt.wantOutput) {
				t.Errorf("output does not contain %q\nGot: %s", tt.wantOutput, output)
			}

			// Verify icon is present
			if !strings.Contains(output, "▶") {
				t.Error("output missing ▶ icon")
			}
		})
	}
}

func TestConsoleSubscriber_OnEvent_Text_StepCompleted(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name       string
		data       events.StepCompletedData
		wantOutput string
		wantEmpty  bool
	}{
		{
			name: "regular step",
			data: events.StepCompletedData{
				StepID:     "step-1",
				Name:       "Install nginx",
				Level:      0,
				DurationMs: 100,
				Changed:    true,
			},
			wantOutput: "Install nginx",
		},
		{
			name: "directory step",
			data: events.StepCompletedData{
				StepID:     "step-2",
				Name:       "templates/",
				Level:      0,
				DurationMs: 50,
				Changed:    false,
			},
			wantEmpty: true, // Directories don't show completed event
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      events.EventStepCompleted,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			if tt.wantEmpty {
				if output != "" {
					t.Errorf("expected no output, got: %s", output)
				}
				return
			}

			if !strings.Contains(output, tt.wantOutput) {
				t.Errorf("output does not contain %q\nGot: %s", tt.wantOutput, output)
			}

			// Verify icon is present (~ for changed, ✓ for unchanged)
			if tt.data.Changed {
				if !strings.Contains(output, "~") {
					t.Error("output missing ~ icon for changed step")
				}
			} else {
				if !strings.Contains(output, "✓") {
					t.Error("output missing ✓ icon for unchanged step")
				}
			}
		})
	}
}

func TestConsoleSubscriber_OnEvent_Text_StepFailed(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	data := events.StepFailedData{
		StepID:       "step-1",
		Name:         "Install nginx",
		Level:        0,
		ErrorMessage: "package not found",
		DurationMs:   100,
	}

	event := events.Event{
		Type:      events.EventStepFailed,
		Timestamp: time.Now(),
		Data:      data,
	}

	output := captureStdout(func() {
		sub.OnEvent(event)
	})

	// Verify step name
	if !strings.Contains(output, "Install nginx") {
		t.Errorf("output does not contain step name\nGot: %s", output)
	}

	// Verify icon is present
	if !strings.Contains(output, "✗") {
		t.Error("output missing ✗ icon")
	}

	// Verify error message
	if !strings.Contains(output, "package not found") {
		t.Errorf("output does not contain error message\nGot: %s", output)
	}
}

func TestConsoleSubscriber_OnEvent_Text_StepSkipped(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name       string
		data       events.StepSkippedData
		wantOutput string
		wantEmpty  bool
	}{
		{
			name: "regular file with reason",
			data: events.StepSkippedData{
				StepID: "step-1",
				Name:   "config.yml",
				Level:  0,
				Reason: "when condition false",
				Depth:  1,
			},
			wantOutput: "config.yml",
		},
		{
			name: "regular file without reason",
			data: events.StepSkippedData{
				StepID: "step-2",
				Name:   "nginx.conf",
				Level:  0,
				Reason: "",
				Depth:  0,
			},
			wantOutput: "nginx.conf",
		},
		{
			name: "root directory",
			data: events.StepSkippedData{
				StepID: "step-3",
				Name:   "templates/",
				Level:  0,
				Reason: "when condition false",
			},
			wantEmpty: true, // Root directory not shown
		},
		{
			name: "subdirectory",
			data: events.StepSkippedData{
				StepID: "step-4",
				Name:   "templates/after/",
				Level:  0,
				Reason: "when condition false", // Note: directories don't show reasons
			},
			wantOutput: "templates/after/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      events.EventStepSkipped,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			if tt.wantEmpty {
				if output != "" {
					t.Errorf("expected no output, got: %s", output)
				}
				return
			}

			if !strings.Contains(output, tt.wantOutput) {
				t.Errorf("output does not contain %q\nGot: %s", tt.wantOutput, output)
			}

			// Verify icon is present for files
			if !strings.HasSuffix(tt.data.Name, "/") && !strings.Contains(output, "-") {
				t.Error("output missing - icon for file")
			}

			// Verify reason if provided (only for files, not directories)
			if tt.data.Reason != "" && !strings.HasSuffix(tt.data.Name, "/") && !strings.Contains(output, tt.data.Reason) {
				t.Errorf("output does not contain reason %q\nGot: %s", tt.data.Reason, output)
			}
		})
	}
}

func TestConsoleSubscriber_OnEvent_Text_RunCompleted(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name       string
		data       events.RunCompletedData
		wantOutput []string
	}{
		{
			name: "successful run",
			data: events.RunCompletedData{
				TotalSteps:   10,
				SuccessSteps: 8,
				FailedSteps:  0,
				SkippedSteps: 2,
				ChangedSteps: 5,
				DurationMs:   1234,
				Success:      true,
				ErrorMessage: "",
			},
			wantOutput: []string{
				"RECAP",
				"ok=3",
				"changed=5",
				"skipped=2",
				"failed=0",
				"1s",
			},
		},
		{
			name: "failed run",
			data: events.RunCompletedData{
				TotalSteps:   5,
				SuccessSteps: 3,
				FailedSteps:  1,
				SkippedSteps: 1,
				ChangedSteps: 2,
				DurationMs:   500,
				Success:      false,
				ErrorMessage: "step failed: command not found",
			},
			wantOutput: []string{
				"RECAP",
				"ok=1",
				"changed=2",
				"skipped=1",
				"failed=1",
				"500ms",
				"step failed: command not found",
			},
		},
		{
			name: "run with no changes",
			data: events.RunCompletedData{
				TotalSteps:   3,
				SuccessSteps: 3,
				FailedSteps:  0,
				SkippedSteps: 0,
				ChangedSteps: 0,
				DurationMs:   200,
				Success:      true,
				ErrorMessage: "",
			},
			wantOutput: []string{
				"RECAP",
				"ok=3",
				"changed=0",
				"skipped=0",
				"failed=0",
				"200ms",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      events.EventRunCompleted,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			for _, want := range tt.wantOutput {
				if !strings.Contains(output, want) {
					t.Errorf("output does not contain %q\nGot: %s", want, output)
				}
			}

			// Verify RECAP prefix is present
			if !strings.Contains(output, "RECAP") {
				t.Error("output missing RECAP prefix")
			}
		})
	}
}

func TestConsoleSubscriber_OnEvent_Text_OutputEvents(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name      string
		eventType events.Type
	}{
		{"stdout event", events.EventStepStdout},
		{"stderr event", events.EventStepStderr},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Data: events.StepOutputData{
					StepID:     "step-1",
					Stream:     "stdout",
					Line:       "test output",
					LineNumber: 1,
				},
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			// Output events should not produce console output
			if output != "" {
				t.Errorf("expected no output for output events, got: %s", output)
			}
		})
	}
}

// TestConsoleSubscriber_StepIDRendered covers #20: every step row carries
// its stable ID as a cross-reference token, while directory header rows
// stay clean.
func TestConsoleSubscriber_StepIDRendered(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name   string
		event  events.Event
		wantID string
		absent bool // ID must NOT appear (directory header)
	}{
		{
			name: "started row",
			event: events.Event{Type: events.EventStepStarted, Data: events.StepStartedData{
				StepID: "step-7", Name: "Install nginx"}},
			wantID: "step-7",
		},
		{
			name: "completed row",
			event: events.Event{Type: events.EventStepCompleted, Data: events.StepCompletedData{
				StepID: "install-nginx", Name: "Install nginx", DurationMs: 5}},
			wantID: "install-nginx",
		},
		{
			name: "failed row",
			event: events.Event{Type: events.EventStepFailed, Data: events.StepFailedData{
				StepID: "step-9", Name: "boom", ErrorMessage: "nope"}},
			wantID: "step-9",
		},
		{
			name: "skipped row",
			event: events.Event{Type: events.EventStepSkipped, Data: events.StepSkippedData{
				StepID: "step-3", Name: "config.yml", Reason: "when false"}},
			wantID: "step-3",
		},
		{
			name: "directory header has no id",
			event: events.Event{Type: events.EventStepSkipped, Data: events.StepSkippedData{
				StepID: "step-4", Name: "templates/after/"}},
			wantID: "step-4",
			absent: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.event.Timestamp = time.Now()
			output := captureStdout(func() { sub.OnEvent(tt.event) })
			got := strings.Contains(output, tt.wantID)
			if tt.absent && got {
				t.Errorf("directory header should not show ID %q\nGot: %s", tt.wantID, output)
			}
			if !tt.absent && !got {
				t.Errorf("output missing step ID %q\nGot: %s", tt.wantID, output)
			}
		})
	}
}

// TestConsoleSubscriber_DurationAlwaysShown covers #20: per-step duration is
// rendered even for fast steps (previously gated at >= 2s).
func TestConsoleSubscriber_DurationAlwaysShown(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)
	event := events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data:      events.StepCompletedData{StepID: "step-1", Name: "fast step", DurationMs: 3},
	}
	output := captureStdout(func() { sub.OnEvent(event) })
	if !strings.Contains(output, "3ms") {
		t.Errorf("output missing per-step duration\nGot: %s", output)
	}
}

// TestConsoleSubscriber_FileByteSummary covers #20: a file create/update
// event's after-write size is appended to the following step.completed row,
// reset on the next step.started so it can't leak onto a later step.
func TestConsoleSubscriber_FileByteSummary(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	// File event stashes size; completed row appends it.
	out := captureStdout(func() {
		sub.OnEvent(events.Event{Type: events.EventFileCreated, Timestamp: time.Now(),
			Data: events.FileOperationData{Path: "/tmp/marker.txt", SizeBytes: 12, Changed: true}})
		sub.OnEvent(events.Event{Type: events.EventStepCompleted, Timestamp: time.Now(),
			Data: events.StepCompletedData{StepID: "step-1", Name: "write marker", Changed: true, DurationMs: 1}})
	})
	if !strings.Contains(out, "12 B") {
		t.Errorf("expected byte summary on file step\nGot: %s", out)
	}

	// A fresh step.started resets the pending size: the next completed row
	// (a non-file step) must not inherit it.
	out = captureStdout(func() {
		sub.OnEvent(events.Event{Type: events.EventFileCreated, Timestamp: time.Now(),
			Data: events.FileOperationData{Path: "/tmp/a", SizeBytes: 99, Changed: true}})
		sub.OnEvent(events.Event{Type: events.EventStepStarted, Timestamp: time.Now(),
			Data: events.StepStartedData{StepID: "step-2", Name: "echo hi"}})
		sub.OnEvent(events.Event{Type: events.EventStepCompleted, Timestamp: time.Now(),
			Data: events.StepCompletedData{StepID: "step-2", Name: "echo hi", DurationMs: 1}})
	})
	if strings.Contains(out, "99 B") {
		t.Errorf("byte summary leaked onto a later step\nGot: %s", out)
	}
}

func TestConsoleSubscriber_OnEvent_Text_UnknownEvent(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	// Test with various event types that should not produce output
	tests := []struct {
		name      string
		eventType events.Type
	}{
		{"file created", events.EventFileCreated},
		{"file updated", events.EventFileUpdated},
		{"directory created", events.EventDirCreated},
		{"template rendered", events.EventTemplateRender},
		{"vars set", events.EventVarsSet},
		{"vars loaded", events.EventVarsLoaded},
		{"plan loaded", events.EventPlanLoaded},
		{"run started", events.EventRunStarted},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Data:      nil,
			}

			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			// These events should not produce console output
			if output != "" {
				t.Errorf("expected no output for %s event, got: %s", tt.eventType, output)
			}
		})
	}
}

func TestConsoleSubscriber_Concurrency(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	// Test concurrent access to SetRedactor and OnEvent
	done := make(chan bool)

	// Goroutine 1: Set redactor
	go func() {
		for i := 0; i < 100; i++ {
			sub.SetRedactor(&mockRedactor{})
		}
		done <- true
	}()

	// Goroutine 2: Send events
	go func() {
		for i := 0; i < 100; i++ {
			event := events.Event{
				Type:      events.EventStepStarted,
				Timestamp: time.Now(),
				Data: events.StepStartedData{
					StepID:     "step-1",
					Name:       "Test step",
					Level:      0,
					GlobalStep: 1,
					Action:     "shell",
				},
			}
			sub.OnEvent(event)
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// Should not panic
}

func TestConsoleSubscriber_InvalidEventData(t *testing.T) {
	sub := NewConsoleSubscriber(1, "text", false)

	tests := []struct {
		name      string
		eventType events.Type
		data      interface{}
	}{
		{
			name:      "step started with wrong data type",
			eventType: events.EventStepStarted,
			data:      "invalid data",
		},
		{
			name:      "step completed with wrong data type",
			eventType: events.EventStepCompleted,
			data:      123,
		},
		{
			name:      "step failed with wrong data type",
			eventType: events.EventStepFailed,
			data:      true,
		},
		{
			name:      "step skipped with wrong data type",
			eventType: events.EventStepSkipped,
			data:      nil,
		},
		{
			name:      "run completed with wrong data type",
			eventType: events.EventRunCompleted,
			data:      []string{"test"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			event := events.Event{
				Type:      tt.eventType,
				Timestamp: time.Now(),
				Data:      tt.data,
			}

			// Should not panic with invalid data
			output := captureStdout(func() {
				sub.OnEvent(event)
			})

			// Should produce no output for invalid data
			if output != "" {
				t.Errorf("expected no output for invalid data, got: %s", output)
			}
		})
	}
}
