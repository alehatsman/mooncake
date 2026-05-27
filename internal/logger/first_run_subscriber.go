package logger

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/events"
)

// FirstRunHintSubscriber prints a single line of next-step guidance after the
// first successful `mooncake apply` on a host. It is a one-shot: once the
// marker file ~/.mooncake/.first-run-completed exists, the subscriber is a
// no-op. Spec 40 §"First-run hint".
//
// Suppression rules:
//   - Marker file already exists.
//   - The run failed (we don't celebrate failures).
//   - Output format is anything other than text (`agent`, `json`, `quiet`)
//     so the hint cannot corrupt downstream parsers.
//   - MOONCAKE_NO_HINTS=1 in the environment.
type FirstRunHintSubscriber struct {
	out          io.Writer
	outputFormat string
}

const firstRunHint = "" +
	"\n★ First run — nice. Try `mooncake plan` to preview changes before `apply`.\n" +
	"  Browse 330+ built-in components under ./presets/ (use them with `use:`).\n"

// NewFirstRunHintSubscriber returns a subscriber that prints the hint to out
// (typically os.Stdout) on the first successful run. outputFormat is the
// CLI's --output-format value; the hint is only printed when it is empty or
// "text".
func NewFirstRunHintSubscriber(out io.Writer, outputFormat string) *FirstRunHintSubscriber {
	return &FirstRunHintSubscriber{out: out, outputFormat: outputFormat}
}

// OnEvent prints the hint when the run completes successfully and no marker
// exists. Marker creation and printing are best-effort: a failure to write
// the marker just means the hint may print again next run.
func (s *FirstRunHintSubscriber) OnEvent(event events.Event) {
	if event.Type != events.EventRunCompleted {
		return
	}
	if os.Getenv("MOONCAKE_NO_HINTS") == "1" {
		return
	}
	if s.outputFormat != "" && s.outputFormat != "text" {
		return
	}
	d, ok := event.Data.(events.RunCompletedData)
	if !ok || d.FailedSteps > 0 {
		return
	}

	marker, err := markerPath()
	if err != nil {
		return
	}
	if _, err := os.Stat(marker); err == nil {
		return // already shown
	}

	fmt.Fprint(s.out, firstRunHint)

	// Best-effort: create the marker. If this fails the user will see the
	// hint again next run, which is annoying but not broken.
	if mkErr := os.MkdirAll(filepath.Dir(marker), 0o700); mkErr == nil {
		_ = os.WriteFile(marker, []byte{}, 0o600)
	}
}

// Close implements the Subscriber interface.
func (s *FirstRunHintSubscriber) Close() {}

// markerPath returns the absolute path of the one-shot first-run marker.
func markerPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".mooncake", ".first-run-completed"), nil
}
