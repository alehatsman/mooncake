package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadPeers_MissingFileReturnsEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "does-not-exist.toml")
	cfg, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(cfg.Peers) != 0 {
		t.Errorf("want empty peers, got %d", len(cfg.Peers))
	}
}

func TestLoadPeers_RoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")

	original := &Config{
		Peers: []Peer{
			{
				Name:      "laptop",
				Addr:      "laptop.lan:7878",
				Transport: TransportAgentd,
				Token:     "tok-laptop",
				Tags:      []string{"linux", "workstation"},
			},
			{
				Name:      "macbook",
				Addr:      "macbook.lan:7878",
				Transport: TransportAgentd,
				Token:     "tok-macbook",
				Tags:      []string{"darwin"},
			},
		},
	}
	if err := SavePeers(path, original); err != nil {
		t.Fatalf("SavePeers: %v", err)
	}

	// File on disk is mode 0600.
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}

	loaded, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(loaded.Peers) != 2 {
		t.Fatalf("want 2 peers, got %d", len(loaded.Peers))
	}
	for i, p := range loaded.Peers {
		want := original.Peers[i]
		if p.Name != want.Name || p.Addr != want.Addr ||
			p.Transport != want.Transport || p.Token != want.Token {
			t.Errorf("peer[%d] mismatch: got %+v, want %+v", i, p, want)
		}
		if len(p.Tags) != len(want.Tags) {
			t.Errorf("peer[%d] tag count mismatch", i)
		}
	}
}

func TestLoadPeers_DefaultsTransportToAgentd(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	// No transport on disk → default to agentd at validate time.
	const content = `[[peers]]
name = "x"
addr = "x:7878"
token = "tok"
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}
	cfg, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if cfg.Peers[0].Transport != TransportAgentd {
		t.Errorf("transport = %q, want %q", cfg.Peers[0].Transport, TransportAgentd)
	}
}

func TestPeer_Validate_RejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		p    Peer
		want string // substring of expected error
	}{
		{"empty name", Peer{Addr: "x:1", Token: "t"}, "name is empty"},
		{"long name", Peer{Name: strings.Repeat("a", 65), Addr: "x:1", Token: "t"}, "exceeds 64"},
		{"bad char in name", Peer{Name: "with space", Addr: "x:1", Token: "t"}, "invalid char"},
		{"missing addr", Peer{Name: "x", Token: "t"}, "addr is required"},
		{"bad addr", Peer{Name: "x", Addr: "no-port", Token: "t"}, "invalid"},
		{"missing token agentd", Peer{Name: "x", Addr: "x:1"}, "token is required"},
		{"unknown transport", Peer{Name: "x", Addr: "x:1", Token: "t", Transport: "wat"}, "unknown transport"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			p := tc.p
			err := p.Validate()
			if err == nil {
				t.Fatal("want error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err=%q does not contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestPeer_Validate_AcceptsValid(t *testing.T) {
	p := Peer{Name: "ok", Addr: "host:1234", Token: "tok"}
	if err := p.Validate(); err != nil {
		t.Errorf("want accept, got %v", err)
	}
	if p.Transport != TransportAgentd {
		t.Errorf("expected Validate to default Transport, got %q", p.Transport)
	}
}

func TestConfig_Validate_RejectsDuplicates(t *testing.T) {
	cfg := &Config{Peers: []Peer{
		{Name: "x", Addr: "h:1", Token: "t"},
		{Name: "x", Addr: "h:2", Token: "t"},
	}}
	err := cfg.Validate()
	if err == nil || !strings.Contains(err.Error(), "duplicate") {
		t.Fatalf("want duplicate-name error, got %v", err)
	}
}

func TestSavePeers_AtomicReplace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	first := &Config{Peers: []Peer{{Name: "v1", Addr: "h:1", Token: "t"}}}
	if err := SavePeers(path, first); err != nil {
		t.Fatalf("first save: %v", err)
	}

	second := &Config{Peers: []Peer{{Name: "v2", Addr: "h:1", Token: "t"}}}
	if err := SavePeers(path, second); err != nil {
		t.Fatalf("second save: %v", err)
	}

	loaded, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(loaded.Peers) != 1 || loaded.Peers[0].Name != "v2" {
		t.Errorf("want one peer named v2, got %+v", loaded.Peers)
	}
	// No leftover .tmp.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected .tmp to be gone, stat err = %v", err)
	}
}

func TestSavePeers_RejectsInvalid(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	cfg := &Config{Peers: []Peer{{Name: ""}}} // invalid
	if err := SavePeers(path, cfg); err == nil {
		t.Fatal("want error saving invalid config")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("expected no file to be created on validation failure")
	}
}

func TestDefaultPeersPath_HonorsXDG(t *testing.T) {
	xdgDir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", xdgDir)
	got, err := DefaultPeersPath()
	if err != nil {
		t.Fatalf("DefaultPeersPath: %v", err)
	}
	want := filepath.Join(xdgDir, "mooncake", "peers.toml")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
