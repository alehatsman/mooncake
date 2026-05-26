package fleet

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v2"
)

// newReadTokenContext builds the minimal *cli.Context that readToken
// inspects: just the --insecure-token-on-cmdline bool flag. App's
// Reader/ErrWriter are stubbed to avoid stdin reads in tests that
// only exercise file/literal paths.
func newReadTokenContext(t *testing.T, insecure bool) *cli.Context {
	t.Helper()
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	fs.Bool("insecure-token-on-cmdline", insecure, "")
	app := cli.NewApp()
	return cli.NewContext(app, fs, nil)
}

// TestReadToken_LiteralRequiresInsecureFlag — F031(a). The literal:
// source must error when --insecure-token-on-cmdline is absent, with
// a message that names the flag so the operator knows the path.
func TestReadToken_LiteralRequiresInsecureFlag(t *testing.T) {
	ctx := newReadTokenContext(t, false)
	tok, err := readToken(ctx, "literal:abc123")
	if err == nil {
		t.Fatalf("expected error; got token %q", tok)
	}
	if !strings.Contains(err.Error(), "insecure-token-on-cmdline") {
		t.Errorf("error should name the opt-in flag; got %v", err)
	}
}

// TestReadToken_LiteralWithFlagAccepts — F031(a). With the opt-in
// flag present, literal: passes the token through unchanged.
func TestReadToken_LiteralWithFlagAccepts(t *testing.T) {
	ctx := newReadTokenContext(t, true)
	tok, err := readToken(ctx, "literal:abc123")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tok != "abc123" {
		t.Errorf("token = %q, want %q", tok, "abc123")
	}
}

// TestReadToken_FileRejectsGroupReadable — F031(b). A token file with
// any group or world bits set must be refused, mirroring the
// password-file owner-only invariant (F030).
func TestReadToken_FileRejectsGroupReadable(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(path, []byte("xyz"), 0o644); err != nil {
		t.Fatal(err)
	}
	ctx := newReadTokenContext(t, false)
	tok, err := readToken(ctx, "file:"+path)
	if err == nil {
		t.Fatalf("expected error on 0644 token file; got token %q", tok)
	}
	if !strings.Contains(err.Error(), "group/world-accessible") {
		t.Errorf("error should name the perm issue; got %v", err)
	}
}

// TestReadToken_FileAcceptsOwnerOnly — F031(b). 0600 passes (and so
// do stricter modes — F030's bitmask logic covers it for the
// password-side; here we rely on the same `&0o077 != 0` check).
func TestReadToken_FileAcceptsOwnerOnly(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "token.txt")
	if err := os.WriteFile(path, []byte("xyz\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx := newReadTokenContext(t, false)
	tok, err := readToken(ctx, "file:"+path)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if tok != "xyz" {
		t.Errorf("token = %q, want %q (whitespace must be trimmed)", tok, "xyz")
	}
}
