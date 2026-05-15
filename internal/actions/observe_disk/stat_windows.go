//go:build windows

package observe_disk

import (
	"fmt"
	"syscall"
	"unsafe"
)

// statPath uses GetDiskFreeSpaceExW. Inode counts and read-only flag
// are filesystem-agnostic-unavailable on Windows from this API; left
// as zero values.
func statPath(path string) (DiskObservation, error) {
	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return DiskObservation{Path: path}, err
	}
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetDiskFreeSpaceExW")

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	r1, _, e := proc.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if r1 == 0 {
		return DiskObservation{Path: path}, fmt.Errorf("GetDiskFreeSpaceExW: %w", e)
	}
	return DiskObservation{
		Path:       path,
		TotalBytes: int64(totalBytes),
		FreeBytes:  int64(freeBytesAvailable),
		UsedBytes:  int64(totalBytes - totalFreeBytes),
	}, nil
}
