// Package explain provides functionality for displaying system information in a human-readable format.
package explain

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/facts"
)

// DisplayFacts prints system information in a readable format
func DisplayFacts(f *facts.Facts) {
	fmt.Println("╭─────────────────────────────────────────────────────────────────────────────────────╮")
	fmt.Println("│                              System Information                                     │")
	fmt.Println("╰─────────────────────────────────────────────────────────────────────────────────────╯")
	fmt.Println()

	// Calculate column widths for alignment based on actual content
	osStr := fmt.Sprintf("OS: %s %s", f.Distribution, f.DistributionVersion)
	cpuStr := fmt.Sprintf("CPU Cores: %d", f.CPUCores)
	pkgMgrStr := fmt.Sprintf("Package Manager: %s", f.PackageManager)

	col1Width := len(osStr)
	if len(cpuStr) > col1Width {
		col1Width = len(cpuStr)
	}
	if f.PackageManager != "" && len(pkgMgrStr) > col1Width {
		col1Width = len(pkgMgrStr)
	}

	col2Width := 30  // Second label + value

	// System
	archStr := fmt.Sprintf("Arch: %s", f.Arch)
	hostnameStr := f.Hostname
	fmt.Printf("%-*s  |  %-*s  |  Hostname: %s\n",
		col1Width, osStr,
		col2Width, archStr,
		hostnameStr)
	fmt.Println()

	// CPU & Memory
	memStr := fmt.Sprintf("%d MB (%.1f GB)", f.MemoryTotalMB, float64(f.MemoryTotalMB)/1024)
	fmt.Printf("%-*s  |  Memory: %s\n",
		col1Width, cpuStr,
		memStr)
	fmt.Println()

	// Software
	if f.PackageManager != "" || f.PythonVersion != "" {
		parts := []string{}
		if f.PackageManager != "" {
			parts = append(parts, fmt.Sprintf("%-*s", col1Width, pkgMgrStr))
		}
		if f.PythonVersion != "" {
			parts = append(parts, fmt.Sprintf("Python: %s", f.PythonVersion))
		}
		fmt.Println(strings.Join(parts, "  |  "))
		fmt.Println()
	}

	// GPUs
	if len(f.GPUs) > 0 {
		fmt.Println("GPUs:")
		for _, gpu := range f.GPUs {
			parts := []string{
				fmt.Sprintf("%s %s", strings.ToUpper(gpu.Vendor), gpu.Model),
			}
			if gpu.Memory != "" {
				parts = append(parts, fmt.Sprintf("Memory: %s", gpu.Memory))
			}
			if gpu.Driver != "" {
				parts = append(parts, fmt.Sprintf("Driver: %s", gpu.Driver))
			}
			fmt.Printf("  • %s\n", strings.Join(parts, ", "))
		}
		fmt.Println()
	}

	// Storage
	if len(f.Disks) > 0 {
		fmt.Println("Storage:")

		// Calculate column widths based on actual content
		deviceWidth := len("Device")
		mountWidth := len("Mount")
		typeWidth := len("Type")

		for _, disk := range f.Disks {
			if len(disk.Device) > deviceWidth {
				deviceWidth = len(disk.Device)
			}
			if len(disk.MountPoint) > mountWidth {
				mountWidth = len(disk.MountPoint)
			}
			if len(disk.Filesystem) > typeWidth {
				typeWidth = len(disk.Filesystem)
			}
		}

		// Add padding
		deviceWidth += 2
		mountWidth += 2
		typeWidth += 2

		// Print header
		fmt.Printf("  %-*s %-*s %-*s %12s %12s %12s\n",
			deviceWidth, "Device",
			mountWidth, "Mount",
			typeWidth, "Type",
			"Size", "Used", "Avail")

		totalWidth := deviceWidth + mountWidth + typeWidth + 40
		fmt.Println("  " + strings.Repeat("─", totalWidth))

		// Print data rows
		for _, disk := range f.Disks {
			fmt.Printf("  %-*s %-*s %-*s %10d GB %10d GB %10d GB\n",
				deviceWidth, disk.Device,
				mountWidth, disk.MountPoint,
				typeWidth, disk.Filesystem,
				disk.SizeGB, disk.UsedGB, disk.AvailGB)
		}
		fmt.Println()
	}

	// Network - only show interfaces that are up and have addresses
	var relevantIfaces []facts.NetworkInterface
	for _, iface := range f.NetworkInterfaces {
		if iface.Up && len(iface.Addresses) > 0 {
			relevantIfaces = append(relevantIfaces, iface)
		}
	}

	if len(relevantIfaces) > 0 {
		fmt.Println("Network Interfaces:")
		for _, iface := range relevantIfaces {
			// Only show main interfaces (en*, eth*, wlan*)
			if !strings.HasPrefix(iface.Name, "en") &&
				!strings.HasPrefix(iface.Name, "eth") &&
				!strings.HasPrefix(iface.Name, "wlan") {
				continue
			}

			parts := []string{iface.Name}
			if iface.MACAddress != "" {
				parts = append(parts, fmt.Sprintf("MAC: %s", iface.MACAddress))
			}
			if len(iface.Addresses) > 0 {
				// Show only IPv4 addresses
				for _, addr := range iface.Addresses {
					if !strings.Contains(addr, ":") { // Skip IPv6
						parts = append(parts, addr)
					}
				}
			}
			fmt.Printf("  • %s\n", strings.Join(parts, "  |  "))
		}
	}
}
