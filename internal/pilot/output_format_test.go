package pilot

import (
	"os"
	"testing"

	"github.com/alehatsman/mooncake/internal/logger"
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

func TestExecutorLogger(t *testing.T) {
	// JSON mode must hand the executor a no-op logger so step-failure prose
	// (logged via ec.Svc.Logger.Errorf) never lands on the NDJSON stdout
	// stream (#63).
	if _, ok := executorLogger(OutputFormatJSON).(*logger.DiscardLogger); !ok {
		t.Errorf("executorLogger(json) = %T, want *logger.DiscardLogger", executorLogger(OutputFormatJSON))
	}
	// Text mode keeps a real console logger so the operator still sees errors.
	for _, in := range []string{OutputFormatText, ""} {
		if _, isDiscard := executorLogger(in).(*logger.DiscardLogger); isDiscard {
			t.Errorf("executorLogger(%q) = DiscardLogger, want a console logger", in)
		}
	}
}
