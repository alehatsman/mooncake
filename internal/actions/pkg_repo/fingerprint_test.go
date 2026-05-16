//nolint:revive // package name follows action convention
package pkg_repo

// F034 regression: pkg.repo MUST verify the fetched gpg key against
// the operator-pinned fingerprint before writing it to the trusted
// keyring. Pre-fix the fingerprint field was captured and rendered
// but never compared — these tests would all fail (verifyKeyFingerprint
// was a no-op or didn't exist).

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"golang.org/x/crypto/openpgp" //nolint:staticcheck // same vendor as fingerprint.go
)

// generateTestKey creates a small OpenPGP key entity and returns its
// serialized binary form plus its uppercase-hex primary fingerprint.
// Used to drive the real verifier without depending on a fixture file
// or a remote key server.
func generateTestKey(t *testing.T) (body []byte, fingerprint string) {
	t.Helper()
	entity, err := openpgp.NewEntity("F034 Test", "regression key", "f034@test", nil)
	if err != nil {
		t.Fatalf("openpgp.NewEntity: %v", err)
	}
	// Drop signing-only subkeys / clear timing fields that NewEntity adds
	// so the serialized output stays small + deterministic.
	for _, id := range entity.Identities {
		id.SelfSignature.CreationTime = time.Unix(0, 0)
	}
	var buf bytes.Buffer
	if err := entity.Serialize(&buf); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	return buf.Bytes(), strings.ToUpper(fingerprintHex(entity.PrimaryKey.Fingerprint[:]))
}

func fingerprintHex(b []byte) string {
	var sb strings.Builder
	const hexAlpha = "0123456789ABCDEF"
	for _, x := range b {
		sb.WriteByte(hexAlpha[x>>4])
		sb.WriteByte(hexAlpha[x&0x0f])
	}
	return sb.String()
}

func TestRealVerifyKeyFingerprint_MatchAccepts(t *testing.T) {
	body, fp := generateTestKey(t)
	if err := realVerifyKeyFingerprint(body, fp); err != nil {
		t.Errorf("expected match to accept, got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_MismatchRejects(t *testing.T) {
	body, _ := generateTestKey(t)
	wrong := "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF"
	err := realVerifyKeyFingerprint(body, wrong)
	if err == nil {
		t.Fatal("expected mismatch to reject; got nil error")
	}
	if !strings.Contains(err.Error(), "does not match pinned") {
		t.Errorf("error should name the mismatch; got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_TolerantOfFormatting(t *testing.T) {
	body, fp := generateTestKey(t)
	// Same fingerprint, written with the spacing gpg --fingerprint
	// emits.
	spaced := fp[:4] + " " + fp[4:8] + " " + fp[8:12] + " " + fp[12:16] +
		" " + fp[16:20] + "  " + fp[20:24] + " " + fp[24:28] + " " +
		fp[28:32] + " " + fp[32:36] + " " + fp[36:]
	if err := realVerifyKeyFingerprint(body, spaced); err != nil {
		t.Errorf("expected spaced form to match canonical, got %v", err)
	}
	// 0x prefix
	if err := realVerifyKeyFingerprint(body, "0x"+strings.ToLower(fp)); err != nil {
		t.Errorf("expected 0x-prefixed lowercase form to match, got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_RejectsMalformedKey(t *testing.T) {
	err := realVerifyKeyFingerprint([]byte("not a gpg key"), "00")
	if err == nil {
		t.Fatal("expected parse failure on garbage input")
	}
	if !strings.Contains(err.Error(), "parse fetched gpg key") {
		t.Errorf("error should mention parse failure; got %v", err)
	}
}

// TestApply_RefusesAndDoesNotWriteOnFingerprintMismatch is the
// end-to-end shape: apply runs with a real verifier (not the no-op
// stub), fingerprint mismatch must abort BEFORE writeAtomic.
func TestApply_RefusesAndDoesNotWriteOnFingerprintMismatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	// Replace the no-op verifier the stub installed with the real one
	// for this test only — same lifetime as the other stubFS overrides.
	verifyKeyFingerprint = realVerifyKeyFingerprint
	// Generate a real key whose fingerprint we know.
	keyBody, realFP := generateTestKey(t)
	s.keyBody = keyBody

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "mismatch",
		Apt: &config.PkgRepoApt{
			URI:               "https://example.com/repo",
			Suites:            []string{"stable"},
			GPGKeyURL:         "https://example.com/key.gpg",
			GPGKeyFingerprint: "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF",
		},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected Run to reject fingerprint mismatch, got nil error")
	}
	if !strings.Contains(err.Error(), "does not match pinned") {
		t.Errorf("error should name the mismatch; got %v", err)
	}
	// Crucially: the keyring must NOT have been written.
	keyringPath := filepath.Join(s.keyringsDir, "mismatch.gpg")
	if _, statErr := os.Stat(keyringPath); statErr == nil {
		t.Errorf("keyring was written despite fingerprint mismatch: %s", keyringPath)
	}
	// And apt-get update must NOT have been called.
	if s.updateCalled != 0 {
		t.Errorf("apt-get update called despite failure: %d times", s.updateCalled)
	}
	_ = realFP // referenced for symmetry with the match-case test below
}

func TestApply_AcceptsAndWritesOnFingerprintMatch(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStubFS(t)
	verifyKeyFingerprint = realVerifyKeyFingerprint
	keyBody, realFP := generateTestKey(t)
	s.keyBody = keyBody

	step := &config.Step{PkgRepo: &config.PkgRepo{
		Name: "match",
		Apt: &config.PkgRepoApt{
			URI:               "https://example.com/repo",
			Suites:            []string{"stable"},
			GPGKeyURL:         "https://example.com/key.gpg",
			GPGKeyFingerprint: realFP,
		},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("Run: %v", err)
	}
	keyringPath := filepath.Join(s.keyringsDir, "match.gpg")
	if _, err := os.Stat(keyringPath); err != nil {
		t.Errorf("keyring not written on match: %v", err)
	}
}
