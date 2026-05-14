package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
)

func TestParseFilterFlags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    [][]filterTerm
		wantErr string
	}{
		{
			name: "empty input",
			args: nil,
			want: nil,
		},
		{
			name: "single flag, single term",
			args: []string{"tag=a"},
			want: [][]filterTerm{{{key: "tag", value: "a"}}},
		},
		{
			name: "single flag, AND within commas",
			args: []string{"tag=a,tag=b"},
			want: [][]filterTerm{{
				{key: "tag", value: "a"},
				{key: "tag", value: "b"},
			}},
		},
		{
			name: "multiple flags, OR across",
			args: []string{"tag=a", "tag=b"},
			want: [][]filterTerm{
				{{key: "tag", value: "a"}},
				{{key: "tag", value: "b"}},
			},
		},
		{
			name: "AND-within + OR-across",
			args: []string{"tag=a,tag=b", "tag=c"},
			want: [][]filterTerm{
				{{key: "tag", value: "a"}, {key: "tag", value: "b"}},
				{{key: "tag", value: "c"}},
			},
		},
		{
			name: "value can contain equals (key=value where value itself has =)",
			args: []string{"tag=os=darwin"},
			want: [][]filterTerm{{{key: "tag", value: "os=darwin"}}},
		},
		{
			name: "whitespace trimmed",
			args: []string{"  tag  =  a  , tag=b "},
			want: [][]filterTerm{{
				{key: "tag", value: "a"},
				{key: "tag", value: "b"},
			}},
		},
		{
			name: "empty comma slot is skipped",
			args: []string{"tag=a,,tag=b"},
			want: [][]filterTerm{{
				{key: "tag", value: "a"},
				{key: "tag", value: "b"},
			}},
		},
		{
			name:    "missing equals",
			args:    []string{"justtag"},
			wantErr: "expected key=value",
		},
		{
			name:    "empty key",
			args:    []string{"=value"},
			wantErr: "key and value must be non-empty",
		},
		{
			name:    "empty value",
			args:    []string{"tag="},
			wantErr: "key and value must be non-empty",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseFilterFlags(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected err: %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidatePeerFilterKeys(t *testing.T) {
	// Spec-50 expands the allowlist from {tag} to {tag, name, os, role}.
	// All four keys must validate; an off-list key must produce an error
	// that lists the valid keys (G3).
	for _, args := range [][]string{
		{"tag=darwin"},
		{"name=laptop"},
		{"role=db"},
		{"os=linux"},
		{"tag=a,name=b,role=c,os=d"},
		{"tag=a", "name=b"},
	} {
		groups, err := parseFilterFlags(args)
		if err != nil {
			t.Fatalf("parse %v: %v", args, err)
		}
		if err := validatePeerFilterKeys(groups); err != nil {
			t.Errorf("expected %v to validate, got %v", args, err)
		}
	}

	bad, _ := parseFilterFlags([]string{"arch=arm64"})
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
		g, err := parseFilterFlags(args)
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
		{"AND within group hit", mustParse(t, "tag=linux,tag=gpu"), desk, true},
		{"AND within group miss", mustParse(t, "tag=linux,tag=gpu"), vps, false},
		{"OR across groups hit on left", mustParse(t, "tag=darwin", "tag=server"), mac, true},
		{"OR across groups hit on right", mustParse(t, "tag=darwin", "tag=server"), vps, true},
		{"OR across groups miss both", mustParse(t, "tag=darwin", "tag=server"), desk, false},
		{
			name:   "(A AND B) OR C — left hits",
			filter: mustParse(t, "tag=linux,tag=gpu", "tag=workstation"),
			peer:   desk,
			want:   true,
		},
		{
			name:   "(A AND B) OR C — right hits",
			filter: mustParse(t, "tag=linux,tag=gpu", "tag=workstation"),
			peer:   mac,
			want:   true,
		},
		{
			name:   "(A AND B) OR C — both miss",
			filter: mustParse(t, "tag=linux,tag=gpu", "tag=workstation"),
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
		{"AND within group: name= AND tag=", mustParse(t, "name=macbook,tag=darwin"), mac, true},
		{"AND within group: name hit, tag miss", mustParse(t, "name=macbook,tag=linux"), mac, false},
		{
			name:   "OR across groups: name= OR tag=",
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

	groups, _ := parseFilterFlags([]string{"os=darwin"})
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
	groups, _ := parseFilterFlags([]string{"tag=a,name=b", "role=c"})
	if peerFilterGroupsUseKey(groups, "os") {
		t.Errorf("no os= term, should be false")
	}
	if !peerFilterGroupsUseKey(groups, "tag") {
		t.Errorf("tag= present, should be true")
	}
	if !peerFilterGroupsUseKey(groups, "name") {
		t.Errorf("name= present, should be true")
	}

	withOS, _ := parseFilterFlags([]string{"os=darwin"})
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

// TestFleetApply_PeerFilterRejectsUnsupportedKey exercises the apply Action
// end-to-end to confirm --peer-filter parse errors propagate as a cli.Exit(2)
// rather than leaking through as a generic error.
func TestFleetApply_PeerFilterRejectsUnsupportedKey(t *testing.T) {
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
		"mooncake", "fleet", "apply",
		"--peers-file", peersPath,
		"--peer-filter", "arch=arm64", // unsupported key — spec-50 added name/os/role but not arch
		planPath,
	})
	if err == nil {
		t.Fatal("want error for unsupported peer-filter key")
	}
	if !strings.Contains(err.Error(), "unsupported --peer-filter key") {
		t.Errorf("err = %v, want substring 'unsupported --peer-filter key'", err)
	}
}

// TestFleetApply_PeerFilterNoMatch verifies the user-facing error when
// --peer-filter excludes every peer (vs. when --peers does — different code
// path, different message).
func TestFleetApply_PeerFilterNoMatch(t *testing.T) {
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
		"mooncake", "fleet", "apply",
		"--peers-file", peersPath,
		"--peer-filter", "tag=nonexistent",
		planPath,
	})
	if err == nil {
		t.Fatal("want error when peer-filter excludes all peers")
	}
	if !strings.Contains(err.Error(), "--peer-filter selected 0") {
		t.Errorf("err = %v, want substring '--peer-filter selected 0'", err)
	}
}
