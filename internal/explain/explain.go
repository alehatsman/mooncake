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

// formatSystem formats system information into lines
func formatSystem(f *facts.Facts) []string {
	var lines []string
	lines = append(lines, "┌─ System")
	lines = append(lines, fmt.Sprintf("│  OS:       %s %s", f.Distribution, f.DistributionVersion))
	lines = append(lines, fmt.Sprintf("│  Arch:     %s", f.Arch))
	lines = append(lines, fmt.Sprintf("│  Hostname: %s", f.Hostname))
	lines = append(lines, "└─")
	return lines
}

// formatSoftware formats software information into lines
func formatSoftware(f *facts.Facts) []string {
	var lines []string
	lines = append(lines, "┌─ Software")
	if f.PackageManager != "" {
		lines = append(lines, fmt.Sprintf("│  Package Manager: %s", f.PackageManager))
	}
	if f.PythonVersion != "" {
		lines = append(lines, fmt.Sprintf("│  Python:          %s", f.PythonVersion))
	}
	lines = append(lines, "└─")
	return lines
}

// formatCPUMemory formats CPU and memory information into lines
func formatCPUMemory(f *facts.Facts) []string {
	var lines []string
	lines = append(lines, "┌─ CPU & Memory")
	lines = append(lines, fmt.Sprintf("│  Cores:  %d", f.CPUCores))
	lines = append(lines, fmt.Sprintf("│  Memory: %d MB (%.1f GB)", f.MemoryTotalMB, float64(f.MemoryTotalMB)/1024))
	lines = append(lines, "└─")
	return lines
}

// formatGPUs formats GPU information into lines
func formatGPUs(gpus []facts.GPU) []string {
	if len(gpus) == 0 {
		return []string{}
	}

	var lines []string
	lines = append(lines, "┌─ GPUs")
	for i, gpu := range gpus {
		if i > 0 {
			lines = append(lines, "│")
		}
		lines = append(lines, fmt.Sprintf("│  Vendor: %s", strings.ToUpper(gpu.Vendor)))
		lines = append(lines, fmt.Sprintf("│  Model:  %s", gpu.Model))
		if gpu.Memory != "" {
			lines = append(lines, fmt.Sprintf("│  Memory: %s", gpu.Memory))
		}
		if gpu.Driver != "" {
			lines = append(lines, fmt.Sprintf("│  Driver: %s", gpu.Driver))
		}
	}
	lines = append(lines, "└─")
	return lines
}

// formatStorage formats storage information into lines
func formatStorage(disks []facts.Disk) []string {
	if len(disks) == 0 {
		return []string{}
	}

	var lines []string
	lines = append(lines, "┌─ Storage")

	for i, disk := range disks {
		if i > 0 {
			lines = append(lines, "│")
		}
		lines = append(lines, fmt.Sprintf("│  Device:     %s", disk.Device))
		lines = append(lines, fmt.Sprintf("│  Mount:      %s", disk.MountPoint))
		lines = append(lines, fmt.Sprintf("│  Filesystem: %s", disk.Filesystem))
		lines = append(lines, fmt.Sprintf("│  Size:       %d GB", disk.SizeGB))
		lines = append(lines, fmt.Sprintf("│  Used:       %d GB (%d%%)", disk.UsedGB, disk.UsedPct))
		lines = append(lines, fmt.Sprintf("│  Available:  %d GB", disk.AvailGB))
	}
	lines = append(lines, "└─")

	return lines
}

// formatNetwork formats network interface information into lines
func formatNetwork(ifaces []facts.NetworkInterface) []string {
	if len(ifaces) == 0 {
		return []string{}
	}

	var lines []string
	lines = append(lines, "┌─ Network Interfaces")

	for i, iface := range ifaces {
		if i > 0 {
			lines = append(lines, "│")
		}
		lines = append(lines, fmt.Sprintf("│  %s [up]", iface.Name))
		if iface.MACAddress != "" {
			lines = append(lines, fmt.Sprintf("│    MAC: %s", iface.MACAddress))
		}
		lines = append(lines, fmt.Sprintf("│    MTU: %d", iface.MTU))
		if len(iface.Addresses) > 0 {
			lines = append(lines, "│    Addresses:")
			for _, addr := range iface.Addresses {
				lines = append(lines, fmt.Sprintf("│      - %s", addr))
			}
		}
	}
	lines = append(lines, "└─")

	return lines
}

// printSideBySide prints two columns of text side by side
func printSideBySide(left, right []string) {
	if len(left) == 0 && len(right) == 0 {
		return
	}

	// Calculate max width of left column
	leftWidth := 0
	for _, line := range left {
		// Remove color codes and measure actual visible length
		visibleLen := len(line)
		if visibleLen > leftWidth {
			leftWidth = visibleLen
		}
	}

	// Add padding
	leftWidth += 4

	// Print both columns
	maxLines := len(left)
	if len(right) > maxLines {
		maxLines = len(right)
	}

	for i := 0; i < maxLines; i++ {
		leftLine := ""
		if i < len(left) {
			leftLine = left[i]
		}

		rightLine := ""
		if i < len(right) {
			rightLine = right[i]
		}

		// Pad left line to fixed width
		padding := leftWidth - len(leftLine)
		if padding < 0 {
			padding = 0
		}

		fmt.Printf("%s%s%s\n", leftLine, strings.Repeat(" ", padding), rightLine)
	}

	fmt.Println()
}

// printHorizontal prints multiple columns horizontally
func printHorizontal(columns [][]string) {
	if len(columns) == 0 {
		return
	}

	// Calculate width for each column
	columnWidths := make([]int, len(columns))
	for i, col := range columns {
		maxWidth := 0
		for _, line := range col {
			if len(line) > maxWidth {
				maxWidth = len(line)
			}
		}
		columnWidths[i] = maxWidth + 4 // Add padding
	}

	// Find max number of lines
	maxLines := 0
	for _, col := range columns {
		if len(col) > maxLines {
			maxLines = len(col)
		}
	}

	// Print all columns horizontally
	for i := 0; i < maxLines; i++ {
		for colIdx, col := range columns {
			line := ""
			if i < len(col) {
				line = col[i]
			}

			// Pad to column width
			padding := columnWidths[colIdx] - len(line)
			if padding < 0 {
				padding = 0
			}

			fmt.Printf("%s%s", line, strings.Repeat(" ", padding))
		}
		fmt.Println()
	}

	fmt.Println()
}
