package plan

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"sort"
)

// HashInputFiles computes a deterministic hash over the contents of the
// given files. Used at both plan time (to record what the plan was
// built from) and apply time (to detect that the source files have
// changed since).
//
// The hash mixes the file path AND content so renames are detected
// (different path → different hash even with identical content).
//
// Returns ErrInputFileMissing if any path is unreadable; callers
// should treat that as a stale-plan condition.
func HashInputFiles(paths []string) (string, error) {
	if len(paths) == 0 {
		return "", nil
	}
	sorted := append([]string(nil), paths...)
	sort.Strings(sorted)

	h := sha256.New()
	for _, p := range sorted {
		data, err := os.ReadFile(p)
		if err != nil {
			if os.IsNotExist(err) {
				return "", fmt.Errorf("%w: %s", ErrInputFileMissing, p)
			}
			return "", fmt.Errorf("read %s: %w", p, err)
		}
		// Path bytes, then a separator, then content, then a record sep.
		h.Write([]byte(p))
		h.Write([]byte{0})
		h.Write(data)
		h.Write([]byte{0, 0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ErrInputFileMissing is returned by HashInputFiles when one of the
// recorded input files no longer exists at apply time. Surfaces as
// part of the stale-plan policy.
var ErrInputFileMissing = errors.New("input file missing")
