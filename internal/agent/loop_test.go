package agent

import (
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
