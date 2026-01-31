package facts

// collectWindowsFacts gathers Windows-specific system information
func collectWindowsFacts(f *Facts) {
	f.Distribution = "windows"
	f.Disks = detectWindowsDisks()
	f.GPUs = detectWindowsGPUs()
	// TODO: Implement more Windows-specific fact collection
	// - Use wmic or PowerShell to get system info
	// - Detect package manager (choco, scoop, winget)
	// - Memory detection
}

// detectWindowsDisks is a stub for Windows disk detection
func detectWindowsDisks() []Disk {
	// TODO: Use wmic or PowerShell to get disk info
	// wmic logicaldisk get caption,filesystem,freespace,size
	return []Disk{}
}

// detectWindowsGPUs is a stub for Windows GPU detection
func detectWindowsGPUs() []GPU {
	// TODO: Use wmic or PowerShell to get GPU info
	// wmic path win32_VideoController get name,driverversion,adapterram
	return []GPU{}
}
