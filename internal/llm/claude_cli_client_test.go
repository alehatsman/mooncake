package llm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestClaudeCLIClient_GeneratePlan(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	tmpDir := t.TempDir()
	mockCLI := filepath.Join(tmpDir, "claude")

	script := `#!/bin/bash
cat << 'EOF'
- shell:
    cmd: echo hello
EOF
`

	if err := os.WriteFile(mockCLI, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	client := &ClaudeCLIClient{
		cliPath: mockCLI,
	}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	expected := "- shell:\n    cmd: echo hello"
	if plan != expected {
		t.Errorf("Expected %q, got %q", expected, plan)
	}
}

func TestNewClaudeCLIClient_NotFound(t *testing.T) {
	origPath := os.Getenv("PATH")
	defer os.Setenv("PATH", origPath)

	os.Setenv("PATH", "/nonexistent")

	_, err := NewClaudeCLIClient()
	if err == nil {
		t.Error("Expected error when claude CLI not found")
	}
}

func TestClaudeCLIClient_WithModel(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	tmpDir := t.TempDir()
	mockCLI := filepath.Join(tmpDir, "claude")

	script := `#!/bin/bash
if [[ "$1" == "--model" ]] && [[ "$2" == "opus" ]]; then
  echo "- shell:"
  echo "    cmd: echo opus"
else
  exit 1
fi
`

	if err := os.WriteFile(mockCLI, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	client := &ClaudeCLIClient{
		cliPath: mockCLI,
	}

	plan, err := client.GeneratePlan(context.Background(), "system", "user", "opus")
	if err != nil {
		t.Fatalf("GeneratePlan failed: %v", err)
	}

	expected := "- shell:\n    cmd: echo opus"
	if plan != expected {
		t.Errorf("Expected %q, got %q", expected, plan)
	}
}

func TestClaudeCLIClient_EmptyOutput(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	tmpDir := t.TempDir()
	mockCLI := filepath.Join(tmpDir, "claude")

	script := `#!/bin/bash
exit 0
`

	if err := os.WriteFile(mockCLI, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	client := &ClaudeCLIClient{
		cliPath: mockCLI,
	}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Error("Expected error for empty output")
	}
}

func TestClaudeCLIClient_CLIError(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping CLI test in short mode")
	}

	tmpDir := t.TempDir()
	mockCLI := filepath.Join(tmpDir, "claude")

	script := `#!/bin/bash
echo "error message" >&2
exit 1
`

	if err := os.WriteFile(mockCLI, []byte(script), 0755); err != nil {
		t.Fatalf("Failed to create mock CLI: %v", err)
	}

	client := &ClaudeCLIClient{
		cliPath: mockCLI,
	}

	_, err := client.GeneratePlan(context.Background(), "system", "user", "")
	if err == nil {
		t.Error("Expected error when CLI exits with error")
	}
	if !contains(err.Error(), "error message") {
		t.Errorf("Expected error to contain stderr output, got: %v", err)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) >= len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
