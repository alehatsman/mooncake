package facts

import (
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// Facts contains collected system information
type Facts struct {
	// Basic
	OS       string
	Arch     string
	Hostname string
	UserHome string

	// Distribution (Linux)
	Distribution        string
	DistributionVersion string
	DistributionMajor   string

	// Network
	IPAddresses []string

	// Hardware
	CPUCores      int
	MemoryTotalMB int64

	// Software
	PythonVersion  string
	PackageManager string
}

// Collect gathers all system facts
func Collect() *Facts {
	f := &Facts{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// Basic facts (cross-platform)
	f.Hostname, _ = os.Hostname()
	f.UserHome, _ = os.UserHomeDir()
	f.CPUCores = runtime.NumCPU()
	f.IPAddresses = collectIPAddresses()
	f.PythonVersion = detectPythonVersion()

	// Platform-specific facts
	switch runtime.GOOS {
	case "linux":
		collectLinuxFacts(f)
	case "darwin":
		collectDarwinFacts(f)
	case "windows":
		collectWindowsFacts(f)
	}

	return f
}

// ToMap converts Facts to a map for use in templates
func (f *Facts) ToMap() map[string]interface{} {
	return map[string]interface{}{
		"os":                   f.OS,
		"arch":                 f.Arch,
		"hostname":             f.Hostname,
		"user_home":            f.UserHome,
		"distribution":         f.Distribution,
		"distribution_version": f.DistributionVersion,
		"distribution_major":   f.DistributionMajor,
		"ip_addresses":         f.IPAddresses,                        // Array for iteration
		"ip_addresses_string":  strings.Join(f.IPAddresses, ", "),   // String for display
		"cpu_cores":            f.CPUCores,
		"memory_total_mb":      f.MemoryTotalMB,
		"python_version":       f.PythonVersion,
		"package_manager":      f.PackageManager,
	}
}

// collectIPAddresses gathers all non-loopback IP addresses
func collectIPAddresses() []string {
	var ips []string

	ifaces, err := net.Interfaces()
	if err != nil {
		return ips
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagLoopback != 0 || iface.Flags&net.FlagUp == 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			if ip == nil || ip.IsLoopback() {
				continue
			}

			ips = append(ips, ip.String())
		}
	}

	return ips
}

// detectPythonVersion attempts to detect Python version
func detectPythonVersion() string {
	// Try python3 first
	for _, cmd := range []string{"python3", "python"} {
		out, err := exec.Command(cmd, "--version").CombinedOutput()
		if err == nil {
			// Parse "Python 3.11.5" -> "3.11.5"
			version := strings.TrimSpace(string(out))
			version = strings.TrimPrefix(version, "Python ")
			return version
		}
	}
	return ""
}
