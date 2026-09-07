package modules

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// resetHashCache drops the HashDir memo. Test-only: production code has no
// reason to invalidate, because a module cache dir is immutable once
// populated.
func resetHashCache() {
	hashCacheMu.Lock()
	hashCache = map[string]string{}
	hashCacheMu.Unlock()
}

// writeTree materializes files (relpath -> content) under a fresh temp dir.
func writeTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

func hashOf(t *testing.T, dir string) string {
	t.Helper()
	resetHashCache()
	h, err := HashDir(dir)
	if err != nil {
		t.Fatalf("HashDir(%s): %v", dir, err)
	}
	return h
}

func TestHashDir_StableAcrossIdenticalTrees(t *testing.T) {
	files := map[string]string{
		"index.yml":              "name: m\n",
		"components/a.yml":       "steps: []\n",
		"components/sub/b.yml":   "steps: []\n",
		"assets/deep/nested.txt": "payload",
	}
	a := hashOf(t, writeTree(t, files))
	b := hashOf(t, writeTree(t, files))

	if a != b {
		t.Errorf("identical trees hashed differently:\n  %s\n  %s", a, b)
	}
	if !strings.HasPrefix(a, HashPrefix) {
		t.Errorf("hash %q missing %q prefix", a, HashPrefix)
	}
}

func TestHashDir_DetectsContentAndPathChanges(t *testing.T) {
	base := map[string]string{"index.yml": "name: m\n", "a.yml": "one"}
	want := hashOf(t, writeTree(t, base))

	t.Run("content change", func(t *testing.T) {
		got := hashOf(t, writeTree(t, map[string]string{
			"index.yml": "name: m\n", "a.yml": "two",
		}))
		if got == want {
			t.Error("changed file content produced the same hash")
		}
	})

	t.Run("rename with identical content", func(t *testing.T) {
		got := hashOf(t, writeTree(t, map[string]string{
			"index.yml": "name: m\n", "b.yml": "one",
		}))
		if got == want {
			t.Error("renamed file produced the same hash; path is not covered")
		}
	})

	t.Run("added file", func(t *testing.T) {
		got := hashOf(t, writeTree(t, map[string]string{
			"index.yml": "name: m\n", "a.yml": "one", "c.yml": "",
		}))
		if got == want {
			t.Error("added empty file produced the same hash")
		}
	})
}

// The .git dir differs between two clones of the same tag (pack checksums,
// reflogs), so including it would make the hash useless. Guard the exclusion.
func TestHashDir_IgnoresGitDir(t *testing.T) {
	dir := writeTree(t, map[string]string{"index.yml": "name: m\n"})
	want := hashOf(t, dir)

	gitObjects := filepath.Join(dir, gitDir, "objects")
	if err := os.MkdirAll(gitObjects, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitObjects, "pack"), []byte("noise"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := hashOf(t, dir); got != want {
		t.Errorf(".git contents changed the hash:\n  before %s\n  after  %s", want, got)
	}
}

func TestHashDir_SkipsNonRegularFiles(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation needs privilege on Windows")
	}
	dir := writeTree(t, map[string]string{"index.yml": "name: m\n"})
	want := hashOf(t, dir)

	if err := os.Symlink("index.yml", filepath.Join(dir, "link.yml")); err != nil {
		t.Skipf("symlink unsupported here: %v", err)
	}
	if got := hashOf(t, dir); got != want {
		t.Errorf("symlink changed the hash; non-regular entries should be skipped:\n"+
			"  before %s\n  after  %s", want, got)
	}
}

func TestHashDir_ErrorsOnMissingOrNonDir(t *testing.T) {
	resetHashCache()
	if _, err := HashDir(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("HashDir on a missing dir returned no error")
	}

	file := filepath.Join(t.TempDir(), "f")
	if err := os.WriteFile(file, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	resetHashCache()
	if _, err := HashDir(file); err == nil {
		t.Error("HashDir on a regular file returned no error")
	}
}

// The memo exists so a run resolving one module from many `use:` steps hashes
// it once. It is keyed by dir and never invalidated within a process.
func TestHashDir_MemoizesByDir(t *testing.T) {
	dir := writeTree(t, map[string]string{"a.yml": "one"})
	first := hashOf(t, dir)

	// Mutate without resetting the cache: the memo should return the old hash.
	if err := os.WriteFile(filepath.Join(dir, "a.yml"), []byte("two"), 0o644); err != nil {
		t.Fatal(err)
	}
	cached, err := HashDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if cached != first {
		t.Errorf("memo missed: got %s, want cached %s", cached, first)
	}
	if fresh := hashOf(t, dir); fresh == first {
		t.Error("hash unchanged after mutating a file with the cache reset")
	}
}
