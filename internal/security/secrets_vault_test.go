package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
)

// makeTestVault returns (vaultDir, identityFile, identity, recipient) for a
// hermetic test vault. Callers own cleanup via t.TempDir.
func makeTestVault(t *testing.T) (vaultDir string, ident age.Identity, recip age.Recipient) {
	t.Helper()
	tmp := t.TempDir()
	vaultDir = filepath.Join(tmp, "vault")
	if err := os.MkdirAll(vaultDir, 0o700); err != nil {
		t.Fatalf("mkdir vault: %v", err)
	}

	id, err := age.GenerateX25519Identity()
	if err != nil {
		t.Fatalf("GenerateX25519Identity: %v", err)
	}
	return vaultDir, id, id.Recipient()
}

// writeVaultSecret encrypts plaintext and writes it to <vaultDir>/<name>.age.
func writeVaultSecret(t *testing.T, vaultDir, name, plaintext string, recip age.Recipient) {
	t.Helper()
	ct, err := AgeEncryptBytes([]byte(plaintext), recip)
	if err != nil {
		t.Fatalf("encrypt %s: %v", name, err)
	}
	dir := filepath.Dir(filepath.Join(vaultDir, name+".age"))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vaultDir, name+".age"), ct, 0o600); err != nil {
		t.Fatalf("write %s.age: %v", name, err)
	}
}

func newTestProvider(t *testing.T, vaultDir string, ident age.Identity) *VaultProvider {
	t.Helper()
	return &VaultProvider{
		cache: make(map[string]string),
		identFn: func() ([]age.Identity, error) {
			return []age.Identity{ident}, nil
		},
		vaultDir: func() (string, error) {
			return vaultDir, nil
		},
	}
}

func TestVaultProvider_HappyPath(t *testing.T) {
	vaultDir, ident, recip := makeTestVault(t)
	writeVaultSecret(t, vaultDir, "db/password", "s3cr3t\n", recip)

	p := newTestProvider(t, vaultDir, ident)
	got, err := p.Resolve("db/password")
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if got != "s3cr3t" {
		t.Errorf("got %q, want %q", got, "s3cr3t")
	}
}

func TestVaultProvider_Cache(t *testing.T) {
	vaultDir, ident, recip := makeTestVault(t)
	writeVaultSecret(t, vaultDir, "token", "abc123", recip)

	p := newTestProvider(t, vaultDir, ident)
	v1, err := p.Resolve("token")
	if err != nil {
		t.Fatalf("first Resolve: %v", err)
	}
	// Overwrite the file to prove the second call returns cached value.
	if err := os.WriteFile(filepath.Join(vaultDir, "token.age"), []byte("garbage"), 0o600); err != nil {
		t.Fatalf("overwrite: %v", err)
	}
	v2, err := p.Resolve("token")
	if err != nil {
		t.Fatalf("second Resolve: %v", err)
	}
	if v1 != v2 {
		t.Errorf("cache miss: v1=%q v2=%q", v1, v2)
	}
}

func TestVaultProvider_NotFound(t *testing.T) {
	vaultDir, ident, _ := makeTestVault(t)
	p := newTestProvider(t, vaultDir, ident)
	_, err := p.Resolve("missing/key")
	if err == nil {
		t.Fatal("expected error for missing secret")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v, want substring 'not found'", err)
	}
}

func TestVaultProvider_PathTraversal(t *testing.T) {
	vaultDir, ident, _ := makeTestVault(t)
	p := newTestProvider(t, vaultDir, ident)
	_, err := p.Resolve("../../etc/passwd")
	if err == nil {
		t.Fatal("expected path-traversal error")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Errorf("err = %v, want substring 'traversal'", err)
	}
}

func TestVaultProvider_EmptyPath(t *testing.T) {
	vaultDir, ident, _ := makeTestVault(t)
	p := newTestProvider(t, vaultDir, ident)
	_, err := p.Resolve("")
	if err == nil {
		t.Fatal("expected error for empty path")
	}
}

func TestVaultProvider_WrongIdentity(t *testing.T) {
	vaultDir, _, recip := makeTestVault(t)
	writeVaultSecret(t, vaultDir, "secret", "val", recip)

	// Use a different identity that can't decrypt.
	wrongID, _ := age.GenerateX25519Identity()
	p := newTestProvider(t, vaultDir, wrongID)
	_, err := p.Resolve("secret")
	if err == nil {
		t.Fatal("expected decrypt error with wrong identity")
	}
}
