package facts

import (
	"runtime"
	"testing"
)

func TestCollect(t *testing.T) {
	facts := Collect()

	// Test basic facts
	if facts.OS != runtime.GOOS {
		t.Errorf("OS = %v, want %v", facts.OS, runtime.GOOS)
	}
	if facts.Arch != runtime.GOARCH {
		t.Errorf("Arch = %v, want %v", facts.Arch, runtime.GOARCH)
	}

	// Hostname should not be empty (usually)
	if facts.Hostname == "" {
		t.Log("Hostname is empty (may be expected in some environments)")
	}

	// UserHome should not be empty
	if facts.UserHome == "" {
		t.Error("UserHome should not be empty")
	}

	// CPU cores should be positive
	if facts.CPUCores <= 0 {
		t.Errorf("CPUCores = %d, should be positive", facts.CPUCores)
	}

	// IP addresses might be empty in test environment, so just check it's not nil
	if facts.IPAddresses == nil {
		t.Error("IPAddresses should not be nil")
	}
}

func TestToMap(t *testing.T) {
	facts := &Facts{
		OS:                  "linux",
		Arch:                "amd64",
		Hostname:            "testhost",
		UserHome:            "/home/user",
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		DistributionMajor:   "22",
		IPAddresses:         []string{"192.168.1.1", "10.0.0.1"},
		CPUCores:            8,
		MemoryTotalMB:       16384,
		PythonVersion:       "3.11.5",
		PackageManager:      "apt",
	}

	m := facts.ToMap()

	// Check all fields are in the map
	if m["os"] != "linux" {
		t.Errorf("os = %v, want linux", m["os"])
	}
	if m["arch"] != "amd64" {
		t.Errorf("arch = %v, want amd64", m["arch"])
	}
	if m["hostname"] != "testhost" {
		t.Errorf("hostname = %v, want testhost", m["hostname"])
	}
	if m["user_home"] != "/home/user" {
		t.Errorf("user_home = %v, want /home/user", m["user_home"])
	}
	if m["distribution"] != "Ubuntu" {
		t.Errorf("distribution = %v, want Ubuntu", m["distribution"])
	}
	if m["cpu_cores"] != 8 {
		t.Errorf("cpu_cores = %v, want 8", m["cpu_cores"])
	}
	if m["python_version"] != "3.11.5" {
		t.Errorf("python_version = %v, want 3.11.5", m["python_version"])
	}

	// Check IP addresses
	ips := m["ip_addresses"].([]string)
	if len(ips) != 2 {
		t.Errorf("ip_addresses length = %d, want 2", len(ips))
	}
	if m["ip_addresses_string"] != "192.168.1.1, 10.0.0.1" {
		t.Errorf("ip_addresses_string = %v, want '192.168.1.1, 10.0.0.1'", m["ip_addresses_string"])
	}
}

func TestCollectIPAddresses(t *testing.T) {
	ips := collectIPAddresses()

	// Should return a slice (might be empty in test environment)
	if ips == nil {
		t.Error("collectIPAddresses should not return nil")
	}

	// If there are IPs, they should not be empty strings
	for _, ip := range ips {
		if ip == "" {
			t.Error("IP address should not be empty string")
		}
	}
}

func TestDetectPythonVersion(t *testing.T) {
	version := detectPythonVersion()

	// Version might be empty if Python is not installed
	// Just check it doesn't panic
	t.Logf("Python version: %s", version)
}

func TestPlatformSpecificFacts(t *testing.T) {
	facts := Collect()

	switch runtime.GOOS {
	case "linux":
		// Linux should set distribution info
		t.Logf("Distribution: %s", facts.Distribution)
		t.Logf("Package Manager: %s", facts.PackageManager)
	case "darwin":
		// Darwin/macOS facts
		t.Logf("Memory: %d MB", facts.MemoryTotalMB)
	case "windows":
		// Windows facts (currently stub)
		t.Log("Windows platform detected")
	default:
		t.Logf("Unknown platform: %s", runtime.GOOS)
	}
}
