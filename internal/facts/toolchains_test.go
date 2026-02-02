package facts

import (
	"strings"
	"testing"
)

func TestDetectToolchains(t *testing.T) {
	docker, git, golang := detectToolchains()

	// Git is likely to be installed, verify format if present
	if git != "" && !strings.Contains(git, ".") {
		t.Errorf("Invalid git version format: %s", git)
	}

	// Docker may not be installed, but verify format if present
	if docker != "" && !strings.Contains(docker, ".") {
		t.Errorf("Invalid docker version format: %s", docker)
	}

	// Go may not be installed, but verify format if present
	if golang != "" && !strings.Contains(golang, ".") {
		t.Errorf("Invalid go version format: %s", golang)
	}
}

func TestDetectToolchainVersion_InvalidCommand(t *testing.T) {
	version := detectToolchainVersion("nonexistent-command-12345", "--version", "")
	if version != "" {
		t.Errorf("Expected empty version for nonexistent command, got %s", version)
	}
}
