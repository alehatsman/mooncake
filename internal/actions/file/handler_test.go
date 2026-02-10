package file

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// mockExecutionContext creates a minimal ExecutionContext for testing
func mockExecutionContext() *executor.ExecutionContext {
	ctx := testutil.NewMockContext()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return &executor.ExecutionContext{
		Variables:      ctx.Variables,
		Template:       tmpl,
		Evaluator:      ctx.GetEvaluator(),
		Logger:         ctx.Log,
		EventPublisher: ctx.Publisher,
		CurrentStepID:  ctx.StepID,
		PathUtil:       pathutil.NewPathExpander(tmpl),
		CurrentDir:     "/tmp",
		DryRun:         false,
	}
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "file" {
		t.Errorf("Name = %v, want 'file'", meta.Name)
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
	if len(meta.EmitsEvents) != 7 {
		t.Errorf("EmitsEvents length = %d, want 7", len(meta.EmitsEvents))
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
			name: "valid file action",
			step: &config.Step{
				File: &config.File{
					Path: "/tmp/test.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "nil file action",
			step: &config.Step{
				File: nil,
			},
			wantErr: true,
		},
		{
			name: "empty path",
			step: &config.Step{
				File: &config.File{
					Path: "",
				},
			},
			wantErr: true,
		},
		{
			name: "valid directory state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/testdir",
					State: "directory",
				},
			},
			wantErr: false,
		},
		{
			name: "valid absent state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "absent",
				},
			},
			wantErr: false,
		},
		{
			name: "valid touch state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "touch",
				},
			},
			wantErr: false,
		},
		{
			name: "valid link state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "link",
					Src:   "/tmp/target",
				},
			},
			wantErr: false,
		},
		{
			name: "valid hardlink state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "hardlink",
					Src:   "/tmp/target",
				},
			},
			wantErr: false,
		},
		{
			name: "valid perms state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "perms",
					Mode:  "0644",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid state",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "invalid",
				},
			},
			wantErr: true,
		},
		{
			name: "link without src",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "link",
				},
			},
			wantErr: true,
		},
		{
			name: "hardlink without src",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "hardlink",
				},
			},
			wantErr: true,
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

func TestHandler_Execute_CreateFile(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	testContent := "test file content"

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: testContent,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if !execResult.Changed {
		t.Error("Result.Changed should be true for new file")
	}

	if execResult.Failed {
		t.Error("Result.Failed should be false")
	}

	// Verify file was created
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Content = %q, want %q", string(content), testContent)
	}

	// Check event was published
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	event := pub.Events[0]
	if event.Type != events.EventFileCreated {
		t.Errorf("Event.Type = %v, want %v", event.Type, events.EventFileCreated)
	}
}

func TestHandler_Execute_UpdateFile(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	oldContent := "old content"
	newContent := "new content"

	// Create file with old content
	err := os.WriteFile(filePath, []byte(oldContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: newContent,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when content changes")
	}

	// Verify content was updated
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != newContent {
		t.Errorf("Content = %q, want %q", string(content), newContent)
	}

	// Check event type
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventFileUpdated {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventFileUpdated)
		}
	}
}

func TestHandler_Execute_FileIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	testContent := "same content"

	// Create file
	err := os.WriteFile(filePath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: testContent,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when content is identical")
	}
}

func TestHandler_Execute_CreateDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true for new directory")
	}

	// Verify directory was created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Path should be a directory")
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventDirCreated {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventDirCreated)
		}
	}
}

func TestHandler_Execute_DirectoryIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	// Create directory
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when directory already exists")
	}
}

func TestHandler_Execute_RemoveFile(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when removing file")
	}

	// Verify file was removed
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File should not exist")
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventFileRemoved {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventFileRemoved)
		}
	}
}

func TestHandler_Execute_RemoveDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	// Create directory
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when removing directory")
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should not exist")
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventDirRemoved {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventDirRemoved)
		}
	}
}

func TestHandler_Execute_AbsentIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "nonexistent.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when file doesn't exist")
	}
}

func TestHandler_Execute_TouchCreateFile(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when creating file")
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should exist")
	}
}

func TestHandler_Execute_TouchUpdateTime(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file with old timestamp
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	oldTime := time.Now().Add(-1 * time.Hour)
	err = os.Chtimes(filePath, oldTime, oldTime)
	if err != nil {
		t.Fatalf("Failed to set old time: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when updating timestamp")
	}

	// Verify timestamp was updated
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	if info.ModTime().Before(time.Now().Add(-1 * time.Minute)) {
		t.Error("File timestamp should be recent")
	}
}

func TestHandler_Execute_CreateSymlink(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create target file
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   targetPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when creating symlink")
	}

	// Verify symlink was created
	linkTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read link: %v", err)
	}

	if linkTarget != targetPath {
		t.Errorf("Link target = %v, want %v", linkTarget, targetPath)
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventLinkCreated {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventLinkCreated)
		}
		linkData, ok := event.Data.(events.LinkCreatedData)
		if ok && linkData.Type != "symlink" {
			t.Errorf("Link type = %v, want 'symlink'", linkData.Type)
		}
	}
}

func TestHandler_Execute_SymlinkIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create target and symlink
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	err = os.Symlink(targetPath, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   targetPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when symlink is correct")
	}
}

func TestHandler_Execute_SymlinkForce(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	oldTarget := filepath.Join(tmpDir, "old.txt")
	newTarget := filepath.Join(tmpDir, "new.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create both targets
	err := os.WriteFile(oldTarget, []byte("old"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old target: %v", err)
	}
	err = os.WriteFile(newTarget, []byte("new"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new target: %v", err)
	}

	// Create link to old target
	err = os.Symlink(oldTarget, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   newTarget,
			Force: true,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when updating link")
	}

	// Verify link points to new target
	linkTarget, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read link: %v", err)
	}

	if linkTarget != newTarget {
		t.Errorf("Link target = %v, want %v", linkTarget, newTarget)
	}
}

func TestHandler_Execute_SymlinkNoForce(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	oldTarget := filepath.Join(tmpDir, "old.txt")
	newTarget := filepath.Join(tmpDir, "new.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create both targets
	err := os.WriteFile(oldTarget, []byte("old"), 0644)
	if err != nil {
		t.Fatalf("Failed to create old target: %v", err)
	}
	err = os.WriteFile(newTarget, []byte("new"), 0644)
	if err != nil {
		t.Fatalf("Failed to create new target: %v", err)
	}

	// Create link to old target
	err = os.Symlink(oldTarget, linkPath)
	if err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   newTarget,
			Force: false,
		},
	}

	_, err = h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error when link exists with different target and force is false")
	}

	if !strings.Contains(err.Error(), "different target") {
		t.Errorf("Error should mention different target, got: %v", err)
	}
}

func TestHandler_Execute_CreateHardlink(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create target file
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   targetPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when creating hardlink")
	}

	// Verify hardlink was created
	srcInfo, err := os.Stat(targetPath)
	if err != nil {
		t.Fatalf("Failed to stat target: %v", err)
	}

	linkInfo, err := os.Stat(linkPath)
	if err != nil {
		t.Fatalf("Failed to stat link: %v", err)
	}

	if !os.SameFile(srcInfo, linkInfo) {
		t.Error("Files should be the same (hardlinked)")
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventLinkCreated {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventLinkCreated)
		}
		linkData, ok := event.Data.(events.LinkCreatedData)
		if ok && linkData.Type != "hardlink" {
			t.Errorf("Link type = %v, want 'hardlink'", linkData.Type)
		}
	}
}

func TestHandler_Execute_HardlinkIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create target and hardlink
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	err = os.Link(targetPath, linkPath)
	if err != nil {
		t.Fatalf("Failed to create hardlink: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   targetPath,
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when hardlink is correct")
	}
}

func TestHandler_Execute_SetPermissions(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file with default permissions
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when changing permissions")
	}

	// Verify permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("File mode = %o, want 0600", mode)
	}

	// Check event
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) > 0 {
		event := pub.Events[0]
		if event.Type != events.EventPermissionsChanged {
			t.Errorf("Event.Type = %v, want %v", event.Type, events.EventPermissionsChanged)
		}
	}
}

func TestHandler_Execute_PermissionsIdempotent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	// Create file with specific permissions
	err := os.WriteFile(filePath, []byte("content"), 0600)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when permissions are already correct")
	}
}

func TestHandler_Execute_WithMode(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "test content",
			Mode:    "0600",
		},
	}

	_, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0600 {
		t.Errorf("File mode = %o, want 0600", mode)
	}
}

func TestHandler_Execute_WithBackup(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")
	oldContent := "old content"
	newContent := "new content"

	// Create file with old content
	err := os.WriteFile(filePath, []byte(oldContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: newContent,
			Backup:  true,
		},
	}

	_, err = h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check that backup file was created
	backupPath := filePath + ".bak"
	backupContent, err := os.ReadFile(backupPath)
	if err != nil {
		t.Fatalf("Failed to read backup: %v", err)
	}

	if string(backupContent) != oldContent {
		t.Errorf("Backup content = %q, want %q", string(backupContent), oldContent)
	}

	// Verify main file was updated
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != newContent {
		t.Errorf("Content = %q, want %q", string(content), newContent)
	}
}

func TestHandler_Execute_TemplateRendering(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	ec.Variables["username"] = "alice"
	ec.Variables["message"] = "hello"

	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "User: {{ username }}, Message: {{ message }}",
		},
	}

	_, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify content was rendered
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "User: alice, Message: hello"
	if string(content) != expected {
		t.Errorf("Content = %q, want %q", string(content), expected)
	}
}

func TestHandler_Execute_InvalidTemplate(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "{{ invalid template syntax",
		},
	}

	_, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid template")
	}

	if !strings.Contains(err.Error(), "render content") {
		t.Errorf("Error should mention render content, got: %v", err)
	}
}

func TestHandler_Execute_NoPublisher(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	ec.EventPublisher = nil

	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should exist")
	}
}

func TestHandler_Execute_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		File: &config.File{
			Path: "/tmp/test.txt",
		},
	}

	_, err := h.Execute(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

func TestHandler_Execute_UnknownState(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "unknown",
		},
	}

	// Bypass validation to test runtime behavior
	_, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on unknown state")
	}

	if !strings.Contains(err.Error(), "unknown file state") {
		t.Errorf("Error should mention unknown file state, got: %v", err)
	}
}

func TestHandler_DryRun(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{
			name: "dry-run file",
			step: &config.Step{
				File: &config.File{
					Path:    "/tmp/test.txt",
					Content: "test content",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run directory",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/testdir",
					State: "directory",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run absent",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "absent",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run touch",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "touch",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run link",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "link",
					Src:   "/tmp/target",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run hardlink",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/link",
					State: "hardlink",
					Src:   "/tmp/target",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run perms",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					State: "perms",
					Mode:  "0644",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with owner",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					Owner: "root",
				},
			},
			wantErr: false,
		},
		{
			name: "dry-run with group",
			step: &config.Step{
				File: &config.File{
					Path:  "/tmp/test.txt",
					Group: "root",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ec := mockExecutionContext()

			err := h.DryRun(ec, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("DryRun() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// Check that something was logged (if no error expected)
			if !tt.wantErr {
				log := ec.Logger.(*testutil.MockLogger)
				if len(log.Logs) == 0 {
					t.Error("DryRun() should log something")
				}
			}
		})
	}
}

func TestHandler_DryRun_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		File: &config.File{
			Path: "/tmp/test.txt",
		},
	}

	err := h.DryRun(ctx, step)
	if err == nil {
		t.Error("DryRun() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

func TestHandler_parseFileMode(t *testing.T) {
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
			name:        "valid octal mode",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
		{
			name:        "valid mode without leading zero",
			modeStr:     "644",
			defaultMode: 0600,
			want:        0644,
		},
		{
			name:        "invalid mode uses default",
			modeStr:     "invalid",
			defaultMode: 0644,
			want:        0644,
		},
		{
			name:        "restrictive mode",
			modeStr:     "0600",
			defaultMode: 0644,
			want:        0600,
		},
		{
			name:        "executable mode",
			modeStr:     "0755",
			defaultMode: 0644,
			want:        0755,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.parseFileMode(tt.modeStr, tt.defaultMode)
			if got != tt.want {
				t.Errorf("parseFileMode() = %o, want %o", got, tt.want)
			}
		})
	}
}

func TestHandler_formatMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name string
		mode os.FileMode
		want string
	}{
		{
			name: "standard file mode",
			mode: 0644,
			want: "0644",
		},
		{
			name: "executable mode",
			mode: 0755,
			want: "0755",
		},
		{
			name: "restrictive mode",
			mode: 0600,
			want: "0600",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := h.formatMode(tt.mode)
			if got != tt.want {
				t.Errorf("formatMode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHandler_parseUserID(t *testing.T) {
	h := &Handler{}

	// Test with numeric UID
	uid, err := h.parseUserID("1000")
	if err != nil {
		t.Errorf("parseUserID() with numeric UID error = %v", err)
	}
	if uid != 1000 {
		t.Errorf("parseUserID() = %d, want 1000", uid)
	}

	// Test with current user
	currentUser, err := user.Current()
	if err == nil {
		uid, err := h.parseUserID(currentUser.Username)
		if err != nil {
			t.Errorf("parseUserID() with username error = %v", err)
		}
		expectedUID, _ := strconv.Atoi(currentUser.Uid)
		if uid != expectedUID {
			t.Errorf("parseUserID() = %d, want %d", uid, expectedUID)
		}
	}

	// Test with invalid user
	_, err = h.parseUserID("nonexistentuser12345")
	if err == nil {
		t.Error("parseUserID() should error with nonexistent user")
	}
}

func TestHandler_parseGroupID(t *testing.T) {
	h := &Handler{}

	// Test with numeric GID
	gid, err := h.parseGroupID("1000")
	if err != nil {
		t.Errorf("parseGroupID() with numeric GID error = %v", err)
	}
	if gid != 1000 {
		t.Errorf("parseGroupID() = %d, want 1000", gid)
	}

	// Test with invalid group
	_, err = h.parseGroupID("nonexistentgroup12345")
	if err == nil {
		t.Error("parseGroupID() should error with nonexistent group")
	}
}

func TestHandler_Execute_DirectoryWithMode(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
			Mode:  "0700",
		},
	}

	_, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify directory permissions
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	mode := info.Mode().Perm()
	if mode != 0700 {
		t.Errorf("Directory mode = %o, want 0700", mode)
	}
}

func TestHandler_Execute_NestedDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "parent", "child", "grandchild")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when creating nested directories")
	}

	// Verify all directories were created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	if !info.IsDir() {
		t.Error("Path should be a directory")
	}
}

func TestHandler_Execute_RemoveNonEmptyDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "testdir")

	// Create directory with a file
	err := os.Mkdir(dirPath, 0755)
	if err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	filePath := filepath.Join(dirPath, "file.txt")
	err = os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when removing directory")
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should not exist")
	}
}

func TestHandler_Execute_PathExpansion(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()

	ec := mockExecutionContext()
	ec.Variables["basedir"] = tmpDir
	ec.Variables["filename"] = "test.txt"

	step := &config.Step{
		File: &config.File{
			Path:    "{{ basedir }}/{{ filename }}",
			Content: "test content",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file was created at correct path
	expectedPath := filepath.Join(tmpDir, "test.txt")
	content, err := os.ReadFile(expectedPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "test content" {
		t.Errorf("Content = %q, want %q", string(content), "test content")
	}
}

func TestHandler_Execute_InvalidPathExpansion(t *testing.T) {
	h := &Handler{}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path: "{{ invalid template syntax",
		},
	}

	_, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid path template")
	}

	if !strings.Contains(err.Error(), "expand path") {
		t.Errorf("Error should mention expand path, got: %v", err)
	}
}

func TestHandler_Execute_OwnershipLinuxOnly(t *testing.T) {
	// Skip on non-Linux
	if runtime.GOOS != "linux" {
		t.Skip("Ownership test only runs on Linux")
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	// Only run if not root
	if currentUser.Uid == "0" {
		t.Skip("Skipping ownership test when running as root")
	}

	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content",
			Owner:   "root",
		},
	}

	result, err := h.Execute(ec, step)
	if err == nil {
		t.Error("Execute() should error when setting ownership without become")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true when ownership fails")
	}
}

func TestHandler_Execute_StatError(t *testing.T) {
	h := &Handler{}

	// Use a path that will cause stat error
	invalidPath := "/proc/does-not-exist/invalid"

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  invalidPath,
			State: "absent",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		// On some systems, stat on /proc paths might fail with permission error
		// This is acceptable behavior
		execResult := result.(*executor.Result)
		if execResult.Changed {
			t.Error("Result.Changed should be false on stat error")
		}
	}
}

func TestHandler_DryRun_InvalidPathTemplate(t *testing.T) {
	h := &Handler{}

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path: "{{ invalid template syntax",
		},
	}

	err := h.DryRun(ec, step)
	// Should not error in dry-run mode - path expansion failure is handled
	if err != nil {
		t.Errorf("DryRun() should handle path expansion errors gracefully, got: %v", err)
	}

	// Check that something was still logged
	log := ec.Logger.(*testutil.MockLogger)
	if len(log.Logs) == 0 {
		t.Error("DryRun() should log something even on path expansion error")
	}
}

func TestHandler_Execute_EmptyContent(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "empty.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when creating empty file")
	}

	// Verify file was created
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if len(content) != 0 {
		t.Errorf("File should be empty, got %d bytes", len(content))
	}
}

func TestHandler_Execute_DefaultState(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content",
			// State not specified, should default to "file"
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file was created
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should exist")
	}
}

func TestHandler_Execute_ResultTiming(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)

	// Verify timing fields are set
	if execResult.StartTime.IsZero() {
		t.Error("StartTime should be set")
	}

	if execResult.EndTime.IsZero() {
		t.Error("EndTime should be set")
	}

	if execResult.Duration <= 0 {
		t.Error("Duration should be positive")
	}

	if execResult.EndTime.Before(execResult.StartTime) {
		t.Error("EndTime should be after StartTime")
	}
}

func TestHandler_Execute_MultipleEvents(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.txt")

	ec := mockExecutionContext()

	// First execution - create file
	step1 := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content1",
		},
	}

	_, err := h.Execute(ec, step1)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Second execution - update file
	step2 := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "content2",
		},
	}

	_, err = h.Execute(ec, step2)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Verify both events were published
	pub := ec.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(pub.Events))
		return
	}

	if pub.Events[0].Type != events.EventFileCreated {
		t.Errorf("First event should be EventFileCreated, got %v", pub.Events[0].Type)
	}

	if pub.Events[1].Type != events.EventFileUpdated {
		t.Errorf("Second event should be EventFileUpdated, got %v", pub.Events[1].Type)
	}
}

// Additional tests for uncovered functions

func TestHandler_CreateDirectoryWithBecome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	h := &Handler{}
	ec := mockExecutionContext()
	ec.SudoPass = "test-password"

	tmpDir := t.TempDir()
	dirPath := filepath.Join(tmpDir, "become-dir")

	step := &config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
			Mode:  "0755",
		},
		Become: true,
	}

	// Will fail without actual sudo, but tests the code path
	_, err := h.Execute(ec, step)
	t.Logf("createDirectoryWithBecome error (expected): %v", err)
}

func TestHandler_CreateFileWithBecome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	h := &Handler{}
	ec := mockExecutionContext()
	ec.SudoPass = "test-password"

	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "become-file.txt")

	step := &config.Step{
		File: &config.File{
			Path:    filePath,
			Content: "test content",
		},
		Become: true,
	}

	// Will fail without actual sudo, but tests the code path
	_, err := h.Execute(ec, step)
	t.Logf("createFileWithBecome error (expected): %v", err)
}

func TestHandler_RemoveWithBecome(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping become test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "remove-become.txt")

	// Create file first
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	ec.SudoPass = "test-password"

	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
		Become: true,
	}

	// Will fail without actual sudo, but tests the code path
	_, err = h.Execute(ec, step)
	t.Logf("removeWithBecome error (expected): %v", err)
}

func TestHandler_SetOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping ownership test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "ownership-test.txt")

	// Create file
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			Owner: currentUser.Username,
			Group: currentUser.Gid,
		},
	}

	// This should work if we're setting ownership to ourselves
	result, err := h.Execute(ec, step)
	if err != nil && !strings.Contains(err.Error(), "permission") {
		t.Logf("setOwnership error (may be expected): %v", err)
	}

	if result != nil {
		execResult := result.(*executor.Result)
		t.Logf("setOwnership result: changed=%v, failed=%v", execResult.Changed, execResult.Failed)
	}
}

func TestHandler_ChownWithBecome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping chown test on non-Linux")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "chown-become.txt")

	// Create file
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()
	ec.SudoPass = "test-password"

	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			Owner: "root",
			Group: "root",
		},
		Become: true,
	}

	// Will fail without actual sudo, but tests the code path
	_, err = h.Execute(ec, step)
	t.Logf("chownWithBecome error (expected): %v", err)
}

func TestHandler_ParseUserID_WithUsername(t *testing.T) {
	h := &Handler{}

	// Test with root user (should exist on all Unix systems)
	if runtime.GOOS != "windows" {
		uid, err := h.parseUserID("root")
		if err != nil {
			t.Errorf("parseUserID('root') error = %v", err)
		} else if uid != 0 {
			t.Errorf("parseUserID('root') = %d, want 0", uid)
		}
	}
}

func TestHandler_ParseGroupID_WithGroupName(t *testing.T) {
	h := &Handler{}

	// Test with numeric GID
	gid, err := h.parseGroupID("0")
	if err != nil {
		t.Errorf("parseGroupID('0') error = %v", err)
	} else if gid != 0 {
		t.Errorf("parseGroupID('0') = %d, want 0", gid)
	}
}

func TestHandler_TouchFile_WithOwnership(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "touch-ownership.txt")

	ec := mockExecutionContext()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
			Owner: currentUser.Username,
		},
	}

	_, err = h.Execute(ec, step)
	if err != nil {
		t.Logf("touchFile with ownership error (may be expected): %v", err)
	}
}

func TestHandler_Execute_RecursiveDirectory(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	// Create a deeply nested directory structure
	deepPath := filepath.Join(tmpDir, "a", "b", "c", "d", "e", "f")

	ec := mockExecutionContext()
	step := &config.Step{
		File: &config.File{
			Path:  deepPath,
			State: "directory",
			Mode:  "0755",
		},
	}

	result, err := h.Execute(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true for recursive directory creation")
	}

	// Verify directory exists
	if _, err := os.Stat(deepPath); os.IsNotExist(err) {
		t.Error("Deeply nested directory should exist")
	}
}

func TestHandler_Execute_SymlinkWithOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping symlink test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "link.txt")

	// Create target
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	ec := mockExecutionContext()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   targetPath,
			Owner: currentUser.Username,
		},
	}

	_, err = h.Execute(ec, step)
	if err != nil {
		t.Logf("symlink with ownership error (may be expected): %v", err)
	}
}

func TestHandler_Execute_HardlinkWithOwnership(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping hardlink test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	targetPath := filepath.Join(tmpDir, "target.txt")
	linkPath := filepath.Join(tmpDir, "hardlink.txt")

	// Create target
	err := os.WriteFile(targetPath, []byte("target"), 0644)
	if err != nil {
		t.Fatalf("Failed to create target: %v", err)
	}

	ec := mockExecutionContext()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	step := &config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   targetPath,
			Owner: currentUser.Username,
		},
	}

	_, err = h.Execute(ec, step)
	if err != nil {
		t.Logf("hardlink with ownership error (may be expected): %v", err)
	}
}

func TestHandler_Execute_PermsWithOwnership(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "perms-ownership.txt")

	// Create file
	err := os.WriteFile(filePath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	ec := mockExecutionContext()

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	step := &config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
			Owner: currentUser.Username,
		},
	}

	_, err = h.Execute(ec, step)
	if err != nil {
		t.Logf("perms with ownership error (may be expected): %v", err)
	}
}
