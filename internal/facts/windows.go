package facts

// collectWindowsFacts gathers Windows-specific system information
func collectWindowsFacts(f *Facts) {
	f.Distribution = "windows"
	// TODO: Implement Windows-specific fact collection
	// - Use wmic or PowerShell to get system info
	// - Detect package manager (choco, scoop, winget)
}
