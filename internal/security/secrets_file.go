package security

// secrets_file.go ships the `file:` secret provider. Reads a single
// file's contents from disk, trims a trailing newline (the Kubernetes
// Secret-mount convention writes secrets with one), and returns the
// remainder. Distinct from `env:` for the common case where a secret
// is too large or sensitive to keep in environment (e.g. a TLS key, a
// service-account JSON blob).
//
// Example:
//
//	content: !secret file:/etc/mooncake/db-password
//	content: !secret file:~/.config/mooncake/secrets/app.token

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// FileProvider resolves "file:<path>" refs. Paths are passed through
// after a `~` expansion to the user's home, but otherwise are used
// verbatim — no globbing, no env-var expansion. Operators should pass
// absolute paths in plans for portability.
type FileProvider struct{}

// Resolve reads `path` and returns the contents with one trailing
// newline stripped. Errors with a redacted message — the path's
// contents are never quoted in the error.
func (FileProvider) Resolve(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("file provider: empty path")
	}
	// Expand a leading ~ to the user's home dir. We deliberately do not
	// run a full template / env expansion: secret refs should be stable
	// across environments, not silently rewritten.
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("file provider: cannot resolve ~ (no HOME)")
		}
		path = filepath.Join(home, path[2:])
	}

	st, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", errors.New("file provider: file not found")
		}
		return "", errors.New("file provider: stat failed")
	}
	if st.IsDir() {
		return "", errors.New("file provider: path is a directory")
	}

	body, err := os.ReadFile(path) // #nosec G304 -- user-specified secret ref, intentional
	if err != nil {
		return "", errors.New("file provider: read failed")
	}
	// Trim one trailing newline (k8s Secret-mount convention) but
	// preserve any other whitespace — a secret legitimately starting or
	// ending with spaces (rare but possible) shouldn't be silently
	// stripped.
	s := string(body)
	if strings.HasSuffix(s, "\r\n") {
		s = s[:len(s)-2]
	} else if strings.HasSuffix(s, "\n") {
		s = s[:len(s)-1]
	}
	if s == "" {
		return "", errors.New("file provider: file is empty")
	}
	return s, nil
}

func init() {
	DefaultRegistry.Register("file", FileProvider{})
}
