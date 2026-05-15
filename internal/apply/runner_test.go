package apply_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
	_ "github.com/alehatsman/mooncake/internal/register" // register handlers
)

// TestRunner_KernelResult_Shape pins the locked R1.1b API: Run
// returns (*KernelResult, error) where the result has exactly the
// four documented fields (Plan, Steps, Events, Summary). A change
// to this test signals a contract break — downstream consumers
// (MCP rollback, agent loop undo, SDK callers) build directly on
// these fields.
func TestRunner_KernelResult_Shape(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "hello.txt")
	cfgPath := writeConfig(t, tmp, `
- name: write hello
  file.write:
    path: `+targetPath+`
    state: file
    content: "hello\n"
    mode: "0644"
`)

	cfg := &apply.Config{
		ConfigPath:   cfgPath,
		OutputFormat: "quiet",
		LogLevel:     "error",
	}

	result, err := apply.NewRunner(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("Run returned nil *KernelResult on success")
	}

	// Plan
	if result.Plan == nil {
		t.Errorf("KernelResult.Plan = nil; want compiled plan")
	}

	// Steps
	if len(result.Steps) == 0 {
		t.Errorf("KernelResult.Steps is empty; want at least 1 record")
	}

	// Events — at minimum run.started + plan.loaded + run.completed.
	if len(result.Events) < 3 {
		t.Errorf("KernelResult.Events length = %d; want >= 3 lifecycle events", len(result.Events))
	}

	// Summary
	if !result.Summary.Success {
		t.Errorf("Summary.Success = false; want true on a clean run")
	}
	if result.Summary.TotalSteps == 0 {
		t.Errorf("Summary.TotalSteps = 0; want > 0")
	}

	// Sanity: file was actually written.
	if _, statErr := os.Stat(targetPath); statErr != nil {
		t.Errorf("apply did not create %s: %v", targetPath, statErr)
	}
}

// TestRunner_Reverse_DeletesCreatedFile is the kernel-surface
// proof that *KernelResult.Reverse() produces an inverse plan
// that would undo a file.write create. file.write captures the
// pre-apply state (Existed=false here); Reverse therefore yields
// a state=absent step targeting the same path.
//
// This is the same algorithm internal/executor/transaction.go uses
// for transaction rollback — R1.1b lifts it to a public kernel
// operation usable across runs / out-of-process / from MCP.
func TestRunner_Reverse_DeletesCreatedFile(t *testing.T) {
	tmp := t.TempDir()
	targetPath := filepath.Join(tmp, "reverse-me.txt")
	cfgPath := writeConfig(t, tmp, `
- name: create reverse-me
  file.write:
    path: `+targetPath+`
    state: file
    content: "to be undone\n"
    mode: "0644"
`)

	cfg := &apply.Config{
		ConfigPath:   cfgPath,
		OutputFormat: "quiet",
		LogLevel:     "error",
	}

	result, err := apply.NewRunner(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("Run returned error: %v", err)
	}
	if _, statErr := os.Stat(targetPath); statErr != nil {
		t.Fatalf("apply did not create target file: %v", statErr)
	}

	inverse, err := result.Reverse()
	if err != nil {
		t.Fatalf("Reverse returned error: %v", err)
	}
	if inverse == nil {
		t.Fatalf("Reverse returned nil plan; want a populated inverse plan")
	}
	if len(inverse.Steps) == 0 {
		t.Fatalf("inverse plan has no steps; expected at least the delete-reverse-me step")
	}

	// At least one inverse step should target our path with state=absent.
	found := false
	for _, s := range inverse.Steps {
		if s.FileWrite != nil && s.FileWrite.Path == targetPath && s.FileWrite.State == "absent" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("inverse plan does not contain a state=absent step for %s; steps=%+v",
			targetPath, inverse.Steps)
	}
}

// TestRunner_Reverse_EmptyOnNilOrEmpty proves the documented
// safety contract: Reverse on a nil receiver or empty result
// returns an empty plan with no error.
func TestRunner_Reverse_EmptyOnNilOrEmpty(t *testing.T) {
	var nilResult *apply.KernelResult
	out, err := nilResult.Reverse()
	if err != nil {
		t.Fatalf("Reverse on nil receiver returned error: %v", err)
	}
	if out == nil {
		t.Fatalf("Reverse on nil receiver returned nil plan; want empty plan")
	}
	if len(out.Steps) != 0 {
		t.Errorf("nil-receiver Reverse: len(Steps) = %d; want 0", len(out.Steps))
	}

	empty := &apply.KernelResult{}
	out2, err := empty.Reverse()
	if err != nil {
		t.Fatalf("Reverse on empty result returned error: %v", err)
	}
	if len(out2.Steps) != 0 {
		t.Errorf("empty-result Reverse: len(Steps) = %d; want 0", len(out2.Steps))
	}
}

// writeConfig writes a one-step mooncake config and returns its path.
// Body must be the YAML list-of-steps content (it's wrapped in nothing
// here; mooncake's root file format is a list of steps directly).
func writeConfig(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, "mooncake.yml")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return path
}
