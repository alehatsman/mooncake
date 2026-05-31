package pilot

import (
	"os"
	"testing"
)

func TestConsoleLogFormat(t *testing.T) {
	if got := consoleLogFormat(OutputFormatJSON); got != "json" {
		t.Errorf("consoleLogFormat(json) = %q, want json", got)
	}
	for _, in := range []string{OutputFormatText, "", "anything"} {
		if got := consoleLogFormat(in); got != "text" {
			t.Errorf("consoleLogFormat(%q) = %q, want text", in, got)
		}
	}
}

func TestCaptureWriter(t *testing.T) {
	// JSON mode must disable the capture's terminal printout (nil writer)
	// so it can't interleave raw command output into the NDJSON stream.
	if w := captureWriter(OutputFormatJSON); w != nil {
		t.Errorf("captureWriter(json) = %v, want nil", w)
	}
	// Text mode prints to stdout as before.
	if w := captureWriter(OutputFormatText); w != os.Stdout {
		t.Errorf("captureWriter(text) = %v, want os.Stdout", w)
	}
	if w := captureWriter(""); w != os.Stdout {
		t.Errorf("captureWriter(\"\") = %v, want os.Stdout", w)
	}
}
