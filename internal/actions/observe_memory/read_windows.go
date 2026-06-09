//go:build windows

package observe_memory

import (
	"fmt"
	"syscall"
	"unsafe"
)

// memoryStatusEx mirrors the Win32 MEMORYSTATUSEX struct.
// dwLength must be set to sizeof(MEMORYSTATUSEX) before the call.
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

func readMemoryWindows() (MemoryObservation, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")

	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	r1, _, e := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return MemoryObservation{}, fmt.Errorf("GlobalMemoryStatusEx: %w", e)
	}

	physUsed := int64(ms.ullTotalPhys) - int64(ms.ullAvailPhys)

	// Page-file capacity = commit limit minus physical RAM.
	// Negative means no page file configured; clamp to zero.
	swapTotal := int64(ms.ullTotalPageFile) - int64(ms.ullTotalPhys)
	if swapTotal < 0 {
		swapTotal = 0
	}
	// Swap used = total committed memory minus the RAM portion.
	totalCommitted := int64(ms.ullTotalPageFile) - int64(ms.ullAvailPageFile)
	swapUsed := totalCommitted - physUsed
	if swapUsed < 0 {
		swapUsed = 0
	}
	if swapUsed > swapTotal {
		swapUsed = swapTotal
	}

	return MemoryObservation{
		TotalBytes:     int64(ms.ullTotalPhys),
		UsedBytes:      physUsed,
		FreeBytes:      int64(ms.ullAvailPhys),
		AvailableBytes: int64(ms.ullAvailPhys),
		SwapTotalBytes: swapTotal,
		SwapUsedBytes:  swapUsed,
	}, nil
}
