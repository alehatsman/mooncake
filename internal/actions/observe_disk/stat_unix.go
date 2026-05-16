//go:build !windows

package observe_disk

import (
	"fmt"
	"syscall"
)

// statPath reads filesystem capacity + inode counts via statfs(2).
// FSType is left empty on this path; obtaining the FS type name
// portably across Linux / macOS / *BSD requires per-OS detail
// (Linux has fsmagic numbers; macOS has Mntonname). Out of scope
// for v1 — callers can derive from the path's mount table if needed.
func statPath(path string) (DiskObservation, error) {
	var st syscall.Statfs_t
	if err := syscall.Statfs(path, &st); err != nil {
		return DiskObservation{Path: path}, fmt.Errorf("statfs %s: %w", path, err)
	}
	bsize := st.Bsize
	total := int64(st.Blocks) * bsize
	free := int64(st.Bfree) * bsize
	avail := int64(st.Bavail) * bsize
	return DiskObservation{
		Path:        path,
		TotalBytes:  total,
		FreeBytes:   avail, // "free to non-root" is what users want
		UsedBytes:   total - free,
		InodesTotal: int64(st.Files),
		InodesUsed:  int64(st.Files) - int64(st.Ffree),
	}, nil
}
