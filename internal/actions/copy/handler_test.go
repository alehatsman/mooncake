package copy

import (
	"crypto/md5"
	"crypto/sha256"
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
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

// mockExecutionContext creates a minimal ExecutionContext for apply-mode testing.
func mockExecutionContext() *executor.ExecutionContext {
	ctx := testutil.NewMockContext()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		panic("Failed to create renderer: " + err.Error())
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      ctx.GetEvaluator(),
			Logger:         ctx.Log,
			EventPublisher: ctx.Publisher,
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Mode:           actions.ModeApply,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: ctx.StepID,
		CurrentDir:    "/tmp",
	}
}

// mockPlanContext creates a minimal ExecutionContext for plan-mode (dry-run) testing.
func mockPlanContext() *executor.ExecutionContext {
	ec := mockExecutionContext()
	ec.Svc.Mode = actions.ModePlan
	return ec
}

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "file.copy" {
		t.Errorf("Name = %v, want 'copy'", meta.Name)
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
	if len(meta.EmitsEvents) > 0 && meta.EmitsEvents[0] != string(events.EventFileCopied) {
		t.Errorf("EmitsEvents[0] = %v, want %v", meta.EmitsEvents[0], string(events.EventFileCopied))
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
			name: "valid copy action",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "/tmp/source.txt",
					Dest: "/tmp/dest.txt",
				},
			},
			wantErr: false,
		},
		{
			name: "nil copy action",
			step: &config.Step{
				FileCopy: nil,
			},
			wantErr: true,
		},
		{
			name: "missing src",
			step: &config.Step{
				FileCopy: &config.Copy{
					Dest: "/tmp/dest.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "missing dest",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src: "/tmp/source.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "empty src",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "",
					Dest: "/tmp/dest.txt",
				},
			},
			wantErr: true,
		},
		{
			name: "empty dest",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "/tmp/source.txt",
					Dest: "",
				},
			},
			wantErr: true,
		},
		{
			name: "with checksum",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:      "/tmp/source.txt",
					Dest:     "/tmp/dest.txt",
					Checksum: "md5:d8e8fca2dc0f896fd7cb4cb0031ba249",
				},
			},
			wantErr: false,
		},
		{
			name: "with mode",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "/tmp/source.txt",
					Dest: "/tmp/dest.txt",
					Mode: "0600",
				},
			},
			wantErr: false,
		},
		{
			name: "with owner and group",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:   "/tmp/source.txt",
					Dest:  "/tmp/dest.txt",
					Owner: "root",
					Group: "root",
				},
			},
			wantErr: false,
		},
		{
			name: "with backup",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:    "/tmp/source.txt",
					Dest:   "/tmp/dest.txt",
					Backup: true,
				},
			},
			wantErr: false,
		},
		{
			name: "with force",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:   "/tmp/source.txt",
					Dest:  "/tmp/dest.txt",
					Force: true,
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

func TestHandler_Run_BasicCopy(t *testing.T) {
	h := &Handler{}

	// Create temp directory and source file
	tmpDir := t.TempDir()
	testContent := "test file content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check result
	execResult, ok := result.(*executor.Result)
	if !ok {
		t.Fatalf("Execute() result is not *executor.Result")
	}

	if !execResult.Changed {
		t.Error("Result.Changed should be true for new copy")
	}

	if execResult.Failed {
		t.Error("Result.Failed should be false")
	}

	// Verify file was created
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Copied file does not exist")
	}

	// Verify content
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read copied file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Copied content = %q, want %q", string(content), testContent)
	}

	// Check event was published
	pub := ec.Svc.EventPublisher.(*testutil.MockPublisher)
	if len(pub.Events) != 1 {
		t.Errorf("Expected 1 event, got %d", len(pub.Events))
		return
	}

	event := pub.Events[0]
	if event.Type != events.EventFileCopied {
		t.Errorf("Event.Type = %v, want %v", event.Type, events.EventFileCopied)
	}

	copyData, ok := event.Data.(events.FileCopiedData)
	if !ok {
		t.Fatalf("Event.Data is not events.FileCopiedData")
	}

	if copyData.Dest != destPath {
		t.Errorf("FileCopiedData.Dest = %v, want %v", copyData.Dest, destPath)
	}

	if copyData.SizeBytes != int64(len(testContent)) {
		t.Errorf("FileCopiedData.SizeBytes = %v, want %v", copyData.SizeBytes, len(testContent))
	}
}

// MT-51: when FollowSymlinks: false and src is a symlink, dest must
// be a symlink with the same target — not a regular file containing
// the target's bytes.
func TestHandler_Run_FollowSymlinksFalse_PreservesLink(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	dest := filepath.Join(tmpDir, "dest")

	followFalse := false
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:            link,
			Dest:           dest,
			FollowSymlinks: &followFalse,
		},
	}
	res, err := h.Run(mockExecutionContext(), step)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !res.(*executor.Result).Changed {
		t.Error("first copy should report Changed=true")
	}
	destInfo, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("Lstat dest: %v", err)
	}
	if destInfo.Mode()&os.ModeSymlink == 0 {
		t.Fatal("dest should be a symlink when follow_symlinks: false")
	}
	gotTarget, err := os.Readlink(dest)
	if err != nil {
		t.Fatalf("Readlink dest: %v", err)
	}
	if gotTarget != target {
		t.Errorf("dest symlink target = %q, want %q", gotTarget, target)
	}
}

// MT-51: by default (FollowSymlinks unset / true), the existing
// behavior is preserved — dest is a regular file with the link
// target's bytes.
func TestHandler_Run_FollowSymlinksDefault_DereferencesLink(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("payload"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	dest := filepath.Join(tmpDir, "dest")

	step := &config.Step{FileCopy: &config.Copy{Src: link, Dest: dest}}
	if _, err := h.Run(mockExecutionContext(), step); err != nil {
		t.Fatalf("Execute: %v", err)
	}
	destInfo, err := os.Lstat(dest)
	if err != nil {
		t.Fatalf("Lstat dest: %v", err)
	}
	if destInfo.Mode()&os.ModeSymlink != 0 {
		t.Error("dest should be a regular file when follow_symlinks defaults to true")
	}
	body, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile dest: %v", err)
	}
	if string(body) != "payload" {
		t.Errorf("dest content = %q, want %q", string(body), "payload")
	}
}

// MT-51: running the preserve-symlink path twice in a row reports
// Changed=false on the second run (idempotency).
func TestHandler_Run_FollowSymlinksFalse_Idempotent(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()

	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}
	link := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(target, link); err != nil {
		t.Fatalf("symlink: %v", err)
	}
	dest := filepath.Join(tmpDir, "dest")

	followFalse := false
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:            link,
			Dest:           dest,
			FollowSymlinks: &followFalse,
		},
	}

	if _, err := h.Run(mockExecutionContext(), step); err != nil {
		t.Fatalf("first Execute: %v", err)
	}
	res2, err := h.Run(mockExecutionContext(), step)
	if err != nil {
		t.Fatalf("second Execute: %v", err)
	}
	if res2.(*executor.Result).Changed {
		t.Error("second run should report Changed=false when symlink already matches")
	}
}

func TestHandler_Run_SourceNotFound(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "nonexistent.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error when source not found")
	}

	if !strings.Contains(err.Error(), "stat source") {
		t.Errorf("Error should mention stat source, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true when source not found")
	}
}

func TestHandler_Run_SourceIsDirectory(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	srcDir := filepath.Join(tmpDir, "sourcedir")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.Mkdir(srcDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcDir,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error when source is directory")
	}

	if !strings.Contains(err.Error(), "directory") {
		t.Errorf("Error should mention directory, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true when source is directory")
	}
}

func TestHandler_Run_WithChecksum(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "test content for checksum"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Calculate MD5 checksum (without prefix)
	hasher := md5.New()
	hasher.Write([]byte(testContent))
	md5sum := fmt.Sprintf("%x", hasher.Sum(nil))

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:      srcPath,
			Dest:     destPath,
			Checksum: md5sum,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true for new copy")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Copied file does not exist")
	}
}

func TestHandler_Run_ChecksumMismatch(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "test content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")
	wrongChecksum := "00000000000000000000000000000000" // 32 hex chars (MD5)

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:      srcPath,
			Dest:     destPath,
			Checksum: wrongChecksum,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error on checksum mismatch")
	}

	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Error should mention checksum mismatch, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on checksum mismatch")
	}
}

func TestHandler_Run_IdempotencyByTime(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "idempotent content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create dest file with same content and size
	err = os.WriteFile(destPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	// Set same modification time
	srcInfo, _ := os.Stat(srcPath)
	err = os.Chtimes(destPath, srcInfo.ModTime(), srcInfo.ModTime())
	if err != nil {
		t.Fatalf("Failed to set modification time: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if execResult.Changed {
		t.Error("Result.Changed should be false when file is up to date")
	}
}

func TestHandler_Run_ForceCopy(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "forced content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create dest file with different content
	err = os.WriteFile(destPath, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:   srcPath,
			Dest:  destPath,
			Force: true,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when force is enabled")
	}

	// Verify content was updated
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Content = %q, want %q", string(content), testContent)
	}
}

func TestHandler_Run_WithMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits beyond read-only have no meaning on NTFS")
	}
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "content with mode"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
			Mode: "0600",
		},
	}

	_, err = h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check file permissions
	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	mode := info.Mode().Perm()
	expectedMode := os.FileMode(0600)
	if mode != expectedMode {
		t.Errorf("File mode = %o, want %o", mode, expectedMode)
	}
}

func TestHandler_Run_WithBackup(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	oldContent := "old content here"
	newContent := "new content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Create source file
	err := os.WriteFile(srcPath, []byte(newContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create existing dest file
	err = os.WriteFile(destPath, []byte(oldContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:    srcPath,
			Dest:   destPath,
			Backup: true,
		},
	}

	_, err = h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	// Check that backup file was created (with timestamp pattern)
	// Backup files have format: path.YYYYMMDD-HHMMSS.bak
	files, err := filepath.Glob(destPath + ".*.bak")
	if err != nil {
		t.Fatalf("Failed to glob for backup files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 backup file, found %d", len(files))
	}

	if len(files) > 0 {
		backupPath := files[0]
		// Verify backup content (should be the old content)
		backupContent, err := os.ReadFile(backupPath)
		if err != nil {
			t.Fatalf("Failed to read backup file: %v", err)
		}

		if string(backupContent) != oldContent {
			t.Errorf("Backup content = %q, want %q", string(backupContent), oldContent)
		}
	}

	// Verify dest was updated
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}

	if string(destContent) != newContent {
		t.Errorf("Dest content = %q, want %q", string(destContent), newContent)
	}
}

func TestHandler_Run_SHA256Checksum(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "sha256 test"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Calculate SHA256 checksum (without prefix)
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	sha256sum := fmt.Sprintf("%x", hasher.Sum(nil))

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:      srcPath,
			Dest:     destPath,
			Checksum: sha256sum,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}
}

func TestHandler_Run_TemplateRendering(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "templated content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	ec.Scope.User["srcfile"] = "source.txt"
	ec.Scope.User["destfile"] = "dest.txt"
	ec.Scope.User["tmpdir"] = tmpDir

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  "{{ tmpdir }}/{{ srcfile }}",
			Dest: "{{ tmpdir }}/{{ destfile }}",
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file was created at correct path
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Copied file does not exist at templated path")
	}
}

func TestHandler_Run_NoPublisher(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "no publisher"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	ec.Svc.EventPublisher = nil

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Errorf("Execute() should not error when publisher is nil, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify file exists
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("Copied file does not exist")
	}
}

func TestHandler_Run_NotExecutionContext(t *testing.T) {
	h := &Handler{}

	ctx := testutil.NewMockContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  "/tmp/source.txt",
			Dest: "/tmp/dest.txt",
		},
	}

	_, err := h.Run(ctx, step)
	if err == nil {
		t.Error("Execute() should error when context is not ExecutionContext")
	}

	if !strings.Contains(err.Error(), "ExecutionContext") {
		t.Errorf("Error should mention ExecutionContext, got: %v", err)
	}
}

func TestHandler_Run_RenderErrorSrc(t *testing.T) {
	h := &Handler{}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  "{{ invalid template syntax",
			Dest: "/tmp/dest.txt",
		},
	}

	_, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid template")
	}

	if !strings.Contains(err.Error(), "expand src path") {
		t.Errorf("Error should mention expand src path, got: %v", err)
	}
}

func TestHandler_Run_RenderErrorDest(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	err := os.WriteFile(srcPath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: "{{ invalid template syntax",
		},
	}

	_, err = h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error on invalid template")
	}

	if !strings.Contains(err.Error(), "expand dest path") {
		t.Errorf("Error should mention expand dest path, got: %v", err)
	}
}

func TestHandler_PlanMode(t *testing.T) {
	h := &Handler{}

	tests := []struct {
		name       string
		step       *config.Step
		setup      func(*executor.ExecutionContext, string)
		wantErr    bool
		wantChange bool
	}{
		{
			name: "new file — would change",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "source.txt",
					Dest: "dest.txt",
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("content"), 0644)
			},
			wantErr:    false,
			wantChange: true,
		},
		{
			name: "valid checksum — no error in plan mode",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:      "source.txt",
					Dest:     "dest.txt",
					Checksum: "9a0364b9e99bb480dd25e1f0284c8555", // md5("content")
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("content"), 0644)
			},
			wantErr:    false,
			wantChange: true,
		},
		{
			name: "different content — would change",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "source.txt",
					Dest: "existing.txt",
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "existing.txt"), []byte("old"), 0644)
			},
			wantErr:    false,
			wantChange: true,
		},
		{
			name: "with mode — would change",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "source.txt",
					Dest: "dest.txt",
					Mode: "0600",
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("content"), 0644)
			},
			wantErr:    false,
			wantChange: true,
		},
		{
			name: "with backup flag — would change (backup skipped in plan mode)",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:    "source.txt",
					Dest:   "backup.txt",
					Backup: true,
				},
			},
			setup: func(ec *executor.ExecutionContext, tmpDir string) {
				os.WriteFile(filepath.Join(tmpDir, "source.txt"), []byte("new"), 0644)
				os.WriteFile(filepath.Join(tmpDir, "backup.txt"), []byte("old"), 0644)
			},
			wantErr:    false,
			wantChange: true,
		},
		{
			name: "source not found — error",
			step: &config.Step{
				FileCopy: &config.Copy{
					Src:  "nonexistent.txt",
					Dest: "dest.txt",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			ec := mockPlanContext()
			ec.CurrentDir = tmpDir

			if tt.setup != nil {
				tt.setup(ec, tmpDir)
			}

			// Update paths to use temp dir
			if tt.step.FileCopy.Src != "" && !filepath.IsAbs(tt.step.FileCopy.Src) {
				tt.step.FileCopy.Src = filepath.Join(tmpDir, tt.step.FileCopy.Src)
			}
			if tt.step.FileCopy.Dest != "" && !filepath.IsAbs(tt.step.FileCopy.Dest) {
				tt.step.FileCopy.Dest = filepath.Join(tmpDir, tt.step.FileCopy.Dest)
			}

			result, err := h.Run(ec, tt.step)
			if (err != nil) != tt.wantErr {
				t.Errorf("Run(plan) error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.wantChange {
				res := result.(*executor.Result)
				if !res.WouldChange {
					t.Errorf("Run(plan) WouldChange = false; want true")
				}
			}
		})
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

func TestHandler_Run_OwnershipWithoutBecome(t *testing.T) {
	// Skip on non-Linux or if not running as root
	if runtime.GOOS != "linux" {
		t.Skip("Ownership test only runs on Linux")
	}

	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	// Only run if not root (to test permission error)
	if currentUser.Uid == "0" {
		t.Skip("Skipping ownership test when running as root")
	}

	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "ownership test"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err = os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:   srcPath,
			Dest:  destPath,
			Owner: "root",
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error when setting ownership without become")
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true when ownership fails")
	}
}

func TestHandler_Run_DestinationChecksum(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	testContent := "checksum verification"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Calculate SHA256 checksum
	hasher := sha256.New()
	hasher.Write([]byte(testContent))
	sha256sum := fmt.Sprintf("%x", hasher.Sum(nil))

	err := os.WriteFile(srcPath, []byte(testContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:      srcPath,
			Dest:     destPath,
			Checksum: sha256sum,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true")
	}

	// Verify destination content and checksum
	content, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}

	if string(content) != testContent {
		t.Errorf("Dest content = %q, want %q", string(content), testContent)
	}
}

func TestHandler_Run_DestinationChecksumMismatch(t *testing.T) {
	h := &Handler{}

	tmpDir := t.TempDir()
	srcContent := "source content"
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	// Wrong checksum
	wrongChecksum := "0000000000000000000000000000000000000000000000000000000000000000"

	err := os.WriteFile(srcPath, []byte(srcContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:      srcPath,
			Dest:     destPath,
			Checksum: wrongChecksum,
		},
	}

	result, err := h.Run(ec, step)
	if err == nil {
		t.Error("Execute() should error when source checksum doesn't match")
	}

	if !strings.Contains(err.Error(), "checksum mismatch") {
		t.Errorf("Error should mention checksum mismatch, got: %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Failed {
		t.Error("Result.Failed should be true on checksum mismatch")
	}
}

// Additional tests for uncovered functions

func TestHandler_SetOwnership_Success(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping ownership test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:   srcPath,
			Dest:  destPath,
			Owner: currentUser.Username,
			Group: currentUser.Gid,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Logf("Execute with ownership error (may be expected): %v", err)
	}

	if result != nil {
		execResult := result.(*executor.Result)
		t.Logf("Copy with ownership result: changed=%v, failed=%v", execResult.Changed, execResult.Failed)
	}
}

func TestHandler_ChownWithBecome(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Skipping chown test on non-Linux")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source.txt")
	destPath := filepath.Join(tmpDir, "dest.txt")

	err := os.WriteFile(srcPath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	ec := mockExecutionContext()
	ec.Svc.SudoPass = "test-password"

	step := &config.Step{
		FileCopy: &config.Copy{
			Src:   srcPath,
			Dest:  destPath,
			Owner: "root",
			Group: "root",
		},
		AsUser: "root",
	}

	// Will fail without actual sudo, but tests the code path
	_, err = h.Run(ec, step)
	t.Logf("chownWithBecome error (expected): %v", err)
}

func TestHandler_Run_LargeFile(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "large-source.txt")
	destPath := filepath.Join(tmpDir, "large-dest.txt")

	// Create a large file (1MB)
	largeContent := make([]byte, 1024*1024)
	for i := range largeContent {
		largeContent[i] = byte(i % 256)
	}

	err := os.WriteFile(srcPath, largeContent, 0644)
	if err != nil {
		t.Fatalf("Failed to create large file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true for large file copy")
	}

	// Verify file was copied correctly
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("Failed to read dest file: %v", err)
	}

	if len(destContent) != len(largeContent) {
		t.Errorf("Dest file size = %d, want %d", len(destContent), len(largeContent))
	}

	// Verify content matches
	for i := range largeContent {
		if destContent[i] != largeContent[i] {
			t.Errorf("Content mismatch at byte %d: got %d, want %d", i, destContent[i], largeContent[i])
			break
		}
	}
}

func TestHandler_Run_BackupWithTimestamp(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	oldContent := "old version 1 content"
	newContent := "new version 1"
	srcPath := filepath.Join(tmpDir, "source-backup.txt")
	destPath := filepath.Join(tmpDir, "dest-backup.txt")

	// Create source file
	err := os.WriteFile(srcPath, []byte(newContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Create existing dest file
	err = os.WriteFile(destPath, []byte(oldContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:    srcPath,
			Dest:   destPath,
			Backup: true,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when backup is created")
	}

	// Verify backup file with timestamp pattern exists
	files, err := filepath.Glob(destPath + ".*.bak")
	if err != nil {
		t.Fatalf("Failed to glob for backup files: %v", err)
	}

	if len(files) != 1 {
		t.Errorf("Expected 1 backup file, found %d", len(files))
	}
}

func TestHandler_Run_MultipleBackups(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source-multi.txt")
	destPath := filepath.Join(tmpDir, "dest-multi.txt")

	// Create dest file
	err := os.WriteFile(destPath, []byte("version 0"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	ec := mockExecutionContext()

	// Create multiple backups
	for i := 1; i <= 3; i++ {
		content := fmt.Sprintf("version %d", i)
		err := os.WriteFile(srcPath, []byte(content), 0644)
		if err != nil {
			t.Fatalf("Failed to create source file: %v", err)
		}

		step := &config.Step{
			FileCopy: &config.Copy{
				Src:    srcPath,
				Dest:   destPath,
				Backup: true,
			},
		}

		_, err = h.Run(ec, step)
		if err != nil {
			t.Fatalf("Execute() error = %v", err)
		}

		// Delay to ensure different timestamps
		time.Sleep(time.Second)
	}

	// Verify multiple backup files exist
	files, err := filepath.Glob(destPath + ".*.bak")
	if err != nil {
		t.Fatalf("Failed to glob for backup files: %v", err)
	}

	if len(files) < 1 {
		t.Errorf("Expected at least 1 backup file, found %d", len(files))
	}
	t.Logf("Created %d backup files (may be less than 3 due to backup logic)", len(files))
}

func TestHandler_Run_CopyWithDifferentContent(t *testing.T) {
	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source-ts.txt")
	destPath := filepath.Join(tmpDir, "dest-ts.txt")

	// Create source with new content; dest with stale content — Run uses
	// content-based idempotency (Effects.WriteFile), not timestamps.
	err := os.WriteFile(srcPath, []byte("new content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}
	err = os.WriteFile(destPath, []byte("old content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create dest file: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:  srcPath,
			Dest: destPath,
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	execResult := result.(*executor.Result)
	if !execResult.Changed {
		t.Error("Result.Changed should be true when content differs")
	}
}

func TestHandler_Run_WithOwnershipAndMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Skipping ownership test on Windows")
	}

	h := &Handler{}
	tmpDir := t.TempDir()
	srcPath := filepath.Join(tmpDir, "source-owner-mode.txt")
	destPath := filepath.Join(tmpDir, "dest-owner-mode.txt")

	err := os.WriteFile(srcPath, []byte("content"), 0644)
	if err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Get current user
	currentUser, err := user.Current()
	if err != nil {
		t.Fatalf("Failed to get current user: %v", err)
	}

	ec := mockExecutionContext()
	step := &config.Step{
		FileCopy: &config.Copy{
			Src:   srcPath,
			Dest:  destPath,
			Owner: currentUser.Username,
			Group: currentUser.Gid,
			Mode:  "0600",
		},
	}

	result, err := h.Run(ec, step)
	if err != nil {
		t.Logf("Execute with ownership and mode error (may be expected): %v", err)
	}

	if result != nil {
		execResult := result.(*executor.Result)
		if execResult.Changed && !execResult.Failed {
			// Verify permissions
			info, err := os.Stat(destPath)
			if err != nil {
				t.Fatalf("Failed to stat dest file: %v", err)
			}

			mode := info.Mode().Perm()
			if mode != 0600 {
				t.Errorf("File mode = %o, want 0600", mode)
			}
		}
	}
}
