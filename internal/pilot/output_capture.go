package pilot

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/events"
)

// pilotStdoutTailBytes caps the captured stdout fed back into the next
// iteration's prompt. 4 KiB keeps the tail well under the model's
// per-message budget while still carrying typical command output (a
// `git status`, a small `ls`, a short test summary). Mirrors the
// truncate-tail policy executor.go uses for step-failure stdout/stderr:
// keep the END (where the diagnostic / final-state lives), drop the
// HEAD.
const pilotStdoutTailBytes = 4096

// stdoutCapture is a pilot-internal events.Subscriber that does TWO
// things at once during executor.Start:
//
//  1. Remembers the LAST cmd/shell-family step's captured stdout so
//     the next iteration's PlanInput.LastIteration.LastStepStdout
//     carries it forward (loop-termination half of the pilot output-
//     capture work — without this the model can't tell the goal is
//     already answered and re-proposes the same diagnostic step
//     forever).
//
//  2. Prints buffered cmd-action stdout to the operator's terminal at
//     step.completed time. The console subscriber already streams
//     shell-action per-line step.stdout events; the cmd action
//     doesn't emit those (it buffers into result.Stdout and surfaces
//     it only via StepCompletedData.Result["stdout"]), so without this
//     hook a `git status` cmd step runs invisibly. The user-facing
//     reason this subscriber exists at all is "we should use some
//     output capture for the step and be able to see it from user
//     perspective" — printing cmd output closes that gap on the most
//     common LLM-emitted action.
//
// Concurrency: the executor delivers events via the events.Publisher
// goroutine pool, so OnEvent runs on a non-pilot goroutine. All
// access to internal state is mu-guarded; the writer is set at
// construction and not reassigned, so reads of it don't race.
type stdoutCapture struct {
	mu sync.Mutex
	// out receives buffered cmd-action stdout at step.completed. Set
	// at construction. Nil disables printing (used by tests so they
	// don't pollute test output with subprocess captures).
	out io.Writer
	// actionByStep maps step IDs to action names, populated on
	// step.started. StepCompletedData doesn't carry the action name —
	// only StepStartedData does — so we track the mapping forward.
	actionByStep map[string]string
	// stdoutByStep accumulates stdout lines per step ID, keyed by the
	// same step.started -> step.completed window.
	stdoutByStep map[string]*strings.Builder
	// lastStdout is the most-recent cmd/shell step's stdout snapshot
	// (already 4 KiB-tail-truncated when stored). The "last" semantics
	// match the spec: each step.completed for a cmd/shell action
	// overwrites the field, so the final cmd/shell step in the plan
	// wins. Empty when no cmd/shell step has completed.
	lastStdout string
}

// newStdoutCapture allocates a fresh capture subscriber. out may be
// nil for tests that only assert capture semantics. Callers Subscribe
// it to the publisher before executor.Start and read Last() after
// publisher.Close() returns (or after a Flush()).
func newStdoutCapture(out io.Writer) *stdoutCapture {
	return &stdoutCapture{
		out:          out,
		actionByStep: make(map[string]string),
		stdoutByStep: make(map[string]*strings.Builder),
	}
}

// OnEvent implements events.Subscriber.
func (c *stdoutCapture) OnEvent(event events.Event) {
	switch event.Type {
	case events.EventStepStarted:
		data, ok := event.Data.(events.StepStartedData)
		if !ok {
			return
		}
		c.mu.Lock()
		c.actionByStep[data.StepID] = data.Action
		c.mu.Unlock()

	case events.EventStepStdout:
		data, ok := event.Data.(events.StepOutputData)
		if !ok {
			return
		}
		c.mu.Lock()
		buf, exists := c.stdoutByStep[data.StepID]
		if !exists {
			buf = &strings.Builder{}
			c.stdoutByStep[data.StepID] = buf
		}
		buf.WriteString(data.Line)
		buf.WriteString("\n")
		c.mu.Unlock()

	case events.EventStepCompleted:
		data, ok := event.Data.(events.StepCompletedData)
		if !ok {
			return
		}
		c.mu.Lock()
		action := c.actionByStep[data.StepID]
		buf := c.stdoutByStep[data.StepID]
		// Free per-step buffers as soon as the step completes; a long
		// plan with many shell steps shouldn't accumulate every line in
		// memory after we've taken the snapshot.
		delete(c.actionByStep, data.StepID)
		delete(c.stdoutByStep, data.StepID)
		c.mu.Unlock()
		if !isShellFamily(action) {
			return
		}
		// Two stdout sources, in priority order:
		//   1. Streamed line events accumulated in buf (shell action
		//      publishes per-line step.stdout events as the subprocess
		//      runs).
		//   2. step.completed.Result["stdout"] (cmd action buffers the
		//      whole stdout and only surfaces it on completion; no per-
		//      line events).
		// Either path can produce non-empty output; we pick the buf
		// path first because it preserves the streaming order, and fall
		// back to the result map for cmd-style buffered output.
		var captured string
		var fromResultMap bool
		if buf != nil && buf.Len() > 0 {
			captured = buf.String()
		} else if s, ok := stdoutFromResult(data.Result); ok {
			captured = s
			fromResultMap = true
		}
		if captured == "" {
			return
		}
		// Print the captured stdout for cmd-action steps (shell-action
		// already streamed line-by-line through the ConsoleSubscriber;
		// re-printing here would double the output). The capture path
		// uses the result map ONLY for cmd, so fromResultMap acts as
		// the cmd-action discriminator without re-typing the action
		// string.
		if fromResultMap && c.out != nil {
			_, _ = fmt.Fprintln(c.out, captured)
		}
		c.mu.Lock()
		c.lastStdout = truncateTail(captured, pilotStdoutTailBytes)
		c.mu.Unlock()
	}
}

// stdoutFromResult extracts the "stdout" key from a step.completed
// Result map. The executor populates this from result.ToMap() (see
// internal/executor/result.go:157); cmd action steps land here
// because they buffer the whole command output instead of publishing
// per-line step.stdout events the way shell does. Returns ("", false)
// when the key is missing or non-string so callers can distinguish
// "no stdout to capture" from "captured empty string".
func stdoutFromResult(result map[string]interface{}) (string, bool) {
	if result == nil {
		return "", false
	}
	v, ok := result["stdout"]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}

// Close implements events.Subscriber. No-op — all resources are
// in-memory maps that the surrounding pilot iteration drops.
func (c *stdoutCapture) Close() {}

// Last returns the captured stdout (4 KiB tail-truncated) from the
// most-recent cmd/shell-family step's completion, or "" if none ran.
// Safe to call after publisher.Close() / Flush().
func (c *stdoutCapture) Last() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.lastStdout
}

// isShellFamily decides which action names produce stdout the model
// should see in the next iteration's prompt. Kept narrow on purpose:
// `cmd` (typed argv) and `shell` (free-form interpreter script) are
// the two action types that actually run user-visible programs and
// publish step.stdout events. File/template/assert actions can emit
// stdout too in principle, but their output is internal status —
// surfacing it would clutter the next prompt without helping the
// model decide.
func isShellFamily(action string) bool {
	switch action {
	case "cmd", "shell":
		return true
	default:
		return false
	}
}

// truncateTail returns the last maxLen bytes of s, prefixed with an
// ellipsis line so the model sees that earlier output was dropped.
// Mirrors internal/executor.truncateTail's contract (kept private to
// the executor package); duplicated here so pilot doesn't reach into
// executor internals just to share an 8-line helper. If the executor
// helper ever gets promoted to a shared package, swap this for the
// import.
func truncateTail(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	dropped := len(s) - maxLen
	return fmt.Sprintf("... <%d bytes truncated> ...\n%s", dropped, s[dropped:])
}
