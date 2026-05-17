// F034 regression: pkg.repo MUST verify the fetched gpg key against
// the operator-pinned fingerprint before writing it to the trusted
// keyring. Pre-fix the fingerprint field was captured and rendered
// but never compared — these tests would all fail
// (VerifyKeyFingerprint was a no-op or didn't exist).
package shared

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/openpgp" //nolint:staticcheck // same vendor as fingerprint.go
)

// generateTestKey creates a small OpenPGP key entity and returns its
// serialized binary form plus its uppercase-hex primary fingerprint.
// Used to drive the real verifier without depending on a fixture
// file or a remote key server. Exported because the apt+dnf apply
// tests reach for it too.
func generateTestKey(t *testing.T) (body []byte, fingerprint string) {
	t.Helper()
	entity, err := openpgp.NewEntity("F034 Test", "regression key", "f034@test", nil)
	if err != nil {
		t.Fatalf("openpgp.NewEntity: %v", err)
	}
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
	if err := RealVerifyKeyFingerprint(body, fp); err != nil {
		t.Errorf("expected match to accept, got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_MismatchRejects(t *testing.T) {
	body, _ := generateTestKey(t)
	wrong := "DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF"
	err := RealVerifyKeyFingerprint(body, wrong)
	if err == nil {
		t.Fatal("expected mismatch to reject; got nil error")
	}
	if !strings.Contains(err.Error(), "does not match pinned") {
		t.Errorf("error should name the mismatch; got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_TolerantOfFormatting(t *testing.T) {
	body, fp := generateTestKey(t)
	spaced := fp[:4] + " " + fp[4:8] + " " + fp[8:12] + " " + fp[12:16] +
		" " + fp[16:20] + "  " + fp[20:24] + " " + fp[24:28] + " " +
		fp[28:32] + " " + fp[32:36] + " " + fp[36:]
	if err := RealVerifyKeyFingerprint(body, spaced); err != nil {
		t.Errorf("expected spaced form to match canonical, got %v", err)
	}
	if err := RealVerifyKeyFingerprint(body, "0x"+strings.ToLower(fp)); err != nil {
		t.Errorf("expected 0x-prefixed lowercase form to match, got %v", err)
	}
}

func TestRealVerifyKeyFingerprint_RejectsMalformedKey(t *testing.T) {
	err := RealVerifyKeyFingerprint([]byte("not a gpg key"), "00")
	if err == nil {
		t.Fatal("expected parse failure on garbage input")
	}
	if !strings.Contains(err.Error(), "parse fetched gpg key") {
		t.Errorf("error should mention parse failure; got %v", err)
	}
}

func TestNormalizeFingerprint(t *testing.T) {
	cases := []struct {
		in, want string
	}{
		{"9DC858229FC7DD38854AE2D88D81803C0EBFCD88", "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"},
		{"9dc858229fc7dd38854ae2d88d81803c0ebfcd88", "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"},
		{"9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88", "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"},
		{"0x9DC858229FC7DD38854AE2D88D81803C0EBFCD88", "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"},
		{"  9DC8:5822:9FC7:DD38:854A:E2D8:8D81:803C:0EBF:CD88  ", "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"},
	}
	for _, c := range cases {
		if got := NormalizeFingerprint(c.in); got != c.want {
			t.Errorf("NormalizeFingerprint(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestRealVerifyKeyFingerprint_UnparseableBytes — pin the
// parse-failure path against non-key bytes. Mirrors what an
// attacker who served raw HTML or a 404 page would look like to the
// verifier.
func TestRealVerifyKeyFingerprint_UnparseableBytes(t *testing.T) {
	err := RealVerifyKeyFingerprint([]byte("not a gpg key"), "9FD3B784BC1C6FC31A8A0A1C1655A0AB68576280")
	if err == nil {
		t.Fatal("expected parse failure on non-key bytes; got nil")
	}
	if !strings.Contains(err.Error(), "parse fetched gpg key") {
		t.Errorf("error should name the failure source; got %q", err.Error())
	}
}
