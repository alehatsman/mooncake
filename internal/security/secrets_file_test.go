package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileProvider_HappyPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "secret.token")
	if err := os.WriteFile(path, []byte("supersecret\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := FileProvider{}.Resolve(path)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "supersecret" {
		t.Errorf("got %q, want 'supersecret' (newline must be trimmed)", got)
	}
}

func TestFileProvider_TrimsCRLF(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "win.token")
	if err := os.WriteFile(path, []byte("token\r\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := FileProvider{}.Resolve(path)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "token" {
		t.Errorf("got %q, want 'token'", got)
	}
}

// TestFileProvider_PreservesInternalWhitespace: only one trailing
// newline is stripped. Internal whitespace must survive — a secret
// legitimately containing newlines (private keys) shouldn't be
// corrupted.
func TestFileProvider_PreservesInternalWhitespace(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "key.pem")
	body := "-----BEGIN-----\nline1\nline2\n-----END-----\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := FileProvider{}.Resolve(path)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	want := strings.TrimSuffix(body, "\n")
	if got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestFileProvider_MissingFileErrors(t *testing.T) {
	_, err := FileProvider{}.Resolve("/tmp/definitely-not-a-secret-file-xyzqq")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want 'not found'", err)
	}
}

func TestFileProvider_DirectoryErrors(t *testing.T) {
	dir := t.TempDir()
	_, err := FileProvider{}.Resolve(dir)
	if err == nil {
		t.Fatal("expected error for directory")
	}
	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("err = %v, want 'directory'", err)
	}
}

func TestFileProvider_EmptyFileErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.token")
	if err := os.WriteFile(path, nil, 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, err := FileProvider{}.Resolve(path)
	if err == nil {
		t.Fatal("expected error for empty file")
	}
	if !strings.Contains(err.Error(), "empty") {
		t.Errorf("err = %v, want 'empty'", err)
	}
}

func TestFileProvider_EmptyPathErrors(t *testing.T) {
	_, err := FileProvider{}.Resolve("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
	if !strings.Contains(err.Error(), "empty path") {
		t.Errorf("err = %v, want 'empty path'", err)
	}
}

// TestFileProvider_ErrorMessageDoesNotIncludePath: error messages on
// failure must not echo the secret path — the path itself can be a
// hint about what's being protected (e.g. /etc/vault-token gives
// away the secret name).
func TestFileProvider_ErrorMessageDoesNotIncludePath(t *testing.T) {
	path := "/tmp/secret-leak-xyzzy"
	_, err := FileProvider{}.Resolve(path)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "xyzzy") {
		t.Errorf("error leaked path: %v", err)
	}
}

func TestFileProvider_RegisteredAsFile(t *testing.T) {
	// Round-trip: ensure DefaultRegistry routes file: through FileProvider.
	dir := t.TempDir()
	path := filepath.Join(dir, "reg.token")
	if err := os.WriteFile(path, []byte("via-registry\n"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := DefaultRegistry.Resolve("file:" + path)
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "via-registry" {
		t.Errorf("got %q, want 'via-registry'", got)
	}
}
