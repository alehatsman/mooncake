package file_patch_apply

import (
	"os"
	"path/filepath"
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

	if meta.Name != "file_patch_apply" {
		t.Errorf("expected name 'file_patch_apply', got '%s'", meta.Name)
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
			name: "valid inline patch",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Path:  "/tmp/test.txt",
					Patch: "@@ -1,3 +1,3 @@\n line1\n-old\n+new\n line3",
				},
			},
			wantErr: false,
		},
		{
			name: "valid patch file",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Path:      "/tmp/test.txt",
					PatchFile: "/tmp/test.patch",
				},
			},
			wantErr: false,
		},
		{
			name: "nil file_patch_apply",
			step: &config.Step{
				FilePatchApply: nil,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Patch: "some patch",
				},
			},
			wantErr: true,
		},
		{
			name: "missing patch and patch_file",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Path: "/tmp/test.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "both patch and patch_file specified",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Path:      "/tmp/test.txt",
					Patch:     "patch content",
					PatchFile: "/tmp/test.patch",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid context_lines",
			step: &config.Step{
				FilePatchApply: &config.FilePatchApply{
					Path:         "/tmp/test.txt",
					Patch:        "patch",
					ContextLines: intPtr(-1),
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

func TestHandler_Execute_SimpleInlinePatch(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nold content\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create simple unified diff patch
	patch := `@@ -1,3 +1,3 @@
 line1
-old content
+new content
 line3`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  testFile,
			Patch: patch,
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

	expected := "line1\nnew content\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_PatchFromFile(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Create patch file
	patchFile := filepath.Join(ctx.CurrentDir, "test.patch")
	patchContent := `@@ -1,3 +1,3 @@
 line1
-line2
+modified line2
 line3`
	if err := os.WriteFile(patchFile, []byte(patchContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:      testFile,
			PatchFile: patchFile,
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

	expected := "line1\nmodified line2\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_MultipleHunks(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "header\nfunction1() {\n  old code\n}\nfunction2() {\n  old code\n}\nfooter"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch with multiple hunks
	patch := `@@ -1,4 +1,4 @@
 header
 function1() {
-  old code
+  new code
 }
@@ -5,4 +5,4 @@
 function2() {
-  old code
+  new code
 }
 footer`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  testFile,
			Patch: patch,
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

	// Verify both hunks were applied
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "header\nfunction1() {\n  new code\n}\nfunction2() {\n  new code\n}\nfooter"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_StrictMode(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with content that won't match the patch
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\ndifferent content\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch that expects different content
	patch := `@@ -1,3 +1,3 @@
 line1
-old content
+new content
 line3`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:   testFile,
			Patch:  patch,
			Strict: true,
		},
	}

	_, err := handler.Execute(ctx, step)
	if err == nil {
		t.Error("expected error in strict mode when patch fails")
	}
}

func TestHandler_Execute_Backup(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "original\ncontent"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	patch := `@@ -1,2 +1,2 @@
-original
+modified
 content`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:   testFile,
			Patch:  patch,
			Backup: true,
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

func TestHandler_Execute_Idempotency(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with already-patched content
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	patchedContent := "line1\nnew content\nline3"
	if err := os.WriteFile(testFile, []byte(patchedContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Apply patch that's already applied
	patch := `@@ -1,3 +1,3 @@
 line1
-old content
+new content
 line3`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  testFile,
			Patch: patch,
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	// Should report unchanged since patch already applied
	if execResult.Changed {
		t.Error("expected result.Changed to be false for already-patched file")
	}
}

func TestHandler_Execute_AdditionPatch(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch that adds a line
	patch := `@@ -1,2 +1,3 @@
 line1
+line2
 line3`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  testFile,
			Patch: patch,
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

	expected := "line1\nline2\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_DeletionPatch(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Patch that removes a line
	patch := `@@ -1,3 +1,2 @@
 line1
-line2
 line3`

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:  testFile,
			Patch: patch,
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

	expected := "line1\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_DryRun(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)
	ctx.DryRun = true

	step := &config.Step{
		FilePatchApply: &config.FilePatchApply{
			Path:   "/tmp/test.txt",
			Patch:  "@@ -1,1 +1,1 @@\n-old\n+new",
			Strict: true,
			Backup: true,
		},
	}

	err := handler.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}
}

// Helper function
func intPtr(i int) *int {
	return &i
}
