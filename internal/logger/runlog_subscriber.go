package logger

import (
	"path/filepath"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/runlog"
)

// RunLogSubscriber appends a compact run record to ~/.mooncake/runs.jsonl on
// EventRunCompleted. Write failures are silently dropped so they never fail a run.
type RunLogSubscriber struct {
	config string
}

// NewRunLogSubscriber creates a subscriber that records runs under the given config basename.
func NewRunLogSubscriber(configPath string) *RunLogSubscriber {
	return &RunLogSubscriber{config: filepath.Base(configPath)}
}

// OnEvent handles incoming events.
func (r *RunLogSubscriber) OnEvent(event events.Event) {
	if event.Type != events.EventRunCompleted {
		return
	}
	d, ok := event.Data.(events.RunCompletedData)
	if !ok {
		return
	}
	okCount := d.SuccessSteps - d.ChangedSteps
	if okCount < 0 {
		okCount = 0
	}
	_ = runlog.Append(runlog.Entry{
		TS:         time.Now().UTC(),
		Config:     r.config,
		Changed:    d.ChangedSteps,
		Ok:         okCount,
		Skipped:    d.SkippedSteps,
		Failed:     d.FailedSteps,
		DurationMs: d.DurationMs,
	})
}

// Close implements the Subscriber interface.
func (r *RunLogSubscriber) Close() {}
