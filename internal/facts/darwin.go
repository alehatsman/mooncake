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
			if strings.Contains(lower, "nvidia") {
				currentGPU.Vendor = "nvidia"
			} else if strings.Contains(lower, "amd") || strings.Contains(lower, "ati") {
				currentGPU.Vendor = "amd"
			} else if strings.Contains(lower, "intel") {
				currentGPU.Vendor = "intel"
			} else if strings.Contains(lower, "apple") {
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
