package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDetectSourceType(t *testing.T) {
	tests := []struct {
		source   string
		expected SourceType
	}{
		{"https://example.com/preset.yml", SourceTypeURL},
		{"http://example.com/preset.yml", SourceTypeURL},
		{"https://github.com/user/repo.git", SourceTypeURL}, // URL takes precedence over .git suffix
		{"git@github.com:user/repo.git", SourceTypeGit},
		{"https://github.com/user/repo/preset.yml", SourceTypeURL}, // URL takes precedence
		{"./local/preset.yml", SourceTypePath},
		{"/absolute/path/preset.yml", SourceTypePath},
	}

	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			result := DetectSourceType(tt.source)
			if result != tt.expected {
				t.Errorf("DetectSourceType(%s) = %s, want %s", tt.source, result, tt.expected)
			}
		})
	}
}

func TestFetchFromPath_File(t *testing.T) {
	// Create temporary directories
	tmpSource := t.TempDir()
	tmpTarget := t.TempDir()

	// Create source file
	sourceFile := filepath.Join(tmpSource, "test.yml")
	content := []byte("name: test\nsteps: []")
	if err := os.WriteFile(sourceFile, content, 0644); err != nil {
		t.Fatalf("Failed to create source file: %v", err)
	}

	// Fetch from path
	targetDir, err := fetchFromPath(sourceFile, tmpTarget)
	if err != nil {
		t.Fatalf("fetchFromPath failed: %v", err)
	}

	// Verify target file exists
	targetFile := filepath.Join(targetDir, "test.yml")
	if _, err := os.Stat(targetFile); os.IsNotExist(err) {
		t.Errorf("Target file does not exist: %s", targetFile)
	}

	// Verify content
	readContent, err := os.ReadFile(targetFile)
	if err != nil {
		t.Fatalf("Failed to read target file: %v", err)
	}

	if string(readContent) != string(content) {
		t.Errorf("Content mismatch: got %s, want %s", readContent, content)
	}
}

func TestFetchFromPath_Directory(t *testing.T) {
	// Create temporary directories
	tmpSource := t.TempDir()
	tmpTarget := t.TempDir()

	// Create source directory structure
	sourceDir := filepath.Join(tmpSource, "preset")
	if err := os.MkdirAll(sourceDir, 0755); err != nil {
		t.Fatalf("Failed to create source directory: %v", err)
	}

	// Create files in source directory
	file1 := filepath.Join(sourceDir, "preset.yml")
	file2 := filepath.Join(sourceDir, "template.j2")
	if err := os.WriteFile(file1, []byte("name: test"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("template content"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	// Fetch from path
	targetDir, err := fetchFromPath(sourceDir, tmpTarget)
	if err != nil {
		t.Fatalf("fetchFromPath failed: %v", err)
	}

	// Verify files exist in target
	targetFile1 := filepath.Join(targetDir, "preset.yml")
	targetFile2 := filepath.Join(targetDir, "template.j2")

	if _, err := os.Stat(targetFile1); os.IsNotExist(err) {
		t.Errorf("Target file1 does not exist: %s", targetFile1)
	}

	if _, err := os.Stat(targetFile2); os.IsNotExist(err) {
		t.Errorf("Target file2 does not exist: %s", targetFile2)
	}
}

func TestFetchFromGit(t *testing.T) {
	// Skip this test as it requires network access and would prompt for credentials
	// Testing git clone requires either mocking or integration test setup
	t.Skip("Skipping git fetch test - requires network access and valid repository")

	tmpDir := t.TempDir()
	_, err := fetchFromGit("https://github.com/user/repo.git", tmpDir)
	if err == nil {
		t.Error("Expected error for non-existent repository")
	}
}
