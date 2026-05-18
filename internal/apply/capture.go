package apply

import (
	"sync"

	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// captureSubscriber records the run's event tail and per-step
// results so Runner.Run can build a *KernelResult after
// executor.Start returns. It piggybacks on the publisher rather
// than threading a new return channel through the executor —
// minimum touch to the executor surface.
//
// Step results are taken from the executor.Result attached to
// each step.completed event by the executor (under the "result"
// key of StepCompletedData.Result). Cross-checked with the
// executor's CurrentResult capture; the publisher path is
// authoritative because it observes the same outcome the
// recap subscribers see.
type captureSubscriber struct {
	mu      sync.Mutex
	events  []events.Event
	stepIDs []string // in observed order
	step    map[string]events.StepStartedData
	result  map[string]*executor.Result
	plan    events.PlanLoadedData
	run     events.RunCompletedData
}

func newCaptureSubscriber() *captureSubscriber {
	return &captureSubscriber{
		step:   make(map[string]events.StepStartedData),
		result: make(map[string]*executor.Result),
	}
}

// OnEvent satisfies events.Subscriber. Every event is appended to
// the audit tail. Plan/step/run events are also indexed for
// post-run reconstruction.
func (c *captureSubscriber) OnEvent(e events.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.events = append(c.events, e)
	switch e.Type {
	case events.EventPlanLoaded:
		if d, ok := e.Data.(events.PlanLoadedData); ok {
			c.plan = d
		}
	case events.EventStepStarted:
		if d, ok := e.Data.(events.StepStartedData); ok {
			c.step[d.StepID] = d
			c.stepIDs = append(c.stepIDs, d.StepID)
		}
	case events.EventStepCompleted:
		if d, ok := e.Data.(events.StepCompletedData); ok {
			c.result[d.StepID] = decodeResult(d.Result, d.Changed)
		}
	case events.EventStepSkipped:
		// StepSkippedData carries no result payload; synthesise
		// a Result with Skipped=true so Reverse() correctly
		// skips reversal (Changed=false).
		if d, ok := e.Data.(events.StepSkippedData); ok {
			c.result[d.StepID] = &executor.Result{
				Skipped: true,
				Reason:  d.Reason,
			}
		}
	case events.EventStepFailed:
		// StepFailedData carries error/exit/stdout/stderr but no
		// Changed flag. Reverse() should never reverse a failed
		// step (idempotency assumption is broken), so Changed=false.
		if d, ok := e.Data.(events.StepFailedData); ok {
			c.result[d.StepID] = &executor.Result{
				Failed: true,
				Rc:     d.ExitCode,
				Stdout: d.Stdout,
				Stderr: d.Stderr,
				Reason: d.ErrorMessage,
			}
		}
	case events.EventRunCompleted:
		if d, ok := e.Data.(events.RunCompletedData); ok {
			c.run = d
		}
	}
}

// Close satisfies events.Subscriber. No resources to release.
func (c *captureSubscriber) Close() {}

// decodeResult rebuilds an executor.Result from the map payload
// the executor attaches to step.completed events. Falls back to
// a minimal Result with just the Changed flag when the map is
// nil or lacks the typed fields.
func decodeResult(m map[string]interface{}, changed bool) *executor.Result {
	r := &executor.Result{Changed: changed}
	if m == nil {
		return r
	}
	if v, ok := m["stdout"].(string); ok {
		r.Stdout = v
	}
	if v, ok := m["stderr"].(string); ok {
		r.Stderr = v
	}
	if v, ok := m["rc"].(int); ok {
		r.Rc = v
	}
	if v, ok := m["failed"].(bool); ok {
		r.Failed = v
	}
	if v, ok := m["skipped"].(bool); ok {
		r.Skipped = v
	}
	if v, ok := m["would_change"].(bool); ok {
		r.WouldChange = v
	}
	if v, ok := m["reason"].(string); ok {
		r.Reason = v
	}
	if v, ok := m["checkable"].(bool); ok {
		r.Checkable = v
	}
	if v, ok := m["data"].(map[string]interface{}); ok {
		r.Data = v
	}
	return r
}
