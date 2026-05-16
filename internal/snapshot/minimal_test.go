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
	// gitCleanEnv is also used by Collect's helpers (see minimal.go); we
	// reuse it here for the fixture-setup subprocesses so a pre-commit /
	// pre-push run of this test doesn't inherit the host repo's GIT_DIR.
	cleanEnv := gitCleanEnv()

	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = tmpDir
		cmd.Env = cleanEnv
		if err := cmd.Run(); err != nil {
			t.Fatalf("git %v: %v", args, err)
		}
	}

	runGit("init")
	runGit("config", "user.email", "test@example.com")
	runGit("config", "user.name", "Test User")

	if err := os.WriteFile(filepath.Join(tmpDir, "test.txt"), []byte("test"), 0644); err != nil {
		t.Fatalf("Failed to write test file: %v", err)
	}

	runGit("add", ".")
	runGit("commit", "-m", "initial")

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
