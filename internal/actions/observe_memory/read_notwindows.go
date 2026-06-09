//go:build !windows

package observe_memory

import (
	"fmt"
	"runtime"
)

// readMemoryWindows is a compile-time stub; the real implementation lives in
// read_windows.go and is only compiled on Windows. On other platforms the
// switch in readMemory() never reaches this case, but the symbol must exist.
func readMemoryWindows() (MemoryObservation, error) {
	return MemoryObservation{}, fmt.Errorf("observe.memory windows backend called on %s (bug)", runtime.GOOS)
}
