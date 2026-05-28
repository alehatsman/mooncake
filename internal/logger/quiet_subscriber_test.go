package logger

import (
	"io"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

func captureQuiet(fn func()) string {
	r, w, _ := os.Pipe()
	old := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = old
	out, _ := io.ReadAll(r)
	return string(out)
}

func makeEvent(typ events.Type, data interface{}) events.Event {
	return events.Event{Type: typ, Timestamp: time.Now(), Data: data}
}

func TestQuietSubscriber_NoOutputOnSuccess(t *testing.T) {
	q := NewQuietSubscriber()
	out := captureQuiet(func() {
		q.OnEvent(makeEvent(events.EventStepCompleted, events.StepCompletedData{Name: "Install neovim"}))
		q.OnEvent(makeEvent(events.EventRunCompleted, events.RunCompletedData{
			SuccessSteps: 5,
			ChangedSteps: 2,
			SkippedSteps: 1,
			FailedSteps:  0,
			DurationMs:   3000,
			Success:      true,
		}))
	})

	if strings.Contains(out, "Install neovim") {
		t.Error("quiet mode should not print successful steps")
	}
	if !strings.Contains(out, "RECAP") {
		t.Error("quiet mode must always print recap")
	}
	if strings.Contains(out, "FAIL") {
		t.Error("no failures, should not print FAIL")
	}
}

func TestQuietSubscriber_PrintsFailures(t *testing.T) {
	q := NewQuietSubscriber()
	out := captureQuiet(func() {
		q.OnEvent(makeEvent(events.EventStepFailed, events.StepFailedData{
			Name:         "Install pyenv",
			ErrorMessage: "exit code 1",
		}))
		q.OnEvent(makeEvent(events.EventRunCompleted, events.RunCompletedData{
			SuccessSteps: 3,
			ChangedSteps: 1,
			SkippedSteps: 0,
			FailedSteps:  1,
			DurationMs:   5000,
			Success:      false,
		}))
	})

	if !strings.Contains(out, "FAIL  Install pyenv  exit code 1") {
		t.Errorf("expected FAIL line, got:\n%s", out)
	}
	if !strings.Contains(out, "RECAP") {
		t.Error("must always print recap")
	}
	if !strings.Contains(out, "failed=1") {
		t.Error("recap must include failed count")
	}
}

func TestQuietSubscriber_RecapCounts(t *testing.T) {
	q := NewQuietSubscriber()
	out := captureQuiet(func() {
		q.OnEvent(makeEvent(events.EventRunCompleted, events.RunCompletedData{
			SuccessSteps: 10,
			ChangedSteps: 3,
			SkippedSteps: 2,
			FailedSteps:  0,
			DurationMs:   120000,
			Success:      true,
		}))
	})

	// ok = SuccessSteps - ChangedSteps = 7
	if !strings.Contains(out, "ok=7") {
		t.Errorf("expected ok=7 in recap, got: %s", out)
	}
	if !strings.Contains(out, "changed=3") {
		t.Errorf("expected changed=3 in recap, got: %s", out)
	}
	if !strings.Contains(out, "skipped=2") {
		t.Errorf("expected skipped=2 in recap, got: %s", out)
	}
	if !strings.Contains(out, "2m0s") {
		t.Errorf("expected 2m0s duration in recap, got: %s", out)
	}
}

func TestQuietSubscriber_SkipsNonLifecycleEvents(t *testing.T) {
	q := NewQuietSubscriber()
	out := captureQuiet(func() {
		q.OnEvent(makeEvent(events.EventStepStarted, events.StepStartedData{Name: "Start step"}))
		q.OnEvent(makeEvent(events.EventStepSkipped, events.StepSkippedData{Name: "Skip step"}))
	})

	if out != "" {
		t.Errorf("expected no output from non-lifecycle events, got: %q", out)
	}
}
