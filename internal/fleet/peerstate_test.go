package fleet

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// withTempStateRoot redirects XDG_STATE_HOME (and LOCALAPPDATA on
// Windows builds) at the test's temp dir so peer-state writes land in
// an isolated location. Returns nothing; the t.Cleanup restores env on
// test exit.
func withTempStateRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("XDG_STATE_HOME", dir)
	// Windows path resolves through LOCALAPPDATA; setting it makes the
	// same test exercise both builds without runtime branching.
	t.Setenv("LOCALAPPDATA", dir)
	return dir
}

func TestPeerState_LoadMissingReturnsZero(t *testing.T) {
	withTempStateRoot(t)
	got, err := LoadPeerState("never-saved")
	if err != nil {
		t.Fatalf("expected nil error for missing state, got %v", err)
	}
	if !got.LastSeenAt.IsZero() {
		t.Fatalf("expected zero PeerState, got %+v", got)
	}
}

func TestPeerState_RoundTrip(t *testing.T) {
	withTempStateRoot(t)
	want := PeerState{
		LastSeenAt:   time.Date(2026, 5, 15, 16, 5, 0, 0, time.UTC),
		LastAddr:     "192.168.1.68:7878",
		LastMooncake: "dev",
	}
	if err := SavePeerState("main_pc", want); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := LoadPeerState("main_pc")
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if !got.LastSeenAt.Equal(want.LastSeenAt) {
		t.Fatalf("LastSeenAt: got %v, want %v", got.LastSeenAt, want.LastSeenAt)
	}
	if got.LastAddr != want.LastAddr {
		t.Fatalf("LastAddr: got %q, want %q", got.LastAddr, want.LastAddr)
	}
	if got.LastMooncake != want.LastMooncake {
		t.Fatalf("LastMooncake: got %q, want %q", got.LastMooncake, want.LastMooncake)
	}
}

func TestPeerState_AtomicWrite_NoTempLeftBehind(t *testing.T) {
	root := withTempStateRoot(t)
	if err := SavePeerState("alpha", PeerState{LastSeenAt: time.Now().UTC()}); err != nil {
		t.Fatal(err)
	}
	// The .tmp staging file must not survive a successful rename.
	tmps, _ := filepath.Glob(filepath.Join(root, "mooncake", "peers", "*.tmp"))
	if len(tmps) != 0 {
		t.Fatalf("expected no .tmp leftovers, found %v", tmps)
	}
}

func TestPeerState_SanitisesPath(t *testing.T) {
	root := withTempStateRoot(t)
	// Names with path-traversal or filesystem-quirky chars should be
	// flattened to underscores. The output file lives strictly under
	// PeerStateDir().
	for _, name := range []string{"../etc/passwd", "weird:name", "with/slash", "back\\slash"} {
		if err := SavePeerState(name, PeerState{LastSeenAt: time.Now().UTC()}); err != nil {
			t.Fatalf("save %q: %v", name, err)
		}
	}
	// Walk the state dir; every file must be under the peers/ subdir.
	dir := filepath.Join(root, "mooncake", "peers")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		// "../etc/passwd" should not land at root/mooncake/etc/passwd
		// — confirmed by ReadDir not seeing escape entries above.
		if strings.ContainsAny(e.Name(), "/\\:") {
			t.Fatalf("unsanitised filename in state dir: %q", e.Name())
		}
	}
}

func TestSanitisePeerName(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"main_pc", "main_pc"},
		{"main-pc", "main-pc"},
		{"box.lan", "box.lan"},
		{"weird name", "weird_name"},
		{"../etc/x", "_etc_x"},
		{"with:colon", "with_colon"},
		{"αβγ", "___"},
		{"....", ""},
	}
	for _, tc := range cases {
		if got := sanitisePeerName(tc.in); got != tc.want {
			t.Errorf("sanitisePeerName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestPeerStatePath_RejectsEmptyAfterSanitise(t *testing.T) {
	withTempStateRoot(t)
	_, err := peerStatePath("....")
	if err == nil {
		t.Fatal("expected error for name that sanitises to empty")
	}
}
