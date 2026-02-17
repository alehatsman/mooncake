package file_delete_range

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

	if meta.Name != "file_delete_range" {
		t.Errorf("expected name 'file_delete_range', got '%s'", meta.Name)
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
				FileDeleteRange: &config.FileDeleteRange{
					Path:        "/tmp/test.txt",
					StartAnchor: "BEGIN",
					EndAnchor:   "END",
				},
			},
			wantErr: false,
		},
		{
			name: "nil file_delete_range",
			step: &config.Step{
				FileDeleteRange: nil,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			step: &config.Step{
				FileDeleteRange: &config.FileDeleteRange{
					StartAnchor: "BEGIN",
					EndAnchor:   "END",
				},
			},
			wantErr: true,
		},
		{
			name: "missing start_anchor",
			step: &config.Step{
				FileDeleteRange: &config.FileDeleteRange{
					Path:      "/tmp/test.txt",
					EndAnchor: "END",
				},
			},
			wantErr: true,
		},
		{
			name: "missing end_anchor",
			step: &config.Step{
				FileDeleteRange: &config.FileDeleteRange{
					Path:        "/tmp/test.txt",
					StartAnchor: "BEGIN",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid start regex",
			step: &config.Step{
				FileDeleteRange: &config.FileDeleteRange{
					Path:        "/tmp/test.txt",
					StartAnchor: "[invalid(regex",
					EndAnchor:   "END",
					Regex:       true,
				},
			},
			wantErr: true,
		},
		{
			name: "invalid end regex",
			step: &config.Step{
				FileDeleteRange: &config.FileDeleteRange{
					Path:        "/tmp/test.txt",
					StartAnchor: "BEGIN",
					EndAnchor:   "[invalid(regex",
					Regex:       true,
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

func TestHandler_Execute_ExclusiveDelete(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\n// BEGIN DEPRECATED\nold code 1\nold code 2\n// END DEPRECATED\nline6"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "// BEGIN DEPRECATED",
			EndAnchor:   "// END DEPRECATED",
			Inclusive:   false, // Keep anchor lines
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

	// Verify file content (anchor lines kept, content between deleted)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "line1\n// BEGIN DEPRECATED\n// END DEPRECATED\nline6"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_InclusiveDelete(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\n// BEGIN\ndelete this\nand this\n// END\nline6"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "// BEGIN",
			EndAnchor:   "// END",
			Inclusive:   true, // Delete anchor lines too
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

	// Verify file content (everything deleted including anchors)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "line1\nline6"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_RegexAnchors(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "header\n<script>\nalert('xss')\nconsole.log('bad')\n</script>\nfooter"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: `^<script>$`,
			EndAnchor:   `^</script>$`,
			Regex:       true,
			Inclusive:   true,
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

	expected := "header\nfooter"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_StartAnchorNotFound(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "NOT FOUND",
			EndAnchor:   "END",
		},
	}

	_, err := handler.Execute(ctx, step)
	if err == nil {
		t.Error("expected error when start anchor not found")
	}
}

func TestHandler_Execute_EndAnchorNotFound(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "START\nline2\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "START",
			EndAnchor:   "NOT FOUND",
		},
	}

	_, err := handler.Execute(ctx, step)
	if err == nil {
		t.Error("expected error when end anchor not found")
	}
}

func TestHandler_Execute_Backup(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "keep\nBEGIN\ndelete\nEND\nkeep"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "BEGIN",
			EndAnchor:   "END",
			Inclusive:   true,
			Backup:      true,
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

func TestHandler_Execute_EmptyRange(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with adjacent anchors
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nSTART\nEND\nline4"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        testFile,
			StartAnchor: "START",
			EndAnchor:   "END",
			Inclusive:   true,
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

	// Verify file content (both anchors deleted, nothing between)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "line1\nline4"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_DryRun(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)
	ctx.DryRun = true

	step := &config.Step{
		FileDeleteRange: &config.FileDeleteRange{
			Path:        "/tmp/test.txt",
			StartAnchor: "BEGIN",
			EndAnchor:   "END",
			Regex:       true,
			Inclusive:   true,
			Backup:      true,
		},
	}

	err := handler.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}
}
