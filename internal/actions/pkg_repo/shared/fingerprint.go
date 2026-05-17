// Package shared fingerprint helpers — exported for the per-driver
// sub-packages.
package shared

// F034: pkg.repo's gpg_key_fingerprint is required at validate-time
// but pre-fix was never compared against the fetched key. This file
// implements the missing verification step: parse the fetched key
// (binary .gpg or ASCII-armored .asc), compute the primary key's V4
// fingerprint, and compare against the operator's pinned value.
//
// Indirected through a package-level var so unit tests can stub the
// verifier (the existing test fixtures use a fake key body that can't
// be parsed by openpgp; only the new F034 regression tests exercise
// the real verifier).

import (
	"bytes"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/openpgp"       //nolint:staticcheck // x/crypto/openpgp is deprecated but the maintained replacements aren't in our module graph yet; in-process fingerprint parsing is preferable to shelling to gpg.
	"golang.org/x/crypto/openpgp/armor" //nolint:staticcheck
)

// VerifyKeyFingerprint is the production fingerprint check. Tests
// replace it with a no-op (or a deterministic stub) when they use the
// fake key body in newStubFS — the real verifier rejects that body
// at openpgp.ReadKeyRing, which would otherwise break tests unrelated
// to fingerprint pinning.
var VerifyKeyFingerprint = RealVerifyKeyFingerprint

// RealVerifyKeyFingerprint parses body as an OpenPGP public key
// (binary or ASCII-armored), extracts the V4 fingerprint of the
// primary key, and compares against want. Comparison is
// case-insensitive and ignores whitespace so an operator can paste
// the fingerprint with or without `gpg --fingerprint` spacing.
//
// Returns a clear error on parse failure or mismatch; no error if
// the parsed fingerprint matches the pinned one.
func RealVerifyKeyFingerprint(body []byte, want string) error {
	if strings.TrimSpace(want) == "" {
		// No pinned fingerprint configured — nothing to compare. Caller
		// should not reach this branch (we early-return at the call site
		// when want is empty), but be defensive.
		return nil
	}
	got, err := primaryKeyFingerprintLocal(body)
	if err != nil {
		return fmt.Errorf("parse fetched gpg key: %w", err)
	}
	if NormalizeFingerprint(got) != NormalizeFingerprint(want) {
		return fmt.Errorf(
			"fetched gpg key fingerprint %s does not match pinned fingerprint %s",
			got, want,
		)
	}
	return nil
}

// primaryKeyFingerprintLocal reads an OpenPGP keyring (binary or
// ASCII-armored) and returns the primary key fingerprint of the first
// entity as uppercase hex.
func primaryKeyFingerprintLocal(body []byte) (string, error) {
	r, err := readerForKey(body)
	if err != nil {
		return "", err
	}
	entities, err := openpgp.ReadKeyRing(r)
	if err != nil {
		return "", fmt.Errorf("read keyring: %w", err)
	}
	if len(entities) == 0 {
		return "", fmt.Errorf("no entities in keyring")
	}
	pk := entities[0].PrimaryKey
	if pk == nil {
		return "", fmt.Errorf("entity has no primary key")
	}
	return strings.ToUpper(fmt.Sprintf("%x", pk.Fingerprint[:])), nil
}

// readerForKey returns a reader that decodes ASCII-armored input
// transparently. Binary input passes through unchanged.
func readerForKey(body []byte) (io.Reader, error) {
	if bytes.HasPrefix(bytes.TrimSpace(body), []byte("-----BEGIN")) {
		block, err := armor.Decode(bytes.NewReader(body))
		if err != nil {
			return nil, fmt.Errorf("decode armor: %w", err)
		}
		return block.Body, nil
	}
	return bytes.NewReader(body), nil
}

// NormalizeFingerprint upper-cases the hex and strips the
// inconsequential separators so two transcriptions of the same
// fingerprint compare equal:
//
//	"9DC8 5822 9FC7 DD38 854A  E2D8 8D81 803C 0EBF CD88"
//	"0x9dc858229fc7dd38854ae2d88d81803c0ebfcd88"
//	"9DC8:5822:9FC7:..." (gpg --fingerprint colon form)
//
// Does NOT validate hex-ness — a non-hex character survives and forces
// a mismatch at compare time, which is exactly the safety property we
// want.
func NormalizeFingerprint(s string) string {
	out := strings.ToUpper(strings.TrimSpace(s))
	out = strings.TrimPrefix(out, "0X")
	out = strings.ReplaceAll(out, " ", "")
	out = strings.ReplaceAll(out, "\t", "")
	out = strings.ReplaceAll(out, ":", "")
	return out
}
