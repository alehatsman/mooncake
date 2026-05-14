package fleet

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestUpsert_AddsNewPeer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	p := Peer{Name: "macbook", Addr: "macbook.lan:7878", Transport: TransportAgentd, Token: "tok-mac"}

	added, diff, err := Upsert(path, p)
	if err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	if !added {
		t.Errorf("added = false, want true for fresh insert")
	}
	if len(diff) != 0 {
		t.Errorf("diff = %v, want empty for fresh insert", diff)
	}

	loaded, err := LoadPeers(path)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(loaded.Peers) != 1 || loaded.Peers[0].Name != "macbook" {
		t.Errorf("loaded %+v", loaded.Peers)
	}
}

func TestUpsert_ReplacesExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	first := Peer{Name: "pc", Addr: "old.lan:7878", Transport: TransportAgentd, Token: "old-tok", Tags: []string{"old"}}
	if _, _, err := Upsert(path, first); err != nil {
		t.Fatalf("first Upsert: %v", err)
	}

	second := Peer{Name: "pc", Addr: "new.lan:7878", Transport: TransportAgentd, Token: "new-tok", Tags: []string{"new", "shiny"}}
	added, diff, err := Upsert(path, second)
	if err != nil {
		t.Fatalf("second Upsert: %v", err)
	}
	if added {
		t.Errorf("added = true on replace, want false")
	}
	if len(diff) == 0 {
		t.Fatalf("diff is empty; expected addr+token+tags changes")
	}
	joined := strings.Join(diff, "\n")
	for _, want := range []string{"addr:", "token:", "tags:"} {
		if !strings.Contains(joined, want) {
			t.Errorf("diff missing %q. Got:\n%s", want, joined)
		}
	}

	loaded, _ := LoadPeers(path)
	if len(loaded.Peers) != 1 {
		t.Errorf("want exactly 1 peer after replace, got %d", len(loaded.Peers))
	}
	if loaded.Peers[0].Addr != "new.lan:7878" {
		t.Errorf("addr not replaced: %s", loaded.Peers[0].Addr)
	}
}

func TestUpsert_NoChangeYieldsEmptyDiff(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	p := Peer{Name: "pc", Addr: "h:7878", Transport: TransportAgentd, Token: "t"}
	if _, _, err := Upsert(path, p); err != nil {
		t.Fatalf("first: %v", err)
	}
	added, diff, err := Upsert(path, p)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if added {
		t.Errorf("added = true on identical replace")
	}
	if len(diff) != 0 {
		t.Errorf("diff = %v, want empty for unchanged peer", diff)
	}
}

func TestUpsert_RejectsInvalidPeer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	bad := Peer{Name: "", Addr: "h:1", Token: "t"}
	_, _, err := Upsert(path, bad)
	if err == nil {
		t.Fatal("want error on invalid peer")
	}
}

func TestUpsert_AppendsAlongsideExisting(t *testing.T) {
	path := filepath.Join(t.TempDir(), "peers.toml")
	p1 := Peer{Name: "a", Addr: "a.lan:7878", Transport: TransportAgentd, Token: "ta"}
	p2 := Peer{Name: "b", Addr: "b.lan:7878", Transport: TransportAgentd, Token: "tb"}
	if _, _, err := Upsert(path, p1); err != nil {
		t.Fatalf("a: %v", err)
	}
	if added, _, err := Upsert(path, p2); err != nil || !added {
		t.Fatalf("b: added=%v err=%v", added, err)
	}
	loaded, _ := LoadPeers(path)
	if len(loaded.Peers) != 2 {
		t.Errorf("want 2 peers, got %d", len(loaded.Peers))
	}
}
