package facts

import (
	"os/exec"
	"testing"
)

// TestDetectToolchainVersion_Docker tests Docker version detection
func TestDetectToolchainVersion_Docker(t *testing.T) {
	version := detectToolchainVersion("docker", "--version", "Docker version ")

	// Docker might not be installed
	if version == "" {
		t.Skip("Docker not installed")
	}

	t.Logf("Docker version: %s", version)
}

// TestDetectToolchainVersion_Git tests Git version detection
func TestDetectToolchainVersion_Git(t *testing.T) {
	version := detectToolchainVersion("git", "--version", "git version ")

	// Git should be available on dev machines
	if version == "" {
		t.Log("Git not found, but this is acceptable")
	} else {
		t.Logf("Git version: %s", version)
	}
}

// TestDetectToolchainVersion_Go tests Go version detection
func TestDetectToolchainVersion_Go(t *testing.T) {
	version := detectToolchainVersion("go", "version", "go version go")

	// Go should be available since we're running tests
	if version == "" {
		t.Error("Go should be available during testing")
	}

	t.Logf("Go version: %s", version)
}

// TestDetectToolchainVersion_Python tests Python version detection
func TestDetectToolchainVersion_Python(t *testing.T) {
	version := detectToolchainVersion("python3", "--version", "Python ")

	// Python might not be installed
	if version == "" {
		t.Skip("Python3 not installed")
	}

	t.Logf("Python3 version: %s", version)
}

// TestDetectToolchainVersion_NonExistent tests handling of non-existent command
func TestDetectToolchainVersion_NonExistent(t *testing.T) {
	version := detectToolchainVersion("nonexistent-command-12345", "--version", "")

	if version != "" {
		t.Errorf("Should return empty string for non-existent command, got: %s", version)
	}
}

// TestDetectOllamaVersion_NotInstalled tests Ollama detection when not installed
func TestDetectOllamaVersion_NotInstalled(t *testing.T) {
	// This test will skip if Ollama is installed
	_, err := exec.LookPath("ollama")
	if err == nil {
		t.Skip("Ollama is installed, skipping not-installed test")
	}

	version := detectOllamaVersion()

	if version != "" {
		t.Error("Version should be empty when Ollama not installed")
	}
}

// TestDetectOllamaVersion_Installed tests Ollama detection when installed
func TestDetectOllamaVersion_Installed(t *testing.T) {
	// Check if ollama is installed
	_, err := exec.LookPath("ollama")
	if err != nil {
		t.Skip("Ollama not installed")
	}

	version := detectOllamaVersion()

	if version == "" {
		t.Error("Version should not be empty when Ollama installed")
	}

	t.Logf("Ollama version: %s", version)
}

// TestDetectOllamaModels_NotInstalled tests model detection when Ollama not installed
func TestDetectOllamaModels_NotInstalled(t *testing.T) {
	_, err := exec.LookPath("ollama")
	if err == nil {
		t.Skip("Ollama is installed, skipping not-installed test")
	}

	models := detectOllamaModels()

	if len(models) != 0 {
		t.Errorf("Should return empty slice when Ollama not installed, got %d models", len(models))
	}
}

// TestDetectOllamaModels_Installed tests model detection when Ollama installed
func TestDetectOllamaModels_Installed(t *testing.T) {
	_, err := exec.LookPath("ollama")
	if err != nil {
		t.Skip("Ollama not installed")
	}

	models := detectOllamaModels()

	// Models list depends on what's installed
	t.Logf("Detected %d Ollama model(s)", len(models))

	for _, model := range models {
		if model.Name == "" {
			t.Error("Model should have a name")
		}
		t.Logf("  Model: %s (%s)", model.Name, model.Size)
	}
}

// TestDetectOllamaEndpoint_Default tests endpoint detection
func TestDetectOllamaEndpoint_Default(t *testing.T) {
	// Test the function's ability to detect endpoint
	// It checks OLLAMA_HOST env var or returns default
	endpoint := detectOllamaEndpoint()

	// Should return either custom endpoint or default
	if endpoint == "" {
		t.Error("detectOllamaEndpoint() should return endpoint (env var or default)")
	}

	t.Logf("Ollama endpoint: %s", endpoint)
}

// TestDetectToolchains_Complete tests complete toolchain detection
func TestDetectToolchains_Complete(t *testing.T) {
	docker, git, golang := detectToolchains()

	// Go should be available since we're running tests
	if golang == "" {
		t.Error("Go should be detected during testing")
	}

	t.Logf("Toolchains detected - Docker: %s, Git: %s, Go: %s",
		docker, git, golang)
}
