package file_replace

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

	if meta.Name != "file_replace" {
		t.Errorf("expected name 'file_replace', got '%s'", meta.Name)
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
			name: "valid configuration",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Path:    "/tmp/test.txt",
					Pattern: "old",
					Replace: "new",
				},
			},
			wantErr: false,
		},
		{
			name: "nil file_replace",
			step: &config.Step{
				FileReplace: nil,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Pattern: "old",
					Replace: "new",
				},
			},
			wantErr: true,
		},
		{
			name: "missing pattern",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Path:    "/tmp/test.txt",
					Replace: "new",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Path:    "/tmp/test.txt",
					Pattern: "[invalid(regex",
					Replace: "new",
					Flags: &config.ReplaceFlags{
						Regex: true,
					},
				},
			},
			wantErr: true,
		},
		{
			name: "zero count",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Path:    "/tmp/test.txt",
					Pattern: "old",
					Replace: "new",
					Count:   ptrInt(0),
				},
			},
			wantErr: true,
		},
		{
			name: "negative count",
			step: &config.Step{
				FileReplace: &config.FileReplace{
					Path:    "/tmp/test.txt",
					Pattern: "old",
					Replace: "new",
					Count:   ptrInt(-1),
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

func TestHandler_Execute_LiteralReplacement(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "Hello old world\nThis is old code\nOld values here"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "old",
			Replace: "new",
			Flags: &config.ReplaceFlags{
				Regex: false, // Literal mode
			},
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

	expected := "Hello new world\nThis is new code\nOld values here" // "Old" with capital O unchanged
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_RegexReplacement(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "oldapi.com/v1\noldapi.com/v2\nnewapi.com/v1"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: `oldapi\.com`,
			Replace: "newapi.com",
			Flags: &config.ReplaceFlags{
				Regex: true,
			},
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

	expected := "newapi.com/v1\nnewapi.com/v2\nnewapi.com/v1"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_CountLimit(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "foo foo foo foo"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "foo",
			Replace: "bar",
			Count:   ptrInt(2),
			Flags: &config.ReplaceFlags{
				Regex: false,
			},
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

	// Verify file content (only first 2 replaced)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "bar bar foo foo"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_CaseInsensitive(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "Error error ERROR"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "error",
			Replace: "warning",
			Flags: &config.ReplaceFlags{
				Regex:           true,
				CaseInsensitive: true,
			},
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

	expected := "warning warning warning"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_NoMatch(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "Hello world"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "notfound",
			Replace: "replacement",
		},
	}

	_, err := handler.Execute(ctx, step)
	if err == nil {
		t.Error("expected error when no matches found")
	}
}

func TestHandler_Execute_NoMatchAllowed(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "Hello world"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:         testFile,
			Pattern:      "notfound",
			Replace:      "replacement",
			AllowNoMatch: true,
		},
	}

	result, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("expected result.Changed to be false when no match")
	}
}

func TestHandler_Execute_Backup(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "original content"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "original",
			Replace: "modified",
			Backup:  true,
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

	// Create test file with already-replaced content
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	content := "Hello new world"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    testFile,
			Pattern: "old",
			Replace: "new",
		},
	}

	// First execution - no match
	result, err := handler.Execute(ctx, step)

	// Should fail because pattern not found and AllowNoMatch is false
	if err == nil {
		t.Error("expected error when pattern not found")
	}

	// Now try with AllowNoMatch
	step.FileReplace.AllowNoMatch = true
	result, err = handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("expected result.Changed to be false (idempotent)")
	}

	// Verify content unchanged
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	if string(newContent) != content {
		t.Error("content was modified when it shouldn't be")
	}
}

func TestHandler_DryRun(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)
	ctx.DryRun = true

	step := &config.Step{
		FileReplace: &config.FileReplace{
			Path:    "/tmp/test.txt",
			Pattern: "old",
			Replace: "new",
			Count:   ptrInt(5),
			Backup:  true,
			Flags: &config.ReplaceFlags{
				Regex:           true,
				Multiline:       true,
				CaseInsensitive: true,
			},
		},
	}

	err := handler.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}
}

// Helper function to create int pointer
func ptrInt(v int) *int {
	return &v
}
