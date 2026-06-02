// Package utils provides utility functions for file operations.
package utils

import (
	"crypto/md5" // #nosec G501 -- MD5 used for integrity checks, not security
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"strings"
)

// CalculateSHA256 calculates the SHA256 checksum of a file.
func CalculateSHA256(path string) (string, error) {
	return hashFile(sha256.New(), path)
}

// CalculateMD5 calculates the MD5 checksum of a file.
// Note: MD5 is deprecated for security purposes but still commonly used for integrity checks.
func CalculateMD5(path string) (string, error) {
	// #nosec G401 -- MD5 used for integrity checks, not security
	return hashFile(md5.New(), path)
}

// hashFile streams path's contents through h and returns the hex
// digest. Caller owns the hash.Hash so SHA256 / MD5 / future
// algorithms (blake2, sha1) plug in without copy-pasting the
// open / io.Copy / close / hex.EncodeToString boilerplate that
// `dupl` previously flagged between CalculateSHA256 and CalculateMD5.
func hashFile(h hash.Hash, path string) (checksum string, err error) {
	// #nosec G304 -- file path from user config is intentional functionality
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()
	if _, err = io.Copy(h, f); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// VerifyChecksum verifies a file's checksum against an expected value.
// Supports both SHA256 and MD5 based on the length of the expected checksum.
func VerifyChecksum(path, expected string) (bool, error) {
	var actual string
	var err error

	// Detect checksum type by length
	switch len(expected) {
	case 64: // SHA256
		actual, err = CalculateSHA256(path)
	case 32: // MD5
		actual, err = CalculateMD5(path)
	default:
		return false, fmt.Errorf("unsupported checksum format (expected 32 or 64 hex characters, got %d)", len(expected))
	}

	if err != nil {
		return false, err
	}

	// Hex digests are case-insensitive; hex.EncodeToString emits
	// lowercase while operators often paste uppercase vendor-published
	// digests. Compare case-insensitively so valid uppercase digests
	// don't spuriously fail.
	return strings.EqualFold(actual, expected), nil
}
