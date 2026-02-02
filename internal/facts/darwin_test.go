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
