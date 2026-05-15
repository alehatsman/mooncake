package main

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
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

// TestFleetApply_PeerBareNameNoMatch exercises the apply Action's
// error path when `--peer <name>` references a peer that isn't in
// peers.toml. The error must surface the unknown name so the user
// can spot a typo.
func TestFleetApply_PeerBareNameNoMatch(t *testing.T) {
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
		"--peer", "nonexistent",
		planPath,
	})
	if err == nil {
		t.Fatal("want error when --peer matches no peers")
	}
	if !strings.Contains(err.Error(), "--peer selected 0") {
		t.Errorf("err = %v, want substring '--peer selected 0'", err)
	}
	if !strings.Contains(err.Error(), "unknown: nonexistent") {
		t.Errorf("err = %v, want substring 'unknown: nonexistent'", err)
	}
}

// TestRenderStatusTable_Snapshot locks in the rendered table layout for a
// fixed mix of states. Anchors the column ordering and the trailing
// summary line; a future tweak to either is intentional, not accidental.
func TestRenderStatusTable_Snapshot(t *testing.T) {
	rows := []fleet.Status{
		{
			Name: "alpha", Addr: "alpha.lan:7878",
			State:         fleet.StateOK,
			Accessible:    true,
			OS:            "ubuntu 24.04",
			Arch:          "amd64",
			Mooncake:      "0.9.0",
			QueueDepth:    0,
			LastRunStatus: "success",
			LastRunAge:    "2m ago",
		},
		{
			Name: "beta", Addr: "beta.lan:7878",
			State:         fleet.StateRunning,
			Accessible:    true,
			Running:       true,
			OS:            "darwin 14.4",
			Arch:          "arm64",
			Mooncake:      "0.9.0",
			QueueDepth:    1,
			RunsRunning:   1,
			LastRunStatus: "running",
			LastRunAge:    "in flight",
		},
		{
			Name: "gamma", Addr: "gamma.lan:7878",
			State:         fleet.StateFailed,
			Accessible:    true,
			OS:            "ubuntu 24.04",
			Arch:          "amd64",
			Mooncake:      "0.9.0",
			QueueDepth:    0,
			LastRunStatus: "failed",
			LastRunAge:    "18h ago",
		},
		{
			Name: "delta", Addr: "delta.lan:7878",
			State:      fleet.StateUnreachable,
			QueueDepth: -1, // dash in the QUEUE column
			Error:      "dial tcp: connection refused",
		},
	}
	var buf bytes.Buffer
	renderStatusTable(&buf, rows, false)
	got := buf.String()

	// Be explicit about the layout invariants rather than diffing whole
	// strings — tabwriter spacing is sensitive to terminal width.
	wantSubstrings := []string{
		"HOST", "ADDR", "ACCESSIBLE", "RUNNING", "OS", "MOONCAKE", "QUEUE", "LAST RUN",
		"alpha", "yes", "ubuntu 24.04 (amd64)", "success 2m ago",
		"beta", "darwin 14.4 (arm64)", "in flight",
		"gamma", "failed 18h ago",
		"delta", "—",
		"✗ 3/4 accessible, 1 running, 1 last-failed, 1 unreachable",
		"delta: dial tcp: connection refused",
	}
	for _, s := range wantSubstrings {
		if !strings.Contains(got, s) {
			t.Errorf("rendered table missing substring %q. Output:\n%s", s, got)
		}
	}
}

// TestRenderStatusTable_AllHealthyShowsTick — when no failures or
// unreachable peers, the summary line leads with a check, not an X.
func TestRenderStatusTable_AllHealthyShowsTick(t *testing.T) {
	rows := []fleet.Status{
		{Name: "alpha", Addr: "a:7878", State: fleet.StateOK, Accessible: true, Mooncake: "0.9.0"},
		{Name: "beta", Addr: "b:7878", State: fleet.StateRunning, Accessible: true, Running: true, Mooncake: "0.9.0", RunsRunning: 1},
	}
	var buf bytes.Buffer
	renderStatusTable(&buf, rows, false)
	got := buf.String()
	if !strings.Contains(got, "✔ 2/2 accessible, 1 running") {
		t.Errorf("want ✔ summary, got:\n%s", got)
	}
	if strings.Contains(got, "✗") {
		t.Errorf("unexpected ✗ in summary:\n%s", got)
	}
}

// TestEmitJSON_OneRecordPerLine — locks the JSONL output contract:
// exactly one JSON object per peer, separated by '\n'. Each record must
// include the columns scripts will read (name, state, mooncake, etc).
func TestEmitJSON_OneRecordPerLine(t *testing.T) {
	rows := []fleet.Status{
		{Name: "alpha", Addr: "a:7878", State: fleet.StateOK, Mooncake: "0.9.0"},
		{Name: "beta", Addr: "b:7878", State: fleet.StateUnreachable, Error: "boom"},
	}
	var buf bytes.Buffer
	if err := emitJSON(&buf, rows); err != nil {
		t.Fatalf("emitJSON: %v", err)
	}
	lines := strings.Split(strings.TrimRight(buf.String(), "\n"), "\n")
	if len(lines) != len(rows) {
		t.Fatalf("got %d lines, want %d: %q", len(lines), len(rows), buf.String())
	}
	// Decode each line individually; if any is malformed JSON the test
	// fails with a useful message.
	for i, ln := range lines {
		var obj map[string]any
		if err := json.Unmarshal([]byte(ln), &obj); err != nil {
			t.Errorf("line %d malformed: %v\n%s", i, err, ln)
		}
		if obj["name"] == "" {
			t.Errorf("line %d missing name: %s", i, ln)
		}
		if obj["state"] == "" {
			t.Errorf("line %d missing state: %s", i, ln)
		}
	}
}

// TestStatusExitCode walks the 0/1/2 mapping. Mirrors fleet apply's
// exit-code contract so users / scripts see consistent behavior.
func TestStatusExitCode(t *testing.T) {
	tests := []struct {
		name   string
		states []fleet.State
		want   int // -1 means nil (exit 0)
	}{
		{"all ok", []fleet.State{fleet.StateOK, fleet.StateOK}, -1},
		{"mix ok+running", []fleet.State{fleet.StateOK, fleet.StateRunning}, -1},
		{"failed wins over ok", []fleet.State{fleet.StateOK, fleet.StateFailed}, 1},
		{"unreachable wins over failed", []fleet.State{fleet.StateFailed, fleet.StateUnreachable}, 2},
		{"unreachable wins over running", []fleet.State{fleet.StateRunning, fleet.StateUnreachable}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([]fleet.Status, len(tt.states))
			for i, s := range tt.states {
				rows[i] = fleet.Status{Name: "p", State: s}
			}
			err := statusExitCode(rows)
			if tt.want == -1 {
				if err != nil {
					t.Errorf("want nil, got %v", err)
				}
				return
			}
			ec, ok := err.(cli.ExitCoder)
			if !ok {
				t.Fatalf("want ExitCoder, got %T: %v", err, err)
			}
			if ec.ExitCode() != tt.want {
				t.Errorf("exit code = %d, want %d", ec.ExitCode(), tt.want)
			}
		})
	}
}

