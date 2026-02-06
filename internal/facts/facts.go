package facts

import (
	"net"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

const (
	// OS constants
	osLinux   = "linux"
	osDarwin  = "darwin"
	osWindows = "windows"

	// Package manager constants
	pkgManagerDnf = "dnf"
)

// Facts contains collected system information.
type Facts struct {
	// Basic
	OS       string
	Arch     string
	Hostname string
	Username string
	UserHome string

	// Distribution (Linux)
	Distribution        string
	DistributionVersion string
	DistributionMajor   string

	// Network
	IPAddresses       []string
	NetworkInterfaces []NetworkInterface

	// Hardware
	CPUCores      int
	MemoryTotalMB int64
	Disks         []Disk
	GPUs          []GPU

	// OS Details
	KernelVersion string // "6.5.0-14-generic" (Linux), "23.6.0" (macOS)

	// CPU Extended
	CPUModel string   // "Intel(R) Core(TM) i9-9900K CPU @ 3.60GHz"
	CPUFlags []string // ["avx", "avx2", "sse4_2", "fma", ...]

	// Memory Extended
	MemoryFreeMB int64 // Available memory
	SwapTotalMB  int64 // Swap size
	SwapFreeMB   int64 // Swap available

	// Network Extended
	DefaultGateway string   // "192.168.1.1"
	DNSServers     []string // ["8.8.8.8", "1.1.1.1"]

	// Software
	PythonVersion  string
	PackageManager string

	// Toolchains
	DockerVersion string // "24.0.7"
	GitVersion    string // "2.43.0"
	GoVersion     string // "1.21.5"

	// Ollama (optional)
	OllamaVersion  string        // "0.1.47"
	OllamaModels   []OllamaModel // List of installed models
	OllamaEndpoint string        // "http://localhost:11434"
}

// NetworkInterface represents a network interface.
type NetworkInterface struct {
	Name       string
	MACAddress string
	MTU        int
	Addresses  []string
	Up         bool
}

// Disk represents a storage device.
type Disk struct {
	Device     string
	MountPoint string
	Filesystem string
	SizeGB     int64
	UsedGB     int64
	AvailGB    int64
	UsedPct    int
}

// GPU represents a graphics card.
type GPU struct {
	Vendor      string // nvidia, amd, intel
	Model       string
	Memory      string // e.g. "8GB", "24GB"
	Driver      string
	CUDAVersion string // "12.3" (NVIDIA only)
}

// OllamaModel represents an installed Ollama model.
type OllamaModel struct {
	Name       string // e.g., "llama3.1:8b"
	Size       string // e.g., "4.7 GB"
	Digest     string // SHA256 hash
	ModifiedAt string // ISO timestamp
}

// collectUncached gathers all system facts without caching.
// This is called internally by Collect() in cache.go.
func collectUncached() *Facts {
	f := &Facts{
		OS:   runtime.GOOS,
		Arch: runtime.GOARCH,
	}

	// Basic facts (cross-platform)
	f.Hostname, _ = os.Hostname()
	f.UserHome, _ = os.UserHomeDir()
	if currentUser, err := user.Current(); err == nil {
		f.Username = currentUser.Username
	}
	f.CPUCores = runtime.NumCPU()
	f.IPAddresses = collectIPAddresses()
	f.NetworkInterfaces = collectNetworkInterfaces()
	f.PythonVersion = detectPythonVersion()

	// Platform-specific facts
	switch runtime.GOOS {
	case osLinux:
		collectLinuxFacts(f)
	case osDarwin:
		collectDarwinFacts(f)
	case osWindows:
		collectWindowsFacts(f)
	}

	// Toolchains (cross-platform)
	f.DockerVersion, f.GitVersion, f.GoVersion = detectToolchains()

	// Ollama (cross-platform, optional)
	f.OllamaVersion = detectOllamaVersion()
	if f.OllamaVersion != "" {
		f.OllamaModels = detectOllamaModels()
		f.OllamaEndpoint = detectOllamaEndpoint()
	}

	return f
}

// ToMap converts Facts to a map for use in templates.
func (f *Facts) ToMap() map[string]interface{} {
	return map[string]interface{}{
		// Basic
		"os":                   f.OS,
		"arch":                 f.Arch,
		"hostname":             f.Hostname,
		"username":             f.Username,
		"user_home":            f.UserHome,
		"distribution":         f.Distribution,
		"distribution_version": f.DistributionVersion,
		"distribution_major":   f.DistributionMajor,

		// Network
		"ip_addresses":        f.IPAddresses,
		"ip_addresses_string": strings.Join(f.IPAddresses, ", "),
		"network_interfaces":  f.NetworkInterfaces, // CRITICAL: Array for templates

		// Hardware - Basic
		"cpu_cores":       f.CPUCores,
		"memory_total_mb": f.MemoryTotalMB,

		// Hardware - Arrays (CRITICAL: Enable template iteration)
		"disks": f.Disks,
		"gpus":  f.GPUs,

		// OS Details
		"kernel_version": f.KernelVersion,

		// CPU Extended
		"cpu_model":        f.CPUModel,
		"cpu_flags":        f.CPUFlags,
		"cpu_flags_string": strings.Join(f.CPUFlags, " "),

		// Memory Extended
		"memory_free_mb": f.MemoryFreeMB,
		"swap_total_mb":  f.SwapTotalMB,
		"swap_free_mb":   f.SwapFreeMB,

		// Network Extended
		"default_gateway":    f.DefaultGateway,
		"dns_servers":        f.DNSServers,
		"dns_servers_string": strings.Join(f.DNSServers, ", "),

		// Software
		"python_version":  f.PythonVersion,
		"package_manager": f.PackageManager,

		// Package manager convenience booleans (for cleaner conditionals)
		"apt_available":    f.PackageManager == "apt",
		"dnf_available":    f.PackageManager == pkgManagerDnf,
		"yum_available":    f.PackageManager == "yum",
		"pacman_available": f.PackageManager == "pacman",
		"zypper_available": f.PackageManager == "zypper",
		"apk_available":    f.PackageManager == "apk",
		"brew_available":   f.PackageManager == "brew",
		"port_available":   f.PackageManager == "port",

		// OS convenience booleans (for cleaner conditionals)
		"linux":   f.OS == osLinux,
		"darwin":  f.OS == osDarwin,
		"macos":   f.OS == osDarwin, // Alias
		"windows": f.OS == osWindows,

		// Toolchains
		"docker_version": f.DockerVersion,
		"git_version":    f.GitVersion,
		"go_version":     f.GoVersion,

		// Ollama
		"ollama_version":  f.OllamaVersion,
		"ollama_models":   f.OllamaModels,  // Array for template iteration
		"ollama_endpoint": f.OllamaEndpoint,
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

// collectNetworkInterfaces gathers detailed network interface information
func collectNetworkInterfaces() []NetworkInterface {
	var interfaces []NetworkInterface

	ifaces, err := net.Interfaces()
	if err != nil {
		return interfaces
	}

	for _, iface := range ifaces {
		// Skip loopback
		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		ni := NetworkInterface{
			Name:       iface.Name,
			MACAddress: iface.HardwareAddr.String(),
			MTU:        iface.MTU,
			Up:         iface.Flags&net.FlagUp != 0,
		}

		addrs, err := iface.Addrs()
		if err == nil {
			for _, addr := range addrs {
				ni.Addresses = append(ni.Addresses, addr.String())
			}
		}

		interfaces = append(interfaces, ni)
	}

	return interfaces
}

// detectPythonVersion attempts to detect Python version
func detectPythonVersion() string {
	// Try python3 first
	for _, cmd := range []string{"python3", "python"} {
		// #nosec G204 -- cmd is from a trusted list of Python executable names used for version detection
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
