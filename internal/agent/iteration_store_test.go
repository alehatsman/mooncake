package agent

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNextIterationNumber(t *testing.T) {
	tmpDir := t.TempDir()

	num, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("NextIterationNumber failed: %v", err)
	}
	if num != 1 {
		t.Errorf("Expected first iteration to be 1, got %d", num)
	}

	log := &IterationLog{
		Iteration: num,
		Goal:      "test",
		PlanHash:  "abc123",
		Status:    "success",
	}
	if _, err := WriteIterationLog(tmpDir, log); err != nil {
		t.Fatalf("WriteIterationLog failed: %v", err)
	}

	num, err = NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("NextIterationNumber failed: %v", err)
	}
	if num != 2 {
		t.Errorf("Expected second iteration to be 2, got %d", num)
	}
}

func TestComputePlanHash(t *testing.T) {
	plan1 := []byte("name: test\nsteps:\n  - print:\n      msg: hello")
	plan2 := []byte("name: test\nsteps:\n  - print:\n      msg: hello")
	plan3 := []byte("name: test\nsteps:\n  - print:\n      msg: world")

	hash1 := ComputePlanHash(plan1)
	hash2 := ComputePlanHash(plan2)
	hash3 := ComputePlanHash(plan3)

	if hash1 != hash2 {
		t.Errorf("Identical plans produced different hashes: %s != %s", hash1, hash2)
	}

	if hash1 == hash3 {
		t.Errorf("Different plans produced same hash: %s", hash1)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-character hex string, got %d characters", len(hash1))
	}
}

func TestWriteIterationLog(t *testing.T) {
	tmpDir := t.TempDir()

	log := &IterationLog{
		Iteration: 1,
		Goal:      "test goal",
		PlanHash:  "abc123",
		Status:    "success",
		ChangedFiles: []string{"file1.txt", "file2.txt"},
		DiffStat: DiffStat{
			Files:      2,
			Insertions: 10,
			Deletions:  5,
		},
	}

	path, err := WriteIterationLog(tmpDir, log)
	if err != nil {
		t.Fatalf("WriteIterationLog failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".mooncake/iterations/00001.json")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Iteration log file was not created")
	}
}
