// Package explain provides functionality for displaying system information in a human-readable format.
package explain

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/facts"
)

// DisplayFacts prints system information in a readable format
//
//nolint:gocyclo // Display function with many simple conditionals for optional fields
func DisplayFacts(f *facts.Facts) {
	fmt.Println("╭─────────────────────────────────────────────────────────────────────────────────────╮")
	fmt.Println("│                              System Information                                     │")
	fmt.Println("╰─────────────────────────────────────────────────────────────────────────────────────╯")
	fmt.Println()

	// System Information
	fmt.Printf("OS:         %s %s\n", f.Distribution, f.DistributionVersion)
	fmt.Printf("Arch:       %s\n", f.Arch)
	fmt.Printf("Hostname:   %s\n", f.Hostname)
	if f.KernelVersion != "" {
		fmt.Printf("Kernel:     %s\n", f.KernelVersion)
	}
	fmt.Println()

	// CPU Information
	fmt.Println("CPU:")
	fmt.Printf("  Cores:    %d\n", f.CPUCores)
	if f.CPUModel != "" {
		fmt.Printf("  Model:    %s\n", f.CPUModel)
	}
	if len(f.CPUFlags) > 0 {
		// Show only relevant flags (AVX, SSE, etc.)
		importantFlags := []string{}
		for _, flag := range f.CPUFlags {
			lowerFlag := strings.ToLower(flag)
			if strings.HasPrefix(lowerFlag, "avx") ||
				strings.HasPrefix(lowerFlag, "sse") ||
				strings.Contains(lowerFlag, "fma") ||
				strings.Contains(lowerFlag, "aes") {
				importantFlags = append(importantFlags, flag)
			}
		}
		if len(importantFlags) > 0 {
			fmt.Printf("  Flags:    %s\n", strings.Join(importantFlags, " "))
		}
	}
	fmt.Println()

	// Memory Information
	fmt.Println("Memory:")
	fmt.Printf("  Total:    %d MB (%.1f GB)\n", f.MemoryTotalMB, float64(f.MemoryTotalMB)/1024)
	if f.MemoryFreeMB > 0 {
		fmt.Printf("  Free:     %d MB (%.1f GB)\n", f.MemoryFreeMB, float64(f.MemoryFreeMB)/1024)
	}
	if f.SwapTotalMB > 0 {
		fmt.Printf("  Swap:     %d MB total, %d MB free\n", f.SwapTotalMB, f.SwapFreeMB)
	}
	fmt.Println()

	// Software & Development Tools
	if f.PackageManager != "" || f.PythonVersion != "" || f.DockerVersion != "" || f.GitVersion != "" || f.GoVersion != "" || f.OllamaVersion != "" {
		fmt.Println("Software:")
		if f.PackageManager != "" {
			fmt.Printf("  Package Manager: %s\n", f.PackageManager)
		}
		if f.PythonVersion != "" {
			fmt.Printf("  Python:          %s\n", f.PythonVersion)
		}
		if f.DockerVersion != "" {
			fmt.Printf("  Docker:          %s\n", f.DockerVersion)
		}
		if f.GitVersion != "" {
			fmt.Printf("  Git:             %s\n", f.GitVersion)
		}
		if f.GoVersion != "" {
			fmt.Printf("  Go:              %s\n", f.GoVersion)
		}
		if f.OllamaVersion != "" {
			fmt.Printf("  Ollama:          %s\n", f.OllamaVersion)
		}
		fmt.Println()
	}

	// Ollama Models (if installed)
	if f.OllamaVersion != "" && len(f.OllamaModels) > 0 {
		fmt.Println("Ollama Models:")
		fmt.Printf("  Endpoint: %s\n", f.OllamaEndpoint)
		fmt.Printf("  Models:   %d installed\n", len(f.OllamaModels))
		for _, model := range f.OllamaModels {
			parts := []string{model.Name, model.Size}
			if model.ModifiedAt != "" {
				parts = append(parts, fmt.Sprintf("Modified: %s", model.ModifiedAt))
			}
			fmt.Printf("    • %s\n", strings.Join(parts, "  |  "))
		}
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
			if gpu.CUDAVersion != "" {
				parts = append(parts, fmt.Sprintf("CUDA: %s", gpu.CUDAVersion))
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

	// Network Information
	fmt.Println("Network:")
	if f.DefaultGateway != "" {
		fmt.Printf("  Gateway:  %s\n", f.DefaultGateway)
	}
	if len(f.DNSServers) > 0 {
		fmt.Printf("  DNS:      %s\n", strings.Join(f.DNSServers, ", "))
	}
	fmt.Println()

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
