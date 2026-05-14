package agentd

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

func TestLoadOrCreateToken_GeneratesOnFirstCall(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sub", "agentd.token")

	tok, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("LoadOrCreateToken: %v", err)
	}
	if len(tok) < 32 {
		t.Errorf("token too short: %d chars", len(tok))
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("token file mode = %o, want 0600", got)
	}
}

func TestLoadOrCreateToken_Idempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.token")

	first, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("first: %v", err)
	}
	second, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("second: %v", err)
	}
	if first != second {
		t.Errorf("token changed between calls: first=%q second=%q", first, second)
	}
}

func TestLoadOrCreateToken_RegeneratesEmptyFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.token")
	if err := os.WriteFile(path, []byte("   \n"), 0o600); err != nil {
		t.Fatalf("seed empty file: %v", err)
	}

	tok, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("LoadOrCreateToken: %v", err)
	}
	if tok == "" {
		t.Fatal("got empty token from whitespace file")
	}
}

func TestLoadOrCreateToken_TrimsWhitespace(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agentd.token")
	if err := os.WriteFile(path, []byte("  abc123  \n"), 0o600); err != nil {
		t.Fatalf("seed: %v", err)
	}

	tok, err := LoadOrCreateToken(path)
	if err != nil {
		t.Fatalf("LoadOrCreateToken: %v", err)
	}
	if tok != "abc123" {
		t.Errorf("got %q, want %q", tok, "abc123")
	}
}

func TestLoadOrCreateToken_RejectsEmptyPath(t *testing.T) {
	_, err := LoadOrCreateToken("")
	if err == nil {
		t.Fatal("want error on empty path")
	}
}

func TestLoadOrCreateToken_PropagatesReadErrors(t *testing.T) {
	// A directory entry where a file is expected — read returns a non-
	// not-exist error, which the caller must propagate (not silently
	// regenerate over a directory).
	dir := t.TempDir()
	path := filepath.Join(dir, "is-a-dir")
	if err := os.Mkdir(path, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	_, err := LoadOrCreateToken(path)
	if err == nil {
		t.Fatal("want error reading a directory as a file")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Errorf("want non-NotExist error, got: %v", err)
	}
}
