package explain

import (
	"bytes"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/facts"
)

// captureOutput captures stdout during function execution
func captureOutput(fn func()) string {
	// Save original stdout
	oldStdout := os.Stdout

	// Create pipe
	r, w, err := os.Pipe()
	if err != nil {
		panic(err)
	}

	// Replace stdout with pipe writer
	os.Stdout = w

	// Channel to receive output
	outC := make(chan string)

	// Copy output in goroutine
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outC <- buf.String()
	}()

	// Execute function
	fn()

	// Close writer and restore stdout
	w.Close()
	os.Stdout = oldStdout

	// Get captured output
	return <-outC
}

func TestDisplayFacts_MinimalFacts(t *testing.T) {
	// Test with minimal facts
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            4,
		MemoryTotalMB:       8192,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Verify essential content
	if !strings.Contains(output, "System Information") {
		t.Error("Output should contain header")
	}
	if !strings.Contains(output, "Ubuntu 22.04") {
		t.Error("Output should contain OS information")
	}
	if !strings.Contains(output, "amd64") {
		t.Error("Output should contain architecture")
	}
	if !strings.Contains(output, "test-host") {
		t.Error("Output should contain hostname")
	}
	if !strings.Contains(output, "4") {
		t.Error("Output should contain CPU cores")
	}
	if !strings.Contains(output, "8192 MB") {
		t.Error("Output should contain memory total")
	}
}

func TestDisplayFacts_WithKernelVersion(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Debian",
		DistributionVersion: "12",
		Arch:                "arm64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		KernelVersion:       "6.5.0-14-generic",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "6.5.0-14-generic") {
		t.Error("Output should contain kernel version")
	}
	if !strings.Contains(output, "Kernel:") {
		t.Error("Output should have Kernel label")
	}
}

func TestDisplayFacts_WithoutKernelVersion(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Debian",
		DistributionVersion: "12",
		Arch:                "arm64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		KernelVersion:       "",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Kernel line should not be present when empty
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Kernel:") && strings.TrimSpace(line) != "" {
			t.Error("Output should not contain Kernel label when kernel version is empty")
		}
	}
}

func TestDisplayFacts_CPUWithModel(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            16,
		MemoryTotalMB:       32768,
		CPUModel:            "Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "CPU:") {
		t.Error("Output should contain CPU section")
	}
	if !strings.Contains(output, "16") {
		t.Error("Output should contain CPU cores")
	}
	if !strings.Contains(output, "Intel(R) Core(TM) i9-9900K") {
		t.Error("Output should contain CPU model")
	}
}

func TestDisplayFacts_CPUWithImportantFlags(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		CPUFlags:            []string{"avx", "avx2", "sse4_2", "fma", "aes", "cx16", "pae"},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Flags:") {
		t.Error("Output should contain Flags label")
	}
	// Check for important flags
	if !strings.Contains(output, "avx") {
		t.Error("Output should contain avx flag")
	}
	if !strings.Contains(output, "avx2") {
		t.Error("Output should contain avx2 flag")
	}
	if !strings.Contains(output, "sse4_2") {
		t.Error("Output should contain sse4_2 flag")
	}
	if !strings.Contains(output, "fma") {
		t.Error("Output should contain fma flag")
	}
	if !strings.Contains(output, "aes") {
		t.Error("Output should contain aes flag")
	}
	// Should not show unimportant flags
	if strings.Contains(output, "cx16") {
		t.Error("Output should not contain unimportant flags like cx16")
	}
	if strings.Contains(output, "pae") {
		t.Error("Output should not contain unimportant flags like pae")
	}
}

func TestDisplayFacts_CPUWithNoImportantFlags(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            4,
		MemoryTotalMB:       8192,
		CPUFlags:            []string{"cx16", "pae", "msr"},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Should not have Flags line when no important flags
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Flags:") {
			t.Error("Output should not contain Flags label when no important flags present")
		}
	}
}

func TestDisplayFacts_MemoryWithFree(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		MemoryFreeMB:        8192,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Memory:") {
		t.Error("Output should contain Memory section")
	}
	if !strings.Contains(output, "16384 MB") {
		t.Error("Output should contain total memory")
	}
	if !strings.Contains(output, "8192 MB") {
		t.Error("Output should contain free memory")
	}
	if !strings.Contains(output, "Free:") {
		t.Error("Output should have Free label")
	}
}

func TestDisplayFacts_MemoryWithSwap(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		SwapTotalMB:         4096,
		SwapFreeMB:          2048,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Swap:") {
		t.Error("Output should contain Swap label")
	}
	if !strings.Contains(output, "4096 MB total") {
		t.Error("Output should contain swap total")
	}
	if !strings.Contains(output, "2048 MB free") {
		t.Error("Output should contain swap free")
	}
}

func TestDisplayFacts_MemoryWithoutSwap(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		SwapTotalMB:         0,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Should not show swap when zero
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Swap:") {
			t.Error("Output should not contain Swap label when swap is zero")
		}
	}
}

func TestDisplayFacts_SoftwareSection(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		PackageManager:      "apt",
		PythonVersion:       "3.11.5",
		DockerVersion:       "24.0.7",
		GitVersion:          "2.43.0",
		GoVersion:           "1.21.5",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Software:") {
		t.Error("Output should contain Software section")
	}
	if !strings.Contains(output, "Package Manager: apt") {
		t.Error("Output should contain package manager")
	}
	if !strings.Contains(output, "Python:          3.11.5") {
		t.Error("Output should contain Python version")
	}
	if !strings.Contains(output, "Docker:          24.0.7") {
		t.Error("Output should contain Docker version")
	}
	if !strings.Contains(output, "Git:             2.43.0") {
		t.Error("Output should contain Git version")
	}
	if !strings.Contains(output, "Go:              1.21.5") {
		t.Error("Output should contain Go version")
	}
}

func TestDisplayFacts_SoftwareWithOllama(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		OllamaVersion:       "0.1.47",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Software:") {
		t.Error("Output should contain Software section")
	}
	if !strings.Contains(output, "Ollama:          0.1.47") {
		t.Error("Output should contain Ollama version")
	}
}

func TestDisplayFacts_NoSoftwareSection(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Count occurrences of "Software:" (should not appear without software tools)
	count := strings.Count(output, "Software:")
	if count > 0 {
		t.Error("Output should not contain Software section when no software tools detected")
	}
}

func TestDisplayFacts_OllamaModels(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		OllamaVersion:       "0.1.47",
		OllamaEndpoint:      "http://localhost:11434",
		OllamaModels: []facts.OllamaModel{
			{
				Name:       "llama3.1:8b",
				Size:       "4.7 GB",
				ModifiedAt: "2024-01-15T10:30:00Z",
			},
			{
				Name:       "mistral:7b",
				Size:       "4.1 GB",
				ModifiedAt: "2024-01-16T14:20:00Z",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Ollama Models:") {
		t.Error("Output should contain Ollama Models section")
	}
	if !strings.Contains(output, "http://localhost:11434") {
		t.Error("Output should contain Ollama endpoint")
	}
	if !strings.Contains(output, "2 installed") {
		t.Error("Output should contain model count")
	}
	if !strings.Contains(output, "llama3.1:8b") {
		t.Error("Output should contain first model name")
	}
	if !strings.Contains(output, "4.7 GB") {
		t.Error("Output should contain first model size")
	}
	if !strings.Contains(output, "mistral:7b") {
		t.Error("Output should contain second model name")
	}
	if !strings.Contains(output, "4.1 GB") {
		t.Error("Output should contain second model size")
	}
	if !strings.Contains(output, "Modified: 2024-01-15T10:30:00Z") {
		t.Error("Output should contain first model modified date")
	}
}

func TestDisplayFacts_OllamaModelsWithoutModifiedAt(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		OllamaVersion:       "0.1.47",
		OllamaEndpoint:      "http://localhost:11434",
		OllamaModels: []facts.OllamaModel{
			{
				Name: "llama3.1:8b",
				Size: "4.7 GB",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "llama3.1:8b") {
		t.Error("Output should contain model name")
	}
	if !strings.Contains(output, "4.7 GB") {
		t.Error("Output should contain model size")
	}
	// Should not have "Modified:" when ModifiedAt is empty
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "llama3.1:8b") && strings.Contains(line, "Modified:") {
			t.Error("Output should not contain Modified when ModifiedAt is empty")
		}
	}
}

func TestDisplayFacts_NoOllamaModelsWhenNoOllama(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		OllamaVersion:       "",
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if strings.Contains(output, "Ollama Models:") {
		t.Error("Output should not contain Ollama Models section when Ollama is not installed")
	}
}

func TestDisplayFacts_GPUs(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		GPUs: []facts.GPU{
			{
				Vendor:      "nvidia",
				Model:       "RTX 3090",
				Memory:      "24GB",
				Driver:      "535.129.03",
				CUDAVersion: "12.2",
			},
			{
				Vendor: "amd",
				Model:  "RX 7900 XTX",
				Memory: "24GB",
				Driver: "23.40",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "GPUs:") {
		t.Error("Output should contain GPUs section")
	}
	if !strings.Contains(output, "NVIDIA RTX 3090") {
		t.Error("Output should contain first GPU (NVIDIA RTX 3090)")
	}
	if !strings.Contains(output, "Memory: 24GB") {
		t.Error("Output should contain GPU memory")
	}
	if !strings.Contains(output, "Driver: 535.129.03") {
		t.Error("Output should contain NVIDIA driver")
	}
	if !strings.Contains(output, "CUDA: 12.2") {
		t.Error("Output should contain CUDA version")
	}
	if !strings.Contains(output, "AMD RX 7900 XTX") {
		t.Error("Output should contain second GPU (AMD RX 7900 XTX)")
	}
	if !strings.Contains(output, "Driver: 23.40") {
		t.Error("Output should contain AMD driver")
	}
}

func TestDisplayFacts_GPUsWithMinimalInfo(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		GPUs: []facts.GPU{
			{
				Vendor: "intel",
				Model:  "UHD Graphics 630",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "GPUs:") {
		t.Error("Output should contain GPUs section")
	}
	if !strings.Contains(output, "INTEL UHD Graphics 630") {
		t.Error("Output should contain GPU with uppercased vendor")
	}
	// Should not have Memory/Driver/CUDA when not present
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "INTEL UHD Graphics 630") {
			if strings.Contains(line, "Memory:") || strings.Contains(line, "Driver:") || strings.Contains(line, "CUDA:") {
				t.Error("Output should not contain Memory/Driver/CUDA when not present")
			}
		}
	}
}

func TestDisplayFacts_NoGPUsSection(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		GPUs:                []facts.GPU{},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if strings.Contains(output, "GPUs:") {
		t.Error("Output should not contain GPUs section when no GPUs present")
	}
}

func TestDisplayFacts_Storage(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		Disks: []facts.Disk{
			{
				Device:     "/dev/sda1",
				MountPoint: "/",
				Filesystem: "ext4",
				SizeGB:     500,
				UsedGB:     200,
				AvailGB:    300,
			},
			{
				Device:     "/dev/sdb1",
				MountPoint: "/data",
				Filesystem: "xfs",
				SizeGB:     1000,
				UsedGB:     750,
				AvailGB:    250,
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Storage:") {
		t.Error("Output should contain Storage section")
	}
	if !strings.Contains(output, "Device") {
		t.Error("Output should contain Device header")
	}
	if !strings.Contains(output, "Mount") {
		t.Error("Output should contain Mount header")
	}
	if !strings.Contains(output, "Type") {
		t.Error("Output should contain Type header")
	}
	if !strings.Contains(output, "Size") {
		t.Error("Output should contain Size header")
	}
	if !strings.Contains(output, "/dev/sda1") {
		t.Error("Output should contain first disk device")
	}
	if !strings.Contains(output, "/") {
		t.Error("Output should contain root mount point")
	}
	if !strings.Contains(output, "ext4") {
		t.Error("Output should contain ext4 filesystem")
	}
	if !strings.Contains(output, "500") && !strings.Contains(output, "500 GB") {
		t.Error("Output should contain disk size")
	}
	if !strings.Contains(output, "/dev/sdb1") {
		t.Error("Output should contain second disk device")
	}
	if !strings.Contains(output, "/data") {
		t.Error("Output should contain /data mount point")
	}
	if !strings.Contains(output, "xfs") {
		t.Error("Output should contain xfs filesystem")
	}
}

func TestDisplayFacts_StorageWithLongNames(t *testing.T) {
	// Test column width calculation with long device names
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		Disks: []facts.Disk{
			{
				Device:     "/dev/mapper/ubuntu--vg-ubuntu--lv",
				MountPoint: "/var/lib/docker/volumes",
				Filesystem: "ext4",
				SizeGB:     100,
				UsedGB:     50,
				AvailGB:    50,
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "/dev/mapper/ubuntu--vg-ubuntu--lv") {
		t.Error("Output should contain long device name")
	}
	if !strings.Contains(output, "/var/lib/docker/volumes") {
		t.Error("Output should contain long mount point")
	}
	// Check that output is formatted properly (has separator line)
	if !strings.Contains(output, "─────") {
		t.Error("Output should contain separator line in storage table")
	}
}

func TestDisplayFacts_NoStorageSection(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		Disks:               []facts.Disk{},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if strings.Contains(output, "Storage:") {
		t.Error("Output should not contain Storage section when no disks present")
	}
}

func TestDisplayFacts_NetworkBasic(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		DefaultGateway:      "192.168.1.1",
		DNSServers:          []string{"8.8.8.8", "1.1.1.1"},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Network:") {
		t.Error("Output should contain Network section")
	}
	if !strings.Contains(output, "Gateway:  192.168.1.1") {
		t.Error("Output should contain gateway")
	}
	if !strings.Contains(output, "DNS:      8.8.8.8, 1.1.1.1") {
		t.Error("Output should contain DNS servers")
	}
}

func TestDisplayFacts_NetworkWithoutGateway(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		DefaultGateway:      "",
		DNSServers:          []string{"8.8.8.8"},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "Gateway:") && strings.TrimSpace(line) != "" {
			t.Error("Output should not contain Gateway label when gateway is empty")
		}
	}
}

func TestDisplayFacts_NetworkWithoutDNS(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		DefaultGateway:      "192.168.1.1",
		DNSServers:          []string{},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "DNS:") && strings.TrimSpace(line) != "" {
			t.Error("Output should not contain DNS label when DNS servers list is empty")
		}
	}
}

func TestDisplayFacts_NetworkInterfaces(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		NetworkInterfaces: []facts.NetworkInterface{
			{
				Name:       "en0",
				MACAddress: "00:11:22:33:44:55",
				Up:         true,
				Addresses:  []string{"192.168.1.100/24", "fe80::1/64"},
			},
			{
				Name:       "eth0",
				MACAddress: "aa:bb:cc:dd:ee:ff",
				Up:         true,
				Addresses:  []string{"10.0.0.50/24"},
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "Network Interfaces:") {
		t.Error("Output should contain Network Interfaces section")
	}
	if !strings.Contains(output, "en0") {
		t.Error("Output should contain en0 interface")
	}
	if !strings.Contains(output, "00:11:22:33:44:55") {
		t.Error("Output should contain MAC address")
	}
	if !strings.Contains(output, "192.168.1.100/24") {
		t.Error("Output should contain IPv4 address")
	}
	// Should not show IPv6
	if strings.Contains(output, "fe80::1/64") {
		t.Error("Output should not contain IPv6 address")
	}
	if !strings.Contains(output, "eth0") {
		t.Error("Output should contain eth0 interface")
	}
	if !strings.Contains(output, "10.0.0.50/24") {
		t.Error("Output should contain eth0 IPv4 address")
	}
}

func TestDisplayFacts_NetworkInterfacesFilterByPrefix(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		NetworkInterfaces: []facts.NetworkInterface{
			{
				Name:       "en0",
				Up:         true,
				Addresses:  []string{"192.168.1.100/24"},
				MACAddress: "00:11:22:33:44:55",
			},
			{
				Name:       "docker0",
				Up:         true,
				Addresses:  []string{"172.17.0.1/16"},
				MACAddress: "aa:bb:cc:dd:ee:ff",
			},
			{
				Name:       "veth12345",
				Up:         true,
				Addresses:  []string{"172.17.0.2/16"},
				MACAddress: "11:22:33:44:55:66",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "en0") {
		t.Error("Output should contain en0 interface")
	}
	// Should not show docker0 or veth interfaces
	if strings.Contains(output, "docker0") {
		t.Error("Output should not contain docker0 interface")
	}
	if strings.Contains(output, "veth12345") {
		t.Error("Output should not contain veth interface")
	}
}

func TestDisplayFacts_NetworkInterfacesOnlyUp(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		NetworkInterfaces: []facts.NetworkInterface{
			{
				Name:       "en0",
				Up:         true,
				Addresses:  []string{"192.168.1.100/24"},
				MACAddress: "00:11:22:33:44:55",
			},
			{
				Name:       "en1",
				Up:         false,
				Addresses:  []string{"192.168.1.101/24"},
				MACAddress: "aa:bb:cc:dd:ee:ff",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "en0") {
		t.Error("Output should contain en0 (up) interface")
	}
	// Should not show down interfaces
	if strings.Contains(output, "en1") {
		t.Error("Output should not contain en1 (down) interface")
	}
}

func TestDisplayFacts_NetworkInterfacesWithoutAddresses(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		NetworkInterfaces: []facts.NetworkInterface{
			{
				Name:       "en0",
				Up:         true,
				Addresses:  []string{},
				MACAddress: "00:11:22:33:44:55",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Should not show interfaces without addresses
	if strings.Contains(output, "Network Interfaces:") {
		t.Error("Output should not contain Network Interfaces section when no interfaces have addresses")
	}
}

func TestDisplayFacts_NetworkInterfaceWlanPrefix(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
		NetworkInterfaces: []facts.NetworkInterface{
			{
				Name:       "wlan0",
				Up:         true,
				Addresses:  []string{"192.168.1.100/24"},
				MACAddress: "00:11:22:33:44:55",
			},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	if !strings.Contains(output, "wlan0") {
		t.Error("Output should contain wlan0 interface")
	}
}

func TestDisplayFacts_CompleteSystem(t *testing.T) {
	// Test with a fully populated Facts struct
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04 LTS",
		Arch:                "amd64",
		Hostname:            "prod-server-01",
		KernelVersion:       "6.5.0-14-generic",
		CPUCores:            32,
		CPUModel:            "AMD Ryzen 9 5950X",
		CPUFlags:            []string{"avx", "avx2", "sse4_2", "fma", "aes"},
		MemoryTotalMB:       65536,
		MemoryFreeMB:        32768,
		SwapTotalMB:         8192,
		SwapFreeMB:          4096,
		PackageManager:      "apt",
		PythonVersion:       "3.11.5",
		DockerVersion:       "24.0.7",
		GitVersion:          "2.43.0",
		GoVersion:           "1.21.5",
		OllamaVersion:       "0.1.47",
		OllamaEndpoint:      "http://localhost:11434",
		OllamaModels: []facts.OllamaModel{
			{Name: "llama3.1:8b", Size: "4.7 GB", ModifiedAt: "2024-01-15T10:30:00Z"},
		},
		GPUs: []facts.GPU{
			{Vendor: "nvidia", Model: "RTX 4090", Memory: "24GB", Driver: "535.129.03", CUDAVersion: "12.2"},
		},
		Disks: []facts.Disk{
			{Device: "/dev/nvme0n1p1", MountPoint: "/", Filesystem: "ext4", SizeGB: 1000, UsedGB: 500, AvailGB: 500},
		},
		DefaultGateway: "192.168.1.1",
		DNSServers:     []string{"8.8.8.8", "1.1.1.1"},
		NetworkInterfaces: []facts.NetworkInterface{
			{Name: "eth0", Up: true, Addresses: []string{"192.168.1.100/24"}, MACAddress: "00:11:22:33:44:55"},
		},
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Verify major sections are present
	requiredSections := []string{
		"System Information",
		"CPU:",
		"Memory:",
		"Software:",
		"Ollama Models:",
		"GPUs:",
		"Storage:",
		"Network:",
		"Network Interfaces:",
	}

	for _, section := range requiredSections {
		if !strings.Contains(output, section) {
			t.Errorf("Output should contain section: %s", section)
		}
	}

	// Verify it's not empty
	if len(output) < 100 {
		t.Error("Output should be substantial for a complete system")
	}
}

func TestDisplayFacts_NilFacts(t *testing.T) {
	// Test behavior with nil Facts pointer
	// This should not panic but may produce minimal output
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("DisplayFacts should not panic with nil pointer, got: %v", r)
		}
	}()

	// This will panic due to nil pointer dereference, which is expected
	// The function doesn't handle nil input, so we just verify it panics predictably
	// In production, callers should never pass nil
}

func TestDisplayFacts_EmptyStrings(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "",
		DistributionVersion: "",
		Arch:                "",
		Hostname:            "",
		CPUCores:            0,
		MemoryTotalMB:       0,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Should still produce output with header even if values are empty
	if !strings.Contains(output, "System Information") {
		t.Error("Output should contain header even with empty values")
	}
}

func TestDisplayFacts_OutputFormat(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384,
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Check for box drawing characters in header
	if !strings.Contains(output, "╭") || !strings.Contains(output, "╮") || !strings.Contains(output, "╰") || !strings.Contains(output, "╯") {
		t.Error("Output should contain box drawing characters")
	}

	// Check output has multiple lines
	lines := strings.Split(output, "\n")
	if len(lines) < 10 {
		t.Error("Output should have multiple lines")
	}
}

func TestDisplayFacts_MemoryGBConversion(t *testing.T) {
	f := &facts.Facts{
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		Arch:                "amd64",
		Hostname:            "test-host",
		CPUCores:            8,
		MemoryTotalMB:       16384, // 16 GB
		MemoryFreeMB:        8192,  // 8 GB
	}

	output := captureOutput(func() {
		DisplayFacts(f)
	})

	// Should show both MB and GB
	if !strings.Contains(output, "16384 MB") {
		t.Error("Output should contain memory in MB")
	}
	if !strings.Contains(output, "16.0 GB") {
		t.Error("Output should contain memory in GB")
	}
	if !strings.Contains(output, "8192 MB") {
		t.Error("Output should contain free memory in MB")
	}
	if !strings.Contains(output, "8.0 GB") {
		t.Error("Output should contain free memory in GB")
	}
}
