package logger

import (
	"encoding/json"
	"io"
	"os"
	"time"

	internalerrrors "github.com/alehatsman/mooncake/internal/errors"
	"github.com/alehatsman/mooncake/internal/events"
)

// stepErrorRecord is the JSON line written to stderr on step failure.
type stepErrorRecord struct {
	Event         string `json:"event"`
	TS            string `json:"ts"`
	Step          string `json:"step"`
	Action        string `json:"action,omitempty"`
	ExitCode      int    `json:"exit_code"`
	Stdout        string `json:"stdout,omitempty"`
	Stderr        string `json:"stderr,omitempty"`
	Hint          string `json:"hint,omitempty"`
	SuggestedStep string `json:"suggested_step,omitempty"`
}

// StderrErrorSubscriber writes a structured JSON error line to stderr on every
// step failure, regardless of the active output format.
type StderrErrorSubscriber struct {
	w io.Writer
}

// NewStderrErrorSubscriber creates a subscriber that writes to os.Stderr.
func NewStderrErrorSubscriber() *StderrErrorSubscriber {
	return &StderrErrorSubscriber{w: os.Stderr}
}

// OnEvent handles incoming events.
func (s *StderrErrorSubscriber) OnEvent(event events.Event) {
	if event.Type != events.EventStepFailed {
		return
	}
	d, ok := event.Data.(events.StepFailedData)
	if !ok {
		return
	}

	rec := stepErrorRecord{
		Event:    "step_error",
		TS:       event.Timestamp.UTC().Format(time.RFC3339),
		Step:     d.Name,
		Action:   d.Action,
		ExitCode: d.ExitCode,
		Stdout:   d.Stdout,
		Stderr:   d.Stderr,
	}

	hint := internalerrrors.InferHint(d.Stderr)
	if hint.Text != "" {
		rec.Hint = hint.Text
		rec.SuggestedStep = hint.SuggestedStep
	}

	_ = json.NewEncoder(s.w).Encode(rec)
}

// Close implements the Subscriber interface.
func (s *StderrErrorSubscriber) Close() {}
