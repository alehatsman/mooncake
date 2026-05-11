package logger

import (
	"bytes"
	"encoding/json"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

func TestStderrErrorSubscriber_OnEvent(t *testing.T) {
	makeEvent := func(d events.StepFailedData) events.Event {
		return events.Event{
			Type:      events.EventStepFailed,
			Timestamp: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			Data:      d,
		}
	}

	t.Run("writes JSON on step failure", func(t *testing.T) {
		var buf bytes.Buffer
		s := &StderrErrorSubscriber{w: &buf}

		s.OnEvent(makeEvent(events.StepFailedData{
			Name:         "Install curl",
			Action:       "shell",
			ErrorMessage: "exit code 1",
			ExitCode:     1,
			Stdout:       "",
			Stderr:       "curl: command not found\n",
		}))

		var rec map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
			t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
		}

		if rec["event"] != "step_error" {
			t.Errorf("event: got %v, want step_error", rec["event"])
		}
		if rec["step"] != "Install curl" {
			t.Errorf("step: got %v", rec["step"])
		}
		if rec["action"] != "shell" {
			t.Errorf("action: got %v", rec["action"])
		}
		if rec["exit_code"] != float64(1) {
			t.Errorf("exit_code: got %v", rec["exit_code"])
		}
		if rec["hint"] != "curl is not installed" {
			t.Errorf("hint: got %v", rec["hint"])
		}
		if rec["suggested_step"] == nil {
			t.Error("suggested_step should be populated")
		}
	})

	t.Run("no hint when stderr is empty", func(t *testing.T) {
		var buf bytes.Buffer
		s := &StderrErrorSubscriber{w: &buf}

		s.OnEvent(makeEvent(events.StepFailedData{
			Name:         "Run script",
			Action:       "shell",
			ErrorMessage: "exit code 2",
			ExitCode:     2,
		}))

		var rec map[string]interface{}
		if err := json.Unmarshal(buf.Bytes(), &rec); err != nil {
			t.Fatalf("not valid JSON: %v", err)
		}
		if _, ok := rec["hint"]; ok {
			t.Error("hint should be absent when no pattern matches")
		}
	})

	t.Run("ignores non-failure events", func(t *testing.T) {
		var buf bytes.Buffer
		s := &StderrErrorSubscriber{w: &buf}

		s.OnEvent(events.Event{
			Type:      events.EventStepCompleted,
			Timestamp: time.Now(),
			Data:      events.StepCompletedData{Name: "step"},
		})

		if buf.Len() != 0 {
			t.Errorf("expected no output for non-failure event, got: %s", buf.String())
		}
	})
}
