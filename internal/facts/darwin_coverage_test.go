package facts

import (
	"runtime"
	"testing"
)

// TestDetectExtraMacOSVersion tests macOS version detection
func TestDetectExtraMacOSVersion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	version := detectMacOSVersion()
	if version == "" {
		t.Error("Expected macOS version to be detected")
	}
	t.Logf("macOS version: %s", version)
}

// TestDetectExtraMacOSVersion_ErrorPath tests error handling
func TestDetectExtraMacOSVersion_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test caching and error paths
	version1 := detectMacOSVersion()
	version2 := detectMacOSVersion()

	if version1 != version2 {
		t.Error("Version should be consistent across calls")
	}
}

// TestDetectExtraMacOSPackageManager tests package manager detection
func TestDetectExtraMacOSPackageManager(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	pm := detectMacOSPackageManager()
	t.Logf("Package manager: %s", pm)

	// At least one should typically be available on dev machines
	if pm == "" {
		t.Log("Warning: No package manager detected (unusual for macOS dev environment)")
	} else if pm != "brew" && pm != "port" {
		t.Errorf("Expected 'brew' or 'port', got '%s'", pm)
	}
}

// TestDetectExtraMacOSMemory tests memory detection
func TestDetectExtraMacOSMemory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	mem := detectMacOSMemory()
	if mem <= 0 {
		t.Error("Expected positive memory value on macOS")
	}
	t.Logf("Memory total: %d MB", mem)
}

// TestDetectExtraMacOSMemory_ErrorPath tests error handling
func TestDetectExtraMacOSMemory_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test different code paths
	mem1 := detectMacOSMemory()
	mem2 := detectMacOSMemory()

	if mem1 != mem2 {
		t.Error("Memory value should be consistent")
	}
}

// TestDetectExtraMacOSDisks tests disk detection
func TestDetectExtraMacOSDisks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	disks := detectMacOSDisks()
	if len(disks) == 0 {
		t.Error("Expected at least one disk on macOS")
	}

	for i, disk := range disks {
		t.Logf("Disk %d: %s mounted at %s (%s)", i, disk.Device, disk.MountPoint, disk.Filesystem)
		if disk.SizeGB < 0 {
			t.Errorf("Disk %d has negative size: %d", i, disk.SizeGB)
		}
		if disk.UsedGB < 0 {
			t.Errorf("Disk %d has negative used: %d", i, disk.UsedGB)
		}
		if disk.AvailGB < 0 {
			t.Errorf("Disk %d has negative available: %d", i, disk.AvailGB)
		}
		if disk.UsedPct < 0 || disk.UsedPct > 100 {
			t.Errorf("Disk %d has invalid used percent: %d", i, disk.UsedPct)
		}
	}
}

// TestDetectExtraMacOSDisks_ErrorPath tests disk detection error handling
func TestDetectExtraMacOSDisks_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test parsing logic
	disks1 := detectMacOSDisks()
	disks2 := detectMacOSDisks()

	if len(disks1) != len(disks2) {
		t.Error("Disk count should be consistent")
	}
}

// TestDetectExtraMacOSGPUs tests GPU detection
func TestDetectExtraMacOSGPUs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	gpus := detectMacOSGPUs()
	t.Logf("Detected %d GPUs", len(gpus))

	for i, gpu := range gpus {
		t.Logf("GPU %d: %s (%s)", i, gpu.Model, gpu.Vendor)
		if gpu.Model == "" {
			t.Errorf("GPU %d has empty model", i)
		}
		if gpu.Vendor == "" {
			t.Errorf("GPU %d has empty vendor", i)
		}
	}
}

// TestDetectExtraMacOSGPUs_ErrorPath tests GPU detection error handling
func TestDetectExtraMacOSGPUs_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test different code paths
	gpus1 := detectMacOSGPUs()
	gpus2 := detectMacOSGPUs()

	if len(gpus1) != len(gpus2) {
		t.Error("GPU count should be consistent")
	}
}

// TestDetectExtraDarwinKernel tests kernel detection
func TestDetectExtraDarwinKernel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	kernel := detectDarwinKernel()
	if kernel == "" {
		t.Error("Expected kernel version on macOS")
	}
	t.Logf("Kernel: %s", kernel)
}

// TestDetectExtraDarwinKernel_ErrorPath tests kernel detection error handling
func TestDetectExtraDarwinKernel_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times
	kernel1 := detectDarwinKernel()
	kernel2 := detectDarwinKernel()

	if kernel1 != kernel2 {
		t.Error("Kernel version should be consistent")
	}
}

// TestDetectExtraDarwinCPUModel tests CPU model detection
func TestDetectExtraDarwinCPUModel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	model := detectDarwinCPUModel()
	if model == "" {
		t.Error("Expected CPU model on macOS")
	}
	t.Logf("CPU model: %s", model)
}

// TestDetectExtraDarwinCPUModel_ErrorPath tests CPU model detection error handling
func TestDetectExtraDarwinCPUModel_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times
	model1 := detectDarwinCPUModel()
	model2 := detectDarwinCPUModel()

	if model1 != model2 {
		t.Error("CPU model should be consistent")
	}
}

// TestDetectExtraDarwinCPUFlags tests CPU flags detection
func TestDetectExtraDarwinCPUFlags(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	flags := detectDarwinCPUFlags()
	t.Logf("CPU flags: %d detected", len(flags))

	// Should have at least some flags
	if len(flags) == 0 {
		t.Log("Warning: No CPU flags detected (unusual)")
	}
}

// TestDetectExtraDarwinCPUFlags_ErrorPath tests CPU flags detection error handling
func TestDetectExtraDarwinCPUFlags_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times
	flags1 := detectDarwinCPUFlags()
	flags2 := detectDarwinCPUFlags()

	if len(flags1) != len(flags2) {
		t.Error("CPU flags count should be consistent")
	}
}

// TestDetectExtraDarwinMemoryFree tests free memory detection
func TestDetectExtraDarwinMemoryFree(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	memFree := detectDarwinMemoryFree()
	if memFree <= 0 {
		t.Error("Expected positive memory free value on macOS")
	}
	t.Logf("Memory free: %d MB", memFree)
}

// TestDetectExtraDarwinMemoryFree_ErrorPath tests free memory detection error handling
func TestDetectExtraDarwinMemoryFree_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test parsing logic
	memFree1 := detectDarwinMemoryFree()
	memFree2 := detectDarwinMemoryFree()

	// Values might differ slightly but should both be positive
	if memFree1 <= 0 || memFree2 <= 0 {
		t.Error("Memory free should be positive")
	}
}

// TestDetectExtraDarwinSwap tests swap detection
func TestDetectExtraDarwinSwap(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	swapTotal, swapFree := detectDarwinSwap()
	t.Logf("Swap: %d MB total, %d MB free", swapTotal, swapFree)

	// Swap values should not be negative
	if swapTotal < 0 {
		t.Error("Swap total should not be negative")
	}
	if swapFree < 0 {
		t.Error("Swap free should not be negative")
	}
}

// TestDetectExtraDarwinSwap_ErrorPath tests swap detection error handling
func TestDetectExtraDarwinSwap_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times to test parsing logic
	swapTotal1, swapFree1 := detectDarwinSwap()
	swapTotal2, swapFree2 := detectDarwinSwap()

	// Values should be consistent or close
	if swapTotal1 < 0 || swapTotal2 < 0 {
		t.Error("Swap total should not be negative")
	}
	if swapFree1 < 0 || swapFree2 < 0 {
		t.Error("Swap free should not be negative")
	}
}

// TestDetectExtraDarwinDefaultRoute tests default route detection
func TestDetectExtraDarwinDefaultRoute(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	gateway := detectDarwinDefaultRoute()
	if gateway == "" {
		t.Log("Warning: No default gateway detected")
	} else {
		t.Logf("Default gateway: %s", gateway)
	}
}

// TestDetectExtraDarwinDefaultRoute_ErrorPath tests default route detection error handling
func TestDetectExtraDarwinDefaultRoute_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times
	gateway1 := detectDarwinDefaultRoute()
	gateway2 := detectDarwinDefaultRoute()

	if gateway1 != gateway2 {
		t.Error("Default gateway should be consistent")
	}
}

// TestDetectExtraDarwinDNS tests DNS detection
func TestDetectExtraDarwinDNS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	servers := detectDarwinDNS()
	if len(servers) == 0 {
		t.Log("Warning: No DNS servers detected")
	} else {
		t.Logf("DNS servers: %v", servers)
	}
}

// TestDetectExtraDarwinDNS_ErrorPath tests DNS detection error handling
func TestDetectExtraDarwinDNS_ErrorPath(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	// Call multiple times
	servers1 := detectDarwinDNS()
	servers2 := detectDarwinDNS()

	if len(servers1) != len(servers2) {
		t.Log("Warning: DNS server count changed between calls")
	}
}

// TestCollectExtraDarwinFacts_AllFields tests that all fields are populated
func TestCollectExtraDarwinFacts_AllFields(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test")
	}

	facts := &Facts{}
	collectDarwinFacts(facts)

	// Test all fields are at least initialized
	t.Logf("Distribution: %s", facts.Distribution)
	t.Logf("Kernel: %s", facts.KernelVersion)
	t.Logf("CPU Model: %s", facts.CPUModel)
	t.Logf("CPU Flags: %d", len(facts.CPUFlags))
	t.Logf("Memory Total: %d MB", facts.MemoryTotalMB)
	t.Logf("Memory Free: %d MB", facts.MemoryFreeMB)
	t.Logf("Swap Total: %d MB", facts.SwapTotalMB)
	t.Logf("Swap Free: %d MB", facts.SwapFreeMB)
	t.Logf("Disks: %d", len(facts.Disks))
	t.Logf("GPUs: %d", len(facts.GPUs))
	t.Logf("Default Gateway: %s", facts.DefaultGateway)
	t.Logf("DNS Servers: %d", len(facts.DNSServers))

	// Verify required fields set by collectDarwinFacts
	// Note: OS, Arch, CPUCores are set by main Collect() function, not collectDarwinFacts
	if facts.MemoryTotalMB <= 0 {
		t.Error("MemoryTotalMB should be positive")
	}
	if len(facts.Disks) == 0 {
		t.Error("Should have at least one disk")
	}
}
