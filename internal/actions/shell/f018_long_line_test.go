package shell

// F018 regression: shell.streamOutput used bufio.Scanner with the
// default 64 KB token cap and never checked scanner.Err(). A
// command emitting a single line > 64 KB returned scan-error
// silently — the rest of stdout was discarded, capture: true
// surfaced truncated output, and result.Failed stayed false. The
// fix raises the scanner buffer to 1 MB and surfaces ErrTooLong
// via the logger.

import (
	"runtime"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestShellHandler_LongLineWithinCap: a command emitting a single line
// of 100 KB (> 64 KB default scanner cap, < 1 MB new cap) must produce
// fully captured stdout, not truncated. Pre-fix: scanner.Scan() returns
// false on the first line and stdout is the empty string.
func TestShellHandler_LongLineWithinCap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("awk not available on Windows")
	}

	const want = 100_000 // > 64 KB, < 1 MB
	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			// awk is more portable than printf '%*s'; produces one long
			// line of x's followed by a newline.
			Cmd: `awk 'BEGIN { for (i=0; i<100000; i++) printf("x") ; printf("\n") }'`,
		},
	}

	result, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}
	r, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("result type %T, want *executor.Result", result)
	}

	stdout := strings.TrimRight(r.Stdout, "\n")
	if got := len(stdout); got != want {
		t.Errorf("captured stdout length = %d, want %d (pre-fix the default 64 KB scanner truncated long lines silently)", got, want)
	}
}

// TestShellHandler_LineOverCapLogsTruncation: a command emitting a
// line longer than the 1 MB new cap should not crash, the captured
// stdout up to the cap is whatever the scanner consumed, AND the
// logger receives an Errorf about the truncation so the operator
// sees the data-loss signal. Pre-fix: ErrTooLong was swallowed
// entirely.
func TestShellHandler_LineOverCapLogsTruncation(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("awk not available on Windows")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			// 2 MB line — exceeds the new 1 MB cap.
			Cmd: `awk 'BEGIN { for (i=0; i<2000000; i++) printf("x") ; printf("\n") }'`,
		},
	}

	_, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error: %v", err)
	}

	logs := ctx.Svc.Logger.(*testutil.MockLogger).Logs
	var matched bool
	for _, line := range logs {
		if strings.Contains(line, "stream stopped early") && strings.Contains(line, "stdout") {
			matched = true
			break
		}
	}
	if !matched {
		t.Errorf("expected logger to receive an Errorf about stdout truncation; got logs:\n%s",
			strings.Join(logs, "\n"))
	}
}

// TestShellHandler_LineOverCapStructuredSurface: F038 regression.
// The F018 fix surfaced overflow via the human logger only —
// programmatic consumers (--output-format json, agentd→SSE, MCP
// tool results) saw the step complete with empty stdout, no signal.
// F038 routes the marker through (a) the captured buffer that
// becomes result.Stdout/result.Stderr and (b) a synthetic step.stderr
// event so live subscribers see it without waiting for step.completed.
func TestShellHandler_LineOverCapStructuredSurface(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("awk not available on Windows")
	}

	h := &Handler{}
	ctx := newMockExecutionContext()

	step := &config.Step{
		Shell: &config.ShellAction{
			Cmd: `awk 'BEGIN { for (i=0; i<2000000; i++) printf("x") ; printf("\n") }'`,
		},
		As: "out",
	}

	result, err := h.Run(ctx, step)
	if err != nil {
		t.Fatalf("Run() error: %v", err)
	}
	r, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("result type %T, want *executor.Result", result)
	}

	// The captured stream that overflowed (stdout in this case) must
	// carry the marker so consumers reading the typed result see it.
	if !strings.Contains(r.Stdout, "mooncake: stdout stream truncated") {
		t.Errorf("result.Stdout should contain the F038 truncation marker; got %d bytes, last 200=%q",
			len(r.Stdout), tailString(r.Stdout, 200))
	}

	// The synthetic step.stderr event must reach subscribers — pin
	// it via the mock publisher's event list.
	pub := ctx.Svc.EventPublisher.(*testutil.MockPublisher)
	var sawSyntheticStderr bool
	for _, ev := range pub.Events {
		if d, ok := ev.Data.(events.StepOutputData); ok && d.Stream == "stderr" &&
			strings.Contains(d.Line, "mooncake: stdout stream truncated") {
			sawSyntheticStderr = true
			break
		}
	}
	if !sawSyntheticStderr {
		t.Error("expected a synthetic step.stderr event carrying the truncation marker; got none")
	}
}

// tailString returns the last n bytes of s for diagnostic prints.
func tailString(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}
