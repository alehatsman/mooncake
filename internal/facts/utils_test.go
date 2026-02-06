package facts

import (
	"testing"
)

// TestExtractMajorVersion tests version string parsing
func TestExtractMajorVersion(t *testing.T) {
	tests := []struct {
		name     string
		version  string
		expected string
	}{
		{"major.minor.patch", "20.04.1", "20"},
		{"major.minor", "11.2", "11"},
		{"major only", "8", "8"},
		{"empty string", "", ""},
		{"with prefix", "v3.2.1", "v3"},
		{"single digit", "7", "7"},
		{"leading zeros", "08.10", "08"},
		{"complex version", "2023.11.1-beta", "2023"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractMajorVersion(tt.version)
			if result != tt.expected {
				t.Errorf("extractMajorVersion(%q) = %q, want %q", tt.version, result, tt.expected)
			}
		})
	}
}

// TestCollectIPAddresses_Details tests IP address collection details
func TestCollectIPAddresses_Details(t *testing.T) {
	ips := collectIPAddresses()

	// Should filter out loopback
	for _, ip := range ips {
		if ip == "127.0.0.1" || ip == "::1" {
			t.Errorf("collectIPAddresses() should not include loopback addresses, got %s", ip)
		}
	}

	// Each IP should be non-empty
	for _, ip := range ips {
		if ip == "" {
			t.Error("collectIPAddresses() should not include empty strings")
		}
	}

	t.Logf("Collected %d IP addresses", len(ips))
}

// TestDetectPythonVersion_Variations tests different Python scenarios
func TestDetectPythonVersion_Variations(t *testing.T) {
	version := detectPythonVersion()

	// If Python is installed, version should be non-empty
	if version != "" {
		t.Logf("Python version detected: %s", version)

		// Version should contain digits
		hasDigit := false
		for _, ch := range version {
			if ch >= '0' && ch <= '9' {
				hasDigit = true
				break
			}
		}
		if !hasDigit {
			t.Errorf("Python version %q should contain digits", version)
		}
	} else {
		t.Log("Python not detected (expected on systems without Python)")
	}
}
