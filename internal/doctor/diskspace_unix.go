//go:build linux || darwin

package doctor

import "syscall"

// diskFree returns bytes available on the filesystem holding dir. Used by
// checkDiskSpace only; surface-level enough to inline rather than pull in
// golang.org/x/sys/unix.
func diskFree(dir string) (uint64, error) {
	var s syscall.Statfs_t
	if err := syscall.Statfs(dir, &s); err != nil {
		return 0, err
	}
	return s.Bavail * uint64(s.Bsize), nil
}
