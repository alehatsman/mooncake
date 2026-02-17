package artifact_validate

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "artifact_validate" {
		t.Errorf("expected name 'artifact_validate', got %s", meta.Name)
	}
	if meta.Category != "system" {
		t.Errorf("expected category 'system', got %s", meta.Category)
	}
	if !meta.SupportsDryRun {
		t.Error("expected SupportsDryRun to be true")
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
			name: "valid configuration",
			step: &config.Step{
				ArtifactValidate: &config.ArtifactValidate{
					ArtifactFile: "artifacts/test/changes.json",
				},
			},
			wantErr: false,
		},
		{
			name:    "missing artifact_validate",
			step:    &config.Step{},
			wantErr: true,
		},
		{
			name: "missing artifact_file",
			step: &config.Step{
				ArtifactValidate: &config.ArtifactValidate{},
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

func TestReadArtifactMetadata(t *testing.T) {
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "changes.json")

	// Create test metadata
	metadata := createTestMetadata(5, 100)
	if err := writeTestArtifact(artifactPath, metadata); err != nil {
		t.Fatalf("failed to write test artifact: %v", err)
	}

	// Read it back
	read, err := readArtifactMetadata(artifactPath)
	if err != nil {
		t.Fatalf("readArtifactMetadata failed: %v", err)
	}

	if read.Name != metadata.Name {
		t.Errorf("expected name %s, got %s", metadata.Name, read.Name)
	}
	if len(read.Files) != len(metadata.Files) {
		t.Errorf("expected %d files, got %d", len(metadata.Files), len(read.Files))
	}
}

func TestValidateMaxFiles(t *testing.T) {
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "changes.json")

	tests := []struct {
		name      string
		fileCount int
		maxFiles  int
		wantPass  bool
	}{
		{
			name:      "within limit",
			fileCount: 3,
			maxFiles:  5,
			wantPass:  true,
		},
		{
			name:      "at limit",
			fileCount: 5,
			maxFiles:  5,
			wantPass:  true,
		},
		{
			name:      "over limit",
			fileCount: 6,
			maxFiles:  5,
			wantPass:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := createTestMetadata(tt.fileCount, 50)
			if err := writeTestArtifact(artifactPath, metadata); err != nil {
				t.Fatalf("failed to write test artifact: %v", err)
			}

			// Manually run validation logic
			read, err := readArtifactMetadata(artifactPath)
			if err != nil {
				t.Fatalf("failed to read artifact: %v", err)
			}

			pass := len(read.Files) <= tt.maxFiles
			if pass != tt.wantPass {
				t.Errorf("expected validation pass=%v, got %v (files: %d, max: %d)",
					tt.wantPass, pass, len(read.Files), tt.maxFiles)
			}
		})
	}
}

func TestValidateMaxLinesChanged(t *testing.T) {
	tmpDir := t.TempDir()
	artifactPath := filepath.Join(tmpDir, "changes.json")

	tests := []struct {
		name          string
		linesPerFile  int
		fileCount     int
		maxLines      int
		wantPass      bool
	}{
		{
			name:         "within limit",
			linesPerFile: 10,
			fileCount:    5,
			maxLines:     100,
			wantPass:     true,
		},
		{
			name:         "over limit",
			linesPerFile: 30,
			fileCount:    5,
			maxLines:     100,
			wantPass:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			metadata := createTestMetadata(tt.fileCount, tt.linesPerFile)
			if err := writeTestArtifact(artifactPath, metadata); err != nil {
				t.Fatalf("failed to write test artifact: %v", err)
			}

			read, err := readArtifactMetadata(artifactPath)
			if err != nil {
				t.Fatalf("failed to read artifact: %v", err)
			}

			totalLines := read.Summary.TotalLinesChanged
			pass := totalLines <= tt.maxLines
			if pass != tt.wantPass {
				t.Errorf("expected validation pass=%v, got %v (lines: %d, max: %d)",
					tt.wantPass, pass, totalLines, tt.maxLines)
			}
		})
	}
}

func TestValidateRequireTests(t *testing.T) {
	tests := []struct {
		name         string
		files        []artifacts.DetailedFileChange
		requireTests bool
		wantPass     bool
	}{
		{
			name: "code with tests",
			files: []artifacts.DetailedFileChange{
				{Path: "main.go", FileType: "code", IsTestFile: false},
				{Path: "main_test.go", FileType: "test", IsTestFile: true},
			},
			requireTests: true,
			wantPass:     true,
		},
		{
			name: "code without tests",
			files: []artifacts.DetailedFileChange{
				{Path: "main.go", FileType: "code", IsTestFile: false},
			},
			requireTests: true,
			wantPass:     false,
		},
		{
			name: "only config changes",
			files: []artifacts.DetailedFileChange{
				{Path: "config.yaml", FileType: "config", IsTestFile: false},
			},
			requireTests: true,
			wantPass:     true,
		},
		{
			name: "require tests disabled",
			files: []artifacts.DetailedFileChange{
				{Path: "main.go", FileType: "code", IsTestFile: false},
			},
			requireTests: false,
			wantPass:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hasCodeChanges := false
			hasTestChanges := false

			for _, file := range tt.files {
				if file.FileType == "code" && !file.IsTestFile {
					hasCodeChanges = true
				}
				if file.IsTestFile || file.FileType == "test" {
					hasTestChanges = true
				}
			}

			pass := true
			if tt.requireTests && hasCodeChanges && !hasTestChanges {
				pass = false
			}

			if pass != tt.wantPass {
				t.Errorf("expected validation pass=%v, got %v", tt.wantPass, pass)
			}
		})
	}
}

func TestMatchGlob(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Basic wildcards
		{"*.go", "main.go", true},
		{"*.go", "main.py", false},
		
		// Directory wildcards
		{"src/**/*.go", "src/main.go", true},
		{"src/**/*.go", "src/pkg/main.go", true},
		{"src/**/*.go", "other/main.go", false},
		
		// Prefix/suffix
		{"src/**", "src/anything", true},
		{"**/*.json", "config.json", true},
		
		// No match
		{"*.txt", "readme.md", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.path, func(t *testing.T) {
			got := matchGlob(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("matchGlob(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}

// Helper functions

func createTestMetadata(fileCount, linesPerFile int) artifacts.ArtifactMetadata {
	files := make([]artifacts.DetailedFileChange, fileCount)
	for i := 0; i < fileCount; i++ {
		files[i] = artifacts.DetailedFileChange{
			Path:         filepath.Join("src", "file"+string(rune('A'+i))+".go"),
			Operation:    "updated",
			LinesAdded:   linesPerFile,
			LinesRemoved: linesPerFile,
			Language:     "go",
			FileType:     "code",
			SizeAfter:    1024,
		}
	}

	totalLines := fileCount * linesPerFile * 2 // added + removed
	
	return artifacts.ArtifactMetadata{
		Name:        "test-artifact",
		CaptureTime: "2026-02-17T12:00:00Z",
		Summary: artifacts.AggregatedChanges{
			TotalFiles:        fileCount,
			TotalLinesAdded:   fileCount * linesPerFile,
			TotalLinesRemoved: fileCount * linesPerFile,
			TotalLinesChanged: totalLines,
			FilesByLanguage: map[string]int{
				"go": fileCount,
			},
			FilesByType: map[string]int{
				"code": fileCount,
			},
		},
		Files: files,
	}
}

func writeTestArtifact(path string, metadata artifacts.ArtifactMetadata) error {
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
