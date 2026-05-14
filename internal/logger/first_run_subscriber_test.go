package logger

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/events"
)

// withTempHome points $HOME at a fresh temp dir so the subscriber's marker
// file doesn't collide with the developer's real ~/.mooncake/.
func withTempHome(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	return dir
}

func successEvent() events.Event {
	return events.Event{
		Type: events.EventRunCompleted,
		Data: events.RunCompletedData{
			SuccessSteps: 3,
			ChangedSteps: 1,
			FailedSteps:  0,
		},
	}
}

func TestFirstRunHint_PrintsOnceAndCreatesMarker(t *testing.T) {
	home := withTempHome(t)
	t.Setenv("MOONCAKE_NO_HINTS", "")

	var buf bytes.Buffer
	sub := NewFirstRunHintSubscriber(&buf, "text")
	sub.OnEvent(successEvent())

	if !strings.Contains(buf.String(), "First run") {
		t.Errorf("expected hint in output, got: %q", buf.String())
	}
	marker := filepath.Join(home, ".mooncake", ".first-run-completed")
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("expected marker file at %s: %v", marker, err)
	}

	// Second call: marker exists, no hint.
	buf.Reset()
	sub.OnEvent(successEvent())
	if buf.Len() != 0 {
		t.Errorf("second run should be silent, got: %q", buf.String())
	}
}

func TestFirstRunHint_SuppressedByEnvVar(t *testing.T) {
	home := withTempHome(t)
	t.Setenv("MOONCAKE_NO_HINTS", "1")

	var buf bytes.Buffer
	sub := NewFirstRunHintSubscriber(&buf, "text")
	sub.OnEvent(successEvent())

	if buf.Len() != 0 {
		t.Errorf("MOONCAKE_NO_HINTS=1 should suppress output, got: %q", buf.String())
	}
	// Marker must NOT be created — env-var suppression is per-invocation,
	// not a permanent dismiss.
	marker := filepath.Join(home, ".mooncake", ".first-run-completed")
	if _, err := os.Stat(marker); err == nil {
		t.Error("marker should not be created when env var suppresses hint")
	}
}

func TestFirstRunHint_SuppressedByNonTextFormat(t *testing.T) {
	withTempHome(t)
	t.Setenv("MOONCAKE_NO_HINTS", "")

	for _, fmt := range []string{"json", "agent", "quiet"} {
		var buf bytes.Buffer
		sub := NewFirstRunHintSubscriber(&buf, fmt)
		sub.OnEvent(successEvent())
		if buf.Len() != 0 {
			t.Errorf("format=%s should suppress hint, got: %q", fmt, buf.String())
		}
	}
}

func TestFirstRunHint_SuppressedOnFailure(t *testing.T) {
	withTempHome(t)
	t.Setenv("MOONCAKE_NO_HINTS", "")

	var buf bytes.Buffer
	sub := NewFirstRunHintSubscriber(&buf, "text")
	sub.OnEvent(events.Event{
		Type: events.EventRunCompleted,
		Data: events.RunCompletedData{SuccessSteps: 1, FailedSteps: 2},
	})

	if buf.Len() != 0 {
		t.Errorf("failed run should not trigger hint, got: %q", buf.String())
	}
}

func TestFirstRunHint_IgnoresOtherEvents(t *testing.T) {
	withTempHome(t)

	var buf bytes.Buffer
	sub := NewFirstRunHintSubscriber(&buf, "text")
	sub.OnEvent(events.Event{Type: events.EventRunStarted})
	if buf.Len() != 0 {
		t.Errorf("non-RunCompleted events must be ignored, got: %q", buf.String())
	}
}
