package mooncake_test

import (
	"os"
	"path/filepath"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// ---------------------------------------------------------------------------
// Read
// ---------------------------------------------------------------------------

func TestRead_FullFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	want := []byte("hello world\n")
	if err := os.WriteFile(path, want, 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := mooncake.Read(path, mooncake.ReadOptions{})
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("Read = %q; want %q", got, want)
	}
}

func TestRead_WithOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := mooncake.Read(path, mooncake.ReadOptions{Offset: 2})
	if err != nil {
		t.Fatalf("Read(offset=2): %v", err)
	}
	if string(got) != "cdef" {
		t.Errorf("got %q; want %q", got, "cdef")
	}
}

func TestRead_WithLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := mooncake.Read(path, mooncake.ReadOptions{Limit: 3})
	if err != nil {
		t.Fatalf("Read(limit=3): %v", err)
	}
	if string(got) != "abc" {
		t.Errorf("got %q; want %q", got, "abc")
	}
}

func TestRead_OffsetAndLimit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := os.WriteFile(path, []byte("abcdef"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := mooncake.Read(path, mooncake.ReadOptions{Offset: 2, Limit: 2})
	if err != nil {
		t.Fatalf("Read(offset=2,limit=2): %v", err)
	}
	if string(got) != "cd" {
		t.Errorf("got %q; want %q", got, "cd")
	}
}

func TestRead_MissingFile(t *testing.T) {
	_, err := mooncake.Read("/nonexistent/path/file.txt", mooncake.ReadOptions{})
	if err == nil {
		t.Fatal("Read on missing file returned nil error")
	}
}

// ---------------------------------------------------------------------------
// Grep
// ---------------------------------------------------------------------------

func TestGrep_FindsMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("foo bar\nbaz\nfoo end\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := mooncake.Grep("foo", mooncake.GrepOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches)=%d; want 2", len(matches))
	}
	if matches[0].Line != 1 || matches[1].Line != 3 {
		t.Errorf("unexpected line numbers: %v", matches)
	}
}

func TestGrep_ExtensionFilter(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("match here\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "b.txt"), []byte("match here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := mooncake.Grep("match", mooncake.GrepOptions{Dir: dir, Extensions: []string{"go"}})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches)=%d; want 1 (go only)", len(matches))
	}
	if filepath.Ext(matches[0].Path) != ".go" {
		t.Errorf("matched non-go file: %s", matches[0].Path)
	}
}

func TestGrep_MaxResults(t *testing.T) {
	dir := t.TempDir()
	content := "line\nline\nline\nline\nline\n"
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := mooncake.Grep("line", mooncake.GrepOptions{Dir: dir, MaxResults: 2})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 2 {
		t.Fatalf("len(matches)=%d; want 2 (capped)", len(matches))
	}
}

func TestGrep_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("Hello World\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := mooncake.Grep("hello", mooncake.GrepOptions{Dir: dir, CaseInsensitive: true})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("len(matches)=%d; want 1", len(matches))
	}
}

func TestGrep_NoMatches(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("nothing here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	matches, err := mooncake.Grep("zzznomatch", mooncake.GrepOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Grep: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected no matches, got %d", len(matches))
	}
}

func TestGrep_InvalidPattern(t *testing.T) {
	_, err := mooncake.Grep("[invalid", mooncake.GrepOptions{Dir: t.TempDir()})
	if err == nil {
		t.Fatal("Grep with invalid regex returned nil error")
	}
}

// ---------------------------------------------------------------------------
// Glob
// ---------------------------------------------------------------------------

func TestGlob_FindsFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a.go", "b.go", "c.txt"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("x"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	paths, err := mooncake.Glob("*.go", mooncake.GlobOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 2 {
		t.Fatalf("len(paths)=%d; want 2", len(paths))
	}
}

func TestGlob_NoMatches(t *testing.T) {
	dir := t.TempDir()

	paths, err := mooncake.Glob("*.go", mooncake.GlobOptions{Dir: dir})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected empty slice, got %v", paths)
	}
}

func TestGlob_AbsolutePattern(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := mooncake.Glob(filepath.Join(dir, "*.txt"), mooncake.GlobOptions{})
	if err != nil {
		t.Fatalf("Glob: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths)=%d; want 1", len(paths))
	}
}

func TestGlob_InvalidPattern(t *testing.T) {
	_, err := mooncake.Glob("[invalid", mooncake.GlobOptions{})
	if err == nil {
		t.Fatal("Glob with invalid pattern returned nil error")
	}
}

// ---------------------------------------------------------------------------
// No-executor invariant
// ---------------------------------------------------------------------------

// TestReadGrepGlob_NoAuditEvents asserts that Read, Grep, and Glob produce no
// run-log / audit events. We prove this by the absence of any ApplyResult:
// none of these helpers return one.
func TestReadGrepGlob_NoAuditEvents(t *testing.T) {
	// Compile-time proof: the return types are ([]byte,error), ([]Match,error),
	// ([]string,error) — not (*ApplyResult,error). If any of these helpers were
	// routed through the executor, they would have to return an ApplyResult.
	// This test is intentionally trivial — the invariant is structural.
	dir := t.TempDir()
	path := filepath.Join(dir, "probe.txt")
	if err := os.WriteFile(path, []byte("probe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := mooncake.Read(path, mooncake.ReadOptions{}); err != nil {
		t.Errorf("Read: %v", err)
	}
	if _, err := mooncake.Grep("probe", mooncake.GrepOptions{Dir: dir}); err != nil {
		t.Errorf("Grep: %v", err)
	}
	if _, err := mooncake.Glob("*.txt", mooncake.GlobOptions{Dir: dir}); err != nil {
		t.Errorf("Glob: %v", err)
	}
}
