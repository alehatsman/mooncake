package logger

import (
	"fmt"
	"sync"

	"github.com/alehatsman/mooncake/internal/events"
)

type failureEntry struct {
	name  string
	error string
}

// QuietSubscriber suppresses all output except failures and the final recap.
// Designed for CI pipelines and scripting where only failures matter.
type QuietSubscriber struct {
	mu       sync.Mutex
	failures []failureEntry
}

// NewQuietSubscriber creates a new QuietSubscriber.
func NewQuietSubscriber() *QuietSubscriber {
	return &QuietSubscriber{}
}

// OnEvent handles an incoming event.
func (q *QuietSubscriber) OnEvent(event events.Event) {
	switch event.Type {
	case events.EventStepFailed:
		d, ok := event.Data.(events.StepFailedData)
		if !ok {
			return
		}
		q.mu.Lock()
		q.failures = append(q.failures, failureEntry{name: d.Name, error: d.ErrorMessage})
		q.mu.Unlock()

	case events.EventRunCompleted:
		d, ok := event.Data.(events.RunCompletedData)
		if !ok {
			return
		}
		q.mu.Lock()
		failures := q.failures
		q.mu.Unlock()

		for _, f := range failures {
			fmt.Printf("FAIL  %s  %s\n", f.name, f.error)
		}
		if len(failures) > 0 {
			fmt.Println()
		}

		ok2 := d.SuccessSteps - d.ChangedSteps
		if ok2 < 0 {
			ok2 = 0
		}
		if d.RevertedSteps > 0 {
			fmt.Printf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d  reverted=%d  %s\n",
				ok2, d.ChangedSteps, d.SkippedSteps, d.FailedSteps,
				d.RevertedSteps, formatDuration(d.DurationMs))
		} else {
			fmt.Printf("RECAP  ok=%d  changed=%d  skipped=%d  failed=%d  %s\n",
				ok2, d.ChangedSteps, d.SkippedSteps, d.FailedSteps,
				formatDuration(d.DurationMs))
		}
	}
}

// Close implements the Subscriber interface.
func (q *QuietSubscriber) Close() {}
