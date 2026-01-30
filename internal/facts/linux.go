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
