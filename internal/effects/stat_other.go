//go:build !unix

package effects

import "os"

// statOwner is a no-op on non-unix platforms; Chown isn't a meaningful
// operation on Windows in this codebase.
func statOwner(_ os.FileInfo) (uid, gid int, ok bool) {
	return 0, 0, false
}
