package facts

import (
	"testing"
)

// TestCollectUncached_ErrorHandling tests error handling in collectUncached
func TestCollectUncached_ErrorHandling(t *testing.T) {
	// Should not panic
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("collectUncached should not panic: %v", r)
		}
	}()

	facts := collectUncached()

	// Should populate at minimum
	if facts.OS == "" {
		t.Error("OS should be populated")
	}
	if facts.CPUCores == 0 {
		t.Error("CPUCores should be populated")
	}
}

// TestFacts_StringFields tests string field conversions in ToMap
func TestFacts_StringFields(t *testing.T) {
	facts := &Facts{
		OS:          "TestOS",
		CPUCores:    4,
		CPUFlags:    []string{"avx", "sse4_2", "fma"},
		IPAddresses: []string{"192.168.1.100", "192.168.1.101"},
		DNSServers:  []string{"8.8.8.8", "1.1.1.1"},
	}

	m := facts.ToMap()

	// Verify string representations exist
	cpuFlagsStr, ok := m["cpu_flags_string"].(string)
	if !ok {
		t.Error("cpu_flags_string should be a string")
	}
	if cpuFlagsStr == "" {
		t.Error("cpu_flags_string should not be empty")
	}

	ipAddressesStr, ok := m["ip_addresses_string"].(string)
	if !ok {
		t.Error("ip_addresses_string should be a string")
	}
	if ipAddressesStr == "" {
		t.Error("ip_addresses_string should not be empty")
	}

	dnsServersStr, ok := m["dns_servers_string"].(string)
	if !ok {
		t.Error("dns_servers_string should be a string")
	}
	if dnsServersStr == "" {
		t.Error("dns_servers_string should not be empty")
	}
}

// TestFacts_ArrayPreservation tests that arrays are preserved in ToMap
func TestFacts_ArrayPreservation(t *testing.T) {
	facts := &Facts{
		OS:       "TestOS",
		CPUCores: 1,
		Disks: []Disk{
			{MountPoint: "/", Filesystem: "ext4", SizeGB: 100},
			{MountPoint: "/home", Filesystem: "ext4", SizeGB: 200},
		},
		GPUs: []GPU{
			{Model: "GPU1", Vendor: "NVIDIA"},
			{Model: "GPU2", Vendor: "AMD"},
		},
		NetworkInterfaces: []NetworkInterface{
			{Name: "eth0", Up: true},
			{Name: "eth1", Up: false},
		},
	}

	m := facts.ToMap()

	// Verify array lengths are preserved
	disks, ok := m["disks"].([]Disk)
	if !ok {
		t.Fatal("disks should be []Disk")
	}
	if len(disks) != 2 {
		t.Errorf("Expected 2 disks, got %d", len(disks))
	}

	gpus, ok := m["gpus"].([]GPU)
	if !ok {
		t.Fatal("gpus should be []GPU")
	}
	if len(gpus) != 2 {
		t.Errorf("Expected 2 GPUs, got %d", len(gpus))
	}

	interfaces, ok := m["network_interfaces"].([]NetworkInterface)
	if !ok {
		t.Fatal("network_interfaces should be []NetworkInterface")
	}
	if len(interfaces) != 2 {
		t.Errorf("Expected 2 interfaces, got %d", len(interfaces))
	}
}
