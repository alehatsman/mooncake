//go:build unix

package effects

import (
	"os"
	"syscall"
)

// statOwner extracts the unix UID/GID from a FileInfo. ok is false if the
// underlying stat info is not a *syscall.Stat_t (unusual; defensive).
func statOwner(info os.FileInfo) (uid, gid int, ok bool) {
	stat, isStat := info.Sys().(*syscall.Stat_t)
	if !isStat {
		return 0, 0, false
	}
	return int(stat.Uid), int(stat.Gid), true
}
