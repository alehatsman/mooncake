package facts

import (
	"os"
	"runtime"
	"strings"
	"testing"
)

// isRunningInDocker detects if tests are running inside a Docker container.
// Docker containers typically have limited hardware access, so some tests need different expectations.
func isRunningInDocker() bool {
	// Check for /.dockerenv file (most reliable)
	if _, err := os.Stat("/.dockerenv"); err == nil {
		return true
	}

	// Check cgroup for docker/containerd
	if data, err := os.ReadFile("/proc/1/cgroup"); err == nil {
		content := string(data)
		if strings.Contains(content, "docker") || strings.Contains(content, "containerd") {
			return true
		}
	}

	return false
}

func TestDetectLinuxKernel(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	kernel := detectLinuxKernel()
	if kernel == "" {
		t.Error("Expected kernel version on Linux")
	}
	t.Logf("Kernel version: %s", kernel)
}

func TestDetectLinuxCPUModel(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	model := detectLinuxCPUModel()
	if model == "" {
		if isRunningInDocker() {
			t.Log("CPU model not available in Docker (expected - limited hardware access)")
		} else {
			t.Error("Expected CPU model on Linux")
		}
	} else {
		t.Logf("CPU model: %s", model)
	}
}

func TestDetectLinuxCPUFlags(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	flags := detectLinuxCPUFlags()
	if len(flags) == 0 {
		if isRunningInDocker() {
			t.Log("CPU flags not available in Docker (expected - limited hardware access)")
		} else {
			t.Error("Expected CPU flags on Linux")
		}
	} else {
		t.Logf("CPU flags count: %d", len(flags))
	}
}

func TestDetectLinuxMemoryFree(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	memFree := detectLinuxMemoryFree()
	if memFree <= 0 {
		t.Error("Expected positive memory free value on Linux")
	}
	t.Logf("Memory free: %d MB", memFree)
}

func TestDetectLinuxSwap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	swapTotal, swapFree := detectLinuxSwap()
	// Swap might be 0 if not configured
	t.Logf("Swap: %d MB total, %d MB free", swapTotal, swapFree)
}

func TestDetectLinuxDefaultRoute(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	gateway := detectLinuxDefaultRoute()
	// Gateway might be empty in some environments
	if gateway != "" {
		t.Logf("Default gateway: %s", gateway)
	} else {
		t.Log("No default gateway detected")
	}
}

func TestDetectLinuxDNS(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	servers := detectLinuxDNS()
	// DNS might be empty in some environments
	if len(servers) > 0 {
		t.Logf("DNS servers: %v", servers)
	} else {
		t.Log("No DNS servers detected")
	}
}

func TestDetectCUDAVersion(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	version := detectCUDAVersion()
	// CUDA might not be installed
	if version != "" {
		t.Logf("CUDA version: %s", version)
	} else {
		t.Log("CUDA not detected (nvidia-smi not found)")
	}
}

func TestCollectLinuxFacts_Integration(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping Linux-specific test")
	}

	inDocker := isRunningInDocker()
	if inDocker {
		t.Log("Running in Docker container - some hardware detection may be limited")
	}

	facts := &Facts{}
	collectLinuxFacts(facts)

	// Check that basic facts are set
	if facts.Distribution == "" {
		t.Log("Warning: Distribution not detected")
	} else {
		t.Logf("Distribution: %s %s", facts.Distribution, facts.DistributionVersion)
	}

	if facts.MemoryTotalMB <= 0 {
		t.Error("MemoryTotalMB should be positive on Linux")
	}

	// Check extended facts
	if facts.KernelVersion == "" {
		t.Error("KernelVersion should be set on Linux")
	}

	// CPU info may not be available in Docker
	if facts.CPUModel == "" {
		if inDocker {
			t.Log("CPUModel not available in Docker (expected)")
		} else {
			t.Error("CPUModel should be set on Linux")
		}
	}
	if len(facts.CPUFlags) == 0 {
		if inDocker {
			t.Log("CPUFlags not available in Docker (expected)")
		} else {
			t.Error("CPUFlags should not be empty on Linux")
		}
	}
}
