//go:build windows

package metrics

import (
	"fmt"
	"runtime"
	"syscall"
	"time"
	"unsafe"
)

func init() {
	Register(&windowsCPUCollector{})
	Register(&windowsMemCollector{})
}

// ----- CPU -------------------------------------------------------------------
// Uses GetSystemTimes (kernel32.dll): two 100ms-apart samples → delta → pct.
// KernelTime on Windows includes IdleTime, so busy = kernel + user - idle.
// Per-core breakdown requires NtQuerySystemInformation; v1 fills all slots
// with the aggregate value.

type windowsCPUCollector struct{}

func (windowsCPUCollector) Name() string       { return "cpu" }
func (windowsCPUCollector) Outputs() []string  { return []string{"cpu_usage_pct", "cpu_usage_per_core"} }
func (windowsCPUCollector) TTL() time.Duration { return 2 * time.Second }

// winFileTime mirrors the Win32 FILETIME structure (100ns intervals).
type winFileTime struct {
	dwLowDateTime  uint32
	dwHighDateTime uint32
}

func (ft winFileTime) value() uint64 {
	return uint64(ft.dwHighDateTime)<<32 | uint64(ft.dwLowDateTime)
}

type winCPUTimes struct{ idle, kernel, user uint64 }

func (t winCPUTimes) total() uint64 { return t.kernel + t.user }
func (t winCPUTimes) busy() uint64  { return t.total() - t.idle }

var getSystemTimes = func() (winCPUTimes, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GetSystemTimes")
	var idle, kernel, user winFileTime
	r1, _, e := proc.Call(
		uintptr(unsafe.Pointer(&idle)),
		uintptr(unsafe.Pointer(&kernel)),
		uintptr(unsafe.Pointer(&user)),
	)
	if r1 == 0 {
		return winCPUTimes{}, fmt.Errorf("GetSystemTimes: %w", e)
	}
	return winCPUTimes{idle: idle.value(), kernel: kernel.value(), user: user.value()}, nil
}

func (windowsCPUCollector) Collect(m *Metrics) error {
	first, err := getSystemTimes()
	if err != nil {
		return err
	}
	time.Sleep(100 * time.Millisecond)
	second, err := getSystemTimes()
	if err != nil {
		return err
	}

	totalDelta := float64(second.total() - first.total())
	if totalDelta <= 0 {
		return nil
	}
	pct := float64(second.busy()-first.busy()) / totalDelta * 100
	if pct < 0 {
		pct = 0
	} else if pct > 100 {
		pct = 100
	}

	m.CPU.UsagePct = pct
	n := runtime.NumCPU()
	m.CPU.UsagePerCore = make([]float64, n)
	for i := range m.CPU.UsagePerCore {
		m.CPU.UsagePerCore[i] = pct
	}
	return nil
}

// ----- Memory ----------------------------------------------------------------
// Uses GlobalMemoryStatusEx (kernel32.dll), same call as observe_memory.

type windowsMemCollector struct{}

func (windowsMemCollector) Name() string { return "mem" }
func (windowsMemCollector) Outputs() []string {
	return []string{"memory_used_mb", "memory_used_pct", "swap_used_mb"}
}
func (windowsMemCollector) TTL() time.Duration { return 5 * time.Second }

// winMemoryStatusEx mirrors the Win32 MEMORYSTATUSEX structure.
type winMemoryStatusEx struct {
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

func (windowsMemCollector) Collect(m *Metrics) error {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")
	var ms winMemoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))
	r1, _, e := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if r1 == 0 {
		return fmt.Errorf("GlobalMemoryStatusEx: %w", e)
	}

	usedBytes := ms.ullTotalPhys - ms.ullAvailPhys
	m.Memory.UsedMB = int64(usedBytes) / (1024 * 1024)
	if ms.ullTotalPhys > 0 {
		m.Memory.UsedPct = float64(usedBytes) / float64(ms.ullTotalPhys) * 100
	}

	// Page-file swap: ullTotalPageFile includes physical RAM; subtract it to
	// get the actual on-disk swap space.
	swapTotal := int64(ms.ullTotalPageFile) - int64(ms.ullTotalPhys)
	if swapTotal < 0 {
		swapTotal = 0
	}
	// Committed swap = total committed (pageFile used) minus RAM portion.
	committed := int64(ms.ullTotalPageFile) - int64(ms.ullAvailPageFile)
	ramUsed := int64(ms.ullTotalPhys) - int64(ms.ullAvailPhys)
	swapUsed := committed - ramUsed
	if swapUsed < 0 {
		swapUsed = 0
	}
	if swapUsed > swapTotal {
		swapUsed = swapTotal
	}
	m.Memory.SwapUsedMB = swapUsed / (1024 * 1024)
	return nil
}
