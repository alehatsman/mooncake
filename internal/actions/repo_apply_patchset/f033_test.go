package repo_apply_patchset

// F033 regression: a patchset whose `--- a/<path>` resolves outside
// the baseDir must be refused. Pre-fix the handler did filepath.Join
// (which would happily produce `/etc/passwd` for `--- a/../../etc/passwd`)
// + a debug-level ValidateNoPathTraversal log, then read+wrote the
// escaped path. The fix uses pathutil.SafeJoin which checks
// filepath.Rel.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
)

// TestF033_TraversingPathRefused_LenientMode: a lenient-mode apply
// must mark the traversing file's PatchResult as failed (Success=false,
// Error names the escape) and must NOT have read or written the
// escape target.
func TestF033_TraversingPathRefused_LenientMode(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Set up an "outside" file the test will assert isn't touched.
	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "untouched.txt")
	original := "ORIGINAL — must not be overwritten\n"
	if err := os.WriteFile(outsidePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	// Build a relative path from ctx.CurrentDir to outsidePath using ..
	// segments. SafeJoin uses filepath.Rel under absolute paths, so the
	// escape must show up as starting with `..`.
	rel, err := filepath.Rel(ctx.CurrentDir, outsidePath)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if !strings.HasPrefix(rel, "..") {
		t.Skipf("test setup produced rel without traversal (%q); platform-dependent", rel)
	}

	patchset := `--- a/` + rel + `
+++ b/` + rel + `
@@ -1 +1 @@
-ORIGINAL — must not be overwritten
+PWNED
`

	step := &config.Step{
		RepoPatch: &config.RepoApplyPatchset{
			Patchset: patchset,
			// Strict defaults to false (lenient).
		},
	}

	_, err = handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute lenient mode should not surface a top-level error for a per-file traversal failure; got %v", err)
	}

	// Most importantly: the file outside baseDir is unchanged.
	got, readErr := os.ReadFile(outsidePath)
	if readErr != nil {
		t.Fatalf("read outside file: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("traversal escape succeeded: outside file was overwritten\ngot:  %q\nwant: %q", got, original)
	}
}

// TestF033_TraversingPathRefused_StrictMode: in strict mode the same
// patchset must (a) leave the outside file untouched and (b) return a
// top-level error naming "escapes patchset base directory".
func TestF033_TraversingPathRefused_StrictMode(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	outsideDir := t.TempDir()
	outsidePath := filepath.Join(outsideDir, "untouched.txt")
	original := "ORIGINAL\n"
	if err := os.WriteFile(outsidePath, []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	rel, err := filepath.Rel(ctx.CurrentDir, outsidePath)
	if err != nil {
		t.Fatalf("rel: %v", err)
	}
	if !strings.HasPrefix(rel, "..") {
		t.Skipf("test setup produced rel without traversal (%q); platform-dependent", rel)
	}

	patchset := `--- a/` + rel + `
+++ b/` + rel + `
@@ -1 +1 @@
-ORIGINAL
+PWNED
`

	step := &config.Step{
		RepoPatch: &config.RepoApplyPatchset{
			Patchset: patchset,
			Strict:   true,
		},
	}

	_, err = handler.Execute(ctx, step)
	if err == nil {
		t.Fatal("expected strict-mode error for traversing patch; got nil")
	}
	if !strings.Contains(err.Error(), "escapes patchset base directory") {
		t.Errorf("strict-mode error should name the escape; got %v", err)
	}

	got, readErr := os.ReadFile(outsidePath)
	if readErr != nil {
		t.Fatalf("read outside file: %v", readErr)
	}
	if string(got) != original {
		t.Errorf("traversal escape succeeded in strict mode: outside file was overwritten\ngot:  %q\nwant: %q", got, original)
	}
}

// Anchor an unused import to executor so the test file still references
// it for IDE goto-definition convenience.
var _ = executor.NewExecutionStats
