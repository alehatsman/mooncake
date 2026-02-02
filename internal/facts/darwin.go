// Package facts provides system information collection for different operating systems.
package facts

import (
	"os/exec"
	"strconv"
	"strings"
)

// collectDarwinFacts gathers macOS-specific system information
func collectDarwinFacts(f *Facts) {
	f.Distribution = "macos"
	f.DistributionVersion = detectMacOSVersion()
	f.DistributionMajor = extractMajorVersion(f.DistributionVersion)
	f.PackageManager = detectMacOSPackageManager()
	f.MemoryTotalMB = detectMacOSMemory()
	f.Disks = detectMacOSDisks()
	f.GPUs = detectMacOSGPUs()

	// Extended facts
	f.KernelVersion = detectDarwinKernel()
	f.CPUModel = detectDarwinCPUModel()
	f.CPUFlags = detectDarwinCPUFlags()
	f.MemoryFreeMB = detectDarwinMemoryFree()
	f.SwapTotalMB, f.SwapFreeMB = detectDarwinSwap()
	f.DefaultGateway = detectDarwinDefaultRoute()
	f.DNSServers = detectDarwinDNS()
}

// detectMacOSVersion gets macOS version from sw_vers
func detectMacOSVersion() string {
	out, err := exec.Command("sw_vers", "-productVersion").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectMacOSPackageManager checks if brew is installed
func detectMacOSPackageManager() string {
	if _, err := exec.LookPath("brew"); err == nil {
		return "brew"
	}
	if _, err := exec.LookPath("port"); err == nil {
		return "port"
	}
	return ""
}

// detectMacOSMemory gets total memory using sysctl
func detectMacOSMemory() int64 {
	out, err := exec.Command("sysctl", "-n", "hw.memsize").Output()
	if err != nil {
		return 0
	}

	bytes, err := strconv.ParseInt(strings.TrimSpace(string(out)), 10, 64)
	if err != nil {
		return 0
	}

	return bytes / 1024 / 1024 // Convert bytes to MB
}

// detectMacOSDisks uses df to get mounted filesystems
func detectMacOSDisks() []Disk {
	var disks []Disk

	out, err := exec.Command("df", "-g").Output()
	if err != nil {
		return disks
	}

	lines := strings.Split(string(out), "\n")
	// Skip header line
	for i, line := range lines {
		if i == 0 || line == "" {
			continue
		}

		fields := strings.Fields(line)
		if len(fields) < 9 {
			continue
		}

		// Skip special filesystems
		device := fields[0]
		mountPoint := fields[8]

		if strings.HasPrefix(device, "map") || strings.HasPrefix(device, "devfs") {
			continue
		}

		// Only show main user-relevant mounts
		// Skip system volumes except root and Data
		if strings.HasPrefix(mountPoint, "/System") &&
		   mountPoint != "/System/Volumes/Data" {
			continue
		}

		// Skip tiny volumes (< 1GB)
		sizeGB, _ := strconv.ParseInt(fields[1], 10, 64)
		if sizeGB < 1 {
			continue
		}

		usedGB, _ := strconv.ParseInt(fields[2], 10, 64)
		availGB, _ := strconv.ParseInt(fields[3], 10, 64)
		usedPct, _ := strconv.Atoi(strings.TrimSuffix(fields[4], "%"))

		disk := Disk{
			Device:     device,
			MountPoint: mountPoint,
			Filesystem: "apfs", // Most macOS disks are APFS
			SizeGB:     sizeGB,
			UsedGB:     usedGB,
			AvailGB:    availGB,
			UsedPct:    usedPct,
		}

		disks = append(disks, disk)
	}

	return disks
}

// detectMacOSGPUs uses system_profiler to detect GPUs
func detectMacOSGPUs() []GPU {
	var gpus []GPU

	out, err := exec.Command("system_profiler", "SPDisplaysDataType").Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(string(out), "\n")
	var currentGPU *GPU

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// New GPU entry - look for chipset model line
		if strings.Contains(trimmed, "Chipset Model:") {
			if currentGPU != nil && currentGPU.Model != "" {
				gpus = append(gpus, *currentGPU)
			}
			parts := strings.Split(trimmed, ":")
			if len(parts) >= 2 {
				currentGPU = &GPU{
					Model: strings.TrimSpace(parts[1]),
				}
			}
		}

		if currentGPU == nil {
			continue
		}

		// Extract vendor
		lower := strings.ToLower(trimmed)
		if strings.Contains(lower, "vendor:") {
			switch {
			case strings.Contains(lower, "nvidia"):
				currentGPU.Vendor = "nvidia"
			case strings.Contains(lower, "amd") || strings.Contains(lower, "ati"):
				currentGPU.Vendor = "amd"
			case strings.Contains(lower, "intel"):
				currentGPU.Vendor = "intel"
			case strings.Contains(lower, "apple"):
				currentGPU.Vendor = "apple"
			}
		}

		// Extract VRAM
		if strings.Contains(lower, "vram") {
			parts := strings.Split(trimmed, ":")
			if len(parts) >= 2 {
				currentGPU.Memory = strings.TrimSpace(parts[1])
			}
		}
	}

	// Add last GPU
	if currentGPU != nil && currentGPU.Model != "" {
		gpus = append(gpus, *currentGPU)
	}

	return gpus
}

// detectDarwinKernel gets kernel version
func detectDarwinKernel() string {
	out, err := exec.Command("uname", "-r").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectDarwinCPUModel gets CPU model from sysctl
func detectDarwinCPUModel() string {
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// detectDarwinCPUFlags gets CPU features/flags
func detectDarwinCPUFlags() []string {
	// Try to get CPU features
	out, err := exec.Command("sysctl", "-n", "machdep.cpu.features").Output()
	if err != nil {
		// On Apple Silicon, features might not be available
		return nil
	}

	features := strings.TrimSpace(string(out))
	if features == "" {
		return nil
	}

	// Convert to lowercase and split
	flags := strings.Fields(strings.ToLower(features))
	return flags
}

// detectDarwinMemoryFree gets available memory using vm_stat
func detectDarwinMemoryFree() int64 {
	out, err := exec.Command("vm_stat").Output()
	if err != nil {
		return 0
	}

	// Parse vm_stat output
	// Format: "Pages free:                  12345."
	lines := strings.Split(string(out), "\n")
	var pagesFree int64
	var pageSize int64 = 4096 // Default page size

	for _, line := range lines {
		if strings.Contains(line, "page size of") {
			// Extract page size from first line
			fields := strings.Fields(line)
			for i, field := range fields {
				if field == "of" && i+1 < len(fields) {
					size, err := strconv.ParseInt(strings.TrimSpace(fields[i+1]), 10, 64)
					if err == nil {
						pageSize = size
					}
					break
				}
			}
		} else if strings.HasPrefix(line, "Pages free:") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				numStr := strings.TrimSpace(parts[1])
				numStr = strings.TrimSuffix(numStr, ".")
				num, err := strconv.ParseInt(numStr, 10, 64)
				if err == nil {
					pagesFree = num
				}
			}
		}
	}

	if pagesFree > 0 {
		return (pagesFree * pageSize) / 1024 / 1024 // Convert to MB
	}

	return 0
}

// detectDarwinSwap gets swap usage using sysctl
func detectDarwinSwap() (swapTotal, swapFree int64) {
	out, err := exec.Command("sysctl", "vm.swapusage").Output()
	if err != nil {
		return 0, 0
	}

	// Format: "vm.swapusage: total = 2048.00M  used = 512.00M  free = 1536.00M  (encrypted)"
	line := string(out)
	parts := strings.Split(line, " ")

	for i, part := range parts {
		if part == "total" && i+2 < len(parts) {
			totalStr := strings.TrimSuffix(parts[i+2], "M")
			total, err := strconv.ParseFloat(totalStr, 64)
			if err == nil {
				swapTotal = int64(total)
			}
		} else if part == "free" && i+2 < len(parts) {
			freeStr := strings.TrimSuffix(parts[i+2], "M")
			free, err := strconv.ParseFloat(freeStr, 64)
			if err == nil {
				swapFree = int64(free)
			}
		}
	}

	return swapTotal, swapFree
}

// detectDarwinDefaultRoute gets default gateway
func detectDarwinDefaultRoute() string {
	out, err := exec.Command("route", "-n", "get", "default").Output()
	if err != nil {
		return ""
	}

	// Format: "   route to: default\ndestination: default\n       mask: default\n    gateway: 192.168.1.1\n..."
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "gateway:") {
			parts := strings.Fields(line)
			if len(parts) >= 2 {
				return parts[1]
			}
		}
	}

	return ""
}

// detectDarwinDNS gets DNS servers using scutil
func detectDarwinDNS() []string {
	var servers []string

	out, err := exec.Command("scutil", "--dns").Output()
	if err != nil {
		return servers
	}

	// Parse scutil --dns output
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "nameserver[") {
			parts := strings.Split(line, ":")
			if len(parts) >= 2 {
				server := strings.TrimSpace(parts[1])
				// Avoid duplicates
				duplicate := false
				for _, existing := range servers {
					if existing == server {
						duplicate = true
						break
					}
				}
				if !duplicate {
					servers = append(servers, server)
				}
			}
		}
	}

	return servers
}
