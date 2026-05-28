package logger

import (
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

// stepOutputEvent builds a step.stdout / step.stderr Event for
// driving the subscriber.
func stepOutputEvent(et events.Type, line string) events.Event {
	return events.Event{
		Type:      et,
		Timestamp: time.Now(),
		Data: events.StepOutputData{
			StepID: "step-1",
			Stream: string(et),
			Line:   line,
		},
	}
}

// TestConsoleSubscriber_StreamOutput_GateMatrix pins the new
// independent-gate contract for step stdout/stderr rendering:
//
//	streamOutput | logLevel<=Debug | render?
//	false        | false           | NO  (operator default)
//	true         | false           | YES (mooncake task default)
//	false        | true            | YES (legacy --log-level debug)
//	true         | true            | YES
//
// Without this matrix, a future refactor could collapse one gate
// into the other and break either the operator UX or the dev-loop UX.
func TestConsoleSubscriber_StreamOutput_GateMatrix(t *testing.T) {
	cases := []struct {
		name         string
		streamOutput bool
		logLevel     int
		wantOutput   bool
	}{
		{"operator-default-info", false, InfoLevel, false},
		{"task-info-streams", true, InfoLevel, true},
		{"operator-debug-streams", false, DebugLevel, true},
		{"task-debug-streams", true, DebugLevel, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sub := NewConsoleSubscriber(c.logLevel, "text", c.streamOutput)
			out := captureStdout(func() {
				sub.OnEvent(stepOutputEvent(events.EventStepStdout, "hello world"))
			})
			got := strings.Contains(out, "hello world")
			if got != c.wantOutput {
				t.Errorf("streamOutput=%v logLevel=%d: rendered=%v, want=%v (output=%q)",
					c.streamOutput, c.logLevel, got, c.wantOutput, out)
			}
		})
	}
}
