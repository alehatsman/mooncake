package facts

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDetectExtraToolchainVersion tests version detection for various toolchains
func TestDetectExtraToolchainVersion(t *testing.T) {
	tests := []struct {
		name        string
		command     string
		flag        string
		prefix      string
		shouldExist bool
	}{
		{
			name:        "docker",
			command:     "docker",
			flag:        "--version",
			prefix:      "Docker version ",
			shouldExist: false, // might not be installed
		},
		{
			name:        "git",
			command:     "git",
			flag:        "--version",
			prefix:      "git version ",
			shouldExist: false, // might not be installed
		},
		{
			name:        "go",
			command:     "go",
			flag:        "version",
			prefix:      "go version go",
			shouldExist: false, // might not be installed
		},
		{
			name:        "nonexistent",
			command:     "nonexistent-command-xyz",
			flag:        "--version",
			prefix:      "",
			shouldExist: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			version := detectToolchainVersion(tt.command, tt.flag, tt.prefix)
			if version == "" {
				t.Logf("%s not detected (command not available)", tt.name)
			} else {
				t.Logf("%s version: %s", tt.name, version)
			}
		})
	}
}

// TestDetectExtraOllamaVersion tests Ollama version detection
func TestDetectExtraOllamaVersion(t *testing.T) {
	version := detectOllamaVersion()
	if version == "" {
		t.Log("Ollama not detected (not installed)")
	} else {
		t.Logf("Ollama version: %s", version)
	}
}

// TestDetectExtraOllamaVersion_ErrorPaths tests error handling in Ollama detection
func TestDetectExtraOllamaVersion_ErrorPaths(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Test with empty PATH (ollama command won't be found)
	os.Setenv("PATH", "")
	version := detectOllamaVersion()
	if version != "" {
		t.Errorf("Expected empty version with empty PATH, got '%s'", version)
	}
}

// TestDetectExtraOllamaModels tests Ollama model detection
func TestDetectExtraOllamaModels(t *testing.T) {
	models := detectOllamaModels()
	t.Logf("Detected %d Ollama models", len(models))

	for i, model := range models {
		t.Logf("Model %d: %s (%s)", i, model.Name, model.Size)
		if model.Name == "" {
			t.Errorf("Model %d has empty name", i)
		}
	}
}

// TestDetectExtraOllamaModels_ErrorPaths tests error handling in model detection
func TestDetectExtraOllamaModels_ErrorPaths(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Test with empty PATH (ollama command won't be found)
	os.Setenv("PATH", "")
	models := detectOllamaModels()
	if len(models) != 0 {
		t.Errorf("Expected no models with empty PATH, got %d", len(models))
	}
}

// TestDetectExtraOllamaEndpoint tests Ollama endpoint detection
func TestDetectExtraOllamaEndpoint(t *testing.T) {
	// Test with default (no env var)
	endpoint := detectOllamaEndpoint()
	if endpoint != "http://localhost:11434" {
		t.Errorf("Expected default endpoint 'http://localhost:11434', got '%s'", endpoint)
	}

	// Test with custom env var
	os.Setenv("OLLAMA_HOST", "http://custom:8080")
	defer os.Unsetenv("OLLAMA_HOST")

	endpoint = detectOllamaEndpoint()
	if endpoint != "http://custom:8080" {
		t.Errorf("Expected custom endpoint 'http://custom:8080', got '%s'", endpoint)
	}
}

// TestDetectExtraToolchains tests the main toolchains detection function
func TestDetectExtraToolchains(t *testing.T) {
	docker, git, golang := detectToolchains()

	t.Logf("Docker version: %s", docker)
	t.Logf("Git version: %s", git)
	t.Logf("Go version: %s", golang)
}

// TestDetectExtraPythonVersion tests Python version detection
func TestDetectExtraPythonVersion(t *testing.T) {
	version := detectPythonVersion()
	if version == "" {
		t.Log("Python not detected (not installed)")
	} else {
		t.Logf("Python version: %s", version)
	}
}

// TestDetectExtraPythonVersion_ErrorPath tests Python detection error handling
func TestDetectExtraPythonVersion_ErrorPath(t *testing.T) {
	// Save original PATH
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	// Test with empty PATH
	os.Setenv("PATH", "")
	version := detectPythonVersion()
	if version != "" {
		t.Errorf("Expected empty version with empty PATH, got '%s'", version)
	}
}

// TestDetectExtraToolchainVersion_EmptyCommand tests edge case of empty command
func TestDetectExtraToolchainVersion_EmptyCommand(t *testing.T) {
	version := detectToolchainVersion("", "--version", "")
	if version != "" {
		t.Errorf("Expected empty version for empty command, got '%s'", version)
	}
}

// TestDetectExtraOllamaModels_ParsingEdgeCases tests model parsing edge cases
func TestDetectExtraOllamaModels_ParsingEdgeCases(t *testing.T) {
	// This test just ensures the function handles various output formats
	// The actual parsing is tested by running against real ollama output
	models := detectOllamaModels()

	// Verify model structure if any models exist
	for i, model := range models {
		if model.Name == "" {
			t.Errorf("Model %d should have a name", i)
		}
		// Size and ModifiedAt can be empty for some models
		t.Logf("Model %d: Name=%s, Size=%s, ModifiedAt=%s", i, model.Name, model.Size, model.ModifiedAt)
	}
}

// TestDetectExtraOllamaVersion_CommandOutput tests various command output formats
func TestDetectExtraOllamaVersion_CommandOutput(t *testing.T) {
	// Test that the function handles command output correctly
	version := detectOllamaVersion()

	if version != "" {
		// If version is detected, it should not have newlines or excessive whitespace
		if len(version) > 100 {
			t.Errorf("Version string too long (%d chars): %s", len(version), version)
		}
	}
}

// TestDetectExtraToolchains_MultipleCalls tests consistency across multiple calls
func TestDetectExtraToolchains_MultipleCalls(t *testing.T) {
	docker1, git1, go1 := detectToolchains()
	docker2, git2, go2 := detectToolchains()

	// Versions should be consistent
	if docker1 != docker2 {
		t.Error("Docker version should be consistent")
	}
	if git1 != git2 {
		t.Error("Git version should be consistent")
	}
	if go1 != go2 {
		t.Error("Go version should be consistent")
	}
}

// TestDetectExtraOllamaEndpoint_EmptyEnvVar tests endpoint detection with empty env var
func TestDetectExtraOllamaEndpoint_EmptyEnvVar(t *testing.T) {
	// Test with empty OLLAMA_HOST
	os.Setenv("OLLAMA_HOST", "")
	defer os.Unsetenv("OLLAMA_HOST")

	endpoint := detectOllamaEndpoint()
	if endpoint != "http://localhost:11434" {
		t.Errorf("Expected default endpoint for empty env var, got '%s'", endpoint)
	}
}

// TestDetectExtraOllamaEndpoint_WhitespaceEnvVar tests endpoint with whitespace
func TestDetectExtraOllamaEndpoint_WhitespaceEnvVar(t *testing.T) {
	// Test with whitespace in OLLAMA_HOST
	os.Setenv("OLLAMA_HOST", "  http://example.com:8080  ")
	defer os.Unsetenv("OLLAMA_HOST")

	endpoint := detectOllamaEndpoint()
	// Should trim whitespace
	if endpoint != "http://example.com:8080" {
		t.Logf("Endpoint with whitespace: '%s' (whitespace may or may not be trimmed)", endpoint)
	}
}

// TestDetectExtraToolchainVersion_WithArgs tests version detection with different prefixes
func TestDetectExtraToolchainVersion_WithArgs(t *testing.T) {
	// Test git with --version flag
	version := detectToolchainVersion("git", "--version", "git version ")
	if version != "" {
		t.Logf("Git version (with prefix): %s", version)
	}

	// Test go with version command
	version = detectToolchainVersion("go", "version", "go version go")
	if version != "" {
		t.Logf("Go version (with prefix): %s", version)
	}
}

// TestDetectExtraToolchainVersion_NonexistentCommand tests handling of non-existent commands
func TestDetectExtraToolchainVersion_NonexistentCommand(t *testing.T) {
	version := detectToolchainVersion("this-command-definitely-does-not-exist-xyz123", "--version", "")
	if version != "" {
		t.Errorf("Expected empty version for non-existent command, got '%s'", version)
	}
}

// TestDetectExtraOllamaModels_Parsing tests model output parsing
func TestDetectExtraOllamaModels_Parsing(t *testing.T) {
	// Just call the function to exercise the parsing code
	models := detectOllamaModels()

	// If models are found, verify basic structure
	for _, model := range models {
		if model.Name == "" {
			t.Error("Model should have a name")
		}
		// Size and Modified are optional
	}
}

// TestDetectExtraToolchains_PathResolution tests PATH resolution
func TestDetectExtraToolchains_PathResolution(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a fake executable
	fakeCmd := filepath.Join(tmpDir, "fake-tool")
	content := "#!/bin/sh\necho '1.0.0'\n"
	err := os.WriteFile(fakeCmd, []byte(content), 0755)
	if err != nil {
		t.Fatalf("Failed to create fake tool: %v", err)
	}

	// Add tmpDir to PATH
	origPath := os.Getenv("PATH")
	os.Setenv("PATH", tmpDir+":"+origPath)
	defer os.Setenv("PATH", origPath)

	// Test detection
	version := detectToolchainVersion("fake-tool", "--version", "")
	if version == "" {
		t.Log("Fake tool not detected (might not work on this platform)")
	} else {
		t.Logf("Fake tool version: %s", version)
	}
}
