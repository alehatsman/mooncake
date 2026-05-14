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
	ok, _ := parseFilterFlags([]string{"tag=darwin"})
	if err := validatePeerFilterKeys(ok); err != nil {
		t.Fatalf("tag should be allowed: %v", err)
	}

	bad, _ := parseFilterFlags([]string{"name=laptop"})
	if err := validatePeerFilterKeys(bad); err == nil {
		t.Fatal("name= should be rejected in v1")
	} else if !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("err = %v, want 'unsupported'", err)
	}

	mixed, _ := parseFilterFlags([]string{"tag=a,os=linux"})
	if err := validatePeerFilterKeys(mixed); err == nil {
		t.Fatal("mixed tag+os should be rejected")
	}
}

func TestPeerMatchesFilters(t *testing.T) {
	mac := fleet.Peer{Name: "macbook", Tags: []string{"darwin", "workstation"}}
	desk := fleet.Peer{Name: "desk1", Tags: []string{"linux", "gpu"}}
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
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := peerMatchesFilters(tt.peer, tt.filter)
			if got != tt.want {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractStepFilterTags(t *testing.T) {
	tests := []struct {
		name    string
		args    []string
		want    []string
		wantErr string
	}{
		{name: "empty", args: nil, want: nil},
		{name: "single", args: []string{"tag=deploy"}, want: []string{"deploy"}},
		{
			name: "multiple flags",
			args: []string{"tag=a", "tag=b"},
			want: []string{"a", "b"},
		},
		{
			name: "comma-separated",
			args: []string{"tag=a,tag=b"},
			want: []string{"a", "b"},
		},
		{
			name: "duplicates deduped",
			args: []string{"tag=a", "tag=a,tag=b"},
			want: []string{"a", "b"},
		},
		{
			name:    "missing equals",
			args:    []string{"deploy"},
			wantErr: "expected key=value",
		},
		{
			name:    "unsupported key",
			args:    []string{"name=foo"},
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
			got, err := extractStepFilterTags(tt.args)
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
		"--peer-filter", "os=darwin", // unsupported key in v1
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
