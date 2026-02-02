package facts

import (
	"os/exec"
	"strings"
)

// detectToolchains probes for common development tools.
func detectToolchains() (docker, git, golang string) {
	docker = detectToolchainVersion("docker", "--version", "Docker version ")
	git = detectToolchainVersion("git", "--version", "git version ")
	golang = detectToolchainVersion("go", "version", "go version go")
	return
}

// detectToolchainVersion runs command and extracts version.
func detectToolchainVersion(cmd, flag, prefix string) string {
	path, err := exec.LookPath(cmd)
	if err != nil {
		return "" // Not installed
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := exec.Command(path, flag).CombinedOutput()
	if err != nil {
		return ""
	}

	version := strings.TrimSpace(string(out))
	version = strings.TrimPrefix(version, prefix)

	// Extract first token (version number)
	fields := strings.Fields(version)
	if len(fields) > 0 {
		// Remove trailing commas (e.g., "24.0.7," -> "24.0.7")
		return strings.TrimSuffix(fields[0], ",")
	}

	return ""
}
