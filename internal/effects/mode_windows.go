//go:build windows

package effects

import "os"

// modeMatches reports whether the actual file mode satisfies the desired
// mode for idempotency purposes.
//
// Windows has no POSIX permission model — NTFS uses ACLs, and Go's os.Chmod
// can only toggle the read-only bit (mode & 0200). Comparing 0644-style
// modes here would force chmod on every run for files that the OS reports
// as 0666, churning idempotency for no real effect. So we treat mode as a
// best-effort hint and always report "matches" on Windows.
//
// If you need write/read enforcement on Windows, do it explicitly with
// ACL-aware tooling — the file action's mode field is documented as a
// POSIX-only hint that becomes a no-op here.
func modeMatches(actual, desired os.FileMode) bool {
	_ = actual
	_ = desired
	return true
}
