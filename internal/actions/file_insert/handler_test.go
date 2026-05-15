package file_insert

import (
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
		Svc: &executor.RunServices{
			Template: tmpl,
			Evaluator: mockCtx.GetEvaluator(),
			Logger: mockCtx.Log,
			EventPublisher: mockCtx.Publisher,
			PathUtil: pathutil.NewPathExpander(tmpl),
			Mode: actions.ModeApply,
		},
		Scope: executor.NewVariableScope(),
		CurrentStepID: mockCtx.StepID,
		CurrentDir: tmpDir,
	}
}

func TestHandler_Metadata(t *testing.T) {
	handler := &Handler{}
	meta := handler.Metadata()

	if meta.Name != "text.insert" {
		t.Errorf("expected name 'file_insert', got '%s'", meta.Name)
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
				TextInsert: &config.FileInsert{
					Path:     "/tmp/test.txt",
					Anchor:   "import",
					Position: "after",
					Content:  "new line",
				},
			},
			wantErr: false,
		},
		{
			name: "nil file_insert",
			step: &config.Step{
				TextInsert: nil,
			},
			wantErr: true,
		},
		{
			name: "missing path",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Anchor:   "import",
					Position: "after",
					Content:  "new line",
				},
			},
			wantErr: true,
		},
		{
			name: "missing anchor",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Path:     "/tmp/test.txt",
					Position: "after",
					Content:  "new line",
				},
			},
			wantErr: true,
		},
		{
			name: "missing position",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Path:    "/tmp/test.txt",
					Anchor:  "import",
					Content: "new line",
				},
			},
			wantErr: true,
		},
		{
			name: "missing content",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Path:     "/tmp/test.txt",
					Anchor:   "import",
					Position: "after",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid position",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Path:     "/tmp/test.txt",
					Anchor:   "import",
					Position: "invalid",
					Content:  "new line",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid regex",
			step: &config.Step{
				TextInsert: &config.FileInsert{
					Path:     "/tmp/test.txt",
					Anchor:   "[invalid(regex",
					Position: "after",
					Content:  "new line",
					Regex:    true,
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

func TestHandler_Execute_InsertAfter(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nimport foo\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "import foo",
			Position: "after",
			Content:  "import bar",
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

	expected := "line1\nimport foo\nimport bar\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

// MT-84: text.insert must be idempotent. Running the same insert
// against a file that already has the insertion immediately after
// the anchor must NOT duplicate the line — Changed=false instead.
func TestHandler_Execute_InsertAfter_Idempotent(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	testFile := filepath.Join(ctx.CurrentDir, "idem.txt")
	originalContent := "line1\nimport foo\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "import foo",
			Position: "after",
			Content:  "import bar",
		},
	}

	// First run: changes are made.
	if _, err := handler.Execute(ctx, step); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	afterFirst, _ := os.ReadFile(testFile)
	want := "line1\nimport foo\nimport bar\nline3"
	if string(afterFirst) != want {
		t.Fatalf("after first run:\nwant: %s\ngot:  %s", want, afterFirst)
	}

	// Second run: same step, content already in place — must be no-op.
	res2, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if res2.(*executor.Result).Changed {
		t.Errorf("second run should report Changed=false (content already present)")
	}
	afterSecond, _ := os.ReadFile(testFile)
	if string(afterSecond) != want {
		t.Errorf("second run mutated file:\nwant: %s\ngot:  %s", want, afterSecond)
	}
}

// MT-84: same shape for position=before — the previous line, not
// the next.
func TestHandler_Execute_InsertBefore_Idempotent(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	testFile := filepath.Join(ctx.CurrentDir, "idem-before.txt")
	originalContent := "line1\nimport foo\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "import foo",
			Position: "before",
			Content:  "import bar",
		},
	}

	if _, err := handler.Execute(ctx, step); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	want := "line1\nimport bar\nimport foo\nline3"
	if got, _ := os.ReadFile(testFile); string(got) != want {
		t.Fatalf("after first run:\nwant: %s\ngot:  %s", want, got)
	}

	res2, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if res2.(*executor.Result).Changed {
		t.Errorf("second run should report Changed=false (content already present)")
	}
}

// MT-84: multi-line insertion content must check all lines, not
// just the first.
func TestHandler_Execute_InsertAfter_Idempotent_MultiLine(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	testFile := filepath.Join(ctx.CurrentDir, "idem-multi.txt")
	originalContent := "line1\n[server]\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "[server]",
			Position: "after",
			Content:  "host=localhost\nport=8080",
		},
	}

	if _, err := handler.Execute(ctx, step); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	res2, err := handler.Execute(ctx, step)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if res2.(*executor.Result).Changed {
		t.Errorf("second run should report Changed=false for multi-line insertion already present")
	}

	got, _ := os.ReadFile(testFile)
	// Expect a single instance of "host=localhost\nport=8080" after
	// the anchor — not duplicated.
	if c := strings.Count(string(got), "host=localhost"); c != 1 {
		t.Errorf("expected exactly 1 occurrence of insertion content; got %d in: %s", c, got)
	}
}

func TestHandler_Execute_InsertBefore(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nexport default\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "export default",
			Position: "before",
			Content:  "// Comment",
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

	expected := "line1\n// Comment\nexport default\nline3"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_RegexAnchor(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "import { foo } from './foo'\nconst x = 1"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   `^import.*from`,
			Position: "after",
			Content:  "import { bar } from './bar'",
			Regex:    true,
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

	expected := "import { foo } from './foo'\nimport { bar } from './bar'\nconst x = 1"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_AllowMultiple(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with multiple anchors
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "import foo\nsome code\nimport bar\nmore code"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:          testFile,
			Anchor:        "import",
			Position:      "after",
			Content:       "// Added",
			AllowMultiple: true,
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

	// Verify file content (inserted after both imports)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "import foo\n// Added\nsome code\nimport bar\n// Added\nmore code"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_SingleMatch(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file with multiple potential anchors
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "import foo\nsome code\nimport bar"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:          testFile,
			Anchor:        "import",
			Position:      "after",
			Content:       "// Added",
			AllowMultiple: false, // Only first match
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

	// Verify file content (inserted after first import only)
	newContent, err := os.ReadFile(testFile)
	if err != nil {
		t.Fatal(err)
	}

	expected := "import foo\n// Added\nsome code\nimport bar"
	if string(newContent) != expected {
		t.Errorf("expected content:\n%s\ngot:\n%s", expected, string(newContent))
	}
}

func TestHandler_Execute_AnchorNotFound(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nline2"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "not found",
			Position: "after",
			Content:  "new content",
		},
	}

	// MT-47: anchor not found is idempotent success — no error, no
	// change. (Original behavior was fail-loud; that broke re-runs of
	// playbooks whose first run altered or removed the anchor.)
	res, err := handler.Execute(ctx, step)
	if err != nil {
		t.Errorf("expected no error on missing anchor (MT-47); got %v", err)
	}
	if r, ok := res.(*executor.Result); ok && r.Changed {
		t.Error("Changed should be false when anchor not found")
	}
	got, _ := os.ReadFile(testFile)
	if string(got) != originalContent {
		t.Errorf("file mutated despite missing anchor: %q", got)
	}
}

func TestHandler_Execute_Backup(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)

	// Create test file
	testFile := filepath.Join(ctx.CurrentDir, "test.txt")
	originalContent := "line1\nanchor\nline3"
	if err := os.WriteFile(testFile, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:     testFile,
			Anchor:   "anchor",
			Position: "after",
			Content:  "inserted",
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

func TestHandler_DryRun(t *testing.T) {
	handler := &Handler{}
	ctx := createTestContext(t)
	ctx.Svc.Mode = actions.ModePlan

	step := &config.Step{
		TextInsert: &config.FileInsert{
			Path:          "/tmp/test.txt",
			Anchor:        "import",
			Position:      "after",
			Content:       "new import",
			Regex:         true,
			AllowMultiple: true,
			Backup:        true,
		},
	}

	err := handler.DryRun(ctx, step)
	if err != nil {
		t.Errorf("DryRun() error = %v", err)
	}
}
