// Package explain provides functionality for displaying system information in a human-readable format.
package explain

import (
	"fmt"
	"strings"

	"github.com/alehatsman/mooncake/internal/facts"
)

// relevantCPUFlagPrefixes are the CPU flag prefixes shown to users.
// AVX/SSE matter for ML workloads; FMA/AES matter for crypto.
var relevantCPUFlagPrefixes = []string{"avx", "sse", "fma", "aes"}

// filterRelevantCPUFlags returns the subset of flags that match
// relevantCPUFlagPrefixes (case-insensitive prefix/contains match).
func filterRelevantCPUFlags(flags []string) []string {
	var out []string
	for _, flag := range flags {
		lower := strings.ToLower(flag)
		for _, prefix := range relevantCPUFlagPrefixes {
			if strings.HasPrefix(lower, prefix) || strings.Contains(lower, prefix) {
				out = append(out, flag)
				break
			}
		}
	}
	return out
}

// storageTableWidths computes the device/mount/type column widths for
// the storage table, including minimum header widths and padding.
func storageTableWidths(disks []facts.Disk) (deviceW, mountW, typeW int) {
	deviceW, mountW, typeW = len("Device"), len("Mount"), len("Type")
	for _, d := range disks {
		if len(d.Device) > deviceW {
			deviceW = len(d.Device)
		}
		if len(d.MountPoint) > mountW {
			mountW = len(d.MountPoint)
		}
		if len(d.Filesystem) > typeW {
			typeW = len(d.Filesystem)
		}
	}
	deviceW += 2
	mountW += 2
	typeW += 2
	return
}

// DisplayFacts prints system information in a readable format. The
// per-section helpers below are pure-extraction from the original
// monolithic implementation (F009 brought it under the gocyclo 35
// cap). Behavior is preserved: each section still prints the same
// lines in the same order with the same blank-line trailers.
func DisplayFacts(f *facts.Facts) {
	printFactsHeader()
	printSystem(f)
	printCPU(f)
	printMemory(f)
	printSoftware(f)
	printOllamaModels(f)
	printGPUs(f)
	printStorage(f)
	printNetwork(f)
	printNetworkInterfaces(f)
}

func printFactsHeader() {
	fmt.Println("╭─────────────────────────────────────────────────────────────────────────────────────╮")
	fmt.Println("│                              System Information                                     │")
	fmt.Println("╰─────────────────────────────────────────────────────────────────────────────────────╯")
	fmt.Println()
}

func printSystem(f *facts.Facts) {
	fmt.Printf("OS:         %s %s\n", f.Distribution, f.DistributionVersion)
	fmt.Printf("Arch:       %s\n", f.Arch)
	fmt.Printf("Hostname:   %s\n", f.Hostname)
	if f.KernelVersion != "" {
		fmt.Printf("Kernel:     %s\n", f.KernelVersion)
	}
	fmt.Println()
}

func printCPU(f *facts.Facts) {
	fmt.Println("CPU:")
	fmt.Printf("  Cores:    %d\n", f.CPUCores)
	if f.CPUModel != "" {
		fmt.Printf("  Model:    %s\n", f.CPUModel)
	}
	if len(f.CPUFlags) > 0 {
		if relevant := filterRelevantCPUFlags(f.CPUFlags); len(relevant) > 0 {
			fmt.Printf("  Flags:    %s\n", strings.Join(relevant, " "))
		}
	}
	fmt.Println()
}

func printMemory(f *facts.Facts) {
	fmt.Println("Memory:")
	fmt.Printf("  Total:    %d MB (%.1f GB)\n", f.MemoryTotalMB, float64(f.MemoryTotalMB)/1024)
	if f.MemoryFreeMB > 0 {
		fmt.Printf("  Free:     %d MB (%.1f GB)\n", f.MemoryFreeMB, float64(f.MemoryFreeMB)/1024)
	}
	if f.SwapTotalMB > 0 {
		fmt.Printf("  Swap:     %d MB total, %d MB free\n", f.SwapTotalMB, f.SwapFreeMB)
	}
	fmt.Println()
}

func printSoftware(f *facts.Facts) {
	if f.PackageManager == "" && f.PythonVersion == "" && f.DockerVersion == "" &&
		f.GitVersion == "" && f.GoVersion == "" && f.OllamaVersion == "" {
		return
	}
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

func printOllamaModels(f *facts.Facts) {
	if f.OllamaVersion == "" || len(f.OllamaModels) == 0 {
		return
	}
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

func printGPUs(f *facts.Facts) {
	if len(f.GPUs) == 0 {
		return
	}
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

func printStorage(f *facts.Facts) {
	if len(f.Disks) == 0 {
		return
	}
	fmt.Println("Storage:")
	deviceW, mountW, typeW := storageTableWidths(f.Disks)
	fmt.Printf("  %-*s %-*s %-*s %12s %12s %12s\n",
		deviceW, "Device", mountW, "Mount", typeW, "Type",
		"Size", "Used", "Avail")
	fmt.Println("  " + strings.Repeat("─", deviceW+mountW+typeW+40))
	for _, disk := range f.Disks {
		fmt.Printf("  %-*s %-*s %-*s %10d GB %10d GB %10d GB\n",
			deviceW, disk.Device, mountW, disk.MountPoint, typeW, disk.Filesystem,
			disk.SizeGB, disk.UsedGB, disk.AvailGB)
	}
	fmt.Println()
}

func printNetwork(f *facts.Facts) {
	fmt.Println("Network:")
	if f.DefaultGateway != "" {
		fmt.Printf("  Gateway:  %s\n", f.DefaultGateway)
	}
	if len(f.DNSServers) > 0 {
		fmt.Printf("  DNS:      %s\n", strings.Join(f.DNSServers, ", "))
	}
	fmt.Println()
}

func printNetworkInterfaces(f *facts.Facts) {
	var relevantIfaces []facts.NetworkInterface
	for _, iface := range f.NetworkInterfaces {
		if iface.Up && len(iface.Addresses) > 0 {
			relevantIfaces = append(relevantIfaces, iface)
		}
	}
	if len(relevantIfaces) == 0 {
		return
	}
	fmt.Println("Network Interfaces:")
	for _, iface := range relevantIfaces {
		// Only show main interfaces (en*, eth*, wlan*). NOTE: this
		// hardcoded allowlist silently drops modern systemd
		// predictable names (wlp*, enp*-but-passes, wlx*) on common
		// Linux distros and all Windows interface names. Tracked in
		// F009 §(b) as a follow-up; the split preserves behavior.
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
			for _, addr := range iface.Addresses {
				if !strings.Contains(addr, ":") { // Skip IPv6
					parts = append(parts, addr)
				}
			}
		}
		fmt.Printf("  • %s\n", strings.Join(parts, "  |  "))
	}
}
