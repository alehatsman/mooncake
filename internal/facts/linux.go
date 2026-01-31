package facts

import (
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// collectLinuxFacts gathers Linux-specific system information
func collectLinuxFacts(f *Facts) {
	f.Distribution, f.DistributionVersion = detectLinuxDistribution()
	f.DistributionMajor = extractMajorVersion(f.DistributionVersion)
	f.PackageManager = detectLinuxPackageManager(f.Distribution)
	f.MemoryTotalMB = detectLinuxMemory()
	f.Disks = detectLinuxDisks()
	f.GPUs = detectLinuxGPUs()
}

// detectLinuxDistribution reads /etc/os-release to identify distribution
func detectLinuxDistribution() (distro, version string) {
	// Try /etc/os-release (standard)
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		// Fallback to /etc/lsb-release
		data, err = os.ReadFile("/etc/lsb-release")
		if err != nil {
			return "", ""
		}
	}

	lines := strings.Split(string(data), "\n")
	var id, versionID string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id = strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
		} else if strings.HasPrefix(line, "VERSION_ID=") {
			versionID = strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}

	return id, versionID
}

// detectLinuxPackageManager determines the package manager based on distribution
func detectLinuxPackageManager(distro string) string {
	switch distro {
	case "ubuntu", "debian", "linuxmint":
		return "apt"
	case "centos", "rhel":
		// Check if dnf is available (CentOS 8+)
		if _, err := exec.LookPath("dnf"); err == nil {
			return "dnf"
		}
		return "yum"
	case "fedora":
		return "dnf"
	case "arch", "manjaro":
		return "pacman"
	case "opensuse", "sles":
		return "zypper"
	case "alpine":
		return "apk"
	default:
		// Try to detect by command availability
		for _, pm := range []string{"apt", "dnf", "yum", "pacman", "zypper", "apk"} {
			if _, err := exec.LookPath(pm); err == nil {
				return pm
			}
		}
		return ""
	}
}

// detectLinuxMemory reads total memory from /proc/meminfo
func detectLinuxMemory() int64 {
	data, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "MemTotal:") {
			// Format: "MemTotal:       16384000 kB"
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				kb, err := strconv.ParseInt(fields[1], 10, 64)
				if err == nil {
					return kb / 1024 // Convert KB to MB
				}
			}
			break
		}
	}

	return 0
}

// extractMajorVersion extracts major version number from version string
func extractMajorVersion(version string) string {
	if version == "" {
		return ""
	}

	// Split on "." and take first part
	parts := strings.Split(version, ".")
	if len(parts) > 0 {
		return parts[0]
	}

	return version
}

// detectLinuxDisks uses df to get mounted filesystems
func detectLinuxDisks() []Disk {
	var disks []Disk

	out, err := exec.Command("df", "-BG", "--output=source,target,fstype,size,used,avail,pcent").Output()
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
		if len(fields) < 7 {
			continue
		}

		// Skip tmpfs, devtmpfs, and other virtual filesystems
		device := fields[0]
		if strings.HasPrefix(device, "tmpfs") || strings.HasPrefix(device, "devtmpfs") ||
			strings.HasPrefix(device, "udev") || strings.HasPrefix(device, "overlay") {
			continue
		}

		disk := Disk{
			Device:     device,
			MountPoint: fields[1],
			Filesystem: fields[2],
			SizeGB:     parseSize(fields[3]),
			UsedGB:     parseSize(fields[4]),
			AvailGB:    parseSize(fields[5]),
			UsedPct:    parsePercent(fields[6]),
		}

		disks = append(disks, disk)
	}

	return disks
}

// detectLinuxGPUs detects NVIDIA and AMD GPUs
func detectLinuxGPUs() []GPU {
	var gpus []GPU

	// Try NVIDIA first
	if nvidiaSmi, err := exec.LookPath("nvidia-smi"); err == nil {
		// #nosec G204 -- nvidia-smi path is validated via exec.LookPath and used for system GPU detection
		out, err := exec.Command(nvidiaSmi, "--query-gpu=name,memory.total,driver_version", "--format=csv,noheader").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				parts := strings.Split(line, ",")
				if len(parts) >= 3 {
					gpu := GPU{
						Vendor: "nvidia",
						Model:  strings.TrimSpace(parts[0]),
						Memory: strings.TrimSpace(parts[1]),
						Driver: strings.TrimSpace(parts[2]),
					}
					gpus = append(gpus, gpu)
				}
			}
		}
	}

	// Try AMD
	if rocmSmi, err := exec.LookPath("rocm-smi"); err == nil {
		// #nosec G204 -- rocm-smi path is validated via exec.LookPath and used for system GPU detection
		out, err := exec.Command(rocmSmi, "--showproductname").Output()
		if err == nil {
			lines := strings.Split(strings.TrimSpace(string(out)), "\n")
			for _, line := range lines {
				if strings.Contains(line, "GPU") && strings.Contains(line, ":") {
					parts := strings.Split(line, ":")
					if len(parts) >= 2 {
						gpu := GPU{
							Vendor: "amd",
							Model:  strings.TrimSpace(parts[1]),
						}
						gpus = append(gpus, gpu)
					}
				}
			}
		}
	}

	// Fallback: Try lspci for basic GPU detection
	if len(gpus) == 0 {
		if lspci, err := exec.LookPath("lspci"); err == nil {
			// #nosec G204 -- lspci path is validated via exec.LookPath and used for system hardware detection
			out, err := exec.Command(lspci).Output()
			if err == nil {
				lines := strings.Split(string(out), "\n")
				for _, line := range lines {
					lower := strings.ToLower(line)
					if strings.Contains(lower, "vga") || strings.Contains(lower, "3d controller") {
						var vendor string
						switch {
						case strings.Contains(lower, "nvidia"):
							vendor = "nvidia"
						case strings.Contains(lower, "amd") || strings.Contains(lower, "ati"):
							vendor = "amd"
						case strings.Contains(lower, "intel"):
							vendor = "intel"
						}

						if vendor != "" {
							// Extract model name (everything after ":")
							parts := strings.SplitN(line, ":", 3)
							model := ""
							if len(parts) >= 3 {
								model = strings.TrimSpace(parts[2])
							}

							gpu := GPU{
								Vendor: vendor,
								Model:  model,
							}
							gpus = append(gpus, gpu)
						}
					}
				}
			}
		}
	}

	return gpus
}

// parseSize converts size string like "100G" to int64
func parseSize(s string) int64 {
	s = strings.TrimSuffix(s, "G")
	val, _ := strconv.ParseInt(s, 10, 64)
	return val
}

// parsePercent converts percent string like "75%" to int
func parsePercent(s string) int {
	s = strings.TrimSuffix(s, "%")
	val, _ := strconv.Atoi(s)
	return val
}
