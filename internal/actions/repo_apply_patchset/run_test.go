package repo_apply_patchset

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool, baseDir string) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: baseDir,
	}
}

// TestRun_PatchsetWouldApply: patchset against unmodified target file
// → plan predicts change. Execute then performs the change.
func TestRun_PatchsetWouldApply(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\nbeta\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patchset := `--- a/a.txt
+++ b/a.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+BETA
 gamma
`
	step := &config.Step{
		RepoPatch: &config.RepoApplyPatchset{
			Patchset: patchset,
		},
	}
	res, err := (&Handler{}).Run(newCtx(t, true, dir), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan: WouldChange should be true; reason=%q", r.Reason)
	}
	// File untouched.
	cur, _ := os.ReadFile(filepath.Join(dir, "a.txt"))
	if string(cur) != "alpha\nbeta\ngamma\n" {
		t.Error("plan must not modify the file")
	}
}

// TestRun_PatchsetAlreadyApplied: target already has the patched
// content → plan reports already-applied.
func TestRun_PatchsetAlreadyApplied(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.txt"), []byte("alpha\nBETA\ngamma\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patchset := `--- a/a.txt
+++ b/a.txt
@@ -1,3 +1,3 @@
 alpha
-beta
+BETA
 gamma
`
	step := &config.Step{
		RepoPatch: &config.RepoApplyPatchset{
			Patchset: patchset,
		},
	}
	res, err := (&Handler{}).Run(newCtx(t, true, dir), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("plan should report already-applied; reason=%q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}
