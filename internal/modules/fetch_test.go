package modules

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// makeFixtureRepo builds a small git repo with an index.yml + component file
// and tags it. Returns the path to the bare clone-source repo.
func makeFixtureRepo(t *testing.T, tag string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(filepath.Join(work, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "init", "-q")
	runGit(t, work, "config", "user.email", "test@example.com")
	runGit(t, work, "config", "user.name", "Test")
	if err := os.WriteFile(filepath.Join(work, "index.yml"), []byte(
		"name: testmod\nexports:\n  default: components/install.yml\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(work, "components", "install.yml"), []byte(
		"name: install\nsteps:\n  - log: \"hi\"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit(t, work, "add", ".")
	runGit(t, work, "commit", "-q", "-m", "init")
	runGit(t, work, "tag", tag)

	bare := filepath.Join(dir, "bare.git")
	runGit(t, "", "clone", "--bare", "-q", work, bare)
	return bare
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	// Disable hooks so the parent project's pre-commit (which assumes a
	// Taskfile-rooted checkout) does not fire inside fixture repos.
	full := append([]string{"-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", full...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestFetcher_FetchAndCacheHit(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()

	f := &Fetcher{
		Root:     cacheRoot,
		CloneURL: func(_ Reference) string { return "file://" + bare },
	}
	ref := Reference{
		Host: "github.com", Owner: "owner", Repo: "testmod", Version: "v1.0.0",
	}

	dir1, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch (miss): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir1, "index.yml")); err != nil {
		t.Fatalf("index.yml not in cache: %v", err)
	}

	// Second fetch hits the cache — break the CloneURL to prove it's not called.
	f.CloneURL = func(_ Reference) string { return "file:///nonexistent" }
	dir2, err := f.Fetch(context.Background(), ref)
	if err != nil {
		t.Fatalf("Fetch (hit): %v", err)
	}
	if dir1 != dir2 {
		t.Errorf("cache dir mismatch: %q vs %q", dir1, dir2)
	}
}

func TestFetcher_MissingTag(t *testing.T) {
	bare := makeFixtureRepo(t, "v1.0.0")
	cacheRoot := t.TempDir()
	f := &Fetcher{
		Root:     cacheRoot,
		CloneURL: func(_ Reference) string { return "file://" + bare },
	}
	ref := Reference{
		Host: "github.com", Owner: "owner", Repo: "testmod", Version: "v9.9.9",
	}
	_, err := f.Fetch(context.Background(), ref)
	if err == nil {
		t.Fatal("expected error for missing tag")
	}
	if !strings.Contains(err.Error(), "no tag v9.9.9") {
		t.Errorf("error = %q, want 'no tag v9.9.9' phrase", err.Error())
	}
}

func TestCacheDir_DefaultRoot(t *testing.T) {
	f := &Fetcher{Root: "/tmp/mc-cache"}
	dir, err := f.CacheDir(Reference{
		Host: "github.com", Owner: "o", Repo: "r", Version: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	want := "/tmp/mc-cache/github.com/o/r@v1"
	if dir != want {
		t.Errorf("CacheDir = %q, want %q", dir, want)
	}
}
