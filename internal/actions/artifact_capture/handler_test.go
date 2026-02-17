package artifact_capture

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/artifacts"
	"github.com/alehatsman/mooncake/internal/config"
)

func TestHandler_Metadata(t *testing.T) {
	h := &Handler{}
	meta := h.Metadata()

	if meta.Name != "artifact_capture" {
		t.Errorf("expected name 'artifact_capture', got %s", meta.Name)
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
				ArtifactCapture: &config.ArtifactCapture{
					Name: "test-artifact",
					Steps: []config.Step{
						{Name: "test step"},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "missing artifact_capture",
			step: &config.Step{},
			wantErr: true,
		},
		{
			name: "missing name",
			step: &config.Step{
				ArtifactCapture: &config.ArtifactCapture{
					Steps: []config.Step{
						{Name: "test step"},
					},
				},
			},
			wantErr: true,
		},
		{
			name: "missing steps",
			step: &config.Step{
				ArtifactCapture: &config.ArtifactCapture{
					Name: "test-artifact",
				},
			},
			wantErr: true,
		},
		{
			name: "invalid format",
			step: &config.Step{
				ArtifactCapture: &config.ArtifactCapture{
					Name:   "test-artifact",
					Format: "invalid",
					Steps: []config.Step{
						{Name: "test step"},
					},
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

func TestWriteJSONArtifact(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := makeTestMetadata()

	err := writeJSONArtifact(tmpDir, metadata)
	if err != nil {
		t.Fatalf("writeJSONArtifact failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, "changes.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("changes.json was not created")
	}
}

func TestWriteMarkdownSummary(t *testing.T) {
	tmpDir := t.TempDir()

	metadata := makeTestMetadata()

	err := writeMarkdownSummary(tmpDir, metadata)
	if err != nil {
		t.Fatalf("writeMarkdownSummary failed: %v", err)
	}

	// Verify file was created
	path := filepath.Join(tmpDir, "SUMMARY.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Error("SUMMARY.md was not created")
	}

	// Read and verify content
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read SUMMARY.md: %v", err)
	}

	contentStr := string(content)
	if len(contentStr) == 0 {
		t.Error("SUMMARY.md is empty")
	}

	// Check for expected sections
	expectedSections := []string{
		"# Artifact Capture:",
		"## Summary",
		"## File Changes",
	}
	for _, section := range expectedSections {
		if !contains(contentStr, section) {
			t.Errorf("SUMMARY.md missing section: %s", section)
		}
	}
}

func TestFileChangeTracker(t *testing.T) {
	tracker := newFileChangeTracker()

	if tracker == nil {
		t.Fatal("newFileChangeTracker returned nil")
	}

	changes := tracker.GetFileChanges()
	if len(changes) != 0 {
		t.Errorf("expected 0 changes, got %d", len(changes))
	}

	// Test Close (should not panic)
	tracker.Close()
}

// Helper functions

func makeTestMetadata() artifacts.ArtifactMetadata {
	return artifacts.ArtifactMetadata{
		Name:        "test-artifact",
		CaptureTime: "2026-02-17T12:00:00Z",
		Summary: artifacts.AggregatedChanges{
			TotalFiles: 1,
			FilesByLanguage: map[string]int{
				"go": 1,
			},
			FilesByType: map[string]int{
				"code": 1,
			},
		},
		Files: []artifacts.DetailedFileChange{
			{
				Path:      "test.go",
				Operation: "created",
				Language:  "go",
				FileType:  "code",
			},
		},
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
