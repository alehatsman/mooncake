package facts

import (
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

// collectWindowsFacts gathers Windows-specific system information
func collectWindowsFacts(f *Facts) {
	f.Distribution = osWindows
	f.Disks = detectWindowsDisks()
	f.GPUs = detectWindowsGPUs()

	// Detect package manager
	if _, err := exec.LookPath("choco"); err == nil {
		f.PackageManager = "choco"
	} else if _, err := exec.LookPath("winget"); err == nil {
		f.PackageManager = "winget"
	} else if _, err := exec.LookPath("scoop"); err == nil {
		f.PackageManager = "scoop"
	}
}

// detectWindowsDisks uses wmic to get disk information
func detectWindowsDisks() []Disk {
	disks := []Disk{}

	// Use wmic to get disk info
	// Format: Caption,FileSystem,FreeSpace,Size
	cmd := exec.Command("wmic", "logicaldisk", "get", "caption,filesystem,freespace,size", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return disks
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node") {
			continue
		}

		// CSV format: Node,Caption,FileSystem,FreeSpace,Size
		fields := strings.Split(line, ",")
		if len(fields) < 5 {
			continue
		}

		caption := strings.TrimSpace(fields[1])
		filesystem := strings.TrimSpace(fields[2])
		freeSpaceStr := strings.TrimSpace(fields[3])
		sizeStr := strings.TrimSpace(fields[4])

		if caption == "" || filesystem == "" {
			continue
		}

		// Parse sizes (in bytes)
		freeSpace, _ := strconv.ParseUint(freeSpaceStr, 10, 64)
		totalSize, _ := strconv.ParseUint(sizeStr, 10, 64)

		if totalSize == 0 {
			continue
		}

		usedSpace := totalSize - freeSpace
		// #nosec G115 -- disk sizes are validated and won't overflow in practice
		sizeGB := int64(totalSize / (1024 * 1024 * 1024))
		// #nosec G115 -- disk sizes are validated and won't overflow in practice
		usedGB := int64(usedSpace / (1024 * 1024 * 1024))
		// #nosec G115 -- disk sizes are validated and won't overflow in practice
		availGB := int64(freeSpace / (1024 * 1024 * 1024))

		// Calculate used percentage
		usedPct := 0
		if sizeGB > 0 {
			usedPct = int((usedGB * 100) / sizeGB)
		}

		disk := Disk{
			Device:     caption,
			MountPoint: caption,
			Filesystem: filesystem,
			SizeGB:     sizeGB,
			UsedGB:     usedGB,
			AvailGB:    availGB,
			UsedPct:    usedPct,
		}
		disks = append(disks, disk)
	}

	return disks
}

// detectWindowsGPUs uses wmic to get GPU information
func detectWindowsGPUs() []GPU {
	gpus := []GPU{}

	// Use wmic to get GPU info
	// Format: Name,DriverVersion,AdapterRAM
	cmd := exec.Command("wmic", "path", "win32_VideoController", "get", "name,driverversion,adapterram", "/format:csv")
	output, err := cmd.Output()
	if err != nil {
		return gpus
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Node") {
			continue
		}

		// CSV format: Node,AdapterRAM,DriverVersion,Name
		fields := strings.Split(line, ",")
		if len(fields) < 4 {
			continue
		}

		ramStr := strings.TrimSpace(fields[1])
		driverVersion := strings.TrimSpace(fields[2])
		name := strings.TrimSpace(fields[3])

		if name == "" {
			continue
		}

		// Parse RAM (in bytes, convert to GB)
		ram, _ := strconv.ParseUint(ramStr, 10, 64)
		memoryGB := ram / (1024 * 1024 * 1024)
		memoryStr := fmt.Sprintf("%dGB", memoryGB)
		if memoryGB == 0 && ram > 0 {
			// If less than 1GB, show in MB
			memoryMB := ram / (1024 * 1024)
			memoryStr = fmt.Sprintf("%dMB", memoryMB)
		}

		// Detect vendor from name
		vendor := "unknown"
		nameLower := strings.ToLower(name)
		if strings.Contains(nameLower, "nvidia") || strings.Contains(nameLower, "geforce") || strings.Contains(nameLower, "quadro") {
			vendor = "nvidia"
		} else if strings.Contains(nameLower, "amd") || strings.Contains(nameLower, "radeon") {
			vendor = "amd"
		} else if strings.Contains(nameLower, "intel") {
			vendor = "intel"
		}

		gpu := GPU{
			Vendor: vendor,
			Model:  name,
			Memory: memoryStr,
			Driver: driverVersion,
		}

		gpus = append(gpus, gpu)
	}

	return gpus
}
