package tool

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

// fetchToTempFile downloads url into a temp file (created via
// os.CreateTemp in dir) and returns the temp file path. The caller is
// responsible for os.Remove on the returned path. The temp file name
// preserves the URL's archive extension so format detection works.
func fetchToTempFile(url, dir string) (string, error) {
	// #nosec G304 -- dir is a mooncake-managed temp location
	tmp, err := os.CreateTemp(dir, "mooncake-tool-*"+archiveSuffix(url))
	if err != nil {
		return "", fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()
	defer func() { _ = tmp.Close() }()

	// #nosec G107 -- URL comes from user-declared mooncake config
	resp, err := http.Get(url)
	if err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("http GET %s: %w", url, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("http GET %s: status %d", url, resp.StatusCode)
	}

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		_ = os.Remove(tmpName)
		return "", fmt.Errorf("write download: %w", err)
	}
	return tmpName, nil
}

// hashFileSHA256 returns the sha256 hex digest of the file at path,
// prefixed with "sha256:" to match mooncake's checksum conventions
// (see internal/utils/checksum.go).
func hashFileSHA256(path string) (string, error) {
	// #nosec G304 -- path is a mooncake-managed temp file
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open for hash: %w", err)
	}
	defer func() { _ = f.Close() }()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("read for hash: %w", err)
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil)), nil
}

// archiveSuffix returns the archive extension for url, suitable for use
// as a temp-file suffix so detectFormat picks the right extractor.
// Falls back to ".bin" for URLs we don't recognize.
func archiveSuffix(url string) string {
	low := strings.ToLower(url)
	switch {
	case strings.HasSuffix(low, ".tar.gz"):
		return ".tar.gz"
	case strings.HasSuffix(low, ".tgz"):
		return ".tgz"
	case strings.HasSuffix(low, ".tar"):
		return ".tar"
	case strings.HasSuffix(low, ".zip"):
		return ".zip"
	}
	return ".bin"
}

// normalizeChecksum strips an optional "sha256:" prefix and lowercases
// the hex. Accepts both "sha256:abc..." and bare "abc..." forms in user
// input.
func normalizeChecksum(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "sha256:")
	s = strings.TrimPrefix(s, "SHA256:")
	return strings.ToLower(s)
}

// checksumsEqual compares two checksums tolerantly of the "sha256:"
// prefix and case.
func checksumsEqual(a, b string) bool {
	return normalizeChecksum(a) == normalizeChecksum(b)
}
