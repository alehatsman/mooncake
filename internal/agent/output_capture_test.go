package agent

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

func emitFailed(c *stdoutCapture, name, action string, exitCode int, stderr, msg string) {
	c.OnEvent(events.Event{
		Type:      events.EventStepFailed,
		Timestamp: time.Now(),
		Data: events.StepFailedData{
			StepID:       name,
			Name:         name,
			Action:       action,
			ExitCode:     exitCode,
			Stderr:       stderr,
			ErrorMessage: msg,
		},
	})
}

// TestStdoutCapture_FailedStep confirms a step.failed event is captured into
// FailedStep() so the loop can feed it back to the planner (#71). Without
// this the failing step's stderr/exit code never reaches the next prompt.
func TestStdoutCapture_FailedStep(t *testing.T) {
	c := newStdoutCapture(nil)
	if c.FailedStep() != nil {
		t.Fatalf("FailedStep() = %+v before any failure, want nil", c.FailedStep())
	}

	emitFailed(c, "run migration", "shell", 7, "psql: connection refused\n", "command failed: exit status 7")

	fs := c.FailedStep()
	if fs == nil {
		t.Fatal("FailedStep() = nil after step.failed, want populated")
	}
	if fs.Name != "run migration" || fs.Action != "shell" || fs.ExitCode != 7 {
		t.Errorf("FailedStep = %+v, want name/action/exit run migration/shell/7", fs)
	}
	if !strings.Contains(fs.Stderr, "connection refused") {
		t.Errorf("FailedStep.Stderr = %q, want it to carry the captured stderr", fs.Stderr)
	}
}

// TestStdoutCapture_FailedStepStderrTruncated confirms the captured stderr is
// re-capped to the prompt budget (the event itself carries up to 64 KiB).
func TestStdoutCapture_FailedStepStderrTruncated(t *testing.T) {
	c := newStdoutCapture(nil)
	big := strings.Repeat("x", agentStdoutTailBytes*2)
	emitFailed(c, "s", "cmd", 1, big, "boom")
	fs := c.FailedStep()
	if fs == nil {
		t.Fatal("FailedStep() = nil, want populated")
	}
	// truncateTail keeps maxLen tail bytes plus a "<N bytes truncated>"
	// marker, so the result is far smaller than the 8 KiB input and carries
	// the marker. The point is the 64 KiB event payload doesn't reach the
	// prompt verbatim.
	if len(fs.Stderr) >= len(big) {
		t.Errorf("FailedStep.Stderr len = %d, want it truncated below the %d-byte input", len(fs.Stderr), len(big))
	}
	if !strings.Contains(fs.Stderr, "truncated") {
		t.Errorf("FailedStep.Stderr should carry the truncation marker; got prefix %q", fs.Stderr[:40])
	}
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
	if len(got) > agentStdoutTailBytes+128 {
		// Allow for the "... <N bytes truncated> ...\n" prefix overhead;
		// 128 bytes is a generous bound on that marker line.
		t.Errorf("Last() = %d bytes, want <= %d (cap + small marker overhead)", len(got), agentStdoutTailBytes+128)
	}
}

// TestStdoutCapture_CmdActionResultMap covers the cmd-action path:
// `cmd` buffers stdout into result.Stdout and emits no per-line
// step.stdout events. The executor surfaces it via
// StepCompletedData.Result["stdout"]. Without this branch the manual
// `mooncake agent run --goal "show git status"` loop wouldn't see any
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

// emitCompletedWithDetail builds a step.completed event with caller-
// chosen action name, changed flag, and Result map. Used by the
// per-step summary tests where the cmd/shell path isn't relevant.
func emitStartedNamed(c *stdoutCapture, stepID, name, action string) {
	c.OnEvent(events.Event{
		Type:      events.EventStepStarted,
		Timestamp: time.Now(),
		Data: events.StepStartedData{
			StepID: stepID,
			Name:   name,
			Action: action,
		},
	})
}

func emitCompletedDetail(c *stdoutCapture, stepID string, changed bool, result map[string]interface{}) {
	c.OnEvent(events.Event{
		Type:      events.EventStepCompleted,
		Timestamp: time.Now(),
		Data: events.StepCompletedData{
			StepID:  stepID,
			Name:    stepID,
			Changed: changed,
			Result:  result,
		},
	})
}

// TestStdoutCapture_SummariesAllActionTypes covers the core gap that
// motivated this work (PICKUP item #1): non-cmd/shell actions complete
// with no signal the LLM can read. Summaries() must produce one line
// per completed step regardless of action — file.write, pkg.install,
// os.service, etc. all need to appear in the next prompt's
// LAST ITERATION block.
func TestStdoutCapture_SummariesAllActionTypes(t *testing.T) {
	c := newStdoutCapture(nil)

	// file.write — handler-set Reason carries the bytes-written detail.
	emitStartedNamed(c, "s1", "write greeting", "file.write")
	emitCompletedDetail(c, "s1", true, map[string]interface{}{
		"status": "changed",
		"reason": "wrote 16 bytes to /tmp/hello",
	})

	// pkg.install — no Reason, fall back to status + action.
	emitStartedNamed(c, "s2", "install jq", "pkg.install")
	emitCompletedDetail(c, "s2", true, map[string]interface{}{
		"status": "changed",
	})

	// os.service — no-op (already in desired state).
	emitStartedNamed(c, "s3", "ensure sshd running", "os.service")
	emitCompletedDetail(c, "s3", false, map[string]interface{}{
		"status": "ok",
		"reason": "service already in desired state",
	})

	got := c.Summaries()
	if len(got) != 3 {
		t.Fatalf("Summaries() len = %d, want 3 (one per completed step)", len(got))
	}
	if !strings.Contains(got[0], "file.write") || !strings.Contains(got[0], "wrote 16 bytes") {
		t.Errorf("file.write summary missing handler Reason; got %q", got[0])
	}
	if !strings.Contains(got[1], "pkg.install") || !strings.Contains(got[1], "changed") {
		t.Errorf("pkg.install summary missing status fallback; got %q", got[1])
	}
	if !strings.Contains(got[2], "os.service") || !strings.Contains(got[2], "already in desired state") {
		t.Errorf("os.service summary missing handler Reason; got %q", got[2])
	}
}

// TestStdoutCapture_SummariesCap covers the agentStepSummariesMax
// bound: a runaway plan must not push the LAST ITERATION block past
// the model's per-message budget. Past the cap, completions are
// dropped and a single "... N more" sentinel is appended.
func TestStdoutCapture_SummariesCap(t *testing.T) {
	c := newStdoutCapture(nil)
	for i := 0; i < agentStepSummariesMax+5; i++ {
		stepID := "step" + string(rune('a'+i%26))
		emitStartedNamed(c, stepID, "s", "file.write")
		emitCompletedDetail(c, stepID, true, map[string]interface{}{
			"status": "changed",
			"reason": "wrote 4 bytes",
		})
	}
	got := c.Summaries()
	// Expect agentStepSummariesMax real lines + 1 sentinel.
	if len(got) != agentStepSummariesMax+1 {
		t.Fatalf("Summaries() len = %d, want %d", len(got), agentStepSummariesMax+1)
	}
	if !strings.Contains(got[len(got)-1], "more step") {
		t.Errorf("missing trailing 'more steps' sentinel; got %q", got[len(got)-1])
	}
}

// TestSummarizeStep_ReasonWins pins the priority order: a handler-set
// Reason always wins over the generic status fallback.
func TestSummarizeStep_ReasonWins(t *testing.T) {
	got := summarizeStep("file.write", "create greeting", true, map[string]interface{}{
		"status": "changed",
		"reason": "wrote 5 bytes to /tmp/x",
	})
	if !strings.Contains(got, "wrote 5 bytes to /tmp/x") {
		t.Errorf("Reason should win; got %q", got)
	}
	if !strings.Contains(got, "file.write") {
		t.Errorf("action should appear in summary; got %q", got)
	}
}

// TestSummarizeStep_ShellStdoutFallback covers the cmd/shell branch
// where the handler set no Reason but stdout has a usable first line
// (e.g. `git status -s` clean tree -> empty; non-clean -> first file).
func TestSummarizeStep_ShellStdoutFallback(t *testing.T) {
	got := summarizeStep("cmd", "git status", true, map[string]interface{}{
		"status": "changed",
		"stdout": " M internal/foo.go\n M internal/bar.go\n",
	})
	if !strings.Contains(got, "M internal/foo.go") {
		t.Errorf("cmd fallback should use first non-empty stdout line; got %q", got)
	}
}

// TestSummarizeStep_GenericFallback covers actions that set neither
// Reason nor stdout — the model still needs a non-empty line so it
// sees the step ran.
func TestSummarizeStep_GenericFallback(t *testing.T) {
	got := summarizeStep("vars", "set env", false, map[string]interface{}{
		"status": "ok",
	})
	if got == "" {
		t.Fatal("generic fallback must return a non-empty line")
	}
	if !strings.Contains(got, "vars") {
		t.Errorf("generic fallback missing action; got %q", got)
	}
}

// TestSummarizeStep_LineCap pins the per-line length bound. A
// handler that dumps a multi-line diff into Reason (template,
// file_replace) must still come out as one dense line so a 30-step
// plan's LAST ITERATION block doesn't blow the prompt budget.
func TestSummarizeStep_LineCap(t *testing.T) {
	hugeReason := strings.Repeat("LONG-LINE ", 100) // ~1000 chars
	got := summarizeStep("template", "render config", true, map[string]interface{}{
		"status": "changed",
		"reason": hugeReason,
	})
	if len(got) > agentStepSummaryLineBytes {
		t.Errorf("line not clamped: len=%d, cap=%d", len(got), agentStepSummaryLineBytes)
	}
}

// TestSummarizeStep_CollapsesNewlines pins the multi-line collapse
// rule: handler Reasons that include diff-style \n separators come
// out as one dense " | "-separated line.
func TestSummarizeStep_CollapsesNewlines(t *testing.T) {
	got := summarizeStep("file.replace", "edit", true, map[string]interface{}{
		"status": "changed",
		"reason": "old: foo\nnew: bar",
	})
	if strings.Contains(got, "\n") {
		t.Errorf("summary should not contain literal newline; got %q", got)
	}
	if !strings.Contains(got, "old: foo | new: bar") {
		t.Errorf("multi-line Reason not collapsed with separator; got %q", got)
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
