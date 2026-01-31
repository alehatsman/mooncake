package filetree

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func TestWalker_GetFileTree(t *testing.T) {
	// Create temporary directory structure for testing
	tmpDir := t.TempDir()

	// Create test files and directories
	testFiles := []string{
		"file1.txt",
		"file2.txt",
		"subdir/file3.txt",
		"subdir/file4.txt",
		"subdir/nested/file5.txt",
	}

	for _, file := range testFiles {
		fullPath := filepath.Join(tmpDir, file)
		dir := filepath.Dir(fullPath)
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatalf("Failed to create directory %s: %v", dir, err)
		}
		if err := os.WriteFile(fullPath, []byte("test content"), 0644); err != nil {
			t.Fatalf("Failed to create file %s: %v", fullPath, err)
		}
	}

	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	t.Run("walk entire directory", func(t *testing.T) {
		items, err := walker.GetFileTree(tmpDir, tmpDir, nil)
		if err != nil {
			t.Fatalf("GetFileTree() error = %v", err)
		}

		// Should include root directory + 3 directories + 5 files = 9 items
		// (tmpDir, subdir, nested, and 5 files)
		expectedMinItems := 5 // At least the files
		if len(items) < expectedMinItems {
			t.Errorf("GetFileTree() returned %d items, want at least %d", len(items), expectedMinItems)
		}

		// Verify we have both files and directories
		hasFile := false
		hasDir := false
		for _, item := range items {
			if item.State == "file" {
				hasFile = true
			}
			if item.State == "directory" {
				hasDir = true
			}
		}

		if !hasFile {
			t.Error("GetFileTree() should include files")
		}
		if !hasDir {
			t.Error("GetFileTree() should include directories")
		}
	})

	t.Run("walk with template variable", func(t *testing.T) {
		context := map[string]interface{}{
			"dir": tmpDir,
		}

		items, err := walker.GetFileTree("{{ dir }}", tmpDir, context)
		if err != nil {
			t.Fatalf("GetFileTree() with template error = %v", err)
		}

		if len(items) == 0 {
			t.Error("GetFileTree() with template returned no items")
		}
	})

	t.Run("walk subdirectory", func(t *testing.T) {
		subdir := filepath.Join(tmpDir, "subdir")
		items, err := walker.GetFileTree(subdir, tmpDir, nil)
		if err != nil {
			t.Fatalf("GetFileTree() subdirectory error = %v", err)
		}

		// Should find files in subdir and nested
		if len(items) == 0 {
			t.Error("GetFileTree() subdirectory returned no items")
		}

		// All items should be within subdir
		for _, item := range items {
			if item.Src != "" && !strings.HasPrefix(item.Src, subdir) {
				t.Errorf("GetFileTree() item %s not within subdir %s", item.Src, subdir)
			}
		}
	})
}

func TestWalker_GetFileTreeErrors(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	t.Run("nonexistent directory", func(t *testing.T) {
		_, err := walker.GetFileTree("/nonexistent/path", "/tmp", nil)
		if err == nil {
			t.Error("GetFileTree() should return error for nonexistent directory")
		}
	})

	t.Run("invalid template", func(t *testing.T) {
		_, err := walker.GetFileTree("{{ unclosed", "/tmp", nil)
		if err == nil {
			t.Error("GetFileTree() should return error for invalid template")
		}
	})
}

func TestWalker_GetFileTreeStructure(t *testing.T) {
	// Create a known directory structure
	tmpDir := t.TempDir()

	// Create specific structure
	if err := os.MkdirAll(filepath.Join(tmpDir, "a"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "b"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	items, err := walker.GetFileTree(tmpDir, tmpDir, nil)
	if err != nil {
		t.Fatalf("GetFileTree() error = %v", err)
	}

	// Verify Item structure
	for _, item := range items {
		// Check that Src is populated
		if item.Src == "" {
			t.Error("Item.Src should not be empty")
		}

		// Check that State is either "file" or "directory"
		if item.State != "file" && item.State != "directory" {
			t.Errorf("Item.State = %q, want 'file' or 'directory'", item.State)
		}

		// Path should be relative to root
		// (may be empty for root directory itself)
	}
}

func TestNewWalker(t *testing.T) {
	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	if walker == nil {
		t.Error("NewWalker() returned nil")
	}

	// Test that it works
	tmpDir := t.TempDir()
	_, err := walker.GetFileTree(tmpDir, tmpDir, nil)
	if err != nil {
		t.Errorf("NewWalker() created non-functional walker: %v", err)
	}
}

func TestWalker_GetFileTreeWithContext(t *testing.T) {
	tmpDir := t.TempDir()

	// Create test file
	testFile := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(testFile, []byte("content"), 0644); err != nil {
		t.Fatal(err)
	}

	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	tests := []struct {
		name    string
		path    string
		context map[string]interface{}
		wantErr bool
	}{
		{
			name:    "with nil context",
			path:    tmpDir,
			context: nil,
			wantErr: false,
		},
		{
			name:    "with empty context",
			path:    tmpDir,
			context: map[string]interface{}{},
			wantErr: false,
		},
		{
			name:    "with template in path",
			path:    "{{ root }}",
			context: map[string]interface{}{"root": tmpDir},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			items, err := walker.GetFileTree(tt.path, tmpDir, tt.context)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFileTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(items) == 0 {
				t.Error("GetFileTree() returned no items")
			}
		})
	}
}

func TestWalker_GetFileTreeFileProperties(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a file and a directory
	testFile := filepath.Join(tmpDir, "file.txt")
	testDir := filepath.Join(tmpDir, "dir")

	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	renderer := template.NewPongo2Renderer()
	pathExpander := pathutil.NewPathExpander(renderer)
	walker := NewWalker(pathExpander)

	items, err := walker.GetFileTree(tmpDir, tmpDir, nil)
	if err != nil {
		t.Fatalf("GetFileTree() error = %v", err)
	}

	// Find the specific file and directory
	var fileItem, dirItem *Item
	for i := range items {
		if items[i].Src == testFile {
			fileItem = &items[i]
		}
		if items[i].Src == testDir {
			dirItem = &items[i]
		}
	}

	if fileItem != nil && fileItem.State != "file" {
		t.Errorf("File item state = %q, want 'file'", fileItem.State)
	}

	if dirItem != nil && dirItem.State != "directory" {
		t.Errorf("Directory item state = %q, want 'directory'", dirItem.State)
	}
}
