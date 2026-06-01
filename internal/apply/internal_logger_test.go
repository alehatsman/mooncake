package apply

import (
	"testing"

	"github.com/alehatsman/mooncake/internal/logger"
)

// TestInternalLogger pins #63: under a machine-readable output format the
// executor's prose logger must be a no-op so step-failure text never leaks
// onto the format's stdout stream. Text mode keeps the console logger.
func TestInternalLogger(t *testing.T) {
	discardFormats := []string{outputFormatJSON, outputFormatAgent, outputFormatQuiet}
	for _, f := range discardFormats {
		t.Run(f+" => discard", func(t *testing.T) {
			if _, ok := internalLogger(f, logger.ErrorLevel).(*logger.DiscardLogger); !ok {
				t.Errorf("internalLogger(%q) = %T, want *logger.DiscardLogger", f, internalLogger(f, logger.ErrorLevel))
			}
		})
	}

	for _, f := range []string{outputFormatText, ""} {
		t.Run(f+" => console", func(t *testing.T) {
			got := internalLogger(f, logger.ErrorLevel)
			if _, isDiscard := got.(*logger.DiscardLogger); isDiscard {
				t.Errorf("internalLogger(%q) = DiscardLogger, want a real console logger", f)
			}
		})
	}
}
