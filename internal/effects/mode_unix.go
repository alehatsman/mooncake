//go:build !windows

package effects

import "os"

// modeMatches reports whether the actual file mode satisfies the desired
// mode for idempotency purposes. On POSIX it's strict permission equality.
func modeMatches(actual, desired os.FileMode) bool {
	return actual.Perm() == desired.Perm()
}
