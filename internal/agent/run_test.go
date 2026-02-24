package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/register"
)

func TestStripMarkdownFences(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "yaml fence",
			input:    "```yaml\nname: test\n```",
			expected: "name: test",
		},
		{
			name:     "yml fence",
			input:    "```yml\nname: test\n```",
			expected: "name: test",
		},
		{
			name:     "generic fence",
			input:    "```\nname: test\n```",
			expected: "name: test",
		},
		{
			name:     "no fence",
			input:    "name: test",
			expected: "name: test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := stripMarkdownFences([]byte(tt.input))
			if strings.TrimSpace(string(result)) != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, string(result))
			}
		})
	}
}

func TestRunIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()

	cmd := exec.Command("git", "init")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config git: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to config git: %v", err)
	}

	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("initial"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	cmd = exec.Command("git", "add", ".")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "-m", "initial")
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	plan := fmt.Sprintf(`- file:
    path: %s
    content: "modified"
`, testFile)

	planPath := filepath.Join(tmpDir, "plan.yml")
	if err := os.WriteFile(planPath, []byte(plan), 0644); err != nil {
		t.Fatalf("Failed to write plan: %v", err)
	}

	opts := RunOptions{
		Goal:     "test goal",
		PlanPath: planPath,
		RepoRoot: tmpDir,
	}

	log, err := Run(opts)
	if err != nil {
		t.Fatalf("Run failed: %v", err)
	}

	if log.Iteration != 1 {
		t.Errorf("Expected iteration 1, got %d", log.Iteration)
	}

	if log.Status != "success" {
		t.Errorf("Expected status success, got %s", log.Status)
	}

	logPath := filepath.Join(tmpDir, ".mooncake/iterations/00001.json")
	if _, err := os.Stat(logPath); os.IsNotExist(err) {
		t.Errorf("Iteration log was not created")
	}
}
