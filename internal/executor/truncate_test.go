package executor

import (
	"strings"
	"testing"
)

// TestTruncateTail_BelowLimit returns the input unchanged when it fits.
func TestTruncateTail_BelowLimit(t *testing.T) {
	in := "short output"
	got := truncateTail(in, 100)
	if got != in {
		t.Errorf("truncateTail(short, 100) = %q, want unchanged", got)
	}
}

// TestTruncateTail_AtLimit returns the input unchanged when length equals cap.
func TestTruncateTail_AtLimit(t *testing.T) {
	in := strings.Repeat("a", 100)
	got := truncateTail(in, 100)
	if got != in {
		t.Errorf("truncateTail(=cap) modified input")
	}
}

// TestTruncateTail_KeepsLastBytes is the core contract: when input
// overflows, the LAST n bytes survive and a "<X bytes truncated>"
// marker prefixes them. Mirrors the step-failure scenario where the
// FAIL: line is at the end of a long stdout stream.
func TestTruncateTail_KeepsLastBytes(t *testing.T) {
	head := strings.Repeat("boring-startup-output\n", 200) // ~4400 bytes
	tail := "FAIL: TestSomething (1.23s)\nFAIL\nexit status 1\n"
	in := head + tail
	got := truncateTail(in, 100)

	if !strings.Contains(got, "bytes truncated") {
		t.Errorf("missing truncation marker in %q", got)
	}
	if !strings.HasSuffix(got, "exit status 1\n") {
		start := len(got) - 40
		if start < 0 {
			start = 0
		}
		t.Errorf("truncateTail dropped the failure tail; suffix = %q", got[start:])
	}
	// The body after the marker should be exactly the last 100 bytes
	// of the input.
	marker := "\n"
	idx := strings.Index(got, marker)
	if idx < 0 {
		t.Fatalf("no newline separator in truncated output")
	}
	body := got[idx+1:]
	if len(body) != 100 {
		t.Errorf("body length = %d, want 100", len(body))
	}
	if body != in[len(in)-100:] {
		t.Errorf("body bytes don't match input tail")
	}
}

// TestTruncateTail_DroppedCount the marker reports the number of
// dropped bytes accurately. Operators reading a truncated CI log
// rely on this to decide whether to re-run with the script directly.
func TestTruncateTail_DroppedCount(t *testing.T) {
	in := strings.Repeat("x", 5000)
	got := truncateTail(in, 1000)
	want := "... <4000 bytes truncated> ..."
	if !strings.Contains(got, want) {
		t.Errorf("marker missing %q; got prefix %q", want, got[:60])
	}
}
