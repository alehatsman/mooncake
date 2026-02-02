package facts

import (
	"runtime"
	"strings"
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

func TestToMap_AllFields(t *testing.T) {
	facts := &Facts{
		OS:                  "linux",
		Arch:                "amd64",
		Hostname:            "testhost",
		Username:            "testuser",
		UserHome:            "/home/testuser",
		Distribution:        "Ubuntu",
		DistributionVersion: "22.04",
		DistributionMajor:   "22",
		IPAddresses:         []string{"192.168.1.1"},
		CPUCores:            4,
		MemoryTotalMB:       8192,
		PythonVersion:       "3.10.0",
		PackageManager:      "apt",
	}

	m := facts.ToMap()

	// Verify all fields are present
	requiredFields := []string{
		"os", "arch", "hostname", "username", "user_home",
		"distribution", "distribution_version", "distribution_major",
		"ip_addresses", "ip_addresses_string", "cpu_cores",
		"memory_total_mb", "python_version", "package_manager",
	}

	for _, field := range requiredFields {
		if _, ok := m[field]; !ok {
			t.Errorf("ToMap() missing field: %s", field)
		}
	}

	// Verify username is included
	if m["username"] != "testuser" {
		t.Errorf("username = %v, want testuser", m["username"])
	}
}

func TestToMap_EmptyIPAddresses(t *testing.T) {
	facts := &Facts{
		OS:          "linux",
		IPAddresses: []string{},
	}

	m := facts.ToMap()

	// Should have empty array and empty string
	ips := m["ip_addresses"].([]string)
	if len(ips) != 0 {
		t.Errorf("ip_addresses length = %d, want 0", len(ips))
	}

	if m["ip_addresses_string"] != "" {
		t.Errorf("ip_addresses_string = %q, want empty string", m["ip_addresses_string"])
	}
}

func TestToMap_SingleIPAddress(t *testing.T) {
	facts := &Facts{
		OS:          "linux",
		IPAddresses: []string{"10.0.0.1"},
	}

	m := facts.ToMap()

	if m["ip_addresses_string"] != "10.0.0.1" {
		t.Errorf("ip_addresses_string = %q, want '10.0.0.1'", m["ip_addresses_string"])
	}
}

func TestCollect_BasicFields(t *testing.T) {
	facts := Collect()

	// OS should match runtime.GOOS
	if facts.OS != runtime.GOOS {
		t.Errorf("OS = %s, want %s", facts.OS, runtime.GOOS)
	}

	// Arch should match runtime.GOARCH
	if facts.Arch != runtime.GOARCH {
		t.Errorf("Arch = %s, want %s", facts.Arch, runtime.GOARCH)
	}

	// CPUCores should be positive
	if facts.CPUCores <= 0 {
		t.Errorf("CPUCores = %d, should be positive", facts.CPUCores)
	}

	// UserHome should typically not be empty
	if facts.UserHome == "" {
		t.Log("Warning: UserHome is empty")
	}

	// IPAddresses should not be nil
	if facts.IPAddresses == nil {
		t.Error("IPAddresses should not be nil")
	}

	// NetworkInterfaces should not be nil
	if facts.NetworkInterfaces == nil {
		t.Error("NetworkInterfaces should not be nil")
	}
}

func TestCollectNetworkInterfaces(t *testing.T) {
	interfaces := collectNetworkInterfaces()

	// Should not be nil
	if interfaces == nil {
		t.Fatal("collectNetworkInterfaces should not return nil")
	}

	// Check structure of interfaces if any exist
	for _, iface := range interfaces {
		// Name should not be empty
		if iface.Name == "" {
			t.Error("Interface name should not be empty")
		}

		// MTU should be positive
		if iface.MTU <= 0 {
			t.Errorf("Interface %s MTU = %d, should be positive", iface.Name, iface.MTU)
		}

		// MACAddress can be empty but should be a valid format if present
		if iface.MACAddress != "" {
			t.Logf("Interface %s MAC: %s", iface.Name, iface.MACAddress)
		}

		// Addresses can be nil or empty - just check it's not unexpected
		if iface.Addresses != nil && len(iface.Addresses) > 0 {
			t.Logf("Interface %s has %d addresses", iface.Name, len(iface.Addresses))
		}
	}
}

func TestCollectIPAddresses_NoLoopback(t *testing.T) {
	ips := collectIPAddresses()

	// Should not contain loopback addresses
	for _, ip := range ips {
		if ip == "127.0.0.1" || ip == "::1" {
			t.Errorf("Should not include loopback address: %s", ip)
		}

		// Should not be empty
		if ip == "" {
			t.Error("IP address should not be empty")
		}
	}
}

func TestCollectIPAddresses_ReturnsSlice(t *testing.T) {
	ips := collectIPAddresses()

	// Should return a slice (not nil)
	if ips == nil {
		t.Fatal("collectIPAddresses should not return nil")
	}
}

func TestDetectPythonVersion_ReturnsString(t *testing.T) {
	version := detectPythonVersion()

	// Should return a string (might be empty if Python not installed)
	if version == "" {
		t.Log("Python not detected (this is ok)")
	} else {
		// If detected, should not contain "Python " prefix
		if strings.HasPrefix(version, "Python ") {
			t.Errorf("Version should not have 'Python ' prefix, got: %s", version)
		}

		// Should look like a version number
		t.Logf("Detected Python version: %s", version)
	}
}

func TestNetworkInterface_Structure(t *testing.T) {
	// Test that NetworkInterface can be created and has all fields
	iface := NetworkInterface{
		Name:       "eth0",
		MACAddress: "00:11:22:33:44:55",
		MTU:        1500,
		Addresses:  []string{"192.168.1.1/24"},
		Up:         true,
	}

	if iface.Name != "eth0" {
		t.Errorf("Name = %s, want eth0", iface.Name)
	}
	if iface.MACAddress != "00:11:22:33:44:55" {
		t.Errorf("MACAddress = %s, want 00:11:22:33:44:55", iface.MACAddress)
	}
	if iface.MTU != 1500 {
		t.Errorf("MTU = %d, want 1500", iface.MTU)
	}
	if len(iface.Addresses) != 1 {
		t.Errorf("Addresses length = %d, want 1", len(iface.Addresses))
	}
	if !iface.Up {
		t.Error("Up should be true")
	}
}

func TestDisk_Structure(t *testing.T) {
	// Test that Disk can be created and has all fields
	disk := Disk{
		Device:     "/dev/sda1",
		MountPoint: "/",
		Filesystem: "ext4",
		SizeGB:     100,
		UsedGB:     50,
		AvailGB:    50,
		UsedPct:    50,
	}

	if disk.Device != "/dev/sda1" {
		t.Errorf("Device = %s, want /dev/sda1", disk.Device)
	}
	if disk.MountPoint != "/" {
		t.Errorf("MountPoint = %s, want /", disk.MountPoint)
	}
	if disk.Filesystem != "ext4" {
		t.Errorf("Filesystem = %s, want ext4", disk.Filesystem)
	}
	if disk.SizeGB != 100 {
		t.Errorf("SizeGB = %d, want 100", disk.SizeGB)
	}
	if disk.UsedGB != 50 {
		t.Errorf("UsedGB = %d, want 50", disk.UsedGB)
	}
	if disk.AvailGB != 50 {
		t.Errorf("AvailGB = %d, want 50", disk.AvailGB)
	}
	if disk.UsedPct != 50 {
		t.Errorf("UsedPct = %d, want 50", disk.UsedPct)
	}
}

func TestGPU_Structure(t *testing.T) {
	// Test that GPU can be created and has all fields
	gpu := GPU{
		Vendor: "nvidia",
		Model:  "RTX 4090",
		Memory: "24GB",
		Driver: "535.54.03",
	}

	if gpu.Vendor != "nvidia" {
		t.Errorf("Vendor = %s, want nvidia", gpu.Vendor)
	}
	if gpu.Model != "RTX 4090" {
		t.Errorf("Model = %s, want RTX 4090", gpu.Model)
	}
	if gpu.Memory != "24GB" {
		t.Errorf("Memory = %s, want 24GB", gpu.Memory)
	}
	if gpu.Driver != "535.54.03" {
		t.Errorf("Driver = %s, want 535.54.03", gpu.Driver)
	}
}

func TestFacts_DefaultValues(t *testing.T) {
	// Test that a new Facts struct has sensible defaults
	facts := &Facts{}

	if facts.IPAddresses == nil {
		// This is ok, zero value
	}
	if facts.NetworkInterfaces == nil {
		// This is ok, zero value
	}
	if facts.Disks == nil {
		// This is ok, zero value
	}
	if facts.GPUs == nil {
		// This is ok, zero value
	}

	// String fields should be empty
	if facts.OS != "" {
		t.Errorf("Default OS should be empty, got %s", facts.OS)
	}
}

func TestCollect_Username(t *testing.T) {
	facts := Collect()

	// Username might be set if user.Current() succeeds
	t.Logf("Username: %s", facts.Username)

	// On most systems, username should be set
	// But we won't fail if it's empty as it might be in some CI environments
	if facts.Username == "" {
		t.Log("Warning: Username is empty (might be expected in some environments)")
	}
}

func TestCollect_Hostname(t *testing.T) {
	facts := Collect()

	// Hostname is usually available
	if facts.Hostname == "" {
		t.Log("Warning: Hostname is empty")
	} else {
		t.Logf("Hostname: %s", facts.Hostname)
	}
}

func TestDarwinFacts_Integration(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("Skipping Darwin-specific test on non-Darwin platform")
	}

	facts := Collect()

	// On macOS, memory should typically be detected
	if facts.MemoryTotalMB <= 0 {
		t.Error("MemoryTotalMB should be positive on macOS")
	}

	// Package manager might be detected
	t.Logf("Package manager: %s", facts.PackageManager)

	// Disks should typically be found
	if len(facts.Disks) == 0 {
		t.Log("Warning: No disks detected")
	} else {
		t.Logf("Found %d disks", len(facts.Disks))
		for _, disk := range facts.Disks {
			t.Logf("Disk: %s mounted at %s (%dGB)", disk.Device, disk.MountPoint, disk.SizeGB)
		}
	}
}

func TestCollectNetworkInterfaces_Integration(t *testing.T) {
	interfaces := collectNetworkInterfaces()

	// Log what we found for debugging
	t.Logf("Found %d network interfaces", len(interfaces))
	for _, iface := range interfaces {
		t.Logf("Interface: %s (MAC: %s, MTU: %d, Up: %v, Addrs: %d)",
			iface.Name, iface.MACAddress, iface.MTU, iface.Up, len(iface.Addresses))
	}

	// On most systems, there should be at least one non-loopback interface
	if len(interfaces) == 0 {
		t.Log("Warning: No network interfaces found (might be expected in some environments)")
	}
}

func TestToMap_NewFields(t *testing.T) {
	facts := &Facts{
		OS:                  "linux",
		Arch:                "amd64",
		KernelVersion:       "6.5.0-14-generic",
		CPUModel:            "Intel(R) Core(TM) i9-9900K",
		CPUFlags:            []string{"avx", "avx2", "sse4_2"},
		MemoryTotalMB:       16384,
		MemoryFreeMB:        8192,
		SwapTotalMB:         4096,
		SwapFreeMB:          2048,
		DefaultGateway:      "192.168.1.1",
		DNSServers:          []string{"8.8.8.8", "1.1.1.1"},
		DockerVersion:       "24.0.7",
		GitVersion:          "2.43.0",
		GoVersion:           "1.21.5",
		Disks:               []Disk{{Device: "/dev/sda1", MountPoint: "/"}},
		GPUs:                []GPU{{Vendor: "nvidia", Model: "RTX 4090"}},
		NetworkInterfaces:   []NetworkInterface{{Name: "eth0", Up: true}},
	}

	m := facts.ToMap()

	// Check new OS fields
	if m["kernel_version"] != "6.5.0-14-generic" {
		t.Errorf("kernel_version = %v, want 6.5.0-14-generic", m["kernel_version"])
	}

	// Check CPU extended fields
	if m["cpu_model"] != "Intel(R) Core(TM) i9-9900K" {
		t.Errorf("cpu_model = %v", m["cpu_model"])
	}
	cpuFlags := m["cpu_flags"].([]string)
	if len(cpuFlags) != 3 {
		t.Errorf("cpu_flags length = %d, want 3", len(cpuFlags))
	}
	if m["cpu_flags_string"] != "avx avx2 sse4_2" {
		t.Errorf("cpu_flags_string = %v", m["cpu_flags_string"])
	}

	// Check memory extended fields
	if m["memory_free_mb"] != int64(8192) {
		t.Errorf("memory_free_mb = %v, want 8192", m["memory_free_mb"])
	}
	if m["swap_total_mb"] != int64(4096) {
		t.Errorf("swap_total_mb = %v, want 4096", m["swap_total_mb"])
	}
	if m["swap_free_mb"] != int64(2048) {
		t.Errorf("swap_free_mb = %v, want 2048", m["swap_free_mb"])
	}

	// Check network extended fields
	if m["default_gateway"] != "192.168.1.1" {
		t.Errorf("default_gateway = %v, want 192.168.1.1", m["default_gateway"])
	}
	dnsServers := m["dns_servers"].([]string)
	if len(dnsServers) != 2 {
		t.Errorf("dns_servers length = %d, want 2", len(dnsServers))
	}
	if m["dns_servers_string"] != "8.8.8.8, 1.1.1.1" {
		t.Errorf("dns_servers_string = %v", m["dns_servers_string"])
	}

	// Check toolchain versions
	if m["docker_version"] != "24.0.7" {
		t.Errorf("docker_version = %v, want 24.0.7", m["docker_version"])
	}
	if m["git_version"] != "2.43.0" {
		t.Errorf("git_version = %v, want 2.43.0", m["git_version"])
	}
	if m["go_version"] != "1.21.5" {
		t.Errorf("go_version = %v, want 1.21.5", m["go_version"])
	}
}

func TestToMap_Arrays(t *testing.T) {
	facts := &Facts{
		Disks: []Disk{
			{Device: "/dev/sda1", MountPoint: "/", SizeGB: 100},
			{Device: "/dev/sdb1", MountPoint: "/data", SizeGB: 500},
		},
		GPUs: []GPU{
			{Vendor: "nvidia", Model: "RTX 4090", CUDAVersion: "12.3"},
		},
		NetworkInterfaces: []NetworkInterface{
			{Name: "eth0", Up: true},
			{Name: "eth1", Up: false},
		},
	}

	m := facts.ToMap()

	// CRITICAL: Verify arrays are exposed for template iteration
	disks, ok := m["disks"].([]Disk)
	if !ok {
		t.Fatal("disks should be []Disk type")
	}
	if len(disks) != 2 {
		t.Errorf("disks length = %d, want 2", len(disks))
	}

	gpus, ok := m["gpus"].([]GPU)
	if !ok {
		t.Fatal("gpus should be []GPU type")
	}
	if len(gpus) != 1 {
		t.Errorf("gpus length = %d, want 1", len(gpus))
	}
	if gpus[0].CUDAVersion != "12.3" {
		t.Errorf("GPU CUDAVersion = %s, want 12.3", gpus[0].CUDAVersion)
	}

	ifaces, ok := m["network_interfaces"].([]NetworkInterface)
	if !ok {
		t.Fatal("network_interfaces should be []NetworkInterface type")
	}
	if len(ifaces) != 2 {
		t.Errorf("network_interfaces length = %d, want 2", len(ifaces))
	}
}

func TestGPU_CUDAVersion(t *testing.T) {
	gpu := GPU{
		Vendor:      "nvidia",
		Model:       "RTX 4090",
		Memory:      "24GB",
		Driver:      "535.54.03",
		CUDAVersion: "12.3",
	}

	if gpu.CUDAVersion != "12.3" {
		t.Errorf("CUDAVersion = %s, want 12.3", gpu.CUDAVersion)
	}
}
