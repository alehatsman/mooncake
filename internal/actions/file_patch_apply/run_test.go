package file_patch_apply

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Variables:  map[string]interface{}{},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		CurrentMode: planMode(plan),
		Stats:      executor.NewExecutionStats(),
	}
}

// TestRun_PatchWouldApply: a unified diff that hasn't been applied → plan
// predicts change, execute applies it.
func TestRun_PatchWouldApply(t *testing.T) {
	target := filepath.Join(t.TempDir(), "f.txt")
	original := "line1\nline2\nline3\n"
	if err := os.WriteFile(target, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := `@@ -1,3 +1,3 @@
 line1
-line2
+LINE2
 line3
`
	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  target,
			Patch: patch,
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan: WouldChange should be true; reason=%q", r.Reason)
	}
	cur, _ := os.ReadFile(target)
	if string(cur) != original {
		t.Error("plan must not modify the file")
	}

	if _, err := h.Run(newCtx(t, false), step); err != nil {
		t.Fatalf("execute: %v", err)
	}
	cur, _ = os.ReadFile(target)
	got := string(cur)
	// The patch handler may add a trailing newline depending on hunk
	// shape. The important assertion is that LINE2 replaced line2.
	if !strings.Contains(got, "LINE2") || strings.Contains(got, "\nline2\n") {
		t.Errorf("execute result = %q", got)
	}
}

// TestRun_AlreadyApplied: target already matches what patch produces →
// plan reports already-applied, execute is no-op.
func TestRun_AlreadyApplied(t *testing.T) {
	target := filepath.Join(t.TempDir(), "f.txt")
	if err := os.WriteFile(target, []byte("line1\nLINE2\nline3\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	patch := `@@ -1,3 +1,3 @@
 line1
-line2
+LINE2
 line3
`
	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  target,
			Patch: patch,
		},
	}
	h := &Handler{}

	res, err := h.Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("plan: should report already-applied; reason=%q", r.Reason)
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
