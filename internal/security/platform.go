package security

import "runtime"

// IsBecomeSupported returns true if the current platform supports become (sudo)
func IsBecomeSupported() bool {
	return runtime.GOOS == "linux" || runtime.GOOS == "darwin"
}
