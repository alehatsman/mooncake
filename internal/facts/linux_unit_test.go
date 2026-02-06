package facts

import (
	"runtime"
	"strings"
	"testing"
)

// TestDetectUnitLinuxMemory_FromMeminfo tests parsing of /proc/meminfo
func TestDetectUnitLinuxMemory_FromMeminfo(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	// Test with real system
	mem := detectLinuxMemory()
	if mem <= 0 {
		t.Error("Expected positive memory value")
	}
	t.Logf("Detected memory: %d MB", mem)
}

// TestDetectUnitLinuxDisks_RealSystem tests disk detection on real system
func TestDetectUnitLinuxDisks_RealSystem(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	disks := detectLinuxDisks()
	// Just verify it doesn't crash and returns some data
	t.Logf("Detected %d disks", len(disks))

	for _, disk := range disks {
		if disk.Device == "" {
			t.Error("Disk should have a device name")
		}
		if disk.MountPoint == "" {
			t.Error("Disk should have a mount point")
		}
		t.Logf("Disk: %s at %s, %d GB", disk.Device, disk.MountPoint, disk.SizeGB)
	}
}

// TestDetectUnitLinuxGPUs_AllVendors tests GPU detection attempts
func TestDetectUnitLinuxGPUs_AllVendors(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	gpus := detectLinuxGPUs()
	t.Logf("Detected %d GPUs", len(gpus))

	// Just verify the function runs without crashing
	for i, gpu := range gpus {
		t.Logf("GPU %d: %s (%s)", i, gpu.Model, gpu.Vendor)
	}
}

// TestDetectUnitLinuxKernel_Uname tests kernel detection
func TestDetectUnitLinuxKernel_Uname(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	kernel := detectLinuxKernel()
	if kernel == "" {
		t.Error("Kernel version should not be empty on Linux")
	}
	t.Logf("Kernel: %s", kernel)

	// Should contain version numbers
	if !strings.Contains(kernel, ".") {
		t.Error("Kernel version should contain dots")
	}
}

// TestDetectUnitLinuxCPUModel_Proc tests CPU model detection
func TestDetectUnitLinuxCPUModel_Proc(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	model := detectLinuxCPUModel()
	// Might be empty in some environments
	t.Logf("CPU model: %s", model)

	if model != "" {
		// Should be a reasonable string
		if len(model) < 3 {
			t.Errorf("CPU model too short: %s", model)
		}
	}
}

// TestDetectUnitLinuxCPUFlags_Proc tests CPU flags detection
func TestDetectUnitLinuxCPUFlags_Proc(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	flags := detectLinuxCPUFlags()
	t.Logf("CPU flags count: %d", len(flags))

	// Check for common flags if any found
	if len(flags) > 0 {
		hasCommon := false
		for _, flag := range flags {
			if flag == "sse" || flag == "sse2" || flag == "fpu" {
				hasCommon = true
				break
			}
		}
		if !hasCommon {
			t.Log("No common CPU flags found (might be in container)")
		}
	}
}

// TestDetectUnitLinuxMemoryFree_Proc tests free memory detection
func TestDetectUnitLinuxMemoryFree_Proc(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	memFree := detectLinuxMemoryFree()
	if memFree <= 0 {
		t.Error("Free memory should be positive")
	}
	t.Logf("Free memory: %d MB", memFree)

	// Sanity check - should be less than total memory
	memTotal := detectLinuxMemory()
	if memFree > memTotal {
		t.Errorf("Free memory (%d) should not exceed total memory (%d)", memFree, memTotal)
	}
}

// TestDetectUnitLinuxSwap_Proc tests swap detection
func TestDetectUnitLinuxSwap_Proc(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	swapTotal, swapFree := detectLinuxSwap()
	t.Logf("Swap: %d MB total, %d MB free", swapTotal, swapFree)

	// Swap values can be 0 (no swap configured) but shouldn't be negative
	if swapTotal < 0 {
		t.Error("Swap total should not be negative")
	}
	if swapFree < 0 {
		t.Error("Swap free should not be negative")
	}
	if swapFree > swapTotal {
		t.Errorf("Swap free (%d) should not exceed swap total (%d)", swapFree, swapTotal)
	}
}

// TestDetectUnitLinuxDefaultRoute_Net tests default gateway detection
func TestDetectUnitLinuxDefaultRoute_Net(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	gateway := detectLinuxDefaultRoute()
	t.Logf("Default gateway: %s", gateway)

	// Gateway might be empty in some environments (containers)
	if gateway != "" {
		// Should look like an IP address
		if !strings.Contains(gateway, ".") && !strings.Contains(gateway, ":") {
			t.Logf("Gateway doesn't look like IP address: %s", gateway)
		}
	}
}

// TestDetectUnitLinuxDNS_ResolvConf tests DNS server detection
func TestDetectUnitLinuxDNS_ResolvConf(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	servers := detectLinuxDNS()
	t.Logf("DNS servers: %v", servers)

	// Verify each looks like an IP
	for _, server := range servers {
		if !strings.Contains(server, ".") && !strings.Contains(server, ":") {
			t.Logf("DNS server doesn't look like IP: %s", server)
		}
	}
}

// TestDetectUnitCUDAVersion_NvidiaSMI tests CUDA detection
func TestDetectUnitCUDAVersion_NvidiaSMI(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	version := detectCUDAVersion()
	t.Logf("CUDA version: %s", version)

	// Most systems won't have CUDA
	if version != "" {
		// Should look like a version number
		if !strings.Contains(version, ".") {
			t.Logf("CUDA version doesn't look like version number: %s", version)
		}
	}
}

// TestDetectUnitLinuxDistribution_OsRelease tests distribution detection
func TestDetectUnitLinuxDistribution_OsRelease(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	name, version := detectLinuxDistribution()
	t.Logf("Distribution: %s %s", name, version)

	// At least name should be detected
	if name == "" {
		t.Log("Distribution name not detected (unusual)")
	}
}

// TestDetectUnitLinuxPackageManager_Distros tests package manager detection
func TestDetectUnitLinuxPackageManager_Distros(t *testing.T) {
	tests := []struct {
		distro          string
		expected        []string // possible values
		mustBeOneOf     bool
	}{
		{"ubuntu", []string{"apt"}, false},
		{"debian", []string{"apt"}, false},
		{"linuxmint", []string{"apt"}, false},
		{"fedora", []string{"dnf"}, false},
		{"centos", []string{"dnf", "yum"}, true}, // depends on dnf availability
		{"arch", []string{"pacman"}, false},
		{"alpine", []string{"apk"}, false},
		{"opensuse", []string{"zypper"}, false},
		// For unknown distros, the function tries to auto-detect by checking command availability
		// So it might return any available package manager or empty string
		{"unknown", []string{"", "apt", "dnf", "yum", "pacman", "zypper", "apk"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.distro, func(t *testing.T) {
			result := detectLinuxPackageManager(tt.distro)
			if tt.mustBeOneOf {
				// Check if result is one of the expected values
				found := false
				for _, exp := range tt.expected {
					if result == exp {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("Expected one of %v for %s, got %s", tt.expected, tt.distro, result)
				}
			} else {
				if result != tt.expected[0] {
					t.Errorf("Expected %s for %s, got %s", tt.expected[0], tt.distro, result)
				}
			}
		})
	}
}

// TestExtractUnitMajorVersion_Variants tests version extraction
func TestExtractUnitMajorVersion_Variants(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"22.04", "22"},
		{"22", "22"},
		{"5.15.0-91-generic", "5"},
		{"", ""},
		{"noversion", "noversion"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := extractMajorVersion(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

// TestParseUnitSize_EdgeCases tests size parsing
func TestParseUnitSize_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"1024G", 1024},
		{"1G", 1},
		{"0G", 0},
		{"", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestParseUnitPercent_EdgeCases tests percent parsing
func TestParseUnitPercent_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"99%", 99},
		{"0%", 0},
		{"100%", 100},
		{"", 0},
		{"invalid", 0},
		{"50", 50}, // no % suffix
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parsePercent(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %d, got %d", tt.expected, result)
			}
		})
	}
}

// TestCollectUnitLinuxFacts_Integration tests full collection
func TestCollectUnitLinuxFacts_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux-only test")
	}

	facts := &Facts{}
	collectLinuxFacts(facts)

	// Verify basic fields
	if facts.MemoryTotalMB <= 0 {
		t.Error("Memory total should be positive")
	}

	// Log all collected facts
	t.Logf("Distribution: %s %s", facts.Distribution, facts.DistributionVersion)
	t.Logf("Package Manager: %s", facts.PackageManager)
	t.Logf("Memory: %d MB", facts.MemoryTotalMB)
	t.Logf("Kernel: %s", facts.KernelVersion)
	t.Logf("CPU Model: %s", facts.CPUModel)
	t.Logf("CPU Flags: %d", len(facts.CPUFlags))
	t.Logf("Memory Free: %d MB", facts.MemoryFreeMB)
	t.Logf("Swap: %d / %d MB", facts.SwapFreeMB, facts.SwapTotalMB)
	t.Logf("Disks: %d", len(facts.Disks))
	t.Logf("GPUs: %d", len(facts.GPUs))
	t.Logf("Gateway: %s", facts.DefaultGateway)
	t.Logf("DNS: %v", facts.DNSServers)
}
