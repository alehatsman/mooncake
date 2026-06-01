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

// pilotStepSummaryLineBytes caps a single per-step summary line. Most
// handler Reasons are short ("wrote 16 bytes to <path>", "service
// already in desired state"), but file-replace / template handlers
// can dump multi-line diffs into Reason — those want to be one
// dense line in the next prompt, not a wall of text. Truncating per-
// line keeps the aggregate manageable when --style plan emits a 30-
// step plan.
const pilotStepSummaryLineBytes = 240

// pilotStepSummariesMax bounds the per-iteration summary slice so a
// long plan can't push the LAST ITERATION block past the model's
// per-message budget. 30 mirrors the schema's "Keep plans small
// (<= 30 steps)" guidance from prompt.go.
const pilotStepSummariesMax = 30

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
	// startedByStep keeps the started-event metadata we need at
	// completion time (action, step name). StepCompletedData omits
	// both — only StepStartedData carries them — so we forward them.
	startedByStep map[string]startedInfo
	// stdoutByStep accumulates stdout lines per step ID, keyed by the
	// same step.started -> step.completed window.
	stdoutByStep map[string]*strings.Builder
	// lastStdout is the most-recent cmd/shell step's stdout snapshot
	// (already 4 KiB-tail-truncated when stored). The "last" semantics
	// match the spec: each step.completed for a cmd/shell action
	// overwrites the field, so the final cmd/shell step in the plan
	// wins. Empty when no cmd/shell step has completed.
	lastStdout string
	// stepSummaries is the per-iteration list of one-line summaries
	// the next prompt's LAST ITERATION block renders. One entry per
	// step.completed event the capture observed (across ALL action
	// types, not just cmd/shell — that's the whole point of this
	// field). Capped at pilotStepSummariesMax; extra completions are
	// dropped with a single "... N more" sentinel appended.
	stepSummaries []string
	// stepSummariesDropped counts step.completed events that arrived
	// after pilotStepSummariesMax was reached, so Summaries() can
	// emit the trailing "... N more steps omitted" line exactly once.
	stepSummariesDropped int
	// failedStep is the LAST step.failed event seen this iteration (a
	// transaction stops at its first failing step, so there's normally
	// exactly one). nil until a step fails. Fed into the next prompt so
	// the planner sees what broke and adapts instead of re-proposing the
	// same failing step (#71).
	failedStep *FailedStepInfo
}

// startedInfo holds the bits of StepStartedData a step.completed
// handler needs to render a useful summary line.
type startedInfo struct {
	action string
	name   string
}

// newStdoutCapture allocates a fresh capture subscriber. out may be
// nil for tests that only assert capture semantics. Callers Subscribe
// it to the publisher before executor.Start and read Last() after
// publisher.Close() returns (or after a Flush()).
func newStdoutCapture(out io.Writer) *stdoutCapture {
	return &stdoutCapture{
		out:           out,
		startedByStep: make(map[string]startedInfo),
		stdoutByStep:  make(map[string]*strings.Builder),
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
		c.startedByStep[data.StepID] = startedInfo{action: data.Action, name: data.Name}
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
		c.handleCompleted(data)

	case events.EventStepFailed:
		data, ok := event.Data.(events.StepFailedData)
		if !ok {
			return
		}
		// Re-cap stderr to the prompt budget (the event carries up to
		// 64 KiB; the next prompt only wants the actionable tail).
		c.mu.Lock()
		c.failedStep = &FailedStepInfo{
			Name:     data.Name,
			Action:   data.Action,
			ExitCode: data.ExitCode,
			Stderr:   truncateTail(data.Stderr, pilotStdoutTailBytes),
			Message:  data.ErrorMessage,
		}
		c.mu.Unlock()
	}
}

// FailedStep returns the last step.failed captured this iteration, or nil if
// no step failed. Read after publisher.Close()/Flush() like Last().
func (c *stdoutCapture) FailedStep() *FailedStepInfo {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.failedStep
}

// handleCompleted is the step.completed branch of OnEvent. Split out so
// the OnEvent dispatcher stays under gocyclo budget — the completion
// path does three things at once (stdout capture, terminal print,
// summary line) and inlining them in OnEvent pushed it past 35.
func (c *stdoutCapture) handleCompleted(data events.StepCompletedData) {
	c.mu.Lock()
	info := c.startedByStep[data.StepID]
	buf := c.stdoutByStep[data.StepID]
	// Free per-step buffers as soon as the step completes; a long
	// plan with many shell steps shouldn't accumulate every line in
	// memory after we've taken the snapshot.
	delete(c.startedByStep, data.StepID)
	delete(c.stdoutByStep, data.StepID)
	c.mu.Unlock()

	// Every completed step contributes a one-line summary (regardless
	// of action) so non-cmd/shell actions surface in the next prompt
	// — file.write/template/pkg/os.service completing with no signal
	// to the LLM was the reason this work exists.
	c.recordSummary(info, data)

	if !isShellFamily(info.action) {
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

// recordSummary appends a one-line summary for the just-completed step
// to c.stepSummaries, bounded by pilotStepSummariesMax. Designed to be
// the only place that mutates stepSummaries / stepSummariesDropped, so
// the slice's per-iteration semantics live in one place.
func (c *stdoutCapture) recordSummary(info startedInfo, data events.StepCompletedData) {
	line := summarizeStep(info.action, info.name, data.Changed, data.Result)
	if line == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stepSummaries) >= pilotStepSummariesMax {
		c.stepSummariesDropped++
		return
	}
	c.stepSummaries = append(c.stepSummaries, line)
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

// Summaries returns the per-step one-line summaries observed during
// the iteration, in step.completed order. Each line is independently
// truncated to pilotStepSummaryLineBytes; the slice is capped at
// pilotStepSummariesMax with a trailing "... N more steps omitted"
// sentinel when more completions arrived. Returns a freshly-allocated
// copy — the caller can hold the slice past publisher.Close() without
// racing the next iteration's capture.
func (c *stdoutCapture) Summaries() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.stepSummaries) == 0 && c.stepSummariesDropped == 0 {
		return nil
	}
	out := make([]string, 0, len(c.stepSummaries)+1)
	out = append(out, c.stepSummaries...)
	if c.stepSummariesDropped > 0 {
		out = append(out, fmt.Sprintf("... %d more step(s) omitted", c.stepSummariesDropped))
	}
	return out
}

// summarizeStep builds the one-line per-step description the next
// iteration's prompt feeds back to the model. The line format is
// stable across all action types:
//
//	<status> <action>[/<name>]: <reason-or-fallback>
//
// Priority order for the "reason" segment:
//
//  1. result["reason"] — handler-supplied one-liner (file/template/
//     copy/download/service all set this for their changed and no-op
//     branches; see internal/actions/*).
//  2. result["stdout"] — first non-empty line for cmd/shell that
//     completed silently (cmd never streams; a short status line
//     buffered into Stdout becomes the summary).
//  3. "<changed=true|false>" — generic last-resort signal so the
//     LLM always sees SOMETHING, even for handlers that set neither.
//
// The line is then clamped to pilotStepSummaryLineBytes — handler
// Reasons can include multi-line diffs (template, file_replace), and
// the model only needs the gist for replan-or-stop decisions.
func summarizeStep(action, name string, changed bool, result map[string]interface{}) string {
	status := summaryStatus(changed, result)
	reason := summaryReason(action, result, changed)

	var b strings.Builder
	b.WriteString(status)
	b.WriteByte(' ')
	if action != "" {
		b.WriteString(action)
	} else {
		b.WriteString("step")
	}
	if name != "" && name != action {
		b.WriteByte('[')
		b.WriteString(name)
		b.WriteByte(']')
	}
	b.WriteString(": ")
	b.WriteString(reason)

	line := b.String()
	if len(line) > pilotStepSummaryLineBytes {
		const ellipsis = "..."
		line = line[:pilotStepSummaryLineBytes-len(ellipsis)] + ellipsis
	}
	// Collapse internal newlines so a multi-line Reason renders as one
	// dense summary line. " | " is the same separator the executor
	// uses elsewhere for inline multi-line diagnostics.
	line = strings.ReplaceAll(line, "\r\n", "\n")
	line = strings.ReplaceAll(line, "\n", " | ")
	return line
}

// summaryStatus extracts the result status from a step.completed
// Result map (populated by executor.Result.ToMap()). Falls back to
// the changed flag when "status" is missing.
func summaryStatus(changed bool, result map[string]interface{}) string {
	if result != nil {
		if s, ok := result["status"].(string); ok && s != "" {
			return s
		}
	}
	if changed {
		return "changed"
	}
	return "ok"
}

// summaryReason picks the best human-readable description for a
// completed step, in the order documented on summarizeStep. Returns a
// non-empty string in every path so the caller doesn't have to
// special-case the no-info case.
func summaryReason(action string, result map[string]interface{}, changed bool) string {
	if result != nil {
		if r, ok := result["reason"].(string); ok && r != "" {
			return r
		}
		if isShellFamily(action) {
			if s, ok := stdoutFromResult(result); ok {
				return firstNonEmptyLine(s)
			}
		}
	}
	if changed {
		return "changed (no detail)"
	}
	return "no change"
}

// firstNonEmptyLine returns the first non-blank line of s, trimmed.
// Used for the cmd/shell fallback when the handler set no Reason but
// the buffered stdout has a useful summary at the top (e.g. `git
// status -s` first line).
func firstNonEmptyLine(s string) string {
	for _, ln := range strings.Split(s, "\n") {
		trimmed := strings.TrimSpace(ln)
		if trimmed != "" {
			return trimmed
		}
	}
	return ""
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
