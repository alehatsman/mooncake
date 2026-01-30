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
