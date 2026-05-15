package fleet

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// controllerIDLen is the on-disk length of a UUIDv4 string.
const controllerIDLen = 36 // 8-4-4-4-12 with four hyphens

// EnsureControllerID returns the controller's stable identity, generating
// and persisting one on first use.
//
// On disk: `$XDG_CONFIG_HOME/mooncake/controller_id`. Format: a UUIDv4
// string written with mode 0600 and a trailing newline. The directory is
// created with mode 0700.
//
// Idempotent across calls. Used as the first segment of sync scope keys
// (`<controller_id>/<dir_hash>`) so the same controller against the same
// plan-dir reuses the same scope on every peer.
func EnsureControllerID() (string, error) {
	path, err := DefaultControllerIDPath()
	if err != nil {
		return "", err
	}
	return ensureControllerIDAt(path)
}

// DefaultControllerIDPath is the location used by EnsureControllerID.
func DefaultControllerIDPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mooncake", "controller_id"), nil
}

func ensureControllerIDAt(path string) (string, error) {
	if path == "" {
		return "", errors.New("controller_id path is empty")
	}

	if data, err := os.ReadFile(path); err == nil {
		id := strings.TrimSpace(string(data))
		if isValidUUIDv4(id) {
			return id, nil
		}
		// Present but malformed — fall through and regenerate. A previously-
		// truncated write is the most likely cause.
	} else if !errors.Is(err, fs.ErrNotExist) {
		return "", fmt.Errorf("read controller_id: %w", err)
	}

	id, err := newUUIDv4()
	if err != nil {
		return "", err
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create controller_id dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(id+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return "", fmt.Errorf("rename: %w", err)
	}
	return id, nil
}

// newUUIDv4 generates a RFC 4122 v4 UUID. Inlined to avoid an external
// dependency for a single 16-byte random number.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	// Version 4 (random): set bits 12-15 of time_hi_and_version to 0100.
	b[6] = (b[6] & 0x0f) | 0x40
	// Variant 10x: set bits 6-7 of clock_seq_hi_and_reserved to 10.
	b[8] = (b[8] & 0x3f) | 0x80
	const hex = "0123456789abcdef"
	out := make([]byte, 36)
	j := 0
	for i, v := range b {
		if i == 4 || i == 6 || i == 8 || i == 10 {
			out[j] = '-'
			j++
		}
		out[j] = hex[v>>4]
		out[j+1] = hex[v&0x0f]
		j += 2
	}
	return string(out), nil
}

// isValidUUIDv4 does a strict shape check on a UUIDv4 string: lowercase hex,
// correct length, hyphens in the right places, version nibble = 4, variant
// nibble in {8,9,a,b}.
func isValidUUIDv4(s string) bool {
	if len(s) != controllerIDLen {
		return false
	}
	for i, c := range s {
		switch i {
		case 8, 13, 18, 23:
			if c != '-' {
				return false
			}
		case 14:
			if c != '4' {
				return false
			}
		case 19:
			if c != '8' && c != '9' && c != 'a' && c != 'b' {
				return false
			}
		default:
			if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
				return false
			}
		}
	}
	return true
}
