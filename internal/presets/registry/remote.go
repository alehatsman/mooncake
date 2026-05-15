package registry

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const (
	officialRegistryName = "official"
	officialRegistryRepo = "alehatsman/mooncake"
	officialRegistryRef  = "master"
)

// RemoteRegistry is a named GitHub preset source.
type RemoteRegistry struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"` // e.g. "alehatsman/mooncake"
	Ref  string `yaml:"ref"`  // branch/tag, default "master"
}

// RegistriesConfig is stored at ~/.config/mooncake/registries.yml
type RegistriesConfig struct {
	Registries []RemoteRegistry `yaml:"registries"`
}

// RemotePresetMeta is one entry from an index.yml
type RemotePresetMeta struct {
	Name        string `yaml:"name"`
	Description string `yaml:"description"`
	Version     string `yaml:"version,omitempty"`
}

// RemoteIndex is the parsed index.yml from a registry
type RemoteIndex struct {
	Version int                `yaml:"version"`
	Presets []RemotePresetMeta `yaml:"presets"`
}

// githubContentsEntry represents one item from the GitHub Contents API response.
type githubContentsEntry struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Type        string `json:"type"` // "file" or "dir"
	DownloadURL string `json:"download_url"`
}

// DefaultRegistriesConfig returns config with the official registry as default.
func DefaultRegistriesConfig() RegistriesConfig {
	return RegistriesConfig{
		Registries: []RemoteRegistry{
			{
				Name: officialRegistryName,
				Repo: officialRegistryRepo,
				Ref:  officialRegistryRef,
			},
		},
	}
}

// registriesConfigPath returns the path to the registries config file.
func registriesConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".config", "mooncake", "registries.yml"), nil
}

// LoadRegistriesConfig loads ~/.config/mooncake/registries.yml.
// Returns the default config (official registry only) if file doesn't exist.
func LoadRegistriesConfig() (RegistriesConfig, error) {
	path, err := registriesConfigPath()
	if err != nil {
		return RegistriesConfig{}, err
	}

	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return DefaultRegistriesConfig(), nil
	}

	data, err := os.ReadFile(path) // #nosec G304 -- path is constructed from home directory
	if err != nil {
		return RegistriesConfig{}, fmt.Errorf("failed to read registries config: %w", err)
	}

	var cfg RegistriesConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return RegistriesConfig{}, fmt.Errorf("failed to parse registries config: %w", err)
	}

	return cfg, nil
}

// SaveRegistriesConfig writes the config to ~/.config/mooncake/registries.yml.
func SaveRegistriesConfig(cfg RegistriesConfig) error {
	path, err := registriesConfigPath()
	if err != nil {
		return err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(path), 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create config directory: %w", mkdirErr)
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to marshal registries config: %w", err)
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("failed to write registries config: %w", err)
	}

	return nil
}

// RegistryCacheDir returns ~/.mooncake/cache/registries/<name>/
func RegistryCacheDir(regName string) (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".mooncake", "cache", "registries", regName), nil
}

// IndexCachePath returns the path where the index.yml is cached for a registry.
func IndexCachePath(regName string) (string, error) {
	dir, err := RegistryCacheDir(regName)
	if err != nil {
		return "", err
	}

	return filepath.Join(dir, "index.yml"), nil
}

// FetchRemoteIndex downloads and parses index.yml from a registry.
// URL: https://raw.githubusercontent.com/{repo}/{ref}/index.yml
func FetchRemoteIndex(reg RemoteRegistry) (RemoteIndex, error) {
	ref := reg.Ref
	if ref == "" {
		ref = officialRegistryRef
	}

	url := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/index.yml", reg.Repo, ref)

	resp, err := http.Get(url) // #nosec G107 -- URL is constructed from registry config
	if err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to fetch index from registry '%s': %w", reg.Name, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return RemoteIndex{}, fmt.Errorf("failed to fetch index from registry '%s': HTTP %d", reg.Name, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to read index response from registry '%s': %w", reg.Name, err)
	}

	var index RemoteIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to parse index from registry '%s': %w", reg.Name, err)
	}

	return index, nil
}

// CacheRemoteIndex fetches and saves the index to the local cache.
func CacheRemoteIndex(reg RemoteRegistry) (RemoteIndex, error) {
	index, err := FetchRemoteIndex(reg)
	if err != nil {
		return RemoteIndex{}, err
	}

	cachePath, err := IndexCachePath(reg.Name)
	if err != nil {
		return RemoteIndex{}, err
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(cachePath), 0750); mkdirErr != nil {
		return RemoteIndex{}, fmt.Errorf("failed to create registry cache directory: %w", mkdirErr)
	}

	data, err := yaml.Marshal(index)
	if err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to marshal index for caching: %w", err)
	}

	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to write cached index: %w", err)
	}

	return index, nil
}

// LoadCachedIndex loads a previously cached index. Returns error if not cached.
func LoadCachedIndex(regName string) (RemoteIndex, error) {
	cachePath, err := IndexCachePath(regName)
	if err != nil {
		return RemoteIndex{}, err
	}

	if _, statErr := os.Stat(cachePath); os.IsNotExist(statErr) {
		return RemoteIndex{}, fmt.Errorf("index not cached for registry '%s'", regName)
	}

	data, err := os.ReadFile(cachePath) // #nosec G304 -- path is constructed from home directory and registry name
	if err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to read cached index for registry '%s': %w", regName, err)
	}

	var index RemoteIndex
	if err := yaml.Unmarshal(data, &index); err != nil {
		return RemoteIndex{}, fmt.Errorf("failed to parse cached index for registry '%s': %w", regName, err)
	}

	return index, nil
}

// listGitHubContents calls the GitHub Contents API to list files/dirs at the given path.
func listGitHubContents(repo, ref, path string) ([]githubContentsEntry, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/contents/%s?ref=%s", repo, path, ref)

	req, err := http.NewRequest(http.MethodGet, url, nil) // #nosec G107 -- URL is constructed from registry config
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list contents at '%s': %w", path, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API returned HTTP %d for path '%s'", resp.StatusCode, path)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read GitHub API response: %w", err)
	}

	var entries []githubContentsEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return nil, fmt.Errorf("failed to parse GitHub API response: %w", err)
	}

	return entries, nil
}

// downloadRawFile downloads a single file from raw.githubusercontent.com and writes it to destPath.
func downloadRawFile(rawURL, destPath string) error {
	resp, err := http.Get(rawURL) // #nosec G107 -- URL comes from GitHub API response
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to download file: HTTP %d", resp.StatusCode)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(destPath), 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create directory for file: %w", mkdirErr)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read file content: %w", err)
	}

	if err := os.WriteFile(destPath, data, 0600); err != nil { // #nosec G306 -- preset files may need to be world-readable
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// downloadContentsRecursive recursively downloads all files from a GitHub directory.
// remoteBasePath is the path prefix in the GitHub repo (e.g. "presets/neovim").
// localBasePath is the root destination directory.
func downloadContentsRecursive(repo, ref, remoteBasePath, localBasePath string) error {
	entries, err := listGitHubContents(repo, ref, remoteBasePath)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		// Compute relative path by stripping the base prefix
		relPath := strings.TrimPrefix(entry.Path, remoteBasePath+"/")
		localPath := filepath.Join(localBasePath, filepath.FromSlash(relPath))

		switch entry.Type {
		case "file":
			if entry.DownloadURL == "" {
				return fmt.Errorf("file '%s' has no download URL", entry.Path)
			}
			if err := downloadRawFile(entry.DownloadURL, localPath); err != nil {
				return fmt.Errorf("failed to download '%s': %w", entry.Path, err)
			}
		case "dir":
			if err := downloadContentsRecursive(repo, ref, entry.Path, localBasePath); err != nil {
				return err
			}
		}
	}

	return nil
}

// DownloadPreset downloads a preset directory from GitHub to destDir.
// Uses GitHub Contents API: https://api.github.com/repos/{repo}/contents/presets/{name}?ref={ref}
// Downloads all files recursively via raw.githubusercontent.com URLs.
func DownloadPreset(reg RemoteRegistry, name string, destDir string) error {
	ref := reg.Ref
	if ref == "" {
		ref = officialRegistryRef
	}

	remotePath := fmt.Sprintf("presets/%s", name)

	if mkdirErr := os.MkdirAll(destDir, 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create destination directory: %w", mkdirErr)
	}

	return downloadContentsRecursive(reg.Repo, ref, remotePath, destDir)
}
