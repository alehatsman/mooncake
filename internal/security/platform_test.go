package security

import (
	"runtime"
	"testing"
)

func TestIsBecomeSupported(t *testing.T) {
	supported := IsBecomeSupported()

	// Should be true on Linux and macOS
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		if !supported {
			t.Errorf("Expected become to be supported on %s", runtime.GOOS)
		}
	} else {
		// Should be false on other platforms
		if supported {
			t.Errorf("Expected become to not be supported on %s", runtime.GOOS)
		}
	}
}
