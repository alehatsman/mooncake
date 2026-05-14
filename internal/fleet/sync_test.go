package fleet

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func writeFile(t *testing.T, path string, body []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, body, 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func sortByRel(entries []FileEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].RelPath < entries[j].RelPath })
}

func TestWalk_FindsRegularFiles(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config.yml"), []byte("steps: []\n"))
	writeFile(t, filepath.Join(root, "presets", "p.yml"), []byte("x"))
	writeFile(t, filepath.Join(root, "vars", "common.yml"), []byte("foo: bar"))

	entries, total, err := Walk(root, 1<<30)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	sortByRel(entries)
	wantRel := []string{"config.yml", "presets/p.yml", "vars/common.yml"}
	if len(entries) != len(wantRel) {
		t.Fatalf("entry count = %d, want %d (%+v)", len(entries), len(wantRel), entries)
	}
	for i, w := range wantRel {
		if entries[i].RelPath != w {
			t.Errorf("entries[%d].RelPath = %q, want %q", i, entries[i].RelPath, w)
		}
		if entries[i].Size <= 0 {
			t.Errorf("entries[%d].Size = %d", i, entries[i].Size)
		}
	}
	if total <= 0 {
		t.Errorf("total = %d", total)
	}
}

func TestWalk_SkipsTopLevelGitAndDSStore(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config.yml"), []byte("x"))
	writeFile(t, filepath.Join(root, ".git", "HEAD"), []byte("ref: refs/heads/main\n"))
	writeFile(t, filepath.Join(root, ".DS_Store"), []byte("\x00"))
	// A non-skipped dotfile should still appear.
	writeFile(t, filepath.Join(root, ".envrc"), []byte("export FOO=1"))

	entries, _, err := Walk(root, 1<<30)
	if err != nil {
		t.Fatalf("Walk: %v", err)
	}
	rel := make(map[string]struct{}, len(entries))
	for _, e := range entries {
		rel[e.RelPath] = struct{}{}
	}
	for _, banned := range []string{".DS_Store", ".git/HEAD"} {
		if _, found := rel[banned]; found {
			t.Errorf("entry %q should have been skipped", banned)
		}
	}
	if _, ok := rel[".envrc"]; !ok {
		t.Errorf(".envrc should NOT be skipped (only .git and .DS_Store)")
	}
	if _, ok := rel["config.yml"]; !ok {
		t.Errorf("config.yml missing from entries")
	}
}

func TestWalk_EnforcesSizeCap(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.bin"), make([]byte, 1024))
	writeFile(t, filepath.Join(root, "b.bin"), make([]byte, 1024))
	// Total = 2048; cap below should reject.
	_, _, err := Walk(root, 1024)
	if err == nil || !strings.Contains(err.Error(), "max-sync-size") {
		t.Errorf("want size-cap error, got %v", err)
	}
}

func TestWalk_ZeroSizeCapDisabled(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "x.txt"), []byte("x"))
	if _, _, err := Walk(root, 0); err != nil {
		t.Errorf("maxBytes=0 should disable check, got %v", err)
	}
}

func TestWalk_RejectsSymlinks(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlinks not exercised on Windows in this test")
	}
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "config.yml"), []byte("x"))
	if err := os.Symlink("/etc/passwd", filepath.Join(root, "evil")); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	_, _, err := Walk(root, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "symlink") {
		t.Errorf("want symlink rejection, got %v", err)
	}
}

func TestWalk_RejectsEmptyDir(t *testing.T) {
	root := t.TempDir()
	_, _, err := Walk(root, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "no files") {
		t.Errorf("want no-files error, got %v", err)
	}
}

func TestWalk_RejectsNonDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "file.txt")
	writeFile(t, path, []byte("x"))
	_, _, err := Walk(path, 1<<30)
	if err == nil || !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("want non-dir error, got %v", err)
	}
}

func TestFileEntry_ComputeSha256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.bin")
	body := []byte("hash me")
	writeFile(t, path, body)
	e := FileEntry{AbsPath: path}
	if err := e.ComputeSha256(); err != nil {
		t.Fatalf("ComputeSha256: %v", err)
	}
	sum := sha256.Sum256(body)
	want := hex.EncodeToString(sum[:])
	if e.Sha256 != want {
		t.Errorf("sha = %q, want %q", e.Sha256, want)
	}
	// Idempotent — second call leaves it alone.
	if err := e.ComputeSha256(); err != nil {
		t.Errorf("second call: %v", err)
	}
	if e.Sha256 != want {
		t.Errorf("sha changed on second call")
	}
}

func TestScopeFor_StableAndShaped(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000000"
	dir := "/home/alice/dotfiles"
	got1, err := ScopeFor(id, dir)
	if err != nil {
		t.Fatalf("ScopeFor: %v", err)
	}
	got2, err := ScopeFor(id, dir)
	if err != nil {
		t.Fatalf("ScopeFor 2nd: %v", err)
	}
	if got1 != got2 {
		t.Errorf("not deterministic: %q vs %q", got1, got2)
	}
	if !strings.HasPrefix(got1, id+"/") {
		t.Errorf("scope %q does not start with controller id", got1)
	}
	suffix := strings.TrimPrefix(got1, id+"/")
	if len(suffix) != 16 {
		t.Errorf("dir-hash segment len = %d, want 16", len(suffix))
	}
}

func TestScopeFor_DifferentDirsDifferentScopes(t *testing.T) {
	id := "00000000-0000-4000-8000-000000000000"
	a, _ := ScopeFor(id, "/a")
	b, _ := ScopeFor(id, "/b")
	if a == b {
		t.Errorf("different dirs produced same scope: %q", a)
	}
}

func TestScopeFor_RejectsEmptyInputs(t *testing.T) {
	if _, err := ScopeFor("", "/x"); err == nil {
		t.Errorf("want error on empty controller id")
	}
	if _, err := ScopeFor("id", ""); err == nil {
		t.Errorf("want error on empty plan-dir")
	}
}

func TestPeerPath(t *testing.T) {
	cases := []struct {
		root, scope, rel, want string
	}{
		{"/var/lib/mooncake/agentd/synced", "abc/def", "config.yml",
			"/var/lib/mooncake/agentd/synced/abc/def/config.yml"},
		{"/synced", "scope", "presets/p.yml",
			"/synced/scope/presets/p.yml"},
		// leading slash on rel is tolerated.
		{"/synced", "scope", "/config.yml", "/synced/scope/config.yml"},
		// empty rel returns just root + scope (used for BaseDir).
		{"/synced", "scope", "", "/synced/scope"},
	}
	for _, tc := range cases {
		got := PeerPath(tc.root, tc.scope, tc.rel)
		if got != tc.want {
			t.Errorf("PeerPath(%q,%q,%q) = %q, want %q",
				tc.root, tc.scope, tc.rel, got, tc.want)
		}
	}
}
