package fleet

import (
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func TestValidatePeerFilterKeys(t *testing.T) {
	// Spec-50 allowlist: {tag, name, os, role}. All four must validate;
	// an off-list key must produce an error that lists the valid keys.
	for _, args := range [][]string{
		{"tag=darwin"},
		{"name=laptop"},
		{"role=db"},
		{"os=linux"},
		{"@tag=a,name=b,role=c,os=d"},
		{"tag=a", "name=b"},
	} {
		groups, err := parsePeerFlags(args)
		if err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		if err := validatePeerFilterKeys(groups); err != nil {
			t.Errorf("expected %v to validate, got %v", args, err)
		}
	}

	bad, _ := parsePeerFlags([]string{"arch=arm64"})
	err := validatePeerFilterKeys(bad)
	if err == nil {
		t.Fatal("arch= should be rejected (not in spec-50 allowlist)")
	}
	for _, want := range []string{"unsupported", "arch", "tag, name, os, role"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want substring %q", err, want)
		}
	}
}

func TestPeerMatchesFilters(t *testing.T) {
	mac := fleet.Peer{Name: "macbook", Tags: []string{"darwin", "workstation"}, Roles: []string{"primary"}}
	desk := fleet.Peer{Name: "desk1", Tags: []string{"linux", "gpu"}, Roles: []string{"db"}}
	vps := fleet.Peer{Name: "vps", Tags: []string{"linux", "server"}}

	mustParse := func(t *testing.T, args ...string) [][]filterTerm {
		t.Helper()
		g, err := parsePeerFlags(args)
		if err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		return g
	}

	tests := []struct {
		name   string
		filter [][]filterTerm
		peer   fleet.Peer
		want   bool
	}{
		{"empty filter matches all", nil, mac, true},
		{"single tag hit", mustParse(t, "tag=darwin"), mac, true},
		{"single tag miss", mustParse(t, "tag=darwin"), desk, false},
		{"AND within group hit", mustParse(t, "@tag=linux,tag=gpu"), desk, true},
		{"AND within group miss", mustParse(t, "@tag=linux,tag=gpu"), vps, false},
		{"OR across flags hit on left", mustParse(t, "tag=darwin", "tag=server"), mac, true},
		{"OR across flags hit on right", mustParse(t, "tag=darwin", "tag=server"), vps, true},
		{"OR across flags miss both", mustParse(t, "tag=darwin", "tag=server"), desk, false},
		{
			name:   "(A AND B) OR C — left hits",
			filter: mustParse(t, "@tag=linux,tag=gpu", "tag=workstation"),
			peer:   desk,
			want:   true,
		},
		{
			name:   "(A AND B) OR C — right hits",
			filter: mustParse(t, "@tag=linux,tag=gpu", "tag=workstation"),
			peer:   mac,
			want:   true,
		},
		{
			name:   "(A AND B) OR C — both miss",
			filter: mustParse(t, "@tag=linux,tag=gpu", "tag=workstation"),
			peer:   vps,
			want:   false,
		},
		{
			name:   "tag value can be key=value style",
			filter: mustParse(t, "tag=os=darwin"),
			peer:   fleet.Peer{Name: "x", Tags: []string{"os=darwin"}},
			want:   true,
		},
		// Spec-50 §G1 — name=
		{"name= exact hit", mustParse(t, "name=macbook"), mac, true},
		{"name= miss", mustParse(t, "name=macbook"), desk, false},
		// Spec-50 §G1 — role=
		{"role= hit", mustParse(t, "role=db"), desk, true},
		{"role= miss against peer with no roles", mustParse(t, "role=db"), vps, false},
		{"role= miss against peer with different roles", mustParse(t, "role=db"), mac, false},
		// Spec-50 §Open question 2 — cross-key AND/OR semantics
		{"AND within group: name= AND tag=", mustParse(t, "@name=macbook,tag=darwin"), mac, true},
		{"AND within group: name hit, tag miss", mustParse(t, "@name=macbook,tag=linux"), mac, false},
		{
			name:   "OR across flags: name= OR tag=",
			filter: mustParse(t, "name=macbook", "tag=gpu"),
			peer:   desk, // matches tag=gpu group
			want:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerMatchesFilters(tt.peer, tt.filter, nil)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

// TestPeerMatchesFilters_OS exercises the os= predicate path including the
// resolver contract: ok=false (probe failed) must drop the peer; an OS
// mismatch on a successful probe also drops; only a matching OS keeps it.
func TestPeerMatchesFilters_OS(t *testing.T) {
	mac := fleet.Peer{Name: "macbook"}
	desk := fleet.Peer{Name: "desk1"}

	osMap := map[string]string{
		"macbook": "darwin",
		"desk1":   "linux",
	}
	resolver := func(p fleet.Peer) (string, bool) {
		o, ok := osMap[p.Name]
		return o, ok
	}

	groups, _ := parsePeerFlags([]string{"os=darwin"})
	if !peerMatchesFilters(mac, groups, resolver) {
		t.Errorf("os=darwin should match mac")
	}
	if peerMatchesFilters(desk, groups, resolver) {
		t.Errorf("os=darwin should not match desk1 (linux)")
	}

	// Unreachable peer: resolver returns ok=false → predicate fails.
	unreach := fleet.Peer{Name: "unreachable"}
	if peerMatchesFilters(unreach, groups, resolver) {
		t.Errorf("os=darwin should not match an unreachable peer (ok=false)")
	}

	// nil resolver: spec says treat every os= predicate as failing. Caller
	// is responsible for only passing nil when no os= terms exist, but we
	// guard defensively.
	if peerMatchesFilters(mac, groups, nil) {
		t.Errorf("nil resolver should make os= predicates fail (defensive)")
	}
}

// TestPeerFilterGroupsUseKey asserts the cheap presence-check the caller
// uses to skip the probe pass entirely when no os= term is present.
func TestPeerFilterGroupsUseKey(t *testing.T) {
	groups, _ := parsePeerFlags([]string{"@tag=a,name=b", "role=c"})
	if peerFilterGroupsUseKey(groups, "os") {
		t.Errorf("no os= term, should be false")
	}
	if !peerFilterGroupsUseKey(groups, "tag") {
		t.Errorf("tag= present, should be true")
	}
	if !peerFilterGroupsUseKey(groups, "name") {
		t.Errorf("name= present, should be true")
	}

	withOS, _ := parsePeerFlags([]string{"os=darwin"})
	if !peerFilterGroupsUseKey(withOS, "os") {
		t.Errorf("os= present, should be true")
	}
}

func TestExtractStepFilter(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		wantTags  []string
		wantNames []string
		wantErr   string
	}{
		{name: "empty", args: nil},
		{name: "single tag", args: []string{"tag=deploy"}, wantTags: []string{"deploy"}},
		{
			name:     "multiple tag flags",
			args:     []string{"tag=a", "tag=b"},
			wantTags: []string{"a", "b"},
		},
		{
			name:     "comma-separated tags",
			args:     []string{"tag=a,tag=b"},
			wantTags: []string{"a", "b"},
		},
		{
			name:     "duplicates deduped",
			args:     []string{"tag=a", "tag=a,tag=b"},
			wantTags: []string{"a", "b"},
		},
		// Spec-50 §G2: name= for step-filter.
		{
			name:      "single name",
			args:      []string{"name=install nvim"},
			wantNames: []string{"install nvim"},
		},
		{
			name:      "mixed tags and names",
			args:      []string{"tag=deploy,name=install nvim", "tag=cron"},
			wantTags:  []string{"deploy", "cron"},
			wantNames: []string{"install nvim"},
		},
		{
			name:      "duplicate names deduped",
			args:      []string{"name=a", "name=a,name=b"},
			wantNames: []string{"a", "b"},
		},
		{
			name:    "missing equals",
			args:    []string{"deploy"},
			wantErr: "expected key=value",
		},
		{
			name:    "unsupported key",
			args:    []string{"os=linux"},
			wantErr: "unsupported --step-filter key",
		},
		{
			name:    "empty value",
			args:    []string{"tag="},
			wantErr: "empty value",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotTags, gotNames, err := extractStepFilter(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(gotTags, tt.wantTags) {
				t.Errorf("tags = %v, want %v", gotTags, tt.wantTags)
			}
			if !reflect.DeepEqual(gotNames, tt.wantNames) {
				t.Errorf("names = %v, want %v", gotNames, tt.wantNames)
			}
		})
	}
}

// TestFleetApply_PeerRejectsUnsupportedKey exercises the apply Action
// end-to-end to confirm --peer parse errors propagate as a cli.Exit(2)
// rather than leaking through as a generic error.
func TestFleetApply_PeerRejectsUnsupportedKey(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	peersPath := filepath.Join(dir, "peers.toml")
	const peers = `[[peers]]
name = "laptop"
addr = "laptop.lan:7878"
token = "t"
tags = ["darwin"]
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
		"mooncake", "fleet",
		"--peers-file", peersPath,
		"apply",
		"--peer", "arch=arm64", // unsupported key — spec-50 allowlist is {tag,name,os,role}
		planPath,
	})
	if err == nil {
		t.Fatal("want error for unsupported --peer key")
	}
	if !strings.Contains(err.Error(), "unsupported --peer key") {
		t.Errorf("err = %v, want substring 'unsupported --peer key'", err)
	}
}

// TestFleetApply_PeerNoMatch verifies the user-facing error when
// --peer excludes every peer.
func TestFleetApply_PeerNoMatch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(dir, "xdg"))

	peersPath := filepath.Join(dir, "peers.toml")
	const peers = `[[peers]]
name = "laptop"
addr = "laptop.lan:7878"
token = "t"
tags = ["linux"]

[[peers]]
name = "macbook"
addr = "macbook.lan:7878"
token = "t"
tags = ["darwin"]
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
		"mooncake", "fleet",
		"--peers-file", peersPath,
		"apply",
		"--peer", "tag=nonexistent",
		planPath,
	})
	if err == nil {
		t.Fatal("want error when --peer excludes all peers")
	}
	if !strings.Contains(err.Error(), "--peer selected 0") {
		t.Errorf("err = %v, want substring '--peer selected 0'", err)
	}
}

// TestFleetApply_PeerAndGroupNotSplitByCLI guards the cli/v2 wiring
// that disables StringSliceFlag auto-comma-splitting: without it, a
// single `--peer @tag=linux,role=db` selector would silently become
// two OR-groups (`tag=linux`, `role=db`) instead of one AND-group.
// Smoke test on a real fleet caught this; this test pins it.
//
// Inspect peerFlag via a custom Action so we can assert what reaches
// `c.StringSlice("peer")` before any parsing happens. If cli splits
// commas, we'd see two values; if the App has
// DisableSliceFlagSeparator=true, we see one value with the comma
// intact.
func TestFleetApply_PeerAndGroupNotSplitByCLI(t *testing.T) {
	var got []string
	app := &cli.App{
		DisableSliceFlagSeparator: true,
		Commands: []*cli.Command{
			{
				Name: "fleet",
				Subcommands: []*cli.Command{
					{
						Name: "apply",
						Flags: []cli.Flag{
							&cli.StringSliceFlag{Name: "peer"},
						},
						Action: func(c *cli.Context) error {
							got = c.StringSlice("peer")
							return nil
						},
					},
				},
			},
		},
		Writer:    io.Discard,
		ErrWriter: io.Discard,
	}
	if err := app.Run([]string{"mooncake", "fleet", "apply", "--peer", "@tag=linux,role=db"}); err != nil {
		t.Fatalf("run: %v", err)
	}
	if len(got) != 1 || got[0] != "@tag=linux,role=db" {
		t.Errorf("peer slice = %v; want one entry with comma intact (cli auto-split is back)", got)
	}
}
