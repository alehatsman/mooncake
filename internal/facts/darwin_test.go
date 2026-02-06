package facts

import (
	"runtime"
	"testing"
)

func TestDetectDarwinKernel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	kernel := detectDarwinKernel()
	if kernel == "" {
		t.Error("Expected kernel version on Darwin")
	}
	t.Logf("Kernel version: %s", kernel)
}

func TestDetectDarwinCPUModel(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	model := detectDarwinCPUModel()
	if model == "" {
		t.Error("Expected CPU model on Darwin")
	}
	t.Logf("CPU model: %s", model)
}

func TestDetectDarwinCPUFlags(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	flags := detectDarwinCPUFlags()
	// Flags might be empty on Apple Silicon
	if len(flags) > 0 {
		t.Logf("CPU flags count: %d", len(flags))
	} else {
		t.Log("CPU flags not available (might be Apple Silicon)")
	}
}

func TestDetectDarwinMemoryFree(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	memFree := detectDarwinMemoryFree()
	if memFree <= 0 {
		t.Error("Expected positive memory free value on Darwin")
	}
	t.Logf("Memory free: %d MB", memFree)
}

func TestDetectDarwinSwap(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	swapTotal, swapFree := detectDarwinSwap()
	// Swap might be 0 if not configured
	t.Logf("Swap: %d MB total, %d MB free", swapTotal, swapFree)
}

func TestDetectDarwinDefaultRoute(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	gateway := detectDarwinDefaultRoute()
	// Gateway might be empty in some environments
	if gateway != "" {
		t.Logf("Default gateway: %s", gateway)
	} else {
		t.Log("No default gateway detected")
	}
}

func TestDetectDarwinDNS(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	servers := detectDarwinDNS()
	// DNS might be empty in some environments
	if len(servers) > 0 {
		t.Logf("DNS servers: %v", servers)
	} else {
		t.Log("No DNS servers detected")
	}
}

func TestCollectDarwinFacts_Integration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	facts := &Facts{}
	collectDarwinFacts(facts)

	// Check that basic facts are set
	if facts.Distribution != "macos" {
		t.Errorf("Distribution = %s, want macos", facts.Distribution)
	}

	if facts.DistributionVersion == "" {
		t.Error("DistributionVersion should be set on Darwin")
	}

	if facts.MemoryTotalMB <= 0 {
		t.Error("MemoryTotalMB should be positive on Darwin")
	}

	// Check extended facts
	if facts.KernelVersion == "" {
		t.Error("KernelVersion should be set on Darwin")
	}
	if facts.CPUModel == "" {
		t.Error("CPUModel should be set on Darwin")
	}
}

func TestDetectMacOSVersion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	version := detectMacOSVersion()
	if version == "" {
		t.Error("Expected macOS version on Darwin")
	}
	t.Logf("macOS version: %s", version)

	// Should contain dots for version format
	if len(version) > 0 && version[0] >= '0' && version[0] <= '9' {
		t.Logf("Version starts with digit as expected")
	}
}

func TestDetectMacOSPackageManager(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	pm := detectMacOSPackageManager()
	// Package manager might be empty if neither brew nor port is installed
	if pm != "" {
		t.Logf("Package manager: %s", pm)
		if pm != "brew" && pm != "port" {
			t.Errorf("Expected 'brew' or 'port', got %s", pm)
		}
	} else {
		t.Log("No package manager detected (neither brew nor port installed)")
	}
}

func TestDetectMacOSMemory(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	mem := detectMacOSMemory()
	if mem <= 0 {
		t.Error("Expected positive memory value on Darwin")
	}
	t.Logf("Memory: %d MB", mem)

	// Sanity check: should be at least 1GB and less than 1TB
	if mem < 1024 || mem > 1024*1024 {
		t.Errorf("Memory value %d MB seems unrealistic", mem)
	}
}

func TestDetectMacOSDisks(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	disks := detectMacOSDisks()
	if len(disks) == 0 {
		t.Error("Expected at least one disk on Darwin")
	}
	t.Logf("Detected %d disks", len(disks))

	// Check disk structure
	for _, disk := range disks {
		if disk.Device == "" {
			t.Error("Disk device should not be empty")
		}
		if disk.MountPoint == "" {
			t.Error("Disk mount point should not be empty")
		}
		t.Logf("Disk: %s mounted at %s (%dGB)", disk.Device, disk.MountPoint, disk.SizeGB)
	}
}

func TestDetectMacOSGPUs(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	gpus := detectMacOSGPUs()
	// GPUs might be 0 on some systems
	if len(gpus) > 0 {
		t.Logf("Detected %d GPUs", len(gpus))
		for _, gpu := range gpus {
			t.Logf("GPU: %s (Vendor: %s)", gpu.Model, gpu.Vendor)
			if gpu.Model == "" {
				t.Error("GPU model should not be empty")
			}
		}
	} else {
		t.Log("No GPUs detected")
	}
}

func TestDetectDarwinCPUFlags_Coverage(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test")
	}

	flags := detectDarwinCPUFlags()

	// Test that we can iterate over flags
	for _, flag := range flags {
		if flag == "" {
			t.Error("CPU flag should not be empty string")
		}
	}

	// On Apple Silicon, sysctl might not return CPU flags
	if len(flags) == 0 {
		t.Log("No CPU flags detected (expected on Apple Silicon)")
	} else {
		t.Logf("Detected %d CPU flags", len(flags))
	}
}
