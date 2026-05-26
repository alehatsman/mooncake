package pilot

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/events"
)

// emitStdoutLine is a tiny helper so the table tests stay readable.
func emitStdoutLine(c *stdoutCapture, stepID, line string, lineNum int) {
	c.OnEvent(events.Event{
		Type:      events.EventStepStdout,
		Timestamp: time.Now(),
		Data: events.StepOutputData{
			StepID:     stepID,
			Stream:     "stdout",
			Line:       line,
			LineNumber: lineNum,
		},
	})
}

func emitStarted(c *stdoutCapture, stepID, action string) {
	c.OnEvent(events.Event{
		Type:      events.EventStepStarted,
		Timestamp: time.Now(),
		Data: events.StepStartedData{
			StepID: stepID,
			Name:   stepID,
			Action: action,
		},
	})
}

func emitCompleted(c *stdoutCapture, stepID string) {
	c.OnEvent(events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID: stepID,
			Name:   stepID,
		},
	})
}

// emitCompletedWithStdout simulates a cmd-action step.completed event
// that carries the buffered stdout via the Result map (cmd doesn't
// publish per-line step.stdout events the way shell does).
func emitCompletedWithStdout(c *stdoutCapture, stepID, stdout string) {
	c.OnEvent(events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID: stepID,
			Name:   stepID,
			Result: map[string]interface{}{
				"stdout": stdout,
			},
		},
	})
}

// TestStdoutCapture_LastCmdStepWins verifies that when multiple
// cmd/shell steps complete in one plan, Last() returns the LAST
// one's stdout — the spec semantics ("the LAST cmd/shell step's
// stdout is what LastStepStdout should hold at end of apply").
func TestStdoutCapture_LastCmdStepWins(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "s1", "cmd")
	emitStdoutLine(c, "s1", "first step output", 1)
	emitCompleted(c, "s1")

	emitStarted(c, "s2", "shell")
	emitStdoutLine(c, "s2", "second step output", 1)
	emitCompleted(c, "s2")

	got := c.Last()
	if !strings.Contains(got, "second step output") {
		t.Errorf("Last() should hold the LAST shell step's stdout; got %q", got)
	}
	if strings.Contains(got, "first step output") {
		t.Errorf("Last() should overwrite earlier cmd step stdout; got %q", got)
	}
}

// TestStdoutCapture_NonShellStepsIgnored covers the spec rule that
// only cmd/shell-family steps feed LastStepStdout. A file/template/
// assert step completing with stdout should NOT overwrite an earlier
// cmd step's capture — the model would otherwise see internal status
// noise instead of the action it cared about.
func TestStdoutCapture_NonShellStepsIgnored(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "cmd1", "cmd")
	emitStdoutLine(c, "cmd1", "git status output", 1)
	emitCompleted(c, "cmd1")

	emitStarted(c, "file1", "file")
	emitStdoutLine(c, "file1", "file-action chatter", 1)
	emitCompleted(c, "file1")

	got := c.Last()
	if !strings.Contains(got, "git status output") {
		t.Errorf("non-shell step should not overwrite cmd capture; got %q", got)
	}
	if strings.Contains(got, "file-action chatter") {
		t.Errorf("file-action stdout must not appear in LastStepStdout; got %q", got)
	}
}

// TestStdoutCapture_EmptyWhenNoShellSteps confirms the zero-value
// case: a plan with only file/template/assert actions leaves
// LastStepStdout empty, so the prompt renderer omits the block.
func TestStdoutCapture_EmptyWhenNoShellSteps(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "f1", "file")
	emitStdoutLine(c, "f1", "noise", 1)
	emitCompleted(c, "f1")

	if got := c.Last(); got != "" {
		t.Errorf("Last() should be empty when no cmd/shell steps ran; got %q", got)
	}
}

// TestStdoutCapture_TailTruncates is the core 4 KiB-tail contract.
// Output longer than the cap MUST keep the tail (where the diagnostic
// / final-state lives) and drop the head — mirroring the executor's
// truncateTail policy for step-failure stdout/stderr.
func TestStdoutCapture_TailTruncates(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "s1", "cmd")
	// 5 KiB of "A" followed by a sentinel tail line.
	bulk := strings.Repeat("A", 5*1024)
	sentinel := "FINAL-LINE-SHOULD-SURVIVE"
	emitStdoutLine(c, "s1", bulk+sentinel, 1)
	emitCompleted(c, "s1")

	got := c.Last()
	if !strings.Contains(got, sentinel) {
		t.Errorf("tail truncation dropped the sentinel; got tail=%q", got[max(0, len(got)-80):])
	}
	if !strings.Contains(got, "bytes truncated") {
		t.Errorf("truncation marker missing — caller can't tell output was dropped; got prefix=%q", got[:min(80, len(got))])
	}
	if len(got) > pilotStdoutTailBytes+128 {
		// Allow for the "... <N bytes truncated> ...\n" prefix overhead;
		// 128 bytes is a generous bound on that marker line.
		t.Errorf("Last() = %d bytes, want <= %d (cap + small marker overhead)", len(got), pilotStdoutTailBytes+128)
	}
}

// TestStdoutCapture_CmdActionResultMap covers the cmd-action path:
// `cmd` buffers stdout into result.Stdout and emits no per-line
// step.stdout events. The executor surfaces it via
// StepCompletedData.Result["stdout"]. Without this branch the manual
// `mooncake pilot run --goal "show git status"` loop wouldn't see any
// captured output, because cmd is the action the LLM almost always
// picks for diagnostic shell-out.
func TestStdoutCapture_CmdActionResultMap(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "s1", "cmd")
	// No emitStdoutLine — cmd action doesn't stream.
	emitCompletedWithStdout(c, "s1", "On branch master\nnothing to commit, working tree clean\n")

	got := c.Last()
	if !strings.Contains(got, "On branch master") {
		t.Errorf("cmd-action stdout from Result map missing; got %q", got)
	}
	if !strings.Contains(got, "working tree clean") {
		t.Errorf("cmd-action stdout missing tail; got %q", got)
	}
}

// TestStdoutCapture_NoStdoutNoOverwrite covers the edge case where a
// cmd step runs but emits no stdout (e.g. `git status -s` on a clean
// tree). The capture for that step is nil, so Last() should NOT be
// overwritten to empty — the prior step's capture (if any) wins.
// This matches the spec's "the last cmd/shell step's stdout" wording
// only when there IS stdout to record; otherwise the field shouldn't
// be wiped.
func TestStdoutCapture_NoStdoutNoOverwrite(t *testing.T) {
	c := newStdoutCapture(nil)

	emitStarted(c, "s1", "cmd")
	emitStdoutLine(c, "s1", "useful output", 1)
	emitCompleted(c, "s1")

	emitStarted(c, "s2", "cmd")
	// No stdout emitted for s2.
	emitCompleted(c, "s2")

	if got := c.Last(); !strings.Contains(got, "useful output") {
		t.Errorf("silent cmd step should not wipe earlier capture; got %q", got)
	}
}

// TestStdoutCapture_PrintsCmdOutput covers the Part-1 UX hook: when a
// writer is wired, cmd-action buffered stdout (from the Result map,
// since cmd doesn't stream per-line) is printed to the operator's
// terminal. Without this the `git status` output ran invisibly even
// though the cmd step ran — the original user complaint that
// motivated this work.
func TestStdoutCapture_PrintsCmdOutput(t *testing.T) {
	var buf bytes.Buffer
	c := newStdoutCapture(&buf)

	emitStarted(c, "s1", "cmd")
	emitCompletedWithStdout(c, "s1", "On branch master")

	if !strings.Contains(buf.String(), "On branch master") {
		t.Errorf("cmd-action stdout should be printed to writer; got %q", buf.String())
	}
}

// TestStdoutCapture_DoesNotDoublePrintShellOutput pins the asymmetry
// in the printer: shell-action stdout already streamed through the
// ConsoleSubscriber as per-line step.stdout events, so reprinting it
// at step.completed would double every line on the operator's
// terminal. The capture subscriber prints ONLY when the stdout came
// from the Result map (the cmd-action path).
func TestStdoutCapture_DoesNotDoublePrintShellOutput(t *testing.T) {
	var buf bytes.Buffer
	c := newStdoutCapture(&buf)

	emitStarted(c, "s1", "shell")
	emitStdoutLine(c, "s1", "streamed line", 1)
	emitCompleted(c, "s1")

	if strings.Contains(buf.String(), "streamed line") {
		t.Errorf("shell-action streamed output should not be re-printed by capture; got %q", buf.String())
	}
}
