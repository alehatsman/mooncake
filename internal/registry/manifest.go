// Package registry provides preset distribution and caching functionality.
package registry

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// ManifestEntry represents a single preset in the registry manifest.
type ManifestEntry struct {
	Name        string    `json:"name"`
	Source      string    `json:"source"`
	Type        string    `json:"type"` // "url", "git", "path"
	SHA256      string    `json:"sha256"`
	InstalledAt time.Time `json:"installed_at"`
	Version     string    `json:"version,omitempty"`
}

// Manifest tracks all installed presets from external sources.
type Manifest struct {
	Presets []ManifestEntry `json:"presets"`
	path    string
}

// LoadManifest loads the manifest from the cache directory.
// Creates a new empty manifest if it doesn't exist.
func LoadManifest(cacheDir string) (*Manifest, error) {
	manifestPath := filepath.Join(cacheDir, "manifest.json")

	// Create cache directory if it doesn't exist
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// If manifest doesn't exist, create empty one
	if _, err := os.Stat(manifestPath); os.IsNotExist(err) {
		return &Manifest{
			Presets: []ManifestEntry{},
			path:    manifestPath,
		}, nil
	}

	// Read existing manifest
	data, err := os.ReadFile(manifestPath) // #nosec G304 -- manifestPath is controlled
	if err != nil {
		return nil, fmt.Errorf("failed to read manifest: %w", err)
	}

	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return nil, fmt.Errorf("failed to parse manifest: %w", err)
	}

	manifest.path = manifestPath
	return &manifest, nil
}

// Save writes the manifest to disk.
func (m *Manifest) Save() error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	if err := os.WriteFile(m.path, data, 0644); err != nil { // #nosec G306 -- manifest is not sensitive
		return fmt.Errorf("failed to write manifest: %w", err)
	}

	return nil
}

// Add adds or updates a preset entry in the manifest.
func (m *Manifest) Add(entry ManifestEntry) {
	// Remove existing entry with same name
	m.Remove(entry.Name)

	// Add new entry
	m.Presets = append(m.Presets, entry)
}

// Remove removes a preset entry from the manifest.
func (m *Manifest) Remove(name string) {
	filtered := make([]ManifestEntry, 0, len(m.Presets))
	for _, entry := range m.Presets {
		if entry.Name != name {
			filtered = append(filtered, entry)
		}
	}
	m.Presets = filtered
}

// Get retrieves a preset entry by name.
func (m *Manifest) Get(name string) *ManifestEntry {
	for i := range m.Presets {
		if m.Presets[i].Name == name {
			return &m.Presets[i]
		}
	}
	return nil
}

// CalculateSHA256 calculates the SHA256 hash of a file.
func CalculateSHA256(path string) (string, error) {
	file, err := os.Open(path) // #nosec G304 -- path is validated by caller
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("failed to calculate hash: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
