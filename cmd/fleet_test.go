package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// newTestFleetApp returns a CLI app wired with `fleet` for tests. Critically
// it suppresses urfave/cli's default ExitErrHandler (which calls os.Exit on
// cli.ExitCoder errors) so the test binary doesn't terminate when an Action
// returns cli.Exit.
func newTestFleetApp() *cli.App {
	return &cli.App{
		Commands:       []*cli.Command{fleetCommand()},
		Writer:         io.Discard,
		ErrWriter:      io.Discard,
		ExitErrHandler: func(*cli.Context, error) {},
	}
}

// TestFleetCommand_Help ensures the command tree wires up: `fleet` exists,
// `fleet apply` is a subcommand, and the flags are reachable.
func TestFleetCommand_Help(t *testing.T) {
	cmd := fleetCommand()
	if cmd.Name != "fleet" {
		t.Fatalf("Name = %q, want fleet", cmd.Name)
	}
	if len(cmd.Subcommands) == 0 {
		t.Fatal("no subcommands registered")
	}
	var apply *cli.Command
	for _, sc := range cmd.Subcommands {
		if sc.Name == "apply" {
			apply = sc
			break
		}
	}
	if apply == nil {
		t.Fatal("`apply` subcommand missing")
	}
	if apply.Action == nil {
		t.Fatal("apply.Action is nil")
	}
}

func TestFleetApply_RequiresPlanArg(t *testing.T) {
	app := newTestFleetApp()
	err := app.Run([]string{"mooncake", "fleet", "apply"})
	if err == nil {
		t.Fatal("want error when plan arg is missing")
	}
	ee, ok := err.(cli.ExitCoder)
	if !ok {
		t.Fatalf("want cli.ExitCoder error, got %T: %v", err, err)
	}
	if ee.ExitCode() != 2 {
		t.Errorf("exit code = %d, want 2", ee.ExitCode())
	}
}

func TestFleetApply_NoPeersConfigured(t *testing.T) {
	// Empty peers file → friendly error and exit 1.
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")
	if err := os.WriteFile(peersPath, []byte(""), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	// Point XDG away from anywhere a real user might have a controller_id.
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	planPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(planPath, []byte("steps: []\n"), 0o600); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	app := newTestFleetApp()
	err := app.Run([]string{"mooncake", "fleet", "apply", "--peers-file", peersPath, planPath})
	if err == nil {
		t.Fatal("want error when no peers configured")
	}
	if !strings.Contains(err.Error(), "no peers configured") {
		t.Errorf("err = %v, want substring 'no peers configured'", err)
	}
}

func TestFleetApply_PeersFilterNoMatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	peersPath := filepath.Join(dir, "peers.toml")
	const peers = `[[peers]]
name = "laptop"
addr = "laptop.lan:7878"
token = "t"
`
	if err := os.WriteFile(peersPath, []byte(peers), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	planPath := filepath.Join(dir, "config.yml")
	if err := os.WriteFile(planPath, []byte("steps: []\n"), 0o600); err != nil {
		t.Fatalf("seed plan: %v", err)
	}

	app := newTestFleetApp()
	err := app.Run([]string{
		"mooncake", "fleet", "apply",
		"--peers-file", peersPath,
		"--peers", "nonexistent",
		planPath,
	})
	if err == nil {
		t.Fatal("want error when filter matches no peers")
	}
	if !strings.Contains(err.Error(), "no peers matched") {
		t.Errorf("err = %v, want substring 'no peers matched'", err)
	}
}
