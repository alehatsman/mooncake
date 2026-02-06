package registry

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultCacheDir(t *testing.T) {
	cacheDir, err := DefaultCacheDir()
	if err != nil {
		t.Fatalf("DefaultCacheDir failed: %v", err)
	}

	// Should contain .mooncake/cache/presets
	if !filepath.IsAbs(cacheDir) {
		t.Errorf("Expected absolute path, got %s", cacheDir)
	}

	if !filepath.HasPrefix(cacheDir, string(filepath.Separator)) {
		t.Errorf("Cache dir should be absolute path")
	}
}

func TestUserPresetsDir(t *testing.T) {
	userDir, err := UserPresetsDir()
	if err != nil {
		t.Fatalf("UserPresetsDir failed: %v", err)
	}

	// Should contain .mooncake/presets
	if !filepath.IsAbs(userDir) {
		t.Errorf("Expected absolute path, got %s", userDir)
	}

	if !filepath.IsAbs(userDir) {
		t.Errorf("User dir should be absolute")
	}
}

func TestInstallToUserDir_FlatFormat(t *testing.T) {
	// Create temporary directories
	tmpCache := t.TempDir()
	tmpUser := t.TempDir()

	// Create cache structure with preset
	presetHash := "abc123"
	cacheDir := filepath.Join(tmpCache, presetHash)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	// Create preset file in cache
	presetFile := filepath.Join(cacheDir, "test.yml")
	content := []byte("name: test\nsteps: []")
	if err := os.WriteFile(presetFile, content, 0644); err != nil {
		t.Fatalf("Failed to create preset file: %v", err)
	}

	// Install to user directory
	if err := InstallToUserDir("test", tmpCache, tmpUser); err != nil {
		t.Fatalf("InstallToUserDir failed: %v", err)
	}

	// Verify installation
	installedFile := filepath.Join(tmpUser, "test.yml")
	if _, err := os.Stat(installedFile); os.IsNotExist(err) {
		t.Errorf("Installed file does not exist: %s", installedFile)
	}

	// Verify content
	installedContent, err := os.ReadFile(installedFile)
	if err != nil {
		t.Fatalf("Failed to read installed file: %v", err)
	}

	if string(installedContent) != string(content) {
		t.Errorf("Content mismatch: got %s, want %s", installedContent, content)
	}
}

func TestInstallToUserDir_DirectoryFormat(t *testing.T) {
	// Create temporary directories
	tmpCache := t.TempDir()
	tmpUser := t.TempDir()

	// Create cache structure with directory preset
	presetHash := "def456"
	cacheDir := filepath.Join(tmpCache, presetHash)
	presetDir := filepath.Join(cacheDir, "test")
	if err := os.MkdirAll(presetDir, 0755); err != nil {
		t.Fatalf("Failed to create preset dir: %v", err)
	}

	// Create preset files
	presetFile := filepath.Join(presetDir, "preset.yml")
	templateFile := filepath.Join(presetDir, "template.j2")
	if err := os.WriteFile(presetFile, []byte("name: test"), 0644); err != nil {
		t.Fatalf("Failed to create preset file: %v", err)
	}
	if err := os.WriteFile(templateFile, []byte("template"), 0644); err != nil {
		t.Fatalf("Failed to create template file: %v", err)
	}

	// Install to user directory
	if err := InstallToUserDir("test", tmpCache, tmpUser); err != nil {
		t.Fatalf("InstallToUserDir failed: %v", err)
	}

	// Verify installation
	installedDir := filepath.Join(tmpUser, "test")
	installedPreset := filepath.Join(installedDir, "preset.yml")
	installedTemplate := filepath.Join(installedDir, "template.j2")

	if _, err := os.Stat(installedPreset); os.IsNotExist(err) {
		t.Errorf("Installed preset file does not exist: %s", installedPreset)
	}

	if _, err := os.Stat(installedTemplate); os.IsNotExist(err) {
		t.Errorf("Installed template file does not exist: %s", installedTemplate)
	}
}

func TestInstallToUserDir_NotFound(t *testing.T) {
	tmpCache := t.TempDir()
	tmpUser := t.TempDir()

	// Try to install non-existent preset
	err := InstallToUserDir("nonexistent", tmpCache, tmpUser)
	if err == nil {
		t.Error("Expected error for non-existent preset")
	}
}
