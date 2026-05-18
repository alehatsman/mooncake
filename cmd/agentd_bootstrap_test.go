package main

import (
	"bytes"
	"io"
	"runtime"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet/install"
)

// newTestAgentdApp wires up the agentd command tree for use in tests.
// Suppresses urfave/cli's default ExitErrHandler so cli.Exit(...) errors
// don't os.Exit the test binary.
func newTestAgentdApp() *cli.App {
	var buf bytes.Buffer
	return &cli.App{
		Commands:       []*cli.Command{agentdCommand()},
		Writer:         &buf,
		ErrWriter:      io.Discard,
		ExitErrHandler: func(*cli.Context, error) {},
	}
}

// TestAgentdCommand_Subcommands wires up the new subcommand tree: bare
// `agentd` is a parent with `run` + `bootstrap` children, no leaf
// Action. This is the only externally-visible breaking change in
// spec-70 — pin it here so a "tidy refactor" that adds an Action back
// trips the test.
func TestAgentdCommand_Subcommands(t *testing.T) {
	cmd := agentdCommand()
	if cmd.Action != nil {
		t.Errorf("agentd parent should have no Action; spec-70 split it into `run` + `bootstrap`")
	}
	want := map[string]bool{"run": false, "bootstrap": false}
	for _, sc := range cmd.Subcommands {
		if _, ok := want[sc.Name]; ok {
			want[sc.Name] = true
		}
	}
	for name, found := range want {
		if !found {
			t.Errorf("`agentd %s` subcommand missing", name)
		}
	}
}

// TestAgentdBootstrap_RejectsWindows pins the v1 platform gate — local
// bootstrap on Windows is a follow-up (spec-70 §Non-goals).
func TestAgentdBootstrap_RejectsWindows(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("only meaningful on windows")
	}
	app := newTestAgentdApp()
	err := app.Run([]string{"mooncake", "agentd", "bootstrap"})
	if err == nil {
		t.Fatal("expected error on windows")
	}
	if !strings.Contains(err.Error(), "Windows") {
		t.Errorf("error should mention Windows; got %v", err)
	}
}

// TestAgentdBootstrap_RejectsUserOnDarwin pins the macOS guard:
// LaunchAgents are a follow-up (spec-70 §Open Questions 1), so
// --user on darwin errors clearly today rather than silently
// installing a LaunchDaemon as the wrong scope.
func TestAgentdBootstrap_RejectsUserOnDarwin(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("only meaningful on darwin")
	}
	app := newTestAgentdApp()
	err := app.Run([]string{"mooncake", "agentd", "bootstrap", "--user"})
	if err == nil {
		t.Fatal("expected error: --user is Linux-only")
	}
	if !strings.Contains(err.Error(), "Linux-only") {
		t.Errorf("error should explain --user is Linux-only; got %v", err)
	}
}

// TestPrintAgentdBootstrapResult_PairHintShape pins the spec-70
// §Design 3 output: the bearer token line is present, and the pair
// hint uses --token-via stdin (NOT --token-via literal) so the
// copy-paste doesn't dump the token into shell history.
func TestPrintAgentdBootstrapResult_PairHintShape(t *testing.T) {
	var buf bytes.Buffer
	printAgentdBootstrapResult(&buf, "linux", true, 7878, install.BootstrapResult{
		Token: "tok-abcdef",
		OS:    "linux",
		Arch:  "amd64",
	})
	got := buf.String()
	if !strings.Contains(got, "tok-abcdef") {
		t.Errorf("output should print the bearer token:\n%s", got)
	}
	if !strings.Contains(got, "--token-via stdin <<<'tok-abcdef'") {
		t.Errorf("pair hint should use --token-via stdin heredoc to dodge shell history:\n%s", got)
	}
	if strings.Contains(got, "--token-via literal:") {
		t.Errorf("pair hint must NOT use --token-via literal:<token> — leaks token to history:\n%s", got)
	}
	if !strings.Contains(got, ":7878") {
		t.Errorf("pair hint should include the agentd port:\n%s", got)
	}
	if !strings.Contains(got, "user-mode") {
		t.Errorf("output should announce user-mode when AsUser=true:\n%s", got)
	}
}

// TestPrintAgentdBootstrapResult_IdempotentMessage pins the
// short-circuit output: when install.Bootstrap returned AlreadyOK,
// the operator-facing message should make clear nothing was reinstalled.
func TestPrintAgentdBootstrapResult_IdempotentMessage(t *testing.T) {
	var buf bytes.Buffer
	printAgentdBootstrapResult(&buf, "linux", false, 7878, install.BootstrapResult{
		Token:     "tok-xyz",
		OS:        "linux",
		Arch:      "amd64",
		AlreadyOK: true,
	})
	got := buf.String()
	if !strings.Contains(got, "already installed") {
		t.Errorf("idempotent path should say `already installed`:\n%s", got)
	}
	if !strings.Contains(got, "tok-xyz") {
		t.Errorf("idempotent path should still print the token:\n%s", got)
	}
}
