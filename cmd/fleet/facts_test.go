package fleet

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// TestFleetFacts_FullPrint asserts the no-key form pretty-prints the full
// facts map as JSON. The output is parseable so we don't depend on
// indentation specifics.
func TestFleetFacts_FullPrint(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Facts: map[string]any{
			"os":         "linux",
			"go_version": "1.22.3",
			"arch":       "amd64",
		},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "laptop"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, out.String())
	}
	if got["os"] != "linux" || got["go_version"] != "1.22.3" {
		t.Errorf("missing expected keys; got: %v", got)
	}
}

// TestFleetFacts_DotPathFilter exercises the <peer> <key> form. The key
// uses dot notation; lookup normalizes dots → underscores to match the
// flat-fact-map convention.
func TestFleetFacts_DotPathFilter(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Facts: map[string]any{
			"go_version":      "1.22.3",
			"os_distribution": "arch",
		},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	tests := []struct {
		key  string
		want string
	}{
		{"go_version", "1.22.3"},
		{"go.version", "1.22.3"}, // dot form maps to underscore key
		{"os.distribution", "arch"},
	}
	for _, tt := range tests {
		t.Run(tt.key, func(t *testing.T) {
			app, out := captureLogsApp()
			err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "laptop", tt.key})
			if err != nil {
				t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
			}
			if strings.TrimSpace(out.String()) != tt.want {
				t.Errorf("got %q, want %q", strings.TrimSpace(out.String()), tt.want)
			}
		})
	}
}

// TestFleetFacts_MissingKeyErrors guards the contract that a missing key
// is a hard error (exit 1) rather than printing empty — the user almost
// certainly typoed the key.
func TestFleetFacts_MissingKeyErrors(t *testing.T) {
	fake := &fakeAgentd{
		ExpectToken: "tok",
		Facts:       map[string]any{"go_version": "1.22.3"},
	}
	addr := fake.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": addr}, "tok")

	app, _ := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "laptop", "nonexistent"})
	if err == nil {
		t.Fatal("expected error for missing key")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want substring 'not found'", err)
	}
}

// TestFleetFacts_QueryFanOut: --query renders a table with one row per
// peer. Tests divergent values across peers (the most common reason to
// run this command) and that the table is alphabetically sorted by host.
func TestFleetFacts_QueryFanOut(t *testing.T) {
	fakeA := &fakeAgentd{ExpectToken: "tok", Facts: map[string]any{"go_version": "1.22.3"}}
	fakeB := &fakeAgentd{ExpectToken: "tok", Facts: map[string]any{"go_version": "1.21.6"}}
	addrA := fakeA.start(t)
	addrB := fakeB.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	// Two peers, deliberately add zebra first so we can verify sort.
	peersPath := writePeersToml(t, dir, map[string]string{"zebra": addrA, "alpha": addrB}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "--query", "go_version"})
	if err != nil {
		t.Fatalf("unexpected error: %v\noutput: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "HOST") || !strings.Contains(got, "GO_VERSION") {
		t.Errorf("missing header in output:\n%s", got)
	}
	if !strings.Contains(got, "1.22.3") || !strings.Contains(got, "1.21.6") {
		t.Errorf("missing values in output:\n%s", got)
	}
	// alpha should come before zebra in the rendered output (alphabetical sort).
	alphaIdx := strings.Index(got, "alpha")
	zebraIdx := strings.Index(got, "zebra")
	if alphaIdx < 0 || zebraIdx < 0 || alphaIdx > zebraIdx {
		t.Errorf("expected alpha before zebra in output (alphabetical sort):\n%s", got)
	}
}

// TestFleetFacts_QueryUnreachablePeerShownAsDash: a peer whose /v1/facts
// errors must show "—  (<reason>)" in the table, not blank, and the call
// must still exit 0 — partial reachability is informative.
func TestFleetFacts_QueryUnreachablePeerShownAsDash(t *testing.T) {
	fakeOK := &fakeAgentd{ExpectToken: "tok", Facts: map[string]any{"go_version": "1.22.3"}}
	fakeBad := &fakeAgentd{ExpectToken: "tok"} // Facts nil → /v1/facts returns 500
	addrOK := fakeOK.start(t)
	addrBad := fakeBad.start(t)
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"ok-host": addrOK, "broken-host": addrBad}, "tok")

	app, out := captureLogsApp()
	err := app.Run([]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "--query", "go_version"})
	if err != nil {
		t.Fatalf("--query must exit 0 on partial reachability, got %v\noutput: %s", err, out.String())
	}

	got := out.String()
	if !strings.Contains(got, "1.22.3") {
		t.Errorf("ok peer's value missing:\n%s", got)
	}
	// The broken peer's row should be present (with an em-dash + reason),
	// not omitted.
	if !strings.Contains(got, "broken-host") {
		t.Errorf("unreachable peer row missing:\n%s", got)
	}
	if !strings.Contains(got, "—") {
		t.Errorf("unreachable peer should be marked with —:\n%s", got)
	}
}

// TestFleetFacts_ArgValidation pins the input contract: --query forbids
// positional args; without --query the form is <peer> [key].
func TestFleetFacts_ArgValidation(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))
	peersPath := writePeersToml(t, dir, map[string]string{"laptop": "127.0.0.1:0"}, "tok")

	tests := []struct {
		name string
		argv []string
		want string
	}{
		{
			"no args, no --query",
			[]string{"mooncake", "fleet", "--peers-file", peersPath, "facts"},
			"expected",
		},
		{
			"three positional args",
			[]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "a", "b", "c"},
			"expected",
		},
		{
			"--query + positional",
			[]string{"mooncake", "fleet", "--peers-file", peersPath, "facts", "--query", "k", "extra"},
			"--query takes no positional",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app, _ := captureLogsApp()
			err := app.Run(tt.argv)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tt.want) {
				t.Errorf("err = %v, want substring %q", err, tt.want)
			}
		})
	}
}
