package registry

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultRegistriesConfig(t *testing.T) {
	cfg := DefaultRegistriesConfig()

	if len(cfg.Registries) == 0 {
		t.Fatal("expected at least one registry in default config")
	}

	official := cfg.Registries[0]
	if official.Name != officialRegistryName {
		t.Errorf("expected registry name %q, got %q", officialRegistryName, official.Name)
	}
	if official.Repo != officialRegistryRepo {
		t.Errorf("expected registry repo %q, got %q", officialRegistryRepo, official.Repo)
	}
	if official.Ref != officialRegistryRef {
		t.Errorf("expected registry ref %q, got %q", officialRegistryRef, official.Ref)
	}
}

func TestLoadRegistriesConfig_Default(t *testing.T) {
	// Point home to a temp directory so no real config file exists
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg, err := LoadRegistriesConfig()
	if err != nil {
		t.Fatalf("LoadRegistriesConfig returned unexpected error: %v", err)
	}

	if len(cfg.Registries) == 0 {
		t.Fatal("expected default registries when no config file exists")
	}

	if cfg.Registries[0].Name != officialRegistryName {
		t.Errorf("expected default registry name %q, got %q", officialRegistryName, cfg.Registries[0].Name)
	}
}

func TestLoadRegistriesConfig_ExistingFile(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Create the config directory and file
	configDir := filepath.Join(tmpHome, ".config", "mooncake")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	customCfg := RegistriesConfig{
		Registries: []RemoteRegistry{
			{Name: "custom", Repo: "user/repo", Ref: "dev"},
		},
	}
	data, err := yaml.Marshal(customCfg)
	if err != nil {
		t.Fatalf("failed to marshal config: %v", err)
	}

	configPath := filepath.Join(configDir, "registries.yml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := LoadRegistriesConfig()
	if err != nil {
		t.Fatalf("LoadRegistriesConfig returned unexpected error: %v", err)
	}

	if len(cfg.Registries) != 1 {
		t.Fatalf("expected 1 registry, got %d", len(cfg.Registries))
	}
	if cfg.Registries[0].Name != "custom" {
		t.Errorf("expected registry name %q, got %q", "custom", cfg.Registries[0].Name)
	}
	if cfg.Registries[0].Repo != "user/repo" {
		t.Errorf("expected repo %q, got %q", "user/repo", cfg.Registries[0].Repo)
	}
}

func TestSaveRegistriesConfig(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	cfg := RegistriesConfig{
		Registries: []RemoteRegistry{
			{Name: "test", Repo: "test/repo", Ref: "main"},
		},
	}

	if err := SaveRegistriesConfig(cfg); err != nil {
		t.Fatalf("SaveRegistriesConfig returned unexpected error: %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpHome, ".config", "mooncake", "registries.yml")
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("expected registries.yml to be created")
	}

	// Reload and verify
	loaded, err := LoadRegistriesConfig()
	if err != nil {
		t.Fatalf("LoadRegistriesConfig after save returned error: %v", err)
	}
	if len(loaded.Registries) != 1 || loaded.Registries[0].Name != "test" {
		t.Errorf("loaded config doesn't match saved config: %+v", loaded)
	}
}

func TestRemoteIndex_ParseYAML(t *testing.T) {
	sampleYAML := `version: 1
presets:
  - name: hyprland
    description: Install Hyprland Wayland compositor
    version: "1.0.0"
  - name: neovim
    description: Install Neovim text editor with plugin manager
    version: "1.0.0"
  - name: zsh
    description: Z shell configuration
`
	var index RemoteIndex
	if err := yaml.Unmarshal([]byte(sampleYAML), &index); err != nil {
		t.Fatalf("failed to parse index YAML: %v", err)
	}

	if index.Version != 1 {
		t.Errorf("expected version 1, got %d", index.Version)
	}

	if len(index.Presets) != 3 {
		t.Fatalf("expected 3 presets, got %d", len(index.Presets))
	}

	tests := []struct {
		name        string
		description string
		version     string
	}{
		{"hyprland", "Install Hyprland Wayland compositor", "1.0.0"},
		{"neovim", "Install Neovim text editor with plugin manager", "1.0.0"},
		{"zsh", "Z shell configuration", ""},
	}

	for i, tt := range tests {
		p := index.Presets[i]
		if p.Name != tt.name {
			t.Errorf("preset[%d] name: expected %q, got %q", i, tt.name, p.Name)
		}
		if p.Description != tt.description {
			t.Errorf("preset[%d] description: expected %q, got %q", i, tt.description, p.Description)
		}
		if p.Version != tt.version {
			t.Errorf("preset[%d] version: expected %q, got %q", i, tt.version, p.Version)
		}
	}
}

func TestRegistryCacheDir(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	dir, err := RegistryCacheDir("official")
	if err != nil {
		t.Fatalf("RegistryCacheDir returned unexpected error: %v", err)
	}

	expected := filepath.Join(tmpHome, ".mooncake", "cache", "registries", "official")
	if dir != expected {
		t.Errorf("expected %q, got %q", expected, dir)
	}
}

func TestIndexCachePath(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	path, err := IndexCachePath("official")
	if err != nil {
		t.Fatalf("IndexCachePath returned unexpected error: %v", err)
	}

	expected := filepath.Join(tmpHome, ".mooncake", "cache", "registries", "official", "index.yml")
	if path != expected {
		t.Errorf("expected %q, got %q", expected, path)
	}
}

func TestLoadCachedIndex_NotFound(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	_, err := LoadCachedIndex("nonexistent")
	if err == nil {
		t.Fatal("expected error when loading non-existent cached index")
	}
}

func TestLoadCachedIndex_Found(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	// Write a cached index file
	cacheDir := filepath.Join(tmpHome, ".mooncake", "cache", "registries", "test")
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		t.Fatalf("failed to create cache dir: %v", err)
	}

	index := RemoteIndex{
		Version: 1,
		Presets: []RemotePresetMeta{
			{Name: "foo", Description: "A test preset", Version: "1.0.0"},
		},
	}
	data, err := yaml.Marshal(index)
	if err != nil {
		t.Fatalf("failed to marshal index: %v", err)
	}

	indexPath := filepath.Join(cacheDir, "index.yml")
	if err := os.WriteFile(indexPath, data, 0600); err != nil {
		t.Fatalf("failed to write index: %v", err)
	}

	loaded, err := LoadCachedIndex("test")
	if err != nil {
		t.Fatalf("LoadCachedIndex returned unexpected error: %v", err)
	}

	if loaded.Version != 1 {
		t.Errorf("expected version 1, got %d", loaded.Version)
	}
	if len(loaded.Presets) != 1 || loaded.Presets[0].Name != "foo" {
		t.Errorf("loaded index presets don't match: %+v", loaded.Presets)
	}
}
