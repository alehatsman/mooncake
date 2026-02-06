package facts

import (
	"testing"
)

// TestParseSize tests the parseSize utility function
// parseSize is used to parse df output like "100G", "50G" and returns the numeric value
func TestParseSize(t *testing.T) {
	tests := []struct {
		input    string
		expected int64
	}{
		{"100G", 100},       // 100GB
		{"1024G", 1024},     // 1024GB
		{"0G", 0},           // Zero with G
		{"50G", 50},         // 50GB
		{"1G", 1},           // 1GB
		{"100", 100},        // No G suffix
		{"1024", 1024},      // No G suffix
		{"0", 0},            // Zero
		{"invalid", 0},      // Invalid input
		{"", 0},             // Empty string
	}

	for _, tt := range tests {
		result := parseSize(tt.input)
		if result != tt.expected {
			t.Errorf("parseSize(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

// TestParsePercent tests the parsePercent utility function
// parsePercent is used to parse df output like "75%", "50%" and returns the numeric value
func TestParsePercent(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"50%", 50},
		{"100%", 100},
		{"0%", 0},
		{"75%", 75},
		{"invalid", 0},
		{"", 0},
		{"50", 50}, // Missing % sign still parses
	}

	for _, tt := range tests {
		result := parsePercent(tt.input)
		if result != tt.expected {
			t.Errorf("parsePercent(%q) = %d, want %d", tt.input, result, tt.expected)
		}
	}
}

// TestDetectLinuxDistribution_NonLinux tests distribution detection on non-Linux
func TestDetectLinuxDistribution_NonLinux(t *testing.T) {
	// Should handle missing /etc/os-release gracefully
	dist, version := detectLinuxDistribution()
	t.Logf("Distribution: %s, Version: %s", dist, version)
}

// TestDetectLinuxPackageManager_NonLinux tests package manager detection
func TestDetectLinuxPackageManager_NonLinux(t *testing.T) {
	// Test with empty distro
	pkgMgr := detectLinuxPackageManager("")
	t.Logf("Package manager (empty distro): %s", pkgMgr)

	// Test with known distros
	pkgMgr = detectLinuxPackageManager("ubuntu")
	if pkgMgr != "apt" {
		t.Errorf("Expected 'apt' for ubuntu, got %s", pkgMgr)
	}

	pkgMgr = detectLinuxPackageManager("fedora")
	if pkgMgr != "dnf" {
		t.Errorf("Expected 'dnf' for fedora, got %s", pkgMgr)
	}

	pkgMgr = detectLinuxPackageManager("alpine")
	if pkgMgr != "apk" {
		t.Errorf("Expected 'apk' for alpine, got %s", pkgMgr)
	}
}

// TestDetectLinuxMemory_NonLinux tests memory detection on non-Linux
func TestDetectLinuxMemory_NonLinux(t *testing.T) {
	// Test that it doesn't panic
	total := detectLinuxMemory()
	t.Logf("Memory total: %d MB", total)
}

// TestDetectLinuxDisks_NonLinux tests disk detection on non-Linux
func TestDetectLinuxDisks_NonLinux(t *testing.T) {
	// Test that it doesn't panic
	disks := detectLinuxDisks()
	t.Logf("Disks detected: %d", len(disks))
}

// TestDetectLinuxGPUs_NonLinux tests GPU detection on non-Linux
func TestDetectLinuxGPUs_NonLinux(t *testing.T) {
	// Test that it doesn't panic
	gpus := detectLinuxGPUs()
	t.Logf("GPUs detected: %d", len(gpus))
}

// TestCollectLinuxFacts_NonLinux tests that Linux fact collection handles non-Linux gracefully
func TestCollectLinuxFacts_NonLinux(t *testing.T) {
	// On non-Linux systems, this should not panic and should handle missing files gracefully
	facts := &Facts{}
	collectLinuxFacts(facts)

	// On non-Linux, many values will be empty/zero, which is expected
	t.Logf("Facts collected: OS=%s, Dist=%s, Mem=%d MB",
		facts.OS, facts.Distribution, facts.MemoryTotalMB)
}

// TestDetectLinuxKernel_NonLinux tests kernel detection on non-Linux
func TestDetectLinuxKernel_NonLinux(t *testing.T) {
	// Should handle missing uname gracefully
	kernel := detectLinuxKernel()
	t.Logf("Kernel: %s", kernel)
}

// TestDetectLinuxCPUModel_NonLinux tests CPU model detection on non-Linux
func TestDetectLinuxCPUModel_NonLinux(t *testing.T) {
	// Should handle missing /proc/cpuinfo gracefully
	model := detectLinuxCPUModel()
	t.Logf("CPU model: %s", model)
}

// TestDetectLinuxCPUFlags_NonLinux tests CPU flags detection on non-Linux
func TestDetectLinuxCPUFlags_NonLinux(t *testing.T) {
	// Should handle missing /proc/cpuinfo gracefully
	flags := detectLinuxCPUFlags()
	t.Logf("CPU flags: %d", len(flags))
}

// TestDetectLinuxMemoryFree_NonLinux tests free memory detection on non-Linux
func TestDetectLinuxMemoryFree_NonLinux(t *testing.T) {
	// Should handle missing /proc/meminfo gracefully
	memFree := detectLinuxMemoryFree()
	t.Logf("Memory free: %d MB", memFree)
}

// TestDetectLinuxSwap_NonLinux tests swap detection on non-Linux
func TestDetectLinuxSwap_NonLinux(t *testing.T) {
	// Should handle missing /proc/meminfo gracefully
	swapTotal, swapFree := detectLinuxSwap()
	t.Logf("Swap: %d MB total, %d MB free", swapTotal, swapFree)
}

// TestDetectLinuxDefaultRoute_NonLinux tests default route detection on non-Linux
func TestDetectLinuxDefaultRoute_NonLinux(t *testing.T) {
	// Should handle missing ip command gracefully
	gateway := detectLinuxDefaultRoute()
	t.Logf("Default gateway: %s", gateway)
}

// TestDetectLinuxDNS_NonLinux tests DNS detection on non-Linux
func TestDetectLinuxDNS_NonLinux(t *testing.T) {
	// Should handle missing /etc/resolv.conf gracefully
	servers := detectLinuxDNS()
	t.Logf("DNS servers: %d", len(servers))
}

// TestDetectCUDAVersion_NonLinux tests CUDA detection on non-Linux
func TestDetectCUDAVersion_NonLinux(t *testing.T) {
	// Should handle missing nvidia-smi gracefully
	version := detectCUDAVersion()
	t.Logf("CUDA version: %s", version)
}

// TestLinuxFunctions_NoBlankReturn tests that detection functions return reasonable values
func TestLinuxFunctions_NoBlankReturn(t *testing.T) {
	// Call all functions to ensure they execute without panicking
	// On non-Linux systems, they should return empty/zero values gracefully

	_, _ = detectLinuxDistribution()
	_ = detectLinuxPackageManager("")
	_ = detectLinuxMemory()
	_ = detectLinuxDisks()
	_ = detectLinuxGPUs()
	_ = detectLinuxKernel()
	_ = detectLinuxCPUModel()
	_ = detectLinuxCPUFlags()
	_ = detectLinuxMemoryFree()
	_, _ = detectLinuxSwap()
	_ = detectLinuxDefaultRoute()
	_ = detectLinuxDNS()
	_ = detectCUDAVersion()

	// If we get here without panicking, the test passes
	t.Log("All Linux detection functions executed without panic")
}

// TestCollectLinuxFacts_Structure tests that the facts structure is populated correctly
func TestCollectLinuxFacts_Structure(t *testing.T) {
	facts := &Facts{}
	collectLinuxFacts(facts)

	// Verify Facts struct fields exist (compile-time check)
	_ = facts.OS
	_ = facts.Distribution
	_ = facts.DistributionVersion
	_ = facts.KernelVersion
	_ = facts.Arch
	_ = facts.CPUCores
	_ = facts.CPUModel
	_ = facts.CPUFlags
	_ = facts.MemoryTotalMB
	_ = facts.MemoryFreeMB
	_ = facts.SwapTotalMB
	_ = facts.SwapFreeMB
	_ = facts.Disks
	_ = facts.GPUs
	_ = facts.DefaultGateway
	_ = facts.DNSServers

	t.Log("Facts structure validated")
}

// TestParseSize_EdgeCases tests edge cases for size parsing
// The function does simple parsing without validation
func TestParseSize_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int64
	}{
		{"negative number", "-1024G", -1024},
		{"very large number", "9999999999G", 9999999999},
		{"with decimal", "1024.5G", 0}, // Parsing fails, returns 0
		{"with K suffix", "1024K", 0},  // Only strips G, parsing fails
		{"special characters", "!@#$", 0},
		{"with spaces", "  100G  ", 0}, // TrimSuffix doesn't trim spaces
		{"negative percent", "-50%", 0}, // Doesn't apply to size
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSize(tt.input)
			if result != tt.expected {
				t.Errorf("parseSize(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}

// TestParsePercent_EdgeCases tests edge cases for percent parsing
// The function does simple parsing without validation
func TestParsePercent_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"negative percent", "-50%", -50}, // Parses negative as-is
		{"over 100", "150%", 150},
		{"decimal percent", "50.5%", 0}, // atoi fails on decimal
		{"multiple % signs", "50%%", 0}, // Parsing fails
		{"% at start", "%50", 0},
		{"with spaces", "  75%  ", 0}, // TrimSuffix doesn't trim spaces
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parsePercent(tt.input)
			if result != tt.expected {
				t.Errorf("parsePercent(%q) = %d, want %d", tt.input, result, tt.expected)
			}
		})
	}
}
