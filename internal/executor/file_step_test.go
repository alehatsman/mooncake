package executor

import (
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/events"
)

// Note: TestFormatMode already exists in dryrun_test.go
// Note: TestParseFileMode already exists in executor_test.go

// TestHandleFile_CreateDirectory tests directory creation
func TestHandleFile_CreateDirectory(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify directory was created
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Directory was not created: %v", err)
	}
	if !info.IsDir() {
		t.Error("Path is not a directory")
	}
}

// TestHandleFile_CreateDirectoryWithMode tests directory creation with custom mode
func TestHandleFile_CreateDirectoryWithMode(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
			Mode:  "0700",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	expectedMode := os.FileMode(0700)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Directory mode = %v, want %v", info.Mode().Perm(), expectedMode)
	}
}

// TestHandleFile_CreateDirectoryIdempotent tests that directory creation is idempotent
func TestHandleFile_CreateDirectoryIdempotent(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	step := config.Step{
		Register: "dir_result",
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	// First creation
	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("First HandleFile failed: %v", err)
	}

	// Verify changed flag
	result, ok := ec.Variables["dir_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered")
	}
	if !result["changed"].(bool) {
		t.Error("First creation should set changed=true")
	}

	// Second creation - should be idempotent
	delete(ec.Variables, "dir_result")
	err = HandleFile(step, ec)
	if err != nil {
		t.Fatalf("Second HandleFile failed: %v", err)
	}

	// Verify changed flag is false
	result2, ok := ec.Variables["dir_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered on second run")
	}
	if result2["changed"].(bool) {
		t.Error("Second creation should set changed=false")
	}
}

// TestHandleFile_CreateEmptyFile tests creating an empty file
func TestHandleFile_CreateEmptyFile(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "file",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify file was created
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("File was not created: %v", err)
	}
	if info.IsDir() {
		t.Error("Path is a directory, expected file")
	}
	if info.Size() != 0 {
		t.Errorf("File size = %d, want 0", info.Size())
	}
}

// TestHandleFile_CreateFileWithContent tests creating a file with content
func TestHandleFile_CreateFileWithContent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	content := "Hello, World!"

	step := config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: content,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify content
	actualContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(actualContent) != content {
		t.Errorf("File content = %q, want %q", string(actualContent), content)
	}
}

// TestHandleFile_CreateFileWithMode tests creating a file with custom mode
func TestHandleFile_CreateFileWithMode(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	step := config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: "test",
			Mode:    "0600",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify permissions
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	expectedMode := os.FileMode(0600)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("File mode = %v, want %v", info.Mode().Perm(), expectedMode)
	}
}

// TestHandleFile_UpdateFileContent tests updating file content
func TestHandleFile_UpdateFileContent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	oldContent := "old content"
	newContent := "new content"

	// Create file with old content
	createTestFile(t, filePath, oldContent)

	step := config.Step{
		Register: "file_result",
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: newContent,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify content was updated
	actualContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(actualContent) != newContent {
		t.Errorf("File content = %q, want %q", string(actualContent), newContent)
	}

	// Verify changed flag
	result, ok := ec.Variables["file_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered")
	}
	if !result["changed"].(bool) {
		t.Error("Content update should set changed=true")
	}
}

// TestHandleFile_FileIdempotent tests that file creation with same content is idempotent
func TestHandleFile_FileIdempotent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	content := "test content"

	step := config.Step{
		Register: "file_result",
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: content,
		},
	}

	// First creation
	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("First HandleFile failed: %v", err)
	}

	// Second creation with same content
	delete(ec.Variables, "file_result")
	err = HandleFile(step, ec)
	if err != nil {
		t.Fatalf("Second HandleFile failed: %v", err)
	}

	// Verify changed flag is false
	result, ok := ec.Variables["file_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered on second run")
	}
	if result["changed"].(bool) {
		t.Error("Second creation with same content should set changed=false")
	}
}

// TestHandleFile_RemoveFile tests file removal
func TestHandleFile_RemoveFile(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file
	createTestFile(t, filePath, "test content")

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify file was removed
	if _, err := os.Stat(filePath); !os.IsNotExist(err) {
		t.Error("File should have been removed")
	}
}

// TestHandleFile_RemoveDirectory tests directory removal
func TestHandleFile_RemoveDirectory(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create empty directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should have been removed")
	}
}

// TestHandleFile_RemoveDirectoryNonEmptyWithoutForce tests removing non-empty dir without force
func TestHandleFile_RemoveDirectoryNonEmptyWithoutForce(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create directory with a file
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	createTestFile(t, filepath.Join(dirPath, "file.txt"), "test")

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
			Force: false,
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for removing non-empty directory without force")
	}
}

// TestHandleFile_RemoveDirectoryRecursive tests removing non-empty dir with force
func TestHandleFile_RemoveDirectoryRecursive(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create directory with nested structure
	if err := os.MkdirAll(filepath.Join(dirPath, "subdir"), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	createTestFile(t, filepath.Join(dirPath, "file.txt"), "test")
	createTestFile(t, filepath.Join(dirPath, "subdir", "file2.txt"), "test2")

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
			Force: true,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify directory was removed
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should have been removed recursively")
	}
}

// TestHandleFile_RemoveAbsentIdempotent tests that removing absent path is idempotent
func TestHandleFile_RemoveAbsentIdempotent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "nonexistent.txt")

	step := config.Step{
		Register: "remove_result",
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify changed flag is false (nothing to remove)
	result, ok := ec.Variables["remove_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered")
	}
	if result["changed"].(bool) {
		t.Error("Removing non-existent file should set changed=false")
	}
}

// TestHandleFile_RemoveSafetyCheck tests safety checks for removal
func TestHandleFile_RemoveSafetyCheck(t *testing.T) {
	ec := newTestExecutionContext(t)

	testCases := []struct {
		name        string
		path        string
		shouldError bool
	}{
		{"empty path", "", false}, // HandleFile skips empty paths without error
		{"root path", "/", true},  // Should error on safety check
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			step := config.Step{
				File: &config.File{
					Path:  tc.path,
					State: "absent",
				},
			}

			err := HandleFile(step, ec)
			if tc.shouldError && err == nil {
				t.Fatal("Expected error for unsafe path removal")
			}
			if !tc.shouldError && err != nil {
				t.Fatalf("Expected no error, got: %v", err)
			}
		})
	}
}

// TestHandleFile_TouchNewFile tests touching a new file
func TestHandleFile_TouchNewFile(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify file was created
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("File was not created: %v", err)
	}
	if info.Size() != 0 {
		t.Errorf("File size = %d, want 0", info.Size())
	}
}

// TestHandleFile_TouchExistingFile tests touching an existing file
func TestHandleFile_TouchExistingFile(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	content := "existing content"

	// Create file with content
	createTestFile(t, filePath, content)

	// Get original mod time
	originalInfo, _ := os.Stat(filePath)
	originalModTime := originalInfo.ModTime()

	// Wait a bit to ensure time difference
	// Note: Some filesystems have low resolution
	// time.Sleep(10 * time.Millisecond)

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify file still exists and content is unchanged
	newContent, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(newContent) != content {
		t.Errorf("Content changed: got %q, want %q", string(newContent), content)
	}

	// Verify timestamp was updated (or at least file still exists)
	newInfo, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	// Just verify the operation completed successfully
	// Timestamp comparison can be flaky on different filesystems
	_ = originalModTime
	_ = newInfo
}

// TestHandleFile_CreateSymlink tests symlink creation
func TestHandleFile_CreateSymlink(t *testing.T) {
	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	// Create source file
	createTestFile(t, srcPath, "source content")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   srcPath,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		if runtime.GOOS == "windows" {
			t.Skip("Symlink creation may require special privileges on Windows")
		}
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify symlink was created
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != srcPath {
		t.Errorf("Symlink target = %q, want %q", target, srcPath)
	}
}

// TestHandleFile_CreateSymlinkMissingSrc tests symlink creation without src
func TestHandleFile_CreateSymlinkMissingSrc(t *testing.T) {
	ec := newTestExecutionContext(t)
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing src")
	}
}

// TestHandleFile_CreateSymlinkIdempotent tests symlink creation is idempotent
func TestHandleFile_CreateSymlinkIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	// Create source file
	createTestFile(t, srcPath, "source content")

	step := config.Step{
		Register: "link_result",
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   srcPath,
		},
	}

	// First creation
	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("First HandleFile failed: %v", err)
	}

	// Second creation - should be idempotent
	delete(ec.Variables, "link_result")
	err = HandleFile(step, ec)
	if err != nil {
		t.Fatalf("Second HandleFile failed: %v", err)
	}

	// Verify changed flag is false
	result, ok := ec.Variables["link_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered on second run")
	}
	if result["changed"].(bool) {
		t.Error("Second link creation should set changed=false")
	}
}

// TestHandleFile_CreateSymlinkForce tests symlink creation with force
func TestHandleFile_CreateSymlinkForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	oldSrcPath := filepath.Join(ec.CurrentDir, "oldsource.txt")
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	// Create source files
	createTestFile(t, srcPath, "new source")
	createTestFile(t, oldSrcPath, "old source")

	// Create initial link to old source
	if err := os.Symlink(oldSrcPath, linkPath); err != nil {
		t.Fatalf("Failed to create initial symlink: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   srcPath,
			Force: true,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify symlink now points to new source
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != srcPath {
		t.Errorf("Symlink target = %q, want %q", target, srcPath)
	}
}

// TestHandleFile_CreateHardlink tests hardlink creation
func TestHandleFile_CreateHardlink(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Hardlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")
	content := "source content"

	// Create source file
	createTestFile(t, srcPath, content)

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcPath,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify hardlink was created
	linkContent, err := os.ReadFile(linkPath)
	if err != nil {
		t.Fatalf("Failed to read hardlink: %v", err)
	}
	if string(linkContent) != content {
		t.Errorf("Hardlink content = %q, want %q", string(linkContent), content)
	}

	// Verify they point to the same inode
	srcInfo, _ := os.Stat(srcPath)
	linkInfo, _ := os.Stat(linkPath)
	if !os.SameFile(srcInfo, linkInfo) {
		t.Error("Hardlink does not point to same file")
	}
}

// TestHandleFile_CreateHardlinkMissingSrc tests hardlink creation without src
func TestHandleFile_CreateHardlinkMissingSrc(t *testing.T) {
	ec := newTestExecutionContext(t)
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for missing src")
	}
}

// TestHandleFile_CreateHardlinkNonExistentSource tests hardlink with non-existent source
func TestHandleFile_CreateHardlinkNonExistentSource(t *testing.T) {
	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "nonexistent.txt")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcPath,
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for non-existent source")
	}
}

// TestHandleFile_CreateHardlinkToDirectory tests hardlink to directory (should fail)
func TestHandleFile_CreateHardlinkToDirectory(t *testing.T) {
	ec := newTestExecutionContext(t)
	srcDir := filepath.Join(ec.CurrentDir, "sourcedir")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink")

	// Create source directory
	if err := os.Mkdir(srcDir, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcDir,
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for hardlink to directory")
	}
}

// TestHandleFile_SetPermissions tests setting permissions on existing file
func TestHandleFile_SetPermissions(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file with default permissions
	createTestFile(t, filePath, "test")

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify permissions were changed
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}

	expectedMode := os.FileMode(0600)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("File mode = %v, want %v", info.Mode().Perm(), expectedMode)
	}
}

// TestHandleFile_SetPermissionsNonExistent tests setting permissions on non-existent file
func TestHandleFile_SetPermissionsNonExistent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "nonexistent.txt")

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for non-existent file")
	}
}

// TestHandleFile_SetPermissionsIdempotent tests that setting same permissions is idempotent
func TestHandleFile_SetPermissionsIdempotent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file with specific permissions
	if err := os.WriteFile(filePath, []byte("test"), 0600); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	step := config.Step{
		Register: "perms_result",
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify changed flag is false (permissions already correct)
	result, ok := ec.Variables["perms_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered")
	}
	if result["changed"].(bool) {
		t.Error("Setting same permissions should set changed=false")
	}
}

// TestHandleFile_DryRunMultipleStates tests dry-run mode for multiple states
func TestHandleFile_DryRunMultipleStates(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true

	testCases := []struct {
		name  string
		state string
	}{
		{"directory", "directory"},
		{"file", "file"},
		{"touch", "touch"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(ec.CurrentDir, tc.name)

			step := config.Step{
				File: &config.File{
					Path:  path,
					State: tc.state,
				},
			}

			err := HandleFile(step, ec)
			if err != nil {
				t.Fatalf("HandleFile in dry-run failed: %v", err)
			}

			// Verify nothing was created
			if _, err := os.Stat(path); !os.IsNotExist(err) {
				t.Errorf("Path should not exist in dry-run mode: %s", path)
			}
		})
	}
}

// TestHandleFile_EventEmission tests event emission
func TestHandleFile_EventEmission(t *testing.T) {
	testCases := []struct {
		name      string
		state     string
		eventType events.EventType
	}{
		{"directory created", "directory", events.EventDirCreated},
		{"file created", "file", events.EventFileCreated},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ec := newTestExecutionContext(t)
			path := filepath.Join(ec.CurrentDir, "test")

			// Subscribe to events
			collector := &testEventCollector{
				events: make([]events.Event, 0),
			}
			ec.EventPublisher.Subscribe(collector)

			step := config.Step{
				File: &config.File{
					Path:  path,
					State: tc.state,
				},
			}

			err := HandleFile(step, ec)
			if err != nil {
				t.Fatalf("HandleFile failed: %v", err)
			}

			// Wait for events to be processed asynchronously
			time.Sleep(50 * time.Millisecond)

			// Find the expected event
			allEvents := collector.getEvents()
			found := false
			for _, evt := range allEvents {
				if evt.Type == tc.eventType {
					found = true
					break
				}
			}

			if !found {
				t.Errorf("Expected event %s to be emitted", tc.eventType)
			}
		})
	}
}

// TestHandleFile_SkipEmptyPath tests skipping when path is empty
func TestHandleFile_SkipEmptyPath(t *testing.T) {
	ec := newTestExecutionContext(t)

	step := config.Step{
		File: &config.File{
			Path:  "",
			State: "file",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile should not error on empty path: %v", err)
	}
}

// TestHandleFile_TemplateRendering tests that templates are rendered in content
func TestHandleFile_TemplateRendering(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.Variables["username"] = "testuser"
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	step := config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: "Hello {{ username }}!",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify templated content
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	expected := "Hello testuser!"
	if string(content) != expected {
		t.Errorf("File content = %q, want %q", string(content), expected)
	}
}

// TestLogContentPreview tests the logContentPreview function
func TestLogContentPreview(t *testing.T) {
	// This is a simple function that just logs, so we just verify it doesn't panic
	ec := newTestExecutionContext(t)

	// Empty content - should not log anything
	logContentPreview(ec.Logger, "", 100)

	// Short content
	logContentPreview(ec.Logger, "short", 100)

	// Long content that should be truncated
	longContent := string(make([]byte, 500))
	logContentPreview(ec.Logger, longContent, 200)
}

// TestParseUserID tests the parseUserID function
func TestParseUserID(t *testing.T) {
	// Test numeric UID
	uid, err := parseUserID("1000")
	if err != nil {
		t.Errorf("parseUserID(\"1000\") failed: %v", err)
	}
	if uid != 1000 {
		t.Errorf("parseUserID(\"1000\") = %d, want 1000", uid)
	}

	// Test invalid user
	_, err = parseUserID("nonexistentuserxyz123")
	if err == nil {
		t.Error("parseUserID(nonexistent) should return error")
	}
}

// TestParseGroupID tests the parseGroupID function
func TestParseGroupID(t *testing.T) {
	// Test numeric GID
	gid, err := parseGroupID("1000")
	if err != nil {
		t.Errorf("parseGroupID(\"1000\") failed: %v", err)
	}
	if gid != 1000 {
		t.Errorf("parseGroupID(\"1000\") = %d, want 1000", gid)
	}

	// Test invalid group
	_, err = parseGroupID("nonexistentgroupxyz123")
	if err == nil {
		t.Error("parseGroupID(nonexistent) should return error")
	}
}

// TestHandleFile_RemoveFileEmitsCorrectEvent tests file removal event
func TestHandleFile_RemoveFileEmitsCorrectEvent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file
	createTestFile(t, filePath, "test content")

	// Subscribe to events
	collector := &testEventCollector{
		events: make([]events.Event, 0),
	}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Find the file.removed event
	allEvents := collector.getEvents()
	found := false
	for _, evt := range allEvents {
		if evt.Type == events.EventFileRemoved {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected file.removed event to be emitted")
	}
}

// TestHandleFile_RemoveDirectoryEmitsCorrectEvent tests directory removal event
func TestHandleFile_RemoveDirectoryEmitsCorrectEvent(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create empty directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	// Subscribe to events
	collector := &testEventCollector{
		events: make([]events.Event, 0),
	}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Find the directory.removed event
	allEvents := collector.getEvents()
	found := false
	for _, evt := range allEvents {
		if evt.Type == events.EventDirRemoved {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected directory.removed event to be emitted")
	}
}

// TestHandleFile_CreateSymlinkToNonExistentTarget tests symlink to non-existent target (should work)
func TestHandleFile_CreateSymlinkToNonExistentTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "nonexistent.txt")
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   srcPath,
		},
	}

	// Symlinks can point to non-existent files
	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify symlink was created
	target, err := os.Readlink(linkPath)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if target != srcPath {
		t.Errorf("Symlink target = %q, want %q", target, srcPath)
	}
}

// TestHandleFile_CreateSymlinkOverFile tests creating symlink over existing file without force
func TestHandleFile_CreateSymlinkOverFile(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Symlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "link.txt")

	// Create source file
	createTestFile(t, srcPath, "source")

	// Create a regular file at link path
	createTestFile(t, linkPath, "existing")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "link",
			Src:   srcPath,
			Force: false,
		},
	}

	// Should fail without force
	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error when creating symlink over existing file without force")
	}
}

// TestHandleFile_CreateHardlinkIdempotent tests hardlink creation is idempotent
func TestHandleFile_CreateHardlinkIdempotent(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Hardlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")
	content := "source content"

	// Create source file
	createTestFile(t, srcPath, content)

	step := config.Step{
		Register: "link_result",
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcPath,
		},
	}

	// First creation
	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("First HandleFile failed: %v", err)
	}

	// Second creation - should be idempotent
	delete(ec.Variables, "link_result")
	err = HandleFile(step, ec)
	if err != nil {
		t.Fatalf("Second HandleFile failed: %v", err)
	}

	// Verify changed flag is false
	result, ok := ec.Variables["link_result"].(map[string]interface{})
	if !ok {
		t.Fatal("Result was not registered on second run")
	}
	if result["changed"].(bool) {
		t.Error("Second hardlink creation should set changed=false")
	}
}

// TestHandleFile_CreateHardlinkForce tests hardlink creation with force
func TestHandleFile_CreateHardlinkForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Hardlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	oldSrcPath := filepath.Join(ec.CurrentDir, "oldsource.txt")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")

	// Create source files
	createTestFile(t, srcPath, "new source")
	createTestFile(t, oldSrcPath, "old source")

	// Create initial hardlink to old source
	if err := os.Link(oldSrcPath, linkPath); err != nil {
		t.Fatalf("Failed to create initial hardlink: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcPath,
			Force: true,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify hardlink now points to new source
	srcInfo, _ := os.Stat(srcPath)
	linkInfo, _ := os.Stat(linkPath)
	if !os.SameFile(srcInfo, linkInfo) {
		t.Error("Hardlink does not point to new source")
	}
}

// TestHandleFile_CreateHardlinkOverExistingWithoutForce tests hardlink over existing file without force
func TestHandleFile_CreateHardlinkOverExistingWithoutForce(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Hardlink tests may be unreliable on Windows")
	}

	ec := newTestExecutionContext(t)
	srcPath := filepath.Join(ec.CurrentDir, "source.txt")
	linkPath := filepath.Join(ec.CurrentDir, "hardlink.txt")

	// Create source file
	createTestFile(t, srcPath, "source")

	// Create a different file at link path
	createTestFile(t, linkPath, "existing")

	step := config.Step{
		File: &config.File{
			Path:  linkPath,
			State: "hardlink",
			Src:   srcPath,
			Force: false,
		},
	}

	// Should fail without force
	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error when creating hardlink over existing file without force")
	}
}

// TestHandleFile_SetPermissionsOnDirectory tests setting permissions on a directory
func TestHandleFile_SetPermissionsOnDirectory(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "perms",
			Mode:  "0700",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify permissions were changed
	info, err := os.Stat(dirPath)
	if err != nil {
		t.Fatalf("Failed to stat directory: %v", err)
	}

	expectedMode := os.FileMode(0700)
	if info.Mode().Perm() != expectedMode {
		t.Errorf("Directory mode = %v, want %v", info.Mode().Perm(), expectedMode)
	}
}

// TestHandleFile_TouchFileEmitsCorrectEvent tests touch file event emission
func TestHandleFile_TouchFileEmitsCorrectEvent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Subscribe to events
	collector := &testEventCollector{
		events: make([]events.Event, 0),
	}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Find the file.created event
	allEvents := collector.getEvents()
	found := false
	for _, evt := range allEvents {
		if evt.Type == events.EventFileCreated {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected file.created event to be emitted for touch on new file")
	}
}

// TestHandleFile_CreateFileEventHasSize tests that file created event includes size
func TestHandleFile_CreateFileEventHasSize(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	content := "test content with some size"

	// Subscribe to events
	collector := &testEventCollector{
		events: make([]events.Event, 0),
	}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: content,
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Find the file.created event
	allEvents := collector.getEvents()
	for _, evt := range allEvents {
		if evt.Type == events.EventFileCreated {
			data, ok := evt.Data.(events.FileOperationData)
			if !ok {
				t.Fatal("Event data is not FileOperationData")
			}
			if data.SizeBytes != int64(len(content)) {
				t.Errorf("Event SizeBytes = %d, want %d", data.SizeBytes, len(content))
			}
			return
		}
	}

	t.Fatal("Expected file.created event to be emitted")
}

// TestHandleFile_SetPermissionsEmitsEvent tests permissions change event
func TestHandleFile_SetPermissionsEmitsEvent(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file with default permissions
	createTestFile(t, filePath, "test")

	// Subscribe to events
	collector := &testEventCollector{
		events: make([]events.Event, 0),
	}
	ec.EventPublisher.Subscribe(collector)

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "perms",
			Mode:  "0600",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Wait for events to be processed
	time.Sleep(50 * time.Millisecond)

	// Find the permissions.changed event
	allEvents := collector.getEvents()
	found := false
	for _, evt := range allEvents {
		if evt.Type == events.EventPermissionsChanged {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected permissions.changed event to be emitted")
	}
}

// TestHandleFile_SetPermissionsRecursiveWithoutSudo tests recursive chmod error without sudo
func TestHandleFile_SetPermissionsRecursiveWithoutSudo(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	step := config.Step{
		File: &config.File{
			Path:    dirPath,
			State:   "perms",
			Mode:    "0700",
			Recurse: true,
		},
	}

	err := HandleFile(step, ec)
	if err == nil {
		t.Fatal("Expected error for recursive permission change without sudo")
	}
	// Check for SetupError with correct component and message
	setupErr, ok := err.(*SetupError)
	if !ok {
		t.Fatalf("Expected SetupError, got %T: %v", err, err)
	}
	if setupErr.Component != "become" {
		t.Errorf("Expected Component 'become', got %q", setupErr.Component)
	}
	if setupErr.Issue != "recursive permission change without sudo not yet implemented (use become: true)" {
		t.Errorf("Unexpected Issue: %v", setupErr.Issue)
	}
}

// TestParseUserID_WithUsername tests parsing actual username
func TestParseUserID_WithUsername(t *testing.T) {
	// Get current user
	currentUser, err := os.UserCacheDir()
	if err != nil {
		t.Skip("Cannot get current user, skipping test")
	}
	_ = currentUser

	// Try to lookup current user by getting user info
	u, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user info, skipping test")
	}

	// Test with username
	uid, err := parseUserID(u.Username)
	if err != nil {
		t.Errorf("parseUserID with username failed: %v", err)
	}

	// Verify it matches the UID
	expectedUID, _ := strconv.Atoi(u.Uid)
	if uid != expectedUID {
		t.Errorf("parseUserID(%q) = %d, want %d", u.Username, uid, expectedUID)
	}
}

// TestParseGroupID_WithGroupName tests parsing actual group name
func TestParseGroupID_WithGroupName(t *testing.T) {
	// Try to get current user's group
	u, err := user.Current()
	if err != nil {
		t.Skip("Cannot get current user info, skipping test")
	}

	// Try to lookup group by GID first
	g, err := user.LookupGroupId(u.Gid)
	if err != nil {
		t.Skip("Cannot lookup group info, skipping test")
	}

	// Test with group name
	gid, err := parseGroupID(g.Name)
	if err != nil {
		t.Errorf("parseGroupID with group name failed: %v", err)
	}

	// Verify it matches the GID
	expectedGID, _ := strconv.Atoi(u.Gid)
	if gid != expectedGID {
		t.Errorf("parseGroupID(%q) = %d, want %d", g.Name, gid, expectedGID)
	}
}

// TestHandleFile_DryRunFileUpdate tests dry-run for file update (not create)
func TestHandleFile_DryRunFileUpdate(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file first
	createTestFile(t, filePath, "old content")

	// Now try to update in dry-run mode
	ec.DryRun = true

	step := config.Step{
		File: &config.File{
			Path:    filePath,
			State:   "file",
			Content: "new content",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile in dry-run failed: %v", err)
	}

	// Verify file was NOT updated
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	if string(content) != "old content" {
		t.Errorf("File should not be updated in dry-run mode, got: %q", string(content))
	}
}

// TestHandleFile_TouchFileUpdatesTimestamp tests that touch updates timestamp
func TestHandleFile_TouchFileUpdatesTimestamp(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")
	content := "existing content"

	// Create file
	createTestFile(t, filePath, content)

	// Set an old mod time
	oldTime := time.Now().Add(-1 * time.Hour)
	if err := os.Chtimes(filePath, oldTime, oldTime); err != nil {
		t.Fatalf("Failed to set old time: %v", err)
	}

	// Get the old mod time
	infoOld, _ := os.Stat(filePath)
	oldModTime := infoOld.ModTime()

	// Small sleep to ensure time difference
	time.Sleep(10 * time.Millisecond)

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "touch",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify timestamp was updated
	infoNew, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("Failed to stat file: %v", err)
	}
	newModTime := infoNew.ModTime()

	if !newModTime.After(oldModTime) {
		t.Error("Touch should update modification time")
	}

	// Verify content is unchanged
	newContent, _ := os.ReadFile(filePath)
	if string(newContent) != content {
		t.Errorf("Content should be unchanged, got: %q", string(newContent))
	}
}

// TestHandleFile_CreateDirectoryDryRun tests directory creation in dry-run mode
func TestHandleFile_CreateDirectoryDryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	ec.DryRun = true
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "directory",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify directory was NOT created
	if _, err := os.Stat(dirPath); !os.IsNotExist(err) {
		t.Error("Directory should not be created in dry-run mode")
	}
}

// TestHandleFile_RemoveFileDryRun tests file removal in dry-run mode
func TestHandleFile_RemoveFileDryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	filePath := filepath.Join(ec.CurrentDir, "testfile.txt")

	// Create file
	createTestFile(t, filePath, "test")

	ec.DryRun = true

	step := config.Step{
		File: &config.File{
			Path:  filePath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify file was NOT removed
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		t.Error("File should not be removed in dry-run mode")
	}
}

// TestHandleFile_RemoveDirectoryDryRun tests directory removal in dry-run mode
func TestHandleFile_RemoveDirectoryDryRun(t *testing.T) {
	ec := newTestExecutionContext(t)
	dirPath := filepath.Join(ec.CurrentDir, "testdir")

	// Create directory
	if err := os.Mkdir(dirPath, 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}

	ec.DryRun = true

	step := config.Step{
		File: &config.File{
			Path:  dirPath,
			State: "absent",
		},
	}

	err := HandleFile(step, ec)
	if err != nil {
		t.Fatalf("HandleFile failed: %v", err)
	}

	// Verify directory was NOT removed
	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		t.Error("Directory should not be removed in dry-run mode")
	}
}
