// Package utils provides utility functions for file operations.
package utils

import (
	"crypto/md5" // #nosec G501 -- MD5 used for integrity checks, not security
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
)

// CalculateSHA256 calculates the SHA256 checksum of a file.
func CalculateSHA256(path string) (checksum string, err error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			// Return close error only if operation succeeded
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	hasher := sha256.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

// CalculateMD5 calculates the MD5 checksum of a file.
// Note: MD5 is deprecated for security purposes but still commonly used for integrity checks.
func CalculateMD5(path string) (checksum string, err error) {
	// #nosec G304 -- File path from user config is intentional functionality
	file, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer func() {
		if closeErr := file.Close(); closeErr != nil && err == nil {
			// Return close error only if operation succeeded
			err = fmt.Errorf("failed to close file: %w", closeErr)
		}
	}()

	// #nosec G401 -- MD5 used for integrity checks, not security
	hasher := md5.New()
	if _, err = io.Copy(hasher, file); err != nil {
		return "", fmt.Errorf("failed to read file: %w", err)
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
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

	return actual == expected, nil
}
