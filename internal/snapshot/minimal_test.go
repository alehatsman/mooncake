package snapshot

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/register"
)

func TestCollect(t *testing.T) {
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

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
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

	if err := os.Mkdir(filepath.Join(tmpDir, "subdir1"), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	if err := os.Mkdir(filepath.Join(tmpDir, "subdir2"), 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	snap, err := Collect(tmpDir)
	if err != nil {
		t.Fatalf("Collect failed: %v", err)
	}

	if snap.Branch != "master" && snap.Branch != "main" {
		t.Errorf("Unexpected branch: %s", snap.Branch)
	}

	if snap.Head == "" {
		t.Errorf("HEAD should not be empty")
	}

	if !snap.Clean {
		t.Errorf("Repo should be clean")
	}

	if len(snap.TopLevelDirs) < 2 {
		t.Errorf("Expected at least 2 top-level dirs, got %d", len(snap.TopLevelDirs))
	}

	if len(snap.Actions) == 0 {
		t.Errorf("Expected registered actions, got none")
	}
}
