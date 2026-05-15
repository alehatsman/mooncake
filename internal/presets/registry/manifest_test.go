package registry

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadManifest(t *testing.T) {
	// Create temporary directory
	tmpDir := t.TempDir()

	// Test loading non-existent manifest (should create empty one)
	manifest, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(manifest.Presets) != 0 {
		t.Errorf("Expected empty manifest, got %d presets", len(manifest.Presets))
	}

	// Add entry and save
	entry := ManifestEntry{
		Name:        "test",
		Source:      "https://example.com/test.yml",
		Type:        "url",
		SHA256:      "abc123",
		InstalledAt: time.Now(),
	}
	manifest.Add(entry)

	if err := manifest.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// Load again and verify
	manifest2, err := LoadManifest(tmpDir)
	if err != nil {
		t.Fatalf("LoadManifest failed: %v", err)
	}

	if len(manifest2.Presets) != 1 {
		t.Errorf("Expected 1 preset, got %d", len(manifest2.Presets))
	}

	if manifest2.Presets[0].Name != "test" {
		t.Errorf("Expected name 'test', got '%s'", manifest2.Presets[0].Name)
	}
}

func TestManifestAdd(t *testing.T) {
	manifest := &Manifest{
		Presets: []ManifestEntry{},
	}

	// Add first entry
	entry1 := ManifestEntry{Name: "foo", Source: "src1", Type: "url", SHA256: "hash1"}
	manifest.Add(entry1)

	if len(manifest.Presets) != 1 {
		t.Errorf("Expected 1 preset, got %d", len(manifest.Presets))
	}

	// Add second entry
	entry2 := ManifestEntry{Name: "bar", Source: "src2", Type: "url", SHA256: "hash2"}
	manifest.Add(entry2)

	if len(manifest.Presets) != 2 {
		t.Errorf("Expected 2 presets, got %d", len(manifest.Presets))
	}

	// Add entry with duplicate name (should replace)
	entry3 := ManifestEntry{Name: "foo", Source: "src3", Type: "url", SHA256: "hash3"}
	manifest.Add(entry3)

	if len(manifest.Presets) != 2 {
		t.Errorf("Expected 2 presets after replacement, got %d", len(manifest.Presets))
	}

	found := manifest.Get("foo")
	if found == nil {
		t.Fatal("Expected to find 'foo'")
	}

	if found.Source != "src3" {
		t.Errorf("Expected source 'src3', got '%s'", found.Source)
	}
}

func TestManifestRemove(t *testing.T) {
	manifest := &Manifest{
		Presets: []ManifestEntry{
			{Name: "foo", Source: "src1", Type: "url", SHA256: "hash1"},
			{Name: "bar", Source: "src2", Type: "url", SHA256: "hash2"},
		},
	}

	manifest.Remove("foo")

	if len(manifest.Presets) != 1 {
		t.Errorf("Expected 1 preset after removal, got %d", len(manifest.Presets))
	}

	if manifest.Presets[0].Name != "bar" {
		t.Errorf("Expected remaining preset to be 'bar', got '%s'", manifest.Presets[0].Name)
	}

	// Remove non-existent (should not error)
	manifest.Remove("nonexistent")

	if len(manifest.Presets) != 1 {
		t.Errorf("Expected 1 preset after removing nonexistent, got %d", len(manifest.Presets))
	}
}

func TestManifestGet(t *testing.T) {
	manifest := &Manifest{
		Presets: []ManifestEntry{
			{Name: "foo", Source: "src1", Type: "url", SHA256: "hash1"},
			{Name: "bar", Source: "src2", Type: "url", SHA256: "hash2"},
		},
	}

	found := manifest.Get("foo")
	if found == nil {
		t.Fatal("Expected to find 'foo'")
	}

	if found.Source != "src1" {
		t.Errorf("Expected source 'src1', got '%s'", found.Source)
	}

	notFound := manifest.Get("nonexistent")
	if notFound != nil {
		t.Error("Expected nil for nonexistent preset")
	}
}

func TestCalculateSHA256(t *testing.T) {
	// Create temporary file
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")

	content := []byte("test content")
	if err := os.WriteFile(tmpFile, content, 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Calculate hash
	hash, err := CalculateSHA256(tmpFile)
	if err != nil {
		t.Fatalf("CalculateSHA256 failed: %v", err)
	}

	// Verify hash is not empty and has correct length (64 hex chars)
	if len(hash) != 64 {
		t.Errorf("Expected hash length 64, got %d", len(hash))
	}

	// Calculate again - should be deterministic
	hash2, err := CalculateSHA256(tmpFile)
	if err != nil {
		t.Fatalf("CalculateSHA256 failed on second call: %v", err)
	}

	if hash != hash2 {
		t.Errorf("Expected deterministic hash, got different values")
	}
}
