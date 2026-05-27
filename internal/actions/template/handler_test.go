package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
)

// Helper to create ExecutionContext for testing
func newTestExecutionContext(ctx *testutil.MockContext, tmpDir string) *executor.ExecutionContext {
	scope := executor.NewVariableScope()
	for k, v := range ctx.Variables {
		scope.User[k] = v
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Logger:         ctx.Log,
			Mode:           ctx.CurrentMode,
			Template:       ctx.Tmpl,
			PathUtil:       pathutil.NewPathExpander(ctx.Tmpl),
			EventPublisher: ctx.Publisher,
		},
		Scope:         scope,
		CurrentDir:    tmpDir,
		CurrentStepID: ctx.StepID,
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "file.template" {
		t.Errorf("Name = %v, want 'template'", meta.Name)
	}
	if meta.Description == "" {
		t.Error("Description is empty")
	}
	if meta.Category != actions.CategoryFile {
		t.Errorf("Category = %v, want %v", meta.Category, actions.CategoryFile)
	}
	if !meta.SupportsDryRun {
		t.Error("SupportsDryRun should be true")
	}
	if !meta.SupportsBecome {
		t.Error("SupportsBecome should be true")
	}
	if len(meta.EmitsEvents) != 1 {
		t.Errorf("EmitsEvents length = %d, want 1", len(meta.EmitsEvents))
	}
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventTemplateRender) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventTemplateRender))
	}
	if meta.Version != "1.0.0" {
		t.Errorf("Version = %v, want '1.0.0'", meta.Version)
	}
}

func TestHandler_Validate(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "valid template action",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src:  "template.j2",
					Dest: "/etc/config",
				},
			},
			wantErr: false,
		},
		{
			name: "nil template action",
			step: &config.Step{
				FileTemplate: nil,
			},
			wantErr: true,
		},
		{
			name: "missing src",
			step: &config.Step{
				FileTemplate: &config.Template{
					Dest: "/etc/config",
				},
			},
			wantErr: true,
		},
		{
			name: "missing dest",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src: "template.j2",
				},
			},
			wantErr: true,
		},
		{
			name: "empty src",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src:  "",
					Dest: "/etc/config",
				},
			},
			wantErr: true,
		},
		{
			name: "empty dest",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src:  "template.j2",
					Dest: "",
				},
			},
			wantErr: true,
		},
		{
			name: "with mode",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src:  "template.j2",
					Dest: "/etc/config",
					Mode: "0755",
				},
			},
			wantErr: false,
		},
		{
			name: "with vars",
			step: &config.Step{
				FileTemplate: &config.Template{
					Src:  "template.j2",
					Dest: "/etc/config",
					Vars: &map[string]interface{}{
						"key": "value",
					},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.Validate(tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestHandler_Run(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name            string
		templateContent string
		variables       map[string]interface{}
		templateVars    *map[string]interface{}
		mode            string
		wantOutput      string
		wantMode        os.FileMode
		wantChanged     bool
		wantErr         bool
	}{
		{
			name:            "simple template",
			templateContent: "Hello, World!",
			variables:       map[string]interface{}{},
			wantOutput:      "Hello, World!",
			wantMode:        0644,
			wantChanged:     true,
			wantErr:         false,
		},
		{
			name:            "template with variable",
			templateContent: "Hello, {{ name }}!",
			variables: map[string]interface{}{
				"name": "Alice",
			},
			wantOutput:  "Hello, Alice!",
			wantMode:    0644,
			wantChanged: true,
			wantErr:     false,
		},
		{
			name:            "template with multiple variables",
			templateContent: "{{ greeting }}, {{ name }}! Count: {{ count }}",
			variables: map[string]interface{}{
				"greeting": "Hi",
				"name":     "Bob",
				"count":    42,
			},
			wantOutput:  "Hi, Bob! Count: 42",
			wantMode:    0644,
			wantChanged: true,
			wantErr:     false,
		},
		{
			name:            "template with custom mode",
			templateContent: "#!/bin/bash\necho hello",
			variables:       map[string]interface{}{},
			mode:            "0755",
			wantOutput:      "#!/bin/bash\necho hello",
			wantMode:        0755,
			wantChanged:     true,
			wantErr:         false,
		},
		{
			name:            "template with additional vars",
			templateContent: "Name: {{ name }}, Age: {{ age }}",
			variables: map[string]interface{}{
				"name": "Charlie",
			},
			templateVars: &map[string]interface{}{
				"age": 30,
			},
			wantOutput:  "Name: Charlie, Age: 30",
			wantMode:    0644,
			wantChanged: true,
			wantErr:     false,
		},
		{
			name:            "template with missing variable renders empty",
			templateContent: "Hello, {{ missing }}!",
			variables:       map[string]interface{}{},
			wantOutput:      "Hello, !",
			wantMode:        0644,
			wantChanged:     true,
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create template file
			srcPath := filepath.Join(tmpDir, "template.j2")
			if err := os.WriteFile(srcPath, []byte(tt.templateContent), 0644); err != nil {
				t.Fatalf("Failed to write template file: %v", err)
			}

			// Destination file
			destPath := filepath.Join(tmpDir, "output.txt")

			// Setup context
			ctx := testutil.NewMockContext()
			ctx.Variables = tt.variables

			// Create ExecutionContext
			execCtx := newTestExecutionContext(ctx, tmpDir)

			// Create step
			step := &config.Step{
				FileTemplate: &config.Template{
					Src:  srcPath,
					Dest: destPath,
					Mode: tt.mode,
					Vars: tt.templateVars,
				},
			}

			// Execute
			result, err := h.Run(execCtx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Execute() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantErr {
				return
			}

			// Check result properties
			execResult, ok := result.(*executor.Result)
			if !ok {
				t.Fatalf("Execute() result is not *executor.Result")
			}

			if execResult.Changed != tt.wantChanged {
				t.Errorf("Result.Changed = %v, want %v", execResult.Changed, tt.wantChanged)
			}

			if execResult.Failed {
				t.Error("Result.Failed should be false")
			}

			// Check file was created with correct content
			content, err := os.ReadFile(destPath)
			if err != nil {
				t.Fatalf("Failed to read output file: %v", err)
			}

			if string(content) != tt.wantOutput {
				t.Errorf("Output content = %q, want %q", string(content), tt.wantOutput)
			}

			// Check file mode
			info, err := os.Stat(destPath)
			if err != nil {
				t.Fatalf("Failed to stat output file: %v", err)
			}

			if info.Mode().Perm() != tt.wantMode {
				t.Errorf("File mode = %o, want %o", info.Mode().Perm(), tt.wantMode)
			}

			// Check event was published
			pub := ctx.Publisher
			if len(pub.Events) != 1 {
				t.Errorf("Expected 1 event to be published, got %d", len(pub.Events))
				return
			}

			event := pub.Events[0]
			if event.Type != events.EventTemplateRender {
				t.Errorf("Event.Type = %v, want %v", event.Type, events.EventTemplateRender)
			}

			templateData, ok := event.Data.(events.TemplateRenderData)
			if !ok {
				t.Fatalf("Event.Data is not events.TemplateRenderData")
			}

			if templateData.TemplatePath != srcPath {
				t.Errorf("TemplateRenderData.TemplatePath = %v, want %v", templateData.TemplatePath, srcPath)
			}

			if templateData.DestPath != destPath {
				t.Errorf("TemplateRenderData.DestPath = %v, want %v", templateData.DestPath, destPath)
			}

			if templateData.Changed != tt.wantChanged {
				t.Errorf("TemplateRenderData.Changed = %v, want %v", templateData.Changed, tt.wantChanged)
			}
		})
	}
}

func TestHandler_Run_Idempotency(t *testing.T) {
	h := &Handler{}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file
	templateContent := "Hello, {{ name }}!"
	srcPath := filepath.Join(tmpDir, "template.j2")
	if err := os.WriteFile(srcPath, []byte(templateContent), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	destPath := filepath.Join(tmpDir, "output.txt")

	// Setup context
	ctx := testutil.NewMockContext()
	ctx.Variables = map[string]interface{}{
		"name": "World",
	}

	execCtx := newTestExecutionContext(ctx, tmpDir)

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	// First execution - should change
	result1, err := h.Run(execCtx, step)
	if err != nil {
		t.Fatalf("First execution failed: %v", err)
	}

	execResult1 := result1.(*executor.Result)
	if !execResult1.Changed {
		t.Error("First execution should report changed=true")
	}

	// Reset publisher events
	ctx.Publisher.Events = []events.Event{}

	// Second execution - should not change (idempotent)
	result2, err := h.Run(execCtx, step)
	if err != nil {
		t.Fatalf("Second execution failed: %v", err)
	}

	execResult2 := result2.(*executor.Result)
	if execResult2.Changed {
		t.Error("Second execution should report changed=false (idempotent)")
	}
}

func TestHandler_Run_MissingTemplateFile(t *testing.T) {
	h := &Handler{}

	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := testutil.NewMockContext()
	execCtx := newTestExecutionContext(ctx, tmpDir)

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  filepath.Join(tmpDir, "nonexistent.j2"),
			Dest: filepath.Join(tmpDir, "output.txt"),
		},
	}

	result, err := h.Run(execCtx, step)
	if err == nil {
		t.Error("Execute() should error when template file doesn't exist")
	}

	if result != nil {
		execResult := result.(*executor.Result)
		if !execResult.Failed {
			t.Error("Result.Failed should be true when template file doesn't exist")
		}
	}
}

func TestHandler_Run_NoPublisher(t *testing.T) {
	h := &Handler{}

	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "template.j2")
	if err := os.WriteFile(srcPath, []byte("Hello!"), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	destPath := filepath.Join(tmpDir, "output.txt")

	ctx := testutil.NewMockContext()
	ctx.Publisher = nil
	execCtx := newTestExecutionContext(ctx, tmpDir)

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(execCtx, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result should indicate change even without publisher")
	}

	// Check file was still created
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "Hello!" {
		t.Errorf("Output content = %q, want 'Hello!'", string(content))
	}
}

func TestHandler_Run_InvalidContext_ApplyMode(t *testing.T) {
	h := &Handler{}

	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "template.j2")
	if err := os.WriteFile(srcPath, []byte("Hello!"), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Use MockContext directly (not ExecutionContext)
	ctx := testutil.NewMockContext()

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  srcPath,
			Dest: filepath.Join(tmpDir, "output.txt"),
		},
	}

	_, err = h.Run(ctx, step)
	if err == nil {
		t.Error("Run() should error when context is not ExecutionContext")
	}

	if err != nil && err.Error() != "context is not an ExecutionContext" {
		t.Errorf("Expected 'context is not an ExecutionContext' error, got: %v", err)
	}
}

func TestHandler_Run_PlanMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name            string
		templateContent string
		variables       map[string]interface{}
		templateVars    *map[string]interface{}
		setupDest       bool // Whether to create dest file
		destContent     string
		wantErr         bool
	}{
		{
			name:            "simple template",
			templateContent: "Hello, World!",
			variables:       map[string]interface{}{},
			wantErr:         false,
		},
		{
			name:            "template with variable",
			templateContent: "Hello, {{ name }}!",
			variables: map[string]interface{}{
				"name": "Alice",
			},
			wantErr: false,
		},
		{
			name:            "template with additional vars",
			templateContent: "Name: {{ name }}, Age: {{ age }}",
			variables: map[string]interface{}{
				"name": "Bob",
			},
			templateVars: &map[string]interface{}{
				"age": 25,
			},
			wantErr: false,
		},
		{
			name:            "dest file already exists - up to date",
			templateContent: "Hello, World!",
			variables:       map[string]interface{}{},
			setupDest:       true,
			destContent:     "Hello, World!",
			wantErr:         false,
		},
		{
			name:            "dest file already exists - needs update",
			templateContent: "Hello, {{ name }}!",
			variables: map[string]interface{}{
				"name": "Alice",
			},
			setupDest:   true,
			destContent: "Hello, Bob!",
			wantErr:     false,
		},
		{
			name:            "template with missing variable",
			templateContent: "Hello, {{ missing }}!",
			variables:       map[string]interface{}{},
			wantErr:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary directory
			tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
			if err != nil {
				t.Fatalf("Failed to create temp dir: %v", err)
			}
			defer os.RemoveAll(tmpDir)

			// Create template file
			srcPath := filepath.Join(tmpDir, "template.j2")
			if err := os.WriteFile(srcPath, []byte(tt.templateContent), 0644); err != nil {
				t.Fatalf("Failed to write template file: %v", err)
			}

			// Destination file
			destPath := filepath.Join(tmpDir, "output.txt")
			if tt.setupDest {
				if err := os.WriteFile(destPath, []byte(tt.destContent), 0644); err != nil {
					t.Fatalf("Failed to write dest file: %v", err)
				}
			}

			// Setup context
			ctx := testutil.NewMockContext()
			ctx.Variables = tt.variables
			ctx.CurrentMode = actions.ModePlan

			execCtx := newTestExecutionContext(ctx, tmpDir)

			// Create step
			step := &config.Step{
				FileTemplate: &config.Template{
					Src:  srcPath,
					Dest: destPath,
					Vars: tt.templateVars,
				},
			}

			// Execute in plan mode
			_, err = h.Run(execCtx, step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run(ModePlan) error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Verify dest file was not modified
			if tt.setupDest {
				content, err := os.ReadFile(destPath)
				if err != nil {
					t.Fatalf("Failed to read dest file: %v", err)
				}
				if string(content) != tt.destContent {
					t.Error("Run(ModePlan) should not modify destination file")
				}
			} else {
				// Verify dest file was not created
				if _, err := os.Stat(destPath); err == nil {
					t.Error("Run(ModePlan) should not create destination file")
				}
			}
		})
	}
}

func TestHandler_Run_PlanMode_MissingTemplateFile(t *testing.T) {
	h := &Handler{}

	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	ctx := testutil.NewMockContext()
	ctx.CurrentMode = actions.ModePlan
	execCtx := newTestExecutionContext(ctx, tmpDir)

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  filepath.Join(tmpDir, "nonexistent.j2"),
			Dest: filepath.Join(tmpDir, "output.txt"),
		},
	}

	_, err = h.Run(execCtx, step)
	if err == nil {
		t.Error("Run(ModePlan) should error when template file doesn't exist")
	}
}

func TestHandler_Run_InvalidContext(t *testing.T) {
	h := &Handler{}

	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	srcPath := filepath.Join(tmpDir, "template.j2")
	if err := os.WriteFile(srcPath, []byte("Hello!"), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	// Use MockContext directly (not ExecutionContext)
	ctx := testutil.NewMockContext()
	ctx.CurrentMode = actions.ModePlan

	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  srcPath,
			Dest: filepath.Join(tmpDir, "output.txt"),
		},
	}

	_, err = h.Run(ctx, step)
	if err == nil {
		t.Error("Run() should error when context is not ExecutionContext")
	}

	if err != nil && err.Error() != "context is not an ExecutionContext" {
		t.Errorf("Expected 'context is not an ExecutionContext' error, got: %v", err)
	}
}

func TestHandler_ParseFileMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name        string
		modeStr     string
		defaultMode os.FileMode
		want        os.FileMode
	}{
		{
			name:        "empty string uses default",
			modeStr:     "",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "valid mode string",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "another valid mode",
			modeStr:     "0600",
			defaultMode: 0644,
			want:        0600,
		},
		{
			name:        "invalid mode uses default",
			modeStr:     "invalid",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "mode without leading zero",
			modeStr:     "755",
			defaultMode: 0644,
			want:        0755,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.parseFileMode(tt.modeStr, tt.defaultMode)
			if got != tt.want {
				t.Errorf("parseFileMode(%q, %o) = %o, want %o", tt.modeStr, tt.defaultMode, got, tt.want)
			}
		})
	}
}

func TestHandler_Run_WithRelativePaths(t *testing.T) {
	h := &Handler{}

	// Create temporary directory
	tmpDir, err := os.MkdirTemp("", "mooncake-template-test-*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create template file
	srcPath := filepath.Join(tmpDir, "template.j2")
	if err := os.WriteFile(srcPath, []byte("Hello, World!"), 0644); err != nil {
		t.Fatalf("Failed to write template file: %v", err)
	}

	destPath := filepath.Join(tmpDir, "output.txt")

	ctx := testutil.NewMockContext()
	execCtx := newTestExecutionContext(ctx, tmpDir)

	// Use relative paths (will be expanded by PathExpander)
	step := &config.Step{
		FileTemplate: &config.Template{
			Src:  "template.j2",
			Dest: "output.txt",
		},
	}

	result, err := h.Run(execCtx, step)
	if err != nil {
		t.Fatalf("Execute() failed with relative paths: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file was created
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read output file: %v", err)
	}

	if string(content) != "Hello, World!" {
		t.Errorf("Output content = %q, want 'Hello, World!'", string(content))
	}
}
