package repo_apply_patchset

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// Test helper to create a test execution context
func createTestContext(t *testing.T) *executor.ExecutionContext {
	t.Helper()

	tmpDir := t.TempDir()
	mockCtx := testutil.NewMockContext()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("Failed to create renderer: %v", err)
	}

	return &executor.ExecutionContext{
		Variables:      mockCtx.Variables,
		Template:       tmpl,
		Evaluator:      mockCtx.GetEvaluator(),
		Logger:         mockCtx.Log,
		EventPublisher: mockCtx.Publisher,
		CurrentStepID:  mockCtx.StepID,
		PathUtil:       pathutil.NewPathExpander(tmpl),
		CurrentDir:     tmpDir,
		DryRun:         false,
	}
}

func TestHandler_Metadata(t *testing.T) {
	handler := &Handler{}
	meta := handler.Metadata()

	if meta.Name != "repo_apply_patchset" {
		t.Errorf("expected name 'repo_apply_patchset', got '%s'", meta.Name)
	}

	if meta.Category != actions.CategoryFile {
		t.Errorf("expected category CategoryFile, got '%s'", meta.Category)
	}

	if !meta.SupportsDryRun {
		t.Error("expected SupportsDryRun to be true")
	}

	if !meta.ImplementsCheck {
		t.Error("expected ImplementsCheck to be true")
	}
}

func TestHandler_Validate(t *testing.T) {
	handler := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid inline patchset",
			step: &config.Step{
				RepoApplyPatchset: &config.RepoApplyPatchset{
					Patchset: "--- a/file.txt\n+++ b/file.txt\n@@ -1,1 +1,1 @@\n-old\n+new",
				},
			},
			wantErr: false,
		},
		{
			name: "valid patchset file",
			step: &config.Step{
				RepoApplyPatchset: &config.RepoApplyPatchset{
					PatchsetFile: "/tmp/changes.patch",
				},
			},
			wantErr: false,
		},
		{
			name: "nil repo_apply_patchset",
			step: &config.Step{
				RepoApplyPatchset: nil,
			},
			wantErr: true,
		},
		{
			name: "missing patchset and patchset_file",
			step: &config.Step{
				RepoApplyPatchset: &config.RepoApplyPatchset{},
			},
			wantErr: true,
		},
		{
			name: "both patchset and patchset_file specified",
			step: &config.Step{
				RepoApplyPatchset: &config.RepoApplyPatchset{
					Patchset:     "patch content",
					PatchsetFile: "/tmp/patch.txt",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := handler.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Execute_SingleFile(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "config.txt")
	originalContent := "host=localhost\nport=3000\ndebug=true"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create patchset for single file
	patchset := `--- a/config.txt
+++ b/config.txt
@@ -1,3 +1,3 @@
 host=localhost
-port=3000
+port=8080
 debug=true`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("expected result.Changed to be true")
	}

	// Verify file content
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "host=localhost\nport=8080\ndebug=true"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_MultipleFiles(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test files
	file1 := filepath.Join(ctx.CurrentDir, "api.js")
	file2 := filepath.Join(ctx.CurrentDir, "db.js")

	if err := os.WriteFile(file1, []byte("const PORT = 3000;\nconst HOST = 'localhost';"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("const DB = 'mongodb://localhost:27017';"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create patchset for multiple files
	patchset := `--- a/api.js
+++ b/api.js
@@ -1,2 +1,2 @@
-const PORT = 3000;
+const PORT = 8080;
 const HOST = 'localhost';
--- a/db.js
+++ b/db.js
@@ -1,1 +1,1 @@
-const DB = 'mongodb://localhost:27017';
+const DB = 'mongodb://db.prod.com:27017';`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("expected result.Changed to be true")
	}

	// Verify both files changed
	api, _ := os.ReadFile(file1)
	if !contains(string(api), "8080") {
		t.Error("api.js was not patched correctly")
	}

	db, _ := os.ReadFile(file2)
	if !contains(string(db), "db.prod.com") {
		t.Error("db.js was not patched correctly")
	}
}

func TestHandler_Execute_StrictMode_Rollback(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test files
	file1 := filepath.Join(ctx.CurrentDir, "good.txt")
	file2 := filepath.Join(ctx.CurrentDir, "bad.txt")

	if err := os.WriteFile(file1, []byte("line1\nline2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Patchset where second file will fail
	patchset := `--- a/good.txt
+++ b/good.txt
@@ -1,2 +1,2 @@
-line1
+modified1
 line2
--- a/bad.txt
+++ b/bad.txt
@@ -1,1 +1,1 @@
-this will not match
+new content`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
			Strict:   true, // Rollback on any failure
		},
	}

	_, err := handler.Execute(ctx, step)
	if err == nil {
		t.Error("expected error in strict mode when patch fails")
	}

	// Verify good.txt was rolled back (unchanged)
	content1, _ := os.ReadFile(file1)
	if string(content1) != "line1\nline2" {
		t.Errorf("expected file to be rolled back, got: %s", string(content1))
	}
}

func TestHandler_Execute_LenientMode(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test files
	file1 := filepath.Join(ctx.CurrentDir, "good.txt")
	file2 := filepath.Join(ctx.CurrentDir, "bad.txt")

	if err := os.WriteFile(file1, []byte("line1\nline2"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file2, []byte("different content"), 0644); err != nil {
		t.Fatal(err)
	}

	// Patchset where second file will fail
	patchset := `--- a/good.txt
+++ b/good.txt
@@ -1,2 +1,2 @@
-line1
+modified1
 line2
--- a/bad.txt
+++ b/bad.txt
@@ -1,1 +1,1 @@
-this will not match
+new content`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
			Strict:   false, // Don't rollback on failures
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v (lenient mode should not error)", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("expected result.Changed to be true")
	}

	// Verify good.txt was patched
	content1, _ := os.ReadFile(file1)
	if !contains(string(content1), "modified1") {
		t.Error("good.txt should have been patched")
	}

	// Verify bad.txt was NOT patched (original content preserved)
	content2, _ := os.ReadFile(file2)
	if string(content2) != "different content" {
		t.Error("bad.txt should have preserved original content")
	}
}

func TestHandler_Execute_Backup(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "config.txt")
	originalContent := "original=content"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	patchset := `--- a/config.txt
+++ b/config.txt
@@ -1,1 +1,1 @@
-original=content
+modified=content`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
			Backup:   true,
		},
	}

	_, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify backup file exists
	backupFile := testFile + ".bak"
	backupContent, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("backup file not created: %v", err)
	}

	if string(backupContent) != originalContent {
		t.Errorf("backup content doesn't match original:\nexpected: %s\ngot: %s",
			originalContent, string(backupContent))
	}
}

func TestHandler_Execute_JSONOutput(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("line1\nline2"), 0644); err != nil {
		t.Fatal(err)
	}

	outputFile := filepath.Join(ctx.CurrentDir, "results.json")

	patchset := `--- a/test.txt
+++ b/test.txt
@@ -1,2 +1,2 @@
-line1
+modified
 line2`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset:   patchset,
			OutputFile: outputFile,
		},
	}

	_, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify JSON output file
	jsonData, err := os.ReadFile(outputFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}

	var results []*PatchResult
	if err := json.Unmarshal(jsonData, &results); err != nil {
		t.Fatalf("failed to parse JSON output: %v", err)
	}

	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}

	if results[0].File != "test.txt" {
		t.Errorf("expected file 'test.txt', got '%s'", results[0].File)
	}

	if !results[0].Success {
		t.Error("expected success to be true")
	}
}

func TestHandler_Execute_Idempotency(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with already-patched content
	testFile := filepath.Join(ctx.CurrentDir, "config.txt")
	patchedContent := "port=8080"
	if err := os.WriteFile(testFile, []byte(patchedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Apply patch that's already applied
	patchset := `--- a/config.txt
+++ b/config.txt
@@ -1,1 +1,1 @@
-port=3000
+port=8080`

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset: patchset,
			Strict:   false, // Lenient mode (don't fail on mismatch)
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	// Should report unchanged since patch couldn't be applied
	if execResult.Changed {
		t.Error("expected result.Changed to be false for already-patched file")
	}
}

func TestHandler_DryRun(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)
	ctx.DryRun = true

	step := &config.Step{
		RepoApplyPatchset: &config.RepoApplyPatchset{
			Patchset:   "--- a/file.txt\n+++ b/file.txt\n@@ -1,1 +1,1 @@\n-old\n+new",
			Strict:     true,
			Backup:     true,
			OutputFile: "/tmp/results.json",
		},
	}

	err := handler.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}
}

// Helper function
func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && strings.Contains(s, substr)
}
