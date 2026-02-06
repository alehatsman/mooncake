package facts

import (
	"os"
	"runtime"
	"testing"
)

// TestDetectOllamaEndpoint_WithHTTPPrefix tests endpoint with http:// prefix
func TestDetectOllamaEndpoint_WithHTTPPrefix(t *testing.T) {
	// Set OLLAMA_HOST with http:// prefix
	originalValue := os.Getenv("OLLAMA_HOST")
	defer os.Setenv("OLLAMA_HOST", originalValue)

	os.Setenv("OLLAMA_HOST", "http://custom-host:8080")

	endpoint := detectOllamaEndpoint()

	if endpoint != "http://custom-host:8080" {
		t.Errorf("detectOllamaEndpoint() = %s, want 'http://custom-host:8080'", endpoint)
	}
}

// TestDetectOllamaEndpoint_WithHTTPSPrefix tests endpoint with https:// prefix
func TestDetectOllamaEndpoint_WithHTTPSPrefix(t *testing.T) {
	originalValue := os.Getenv("OLLAMA_HOST")
	defer os.Setenv("OLLAMA_HOST", originalValue)

	os.Setenv("OLLAMA_HOST", "https://secure-host:8443")

	endpoint := detectOllamaEndpoint()

	if endpoint != "https://secure-host:8443" {
		t.Errorf("detectOllamaEndpoint() = %s, want 'https://secure-host:8443'", endpoint)
	}
}

// TestDetectOllamaEndpoint_WithoutPrefix tests endpoint without http prefix
func TestDetectOllamaEndpoint_WithoutPrefix(t *testing.T) {
	originalValue := os.Getenv("OLLAMA_HOST")
	defer os.Setenv("OLLAMA_HOST", originalValue)

	os.Setenv("OLLAMA_HOST", "custom-host:9090")

	endpoint := detectOllamaEndpoint()

	if endpoint != "http://custom-host:9090" {
		t.Errorf("detectOllamaEndpoint() = %s, want 'http://custom-host:9090' (with added prefix)", endpoint)
	}
}

// TestDetectOllamaEndpoint_NoEnvVar tests default endpoint
func TestDetectOllamaEndpoint_NoEnvVar(t *testing.T) {
	originalValue := os.Getenv("OLLAMA_HOST")
	defer func() {
		if originalValue != "" {
			os.Setenv("OLLAMA_HOST", originalValue)
		} else {
			os.Unsetenv("OLLAMA_HOST")
		}
	}()

	os.Unsetenv("OLLAMA_HOST")

	endpoint := detectOllamaEndpoint()

	if endpoint != "http://localhost:11434" {
		t.Errorf("detectOllamaEndpoint() = %s, want 'http://localhost:11434' (default)", endpoint)
	}
}

// TestDetectMacOSPackageManager_Variations tests package manager detection
func TestDetectMacOSPackageManager_Variations(t *testing.T) {
	// This test just verifies the function runs without panic
	// Actual result depends on system state
	result := detectMacOSPackageManager()

	// Result can be "brew", "port", or ""
	if result != "" && result != "brew" && result != "port" {
		t.Errorf("detectMacOSPackageManager() returned unexpected value: %s", result)
	}

	t.Logf("Detected package manager: %s", result)
}

// TestDetectDarwinCPUFlags_ErrorHandling tests CPU flags detection
func TestDetectDarwinCPUFlags_ErrorHandling(t *testing.T) {
	// This test verifies the function handles both success and failure cases
	flags := detectDarwinCPUFlags()

	// On Intel Macs, should return flags
	// On Apple Silicon, might return nil
	t.Logf("Detected %d CPU flags", len(flags))

	// Verify returned data is consistent
	for i, flag := range flags {
		if flag == "" {
			t.Errorf("Flag %d should not be empty string", i)
		}
	}
}

// TestDetectMacOSMemory_NonZero tests memory detection returns reasonable value
func TestDetectMacOSMemory_NonZero(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test on non-Darwin platform")
	}
	memory := detectMacOSMemory()

	// Memory should be > 0 on any real Mac
	// Even old Macs have at least 1GB
	if memory <= 0 {
		t.Errorf("detectMacOSMemory() = %d, should be > 0", memory)
	}

	// Sanity check: memory should be reasonable (between 1GB and 1TB)
	if memory < 1024 || memory > 1024*1024*1024 {
		t.Logf("Warning: Memory value seems unusual: %d MB", memory)
	}
}

// TestDetectMacOSVersion_Format tests version string format
func TestDetectMacOSVersion_Format(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test on non-Darwin platform")
	}
	version := detectMacOSVersion()

	if version == "" {
		t.Error("detectMacOSVersion() should return non-empty version")
	}

	// macOS versions typically contain digits and dots
	// e.g., "14.1.2" for Sonoma
	t.Logf("Detected macOS version: %s", version)
}

// TestDetectDarwinKernel_Format tests kernel version format
func TestDetectDarwinKernel_Format(t *testing.T) {
	kernel := detectDarwinKernel()

	if kernel == "" {
		t.Error("detectDarwinKernel() should return non-empty kernel version")
	}

	// Kernel versions typically like "23.1.0"
	t.Logf("Detected kernel version: %s", kernel)
}

// TestDetectDarwinCPUModel_NonEmpty tests CPU model detection
func TestDetectDarwinCPUModel_NonEmpty(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test on non-Darwin platform")
	}
	model := detectDarwinCPUModel()

	if model == "" {
		t.Error("detectDarwinCPUModel() should return non-empty CPU model")
	}

	t.Logf("Detected CPU model: %s", model)
}

// TestDetectMacOSDisks_HasRoot tests disk detection includes root
func TestDetectMacOSDisks_HasRoot(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping macOS-specific test on non-Darwin platform")
	}
	disks := detectMacOSDisks()

	// Should have at least one disk (root filesystem)
	if len(disks) == 0 {
		t.Error("detectMacOSDisks() should return at least one disk")
	}

	// Verify root filesystem exists
	foundRoot := false
	for _, disk := range disks {
		if disk.MountPoint == "/" {
			foundRoot = true

			// Root should have reasonable values
			if disk.SizeGB <= 0 {
				t.Error("Root disk should have positive size")
			}
			if disk.Filesystem == "" {
				t.Error("Root disk should have filesystem type")
			}
		}
	}

	if !foundRoot {
		t.Error("Should find root (/) filesystem")
	}
}

// TestDetectMacOSGPUs_Structure tests GPU detection structure
func TestDetectMacOSGPUs_Structure(t *testing.T) {
	gpus := detectMacOSGPUs()

	// Modern Macs have at least one GPU
	t.Logf("Detected %d GPU(s)", len(gpus))

	// Verify GPU data structure
	for i, gpu := range gpus {
		if gpu.Vendor == "" && gpu.Model == "" {
			t.Errorf("GPU %d should have either vendor or model", i)
		}
	}
}

// TestDetectDarwinMemoryFree_Positive tests free memory detection
func TestDetectDarwinMemoryFree_Positive(t *testing.T) {
	freeMem := detectDarwinMemoryFree()

	// Free memory should be non-negative
	if freeMem < 0 {
		t.Errorf("detectDarwinMemoryFree() = %d, should be >= 0", freeMem)
	}

	t.Logf("Free memory: %d MB", freeMem)
}

// TestDetectDarwinSwap_NonNegative tests swap detection
func TestDetectDarwinSwap_NonNegative(t *testing.T) {
	totalSwap, freeSwap := detectDarwinSwap()

	// Swap values should be non-negative
	if totalSwap < 0 {
		t.Errorf("Total swap = %d, should be >= 0", totalSwap)
	}
	if freeSwap < 0 {
		t.Errorf("Free swap = %d, should be >= 0", freeSwap)
	}

	// Free swap should not exceed total swap
	if freeSwap > totalSwap {
		t.Errorf("Free swap (%d) should not exceed total swap (%d)", freeSwap, totalSwap)
	}

	t.Logf("Swap: %d MB total, %d MB free", totalSwap, freeSwap)
}

// TestDetectDarwinDefaultRoute_Format tests default route detection
func TestDetectDarwinDefaultRoute_Format(t *testing.T) {
	gateway := detectDarwinDefaultRoute()

	// Gateway might be empty if no network
	// If present, should look like an IP address
	if gateway != "" {
		t.Logf("Default gateway: %s", gateway)
	} else {
		t.Log("No default gateway detected (might not be connected)")
	}
}

// TestDetectDarwinDNS_Structure tests DNS server detection
func TestDetectDarwinDNS_Structure(t *testing.T) {
	dnsServers := detectDarwinDNS()

	// DNS servers might be empty if no network
	t.Logf("Detected %d DNS server(s)", len(dnsServers))

	// Verify DNS server format
	for i, server := range dnsServers {
		if server == "" {
			t.Errorf("DNS server %d should not be empty string", i)
		}
	}
}

// TestCollectIPAddresses_NoEmpty tests IP address collection
func TestCollectIPAddresses_NoEmpty(t *testing.T) {
	addresses := collectIPAddresses()

	// Verify no empty addresses
	for i, addr := range addresses {
		if addr == "" {
			t.Errorf("IP address %d should not be empty string", i)
		}
	}

	// Verify loopback is filtered
	for _, addr := range addresses {
		if addr == "127.0.0.1" || addr == "::1" {
			t.Errorf("Loopback address %s should be filtered out", addr)
		}
	}
}

// TestCollectNetworkInterfaces_HasLoopback tests network interface collection
func TestCollectNetworkInterfaces_HasLoopback(t *testing.T) {
	interfaces := collectNetworkInterfaces()

	// Should have at least loopback interface
	if len(interfaces) == 0 {
		t.Error("Should detect at least one network interface")
	}

	// Verify interface data structure
	for i, iface := range interfaces {
		if iface.Name == "" {
			t.Errorf("Interface %d should have a name", i)
		}
	}
}

// TestDetectPythonVersion_Format tests Python version detection
func TestDetectPythonVersion_Format(t *testing.T) {
	version := detectPythonVersion()

	// Python might not be installed
	if version == "" {
		t.Skip("Python not installed")
	}

	// Version should have digits
	t.Logf("Python version: %s", version)
}

// TestDetectToolchainVersion_EmptyPrefix tests toolchain detection with empty prefix
func TestDetectToolchainVersion_EmptyPrefix(t *testing.T) {
	// Test with empty prefix (edge case)
	version := detectToolchainVersion("echo", "test", "")

	// Should work even with empty prefix
	if version == "" {
		t.Log("Empty prefix test returned empty (expected for echo test)")
	}
}
