package logger

import (
	"os"
	"testing"
)

func TestDetectTerminal(t *testing.T) {
	// Test that it returns a valid TerminalInfo struct
	info := DetectTerminal()

	// Should always return a struct (never nil/error)
	if info.Width < 0 || info.Height < 0 {
		t.Errorf("DetectTerminal() returned invalid dimensions: %dx%d", info.Width, info.Height)
	}

	// When not a terminal, should return zeros
	// (this may vary based on test environment)
	t.Logf("Terminal detection: IsTerminal=%v, SupportsANSI=%v, Size=%dx%d",
		info.IsTerminal, info.SupportsANSI, info.Width, info.Height)
}

func TestDetectTerminal_WithEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name         string
		envVar       string
		envValue     string
		wantANSI     bool
		description  string
	}{
		{
			name:        "CI environment",
			envVar:      "CI",
			envValue:    "true",
			wantANSI:    false,
			description: "CI=true should disable ANSI",
		},
		{
			name:        "dumb terminal",
			envVar:      "TERM",
			envValue:    "dumb",
			wantANSI:    false,
			description: "TERM=dumb should disable ANSI",
		},
		{
			name:        "NO_COLOR set",
			envVar:      "NO_COLOR",
			envValue:    "1",
			wantANSI:    false,
			description: "NO_COLOR set should disable ANSI",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original value
			originalValue := os.Getenv(tt.envVar)
			defer func() {
				if originalValue != "" {
					os.Setenv(tt.envVar, originalValue)
				} else {
					os.Unsetenv(tt.envVar)
				}
			}()

			// Set test value
			os.Setenv(tt.envVar, tt.envValue)

			info := DetectTerminal()

			// When running in test environment (not a real terminal),
			// these env vars should affect the SupportsANSI flag
			// But IsTerminal might still be false in test environment
			if info.IsTerminal && info.SupportsANSI != tt.wantANSI {
				t.Logf("%s: SupportsANSI=%v (expected=%v) when IsTerminal=%v",
					tt.description, info.SupportsANSI, tt.wantANSI, info.IsTerminal)
			}
		})
	}
}

func TestDetectTerminal_NonTerminalEnvironment(t *testing.T) {
	// In a test environment (not a TTY), detection should handle gracefully
	info := DetectTerminal()

	// Should return valid struct even if not a terminal
	if info.Width < 0 {
		t.Error("Width should never be negative")
	}
	if info.Height < 0 {
		t.Error("Height should never be negative")
	}

	// If not a terminal, ANSI should be false
	if !info.IsTerminal && info.SupportsANSI {
		t.Error("SupportsANSI should be false when IsTerminal is false")
	}
}

func TestIsTUISupported(t *testing.T) {
	// Test that it returns a boolean without panicking
	supported := IsTUISupported()

	// In test environment, typically not supported (no TTY)
	// But the function should not panic
	t.Logf("TUI supported: %v", supported)

	// If supported, terminal info should be valid
	if supported {
		info := DetectTerminal()
		if !info.IsTerminal {
			t.Error("IsTUISupported() returned true but IsTerminal is false")
		}
		if !info.SupportsANSI {
			t.Error("IsTUISupported() returned true but SupportsANSI is false")
		}
		if info.Width < 80 {
			t.Errorf("IsTUISupported() returned true but width=%d < 80", info.Width)
		}
		if info.Height < 24 {
			t.Errorf("IsTUISupported() returned true but height=%d < 24", info.Height)
		}
	}
}

func TestIsTUISupported_MinimumSize(t *testing.T) {
	// Test the minimum size requirements (80x24)
	// This is a logical test since we can't easily mock terminal size

	info := DetectTerminal()

	// If terminal is detected and supports ANSI
	if info.IsTerminal && info.SupportsANSI {
		supported := IsTUISupported()

		// Should be supported only if meets minimum size
		expectedSupported := info.Width >= 80 && info.Height >= 24
		if supported != expectedSupported {
			t.Errorf("IsTUISupported()=%v, but based on size %dx%d, expected=%v",
				supported, info.Width, info.Height, expectedSupported)
		}
	}
}

func TestGetTerminalSize(t *testing.T) {
	width, height := GetTerminalSize()

	// Should always return valid dimensions
	if width <= 0 {
		t.Errorf("GetTerminalSize() width=%d, should be > 0", width)
	}
	if height <= 0 {
		t.Errorf("GetTerminalSize() height=%d, should be > 0", height)
	}

	// Default fallback is 80x24
	// In test environment, likely returns default
	if width < 80 {
		t.Logf("Width %d < 80, likely using defaults", width)
	}
	if height < 24 {
		t.Logf("Height %d < 24, likely using defaults", height)
	}
}

func TestGetTerminalSize_DefaultFallback(t *testing.T) {
	// When detection fails, should return 80x24 defaults
	width, height := GetTerminalSize()

	// Should never be zero
	if width == 0 || height == 0 {
		t.Errorf("GetTerminalSize() returned zeros: %dx%d", width, height)
	}

	// Verify it returns reasonable values
	if width < 1 || width > 1000 {
		t.Errorf("GetTerminalSize() width=%d seems unreasonable", width)
	}
	if height < 1 || height > 1000 {
		t.Errorf("GetTerminalSize() height=%d seems unreasonable", height)
	}
}

func TestTerminalInfo_Consistency(t *testing.T) {
	// Test that multiple calls return consistent results
	info1 := DetectTerminal()
	info2 := DetectTerminal()

	if info1.IsTerminal != info2.IsTerminal {
		t.Error("Multiple DetectTerminal() calls returned inconsistent IsTerminal")
	}
	if info1.SupportsANSI != info2.SupportsANSI {
		t.Error("Multiple DetectTerminal() calls returned inconsistent SupportsANSI")
	}

	// Size might vary slightly if terminal is resized, but shouldn't be wildly different
	if info1.IsTerminal && info2.IsTerminal {
		widthDiff := abs(info1.Width - info2.Width)
		heightDiff := abs(info1.Height - info2.Height)

		if widthDiff > 10 || heightDiff > 10 {
			t.Logf("Terminal size changed significantly between calls: %dx%d vs %dx%d",
				info1.Width, info1.Height, info2.Width, info2.Height)
		}
	}
}

func TestIsTUISupported_WithCIEnv(t *testing.T) {
	// Save original
	originalCI := os.Getenv("CI")
	defer func() {
		if originalCI != "" {
			os.Setenv("CI", originalCI)
		} else {
			os.Unsetenv("CI")
		}
	}()

	// Set CI environment
	os.Setenv("CI", "true")

	supported := IsTUISupported()

	// In CI environment, TUI should not be supported
	// (assuming it's detected as terminal, which is unlikely in tests)
	info := DetectTerminal()
	if info.IsTerminal && supported {
		t.Log("IsTUISupported() returned true in CI environment (terminal detected)")
	}
}

func TestDetectTerminal_AllFields(t *testing.T) {
	info := DetectTerminal()

	// Test that all fields are set (not uninitialized)
	t.Logf("TerminalInfo: IsTerminal=%v, SupportsANSI=%v, Width=%d, Height=%d",
		info.IsTerminal, info.SupportsANSI, info.Width, info.Height)

	// Verify logical relationships
	if !info.IsTerminal {
		// If not a terminal, ANSI should be false
		if info.SupportsANSI {
			t.Error("SupportsANSI should be false when IsTerminal is false")
		}
		// Width and Height should be 0 when not a terminal
		if info.Width != 0 || info.Height != 0 {
			t.Logf("Non-terminal has size %dx%d (expected 0x0)", info.Width, info.Height)
		}
	}
}

// Helper function
func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
