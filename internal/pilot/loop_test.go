package pilot

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNoProgressDetection(t *testing.T) {
	plan1 := []byte("- shell:\n    cmd: echo hello")
	plan2 := []byte("- shell:\n    cmd: echo hello")
	plan3 := []byte("- shell:\n    cmd: echo world")

	hash1 := ComputePlanHash(plan1)
	hash2 := ComputePlanHash(plan2)
	hash3 := ComputePlanHash(plan3)

	if hash1 != hash2 {
		t.Errorf("Identical plans should have same hash")
	}

	if hash1 == hash3 {
		t.Errorf("Different plans should have different hash")
	}
}

func TestIterationNumbering(t *testing.T) {
	tmpDir := t.TempDir()

	num1, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get iteration 1: %v", err)
	}
	if num1 != 1 {
		t.Errorf("Expected iteration 1, got %d", num1)
	}

	log1 := &IterationLog{
		Iteration: num1,
		Goal:      "test",
		Status:    "success",
	}
	WriteIterationLog(tmpDir, log1)

	num2, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get iteration 2: %v", err)
	}
	if num2 != 2 {
		t.Errorf("Expected iteration 2, got %d", num2)
	}
}

// TestSavePlan_FilePerms — F039(c). Plan files contain resolved
// !secret values (post-F037 the planner expands secret markers into
// concrete values before serialization) plus the operator's goal /
// LLM prompt. World-readable permissions on a shared host would leak
// them. The fix pins the directory to 0700 and the file to 0600 —
// matching the rest of mooncake's state-dir convention.
func TestSavePlan_FilePerms(t *testing.T) {
	repoRoot := t.TempDir()
	path, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("plan file perms = %04o, want 0600", got)
	}
	parent := filepath.Dir(path)
	dirInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("iterations dir perms = %04o, want 0700", got)
	}
}

// TestSavePlan_CreatesIterationsDir — F039 also exposed a latent bug
// where the iterations directory was never created; SavePlan would
// `os.WriteFile` into a non-existent path and silently return "" on
// the resulting ENOENT. With MkdirAll the first iteration creates
// the dir as a side-effect.
func TestSavePlan_CreatesIterationsDir(t *testing.T) {
	repoRoot := t.TempDir()
	path, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	if path == "" {
		t.Fatal("SavePlan returned empty path on first iteration (missing MkdirAll regression)")
	}
}

// TestSavePlan_ReturnsErrorOnFailure — F039(d). Pre-fix SavePlan
// silently returned "" on any WriteFile error, leaving the caller to
// guess whether the empty path meant "didn't save" or "shouldn't
// save." Now the error surfaces so the caller can log it.
func TestSavePlan_ReturnsErrorOnFailure(t *testing.T) {
	// Force a write failure by pointing the iterations dir at a path
	// whose parent is a regular file (MkdirAll will refuse).
	repoRoot := t.TempDir()
	// Pre-create a file at the path MkdirAll wants to use as a
	// directory: <repoRoot>/.mooncake/iterations.
	mooncakeDir := filepath.Join(repoRoot, ".mooncake")
	if err := os.MkdirAll(mooncakeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mooncakeDir, "iterations"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err == nil {
		t.Fatal("expected SavePlan error when iterations path is occupied by a file; got nil")
	}
	if !strings.Contains(err.Error(), "create iterations dir") {
		t.Errorf("error should name the failing stage; got %q", err.Error())
	}
}
