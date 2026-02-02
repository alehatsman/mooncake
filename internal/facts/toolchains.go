package facts

import (
	"os"
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

// detectOllamaVersion attempts to detect Ollama version.
func detectOllamaVersion() string {
	path, err := exec.LookPath("ollama")
	if err != nil {
		return "" // Not installed
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := exec.Command(path, "--version").CombinedOutput()
	if err != nil {
		return ""
	}

	// Parse output format: "Warning: client version is 0.15.2"
	// or "ollama version 0.1.47" (older format)
	output := strings.TrimSpace(string(out))

	// Try new format first: "Warning: client version is X.Y.Z"
	if strings.Contains(output, "client version is") {
		parts := strings.Split(output, "client version is")
		if len(parts) >= 2 {
			version := strings.TrimSpace(parts[1])
			fields := strings.Fields(version)
			if len(fields) > 0 {
				return fields[0]
			}
		}
	}

	// Try older format: "ollama version X.Y.Z"
	if strings.HasPrefix(output, "ollama version") {
		version := strings.TrimPrefix(output, "ollama version ")
		fields := strings.Fields(version)
		if len(fields) > 0 {
			return fields[0]
		}
	}

	return ""
}

// detectOllamaModels attempts to list installed Ollama models.
func detectOllamaModels() []OllamaModel {
	path, err := exec.LookPath("ollama")
	if err != nil {
		return nil
	}

	// #nosec G204 -- path validated via exec.LookPath
	out, err := exec.Command(path, "list").CombinedOutput()
	if err != nil {
		return nil
	}

	var models []OllamaModel
	lines := strings.Split(string(out), "\n")

	for i, line := range lines {
		// Skip header line and empty lines
		if i == 0 || strings.TrimSpace(line) == "" {
			continue
		}

		// Parse tabular output: NAME SIZE MODIFIED
		// Example: "llama3.1:8b    4.7 GB  2 weeks ago"
		fields := strings.Fields(line)
		if len(fields) >= 2 {
			model := OllamaModel{
				Name: fields[0],
				Size: fields[1],
			}

			// Handle "4.7 GB" format (size is two fields)
			if len(fields) >= 3 && (fields[2] == "GB" || fields[2] == "MB" || fields[2] == "KB") {
				model.Size = fields[1] + " " + fields[2]
			}

			// Modified time is the remaining fields (e.g., "2 weeks ago")
			if len(fields) >= 4 {
				model.ModifiedAt = strings.Join(fields[3:], " ")
			} else if len(fields) == 3 && fields[2] != "GB" && fields[2] != "MB" && fields[2] != "KB" {
				model.ModifiedAt = fields[2]
			}

			models = append(models, model)
		}
	}

	return models
}

// detectOllamaEndpoint determines the Ollama server endpoint.
func detectOllamaEndpoint() string {
	// Check OLLAMA_HOST environment variable
	if host := os.Getenv("OLLAMA_HOST"); host != "" {
		// If it doesn't start with http, add it
		if !strings.HasPrefix(host, "http://") && !strings.HasPrefix(host, "https://") {
			return "http://" + host
		}
		return host
	}

	// Default endpoint
	return "http://localhost:11434"
}
