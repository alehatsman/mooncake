package main

import (
	"bytes"
	"strings"
	"testing"
)

// TestMT66_BadFlagDoesNotDumpHelp regression-guards against MT-66:
// `mooncake apply --max-plan-age garbage` used to dump the full
// --help output for `apply` before the actual error, burying the
// real message under 100+ lines. With OnUsageError installed on
// every command, only the error itself surfaces.
func TestMT66_BadFlagDoesNotDumpHelp(t *testing.T) {
	app := createApp()
	var out bytes.Buffer
	app.Writer = &out
	app.ErrWriter = &out

	err := app.Run([]string{"mooncake", "apply", "--max-plan-age", "garbage"})
	if err == nil {
		t.Fatal("expected error for invalid --max-plan-age value")
	}

	// The error itself must mention the bad value.
	if !strings.Contains(err.Error(), "max-plan-age") {
		t.Errorf("expected error to mention max-plan-age, got %q", err.Error())
	}

	// MT-66: writer must NOT contain a help dump. Any of these strings
	// would prove the help text was emitted, which is the regression.
	written := out.String()
	for _, banner := range []string{
		"USAGE:",
		"OPTIONS:",
		"COMMANDS:",
	} {
		if strings.Contains(written, banner) {
			t.Errorf("MT-66 regression: app emitted help text containing %q:\n%s", banner, written)
		}
	}
}

// TestMT66_UnknownFlagDoesNotDumpHelp covers the sibling case
// (urfave/cli treats both as usage errors).
func TestMT66_UnknownFlagDoesNotDumpHelp(t *testing.T) {
	app := createApp()
	var out bytes.Buffer
	app.Writer = &out
	app.ErrWriter = &out

	err := app.Run([]string{"mooncake", "plan", "--no-such-flag", "x"})
	if err == nil {
		t.Fatal("expected error for unknown flag")
	}
	if strings.Contains(out.String(), "USAGE:") || strings.Contains(out.String(), "OPTIONS:") {
		t.Errorf("MT-66 regression: help banner leaked into output for unknown flag:\n%s", out.String())
	}
}
