package vault

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"filippo.io/age"
	"github.com/alehatsman/mooncake/internal/security"
)

// setupVault creates a temp vault dir + identity and returns cleanup helper.
func setupVault(t *testing.T) (vaultDir string, ident *age.X25519Identity) {
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
	return vaultDir, id
}

func writeSecret(t *testing.T, vaultDir, name, plaintext string, recips ...age.Recipient) {
	t.Helper()
	ct, err := security.AgeEncryptBytes([]byte(plaintext), recips...)
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

// ---- readRecipientsFile ----

func TestReadRecipientsFile_Empty(t *testing.T) {
	dir := t.TempDir()
	recs, err := readRecipientsFile(dir)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(recs) != 0 {
		t.Errorf("expected 0 records, got %d", len(recs))
	}
}

func TestReadRecipientsFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	id1, _ := age.GenerateX25519Identity()
	id2, _ := age.GenerateX25519Identity()
	in := []namedRecipient{
		{name: "laptop", pubkey: id1.Recipient().String()},
		{name: "", pubkey: id2.Recipient().String()},
	}
	if err := writeRecipientsFile(dir, in); err != nil {
		t.Fatalf("write: %v", err)
	}
	out, err := readRecipientsFile(dir)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("got %d records, want 2", len(out))
	}
	if out[0].name != "laptop" || out[0].pubkey != id1.Recipient().String() {
		t.Errorf("rec[0] = %+v", out[0])
	}
	if out[1].pubkey != id2.Recipient().String() {
		t.Errorf("rec[1] = %+v", out[1])
	}
}

func TestReadRecipientsFile_Dedup(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	dir := t.TempDir()
	// Write same pubkey twice.
	content := id.Recipient().String() + "\n" + id.Recipient().String() + "\n"
	if err := os.WriteFile(filepath.Join(dir, recipientsFile), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	recs, err := readRecipientsFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	// readRecipientsFile does not dedup — collectRecipients does; raw file has 2.
	if len(recs) != 2 {
		t.Errorf("got %d, want 2 (raw file parses both)", len(recs))
	}
}

// ---- collectRecipients ----

func TestCollectRecipients_FromFile(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	dir := t.TempDir()
	recs := []namedRecipient{{name: "test", pubkey: id.Recipient().String()}}
	if err := writeRecipientsFile(dir, recs); err != nil {
		t.Fatal(err)
	}
	// Override identity path to non-existent so only file recipients are used.
	t.Setenv(security.VaultIdentityEnv, filepath.Join(dir, "no-identity.txt"))
	got, err := collectRecipients(dir, nil)
	if err != nil {
		t.Fatalf("collectRecipients: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d recipients, want 1", len(got))
	}
}

func TestCollectRecipients_DedupsOwnKey(t *testing.T) {
	id, _ := age.GenerateX25519Identity()
	dir := t.TempDir()
	// Register own pubkey in recipients.txt.
	recs := []namedRecipient{{name: "self", pubkey: id.Recipient().String()}}
	if err := writeRecipientsFile(dir, recs); err != nil {
		t.Fatal(err)
	}
	// Also set own identity — collectRecipients must not add duplicate.
	identPath := filepath.Join(dir, "identity.txt")
	if err := os.WriteFile(identPath, []byte(id.String()+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Setenv(security.VaultIdentityEnv, identPath)
	got, err := collectRecipients(dir, nil)
	if err != nil {
		t.Fatalf("collectRecipients: %v", err)
	}
	if len(got) != 1 {
		t.Errorf("got %d recipients, want 1 (own key should not be duplicated)", len(got))
	}
}

// ---- rekeyFile ----

func TestRekeyFile_ChangesRecipients(t *testing.T) {
	vaultDir, id1 := setupVault(t)
	id2, _ := age.GenerateX25519Identity()

	// Encrypt for id1 only.
	writeSecret(t, vaultDir, "token", "s3cr3t", id1.Recipient())

	path := filepath.Join(vaultDir, "token.age")
	// Rekey for id2 only.
	if err := rekeyFile(path, []age.Identity{id1}, []age.Recipient{id2.Recipient()}); err != nil {
		t.Fatalf("rekeyFile: %v", err)
	}

	// id1 can no longer decrypt.
	ct, _ := os.ReadFile(path)
	_, err := age.Decrypt(strings.NewReader(string(ct)), id1)
	if err == nil {
		t.Error("id1 should not be able to decrypt after rekey")
	}

	// id2 can decrypt.
	ct, _ = os.ReadFile(path)
	r, err := age.Decrypt(strings.NewReader(string(ct)), id2)
	if err != nil {
		t.Fatalf("id2 decrypt: %v", err)
	}
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(out) != "s3cr3t" {
		t.Errorf("got %q, want 's3cr3t'", string(out))
	}
}
