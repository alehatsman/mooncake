package logger

import "testing"

// Tests for the FORCE_COLOR / NO_COLOR overrides on DetectTerminal.
// The IsTerminal field is not asserted here — under `go test` stdout
// is always a pipe (isTerminal=false), which is precisely the case
// FORCE_COLOR is designed to override.

func TestDetectTerminal_ForceColorEnablesANSIWithoutTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "1")
	t.Setenv("CLICOLOR_FORCE", "")
	t.Setenv("CI", "")
	t.Setenv("TERM", "")

	info := DetectTerminal()
	if !info.SupportsANSI {
		t.Errorf("FORCE_COLOR=1 should enable SupportsANSI even with stdout=pipe")
	}
	if info.IsTerminal {
		t.Errorf("FORCE_COLOR should not lie about IsTerminal")
	}
}

func TestDetectTerminal_CLIColorForceEnablesANSI(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "1")

	info := DetectTerminal()
	if !info.SupportsANSI {
		t.Errorf("CLICOLOR_FORCE=1 should enable SupportsANSI same as FORCE_COLOR")
	}
}

func TestDetectTerminal_NoColorBeatsForceColor(t *testing.T) {
	// NO_COLOR is the explicit user opt-out per https://no-color.org.
	// Even with FORCE_COLOR set, NO_COLOR must win.
	t.Setenv("NO_COLOR", "1")
	t.Setenv("FORCE_COLOR", "1")

	info := DetectTerminal()
	if info.SupportsANSI {
		t.Errorf("NO_COLOR must override FORCE_COLOR")
	}
}

func TestDetectTerminal_NoForceColorAndNoTTY(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	t.Setenv("FORCE_COLOR", "")
	t.Setenv("CLICOLOR_FORCE", "")

	info := DetectTerminal()
	if info.SupportsANSI {
		t.Errorf("without FORCE_COLOR and without TTY, SupportsANSI must be false")
	}
}
