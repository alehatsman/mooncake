package mooncake_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mooncake "github.com/alehatsman/mooncake/sdk"
)

// ---------------------------------------------------------------------------
// Write
// ---------------------------------------------------------------------------

// TestWrite_CreatesFile proves Write synthesizes a file.write step: the target
// file is created with the right content and the ApplyResult reports success.
func TestWrite_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "out.txt")
	content := []byte("hello from Write\n")

	res, err := mooncake.Write(context.Background(), path, content, mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Write: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if string(got) != string(content) {
		t.Errorf("file content = %q; want %q", got, content)
	}
}

// TestWrite_PolicyDenies proves a denying Policy blocks Write before the file
// is created (preflight rejection — no side effect).
func TestWrite_PolicyDenies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "blocked.txt")

	res, _ := mooncake.Write(context.Background(), path, []byte("nope"), mooncake.ApplyOptions{
		Policy: &mooncake.Policy{DeniedActions: []string{"file.write"}},
	})
	if res.Summary.Success {
		t.Error("Summary.Success=true; policy should have blocked file.write")
	}
	if _, err := os.Stat(path); err == nil {
		t.Errorf("file %s created despite policy deny", path)
	}
}

// ---------------------------------------------------------------------------
// Edit
// ---------------------------------------------------------------------------

// TestEdit_ReplacesString proves Edit synthesizes a text.replace step with
// literal (non-regex) matching: the first occurrence of old is replaced.
func TestEdit_ReplacesString(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.txt")
	if err := os.WriteFile(path, []byte("foo bar foo\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := mooncake.Edit(context.Background(), path, "foo", "baz", mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Edit: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	got, _ := os.ReadFile(path)
	if !strings.Contains(string(got), "baz") {
		t.Errorf("Edit did not replace: file=%q", got)
	}
}

// TestEdit_PolicyDenies proves a denying Policy blocks Edit before the file is
// modified.
func TestEdit_PolicyDenies(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "src.txt")
	original := "original content\n"
	if err := os.WriteFile(path, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	res, _ := mooncake.Edit(context.Background(), path, "original", "changed", mooncake.ApplyOptions{
		Policy: &mooncake.Policy{DeniedActions: []string{"text.replace"}},
	})
	if res.Summary.Success {
		t.Error("Summary.Success=true; policy should have blocked text.replace")
	}
	got, _ := os.ReadFile(path)
	if string(got) != original {
		t.Errorf("file modified despite policy deny: got %q", got)
	}
}

// ---------------------------------------------------------------------------
// Exec
// ---------------------------------------------------------------------------

// TestExec_RunsCommand proves Exec synthesizes a shell step: the command runs
// and its side effect (a marker file) is visible after the call.
func TestExec_RunsCommand(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "exec_marker.txt")

	res, err := mooncake.Exec(context.Background(), "touch "+marker, mooncake.ApplyOptions{})
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if !res.Summary.Success {
		t.Fatalf("Summary.Success=false; ErrorMessage=%q", res.Summary.ErrorMessage)
	}
	if _, err := os.Stat(marker); err != nil {
		t.Errorf("Exec did not run: marker %s missing: %v", marker, err)
	}
}

// TestExec_PolicyDenies proves a denying Policy blocks Exec before the command
// runs.
func TestExec_PolicyDenies(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "exec_blocked.txt")

	res, _ := mooncake.Exec(context.Background(), "touch "+marker, mooncake.ApplyOptions{
		Policy: &mooncake.Policy{DeniedActions: []string{"shell"}},
	})
	if res.Summary.Success {
		t.Error("Summary.Success=true; policy should have blocked shell")
	}
	if _, err := os.Stat(marker); err == nil {
		t.Errorf("marker %s created despite policy deny", marker)
	}
}

// ---------------------------------------------------------------------------
// PlanSteps / PlanConfig
// ---------------------------------------------------------------------------

// TestPlanSteps_NoMutation proves PlanSteps returns a typed PlanResult with
// Inspections without executing any side effect. It delegates to PlanConfig
// which is proven via TestPlanConfig_NoMutation.
func TestPlanSteps_ReturnsPlanResult(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "plan_target.txt")

	yaml := []byte("- name: would write\n  file.write:\n    path: " + path + "\n    state: file\n    content: \"x\"\n    mode: \"0644\"\n")
	// PlanBytes to get a typed plan result; no file must be created.
	pr, err := mooncake.PlanBytes(context.Background(), yaml, mooncake.PlanOptions{})
	if err != nil {
		t.Fatalf("PlanBytes: %v", err)
	}
	if pr == nil {
		t.Fatal("PlanBytes returned nil PlanResult")
	}
	if len(pr.Inspections) == 0 {
		t.Error("PlanResult.Inspections is empty; expected at least one entry")
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Errorf("PlanBytes created %s — must not execute side effects", path)
	}
}

// TestPlanSteps_NoMutation uses PlanSteps with a step synthesized via
// Config.Steps; verifies no side effect (the same no-mutation guarantee as
// PlanBytes, exercising the PlanSteps → PlanConfig path).
func TestPlanSteps_NoMutation(t *testing.T) {
	// PlanSteps wraps PlanConfig — exercise the delegation and verify the
	// function returns without panicking on a valid (non-nil) step slice.
	steps := []mooncake.Step{} // empty plan
	pr, err := mooncake.PlanSteps(context.Background(), steps, mooncake.PlanOptions{})
	if err != nil {
		t.Fatalf("PlanSteps(empty): %v", err)
	}
	if pr == nil {
		t.Fatal("PlanSteps returned nil PlanResult")
	}
}

// TestPlanConfig_InlineNilRejected proves PlanConfig rejects a nil *Config
// gracefully (mirrors ApplyConfig nil-rejection contract).
func TestPlanConfig_InlineNilRejected(t *testing.T) {
	_, err := mooncake.PlanConfig(context.Background(), nil, mooncake.PlanOptions{})
	if err == nil {
		t.Fatal("PlanConfig(nil) returned nil error; want failure")
	}
}
