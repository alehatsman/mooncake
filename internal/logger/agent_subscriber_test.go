package logger

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

func captureAgentOutput(t *testing.T, fn func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func TestAgentSubscriber_RunStarted(t *testing.T) {
	sub := NewAgentSubscriber()
	total := 5
	dry := false

	out := captureAgentOutput(t, func() {
		sub.OnEvent(events.Event{
			Type:      events.EventRunStarted,
			Timestamp: time.Now(),
			Data: events.RunStartedData{
				TotalSteps: total,
				DryRun:     dry,
			},
		})
	})

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("not valid JSON: %v\noutput: %q", err, out)
	}
	if m["event"] != "run.started" {
		t.Errorf("event = %v, want run.started", m["event"])
	}
	if m["total_steps"].(float64) != 5 {
		t.Errorf("total_steps = %v, want 5", m["total_steps"])
	}
}

func TestAgentSubscriber_StepLifecycle(t *testing.T) {
	sub := NewAgentSubscriber()

	started := captureAgentOutput(t, func() {
		sub.OnEvent(events.Event{
			Type:      events.EventStepStarted,
			Timestamp: time.Now(),
			Data: events.StepStartedData{
				StepID: "step-0001",
				Name:   "Install vim",
				Action: "package",
			},
		})
	})

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(started), &m); err != nil {
		t.Fatalf("step.started parse error: %v", err)
	}
	if m["event"] != "step.started" {
		t.Errorf("event = %v", m["event"])
	}
	if m["action"] != "package" {
		t.Errorf("action = %v, want package", m["action"])
	}

	completed := captureAgentOutput(t, func() {
		ms := int64(42)
		sub.OnEvent(events.Event{
			Type:      events.EventStepCompleted,
			Timestamp: time.Now(),
			Data: events.StepCompletedData{
				StepID:     "step-0001",
				Name:       "Install vim",
				Changed:    true,
				DurationMs: ms,
			},
		})
	})

	if err := json.Unmarshal([]byte(completed), &m); err != nil {
		t.Fatalf("step.completed parse error: %v", err)
	}
	if m["changed"] != true {
		t.Errorf("changed = %v, want true", m["changed"])
	}
}

func TestAgentSubscriber_StepSkipped(t *testing.T) {
	sub := NewAgentSubscriber()

	out := captureAgentOutput(t, func() {
		sub.OnEvent(events.Event{
			Type:      events.EventStepSkipped,
			Timestamp: time.Now(),
			Data: events.StepSkippedData{
				StepID: "step-0002",
				Name:   "Linux only",
				Reason: `when: os == "linux"`,
			},
		})
	})

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if m["event"] != "step.skipped" {
		t.Errorf("event = %v", m["event"])
	}
	if m["reason"] != `when: os == "linux"` {
		t.Errorf("reason = %v", m["reason"])
	}
}

func TestAgentSubscriber_StepFailed(t *testing.T) {
	sub := NewAgentSubscriber()

	out := captureAgentOutput(t, func() {
		ms := int64(120)
		sub.OnEvent(events.Event{
			Type:      events.EventStepFailed,
			Timestamp: time.Now(),
			Data: events.StepFailedData{
				StepID:       "step-0003",
				Name:         "Install fzf",
				ErrorMessage: "exit code 1",
				DurationMs:   ms,
			},
		})
	})

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if m["error"] != "exit code 1" {
		t.Errorf("error = %v", m["error"])
	}
}

func TestAgentSubscriber_RunCompleted(t *testing.T) {
	sub := NewAgentSubscriber()

	out := captureAgentOutput(t, func() {
		ms := int64(274000)
		sub.OnEvent(events.Event{
			Type:      events.EventRunCompleted,
			Timestamp: time.Now(),
			Data: events.RunCompletedData{
				SuccessSteps: 73,
				ChangedSteps: 12,
				SkippedSteps: 8,
				FailedSteps:  1,
				DurationMs:   ms,
				Success:      false,
				ErrorMessage: "step failed",
			},
		})
	})

	var m map[string]interface{}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if m["event"] != "run.completed" {
		t.Errorf("event = %v", m["event"])
	}
	// ok = 73 - 12 = 61
	if m["ok"].(float64) != 61 {
		t.Errorf("ok = %v, want 61", m["ok"])
	}
	if m["failed"].(float64) != 1 {
		t.Errorf("failed = %v, want 1", m["failed"])
	}
	if m["error"] != "step failed" {
		t.Errorf("error = %v", m["error"])
	}
	if m["success"] != false {
		t.Errorf("success = %v, want false", m["success"])
	}
}

func TestAgentSubscriber_UnknownEventIgnored(t *testing.T) {
	sub := NewAgentSubscriber()

	// Should produce no output for unknown event types
	out := captureAgentOutput(t, func() {
		sub.OnEvent(events.Event{
			Type:      events.EventPlanLoaded,
			Timestamp: time.Now(),
			Data:      events.PlanLoadedData{TotalSteps: 3},
		})
	})

	if out != "" {
		t.Errorf("expected no output for plan.loaded, got: %q", out)
	}
}
