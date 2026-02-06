// Package presets provides preset loading and expansion functionality.
package presets

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/config"
)

// PresetSearchPaths returns the ordered list of directories to search for presets.
// Priority order (highest to lowest):
// 1. ./presets/ (playbook directory)
// 2. ~/.mooncake/presets/ (user presets)
// 3. /usr/local/share/mooncake/presets/ (local installation)
// 4. /usr/share/mooncake/presets/ (system installation)
func PresetSearchPaths() []string {
	paths := []string{
		"./presets",
	}

	// Add user home directory preset path
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".mooncake", "presets"))
	}

	// Add system paths
	paths = append(paths,
		"/usr/local/share/mooncake/presets",
		"/usr/share/mooncake/presets",
	)

	return paths
}

// LoadPreset loads a preset definition by name.
// It searches for presets in two formats:
// 1. Flat: <name>.yml (e.g., presets/ollama.yml)
// 2. Directory: <name>/preset.yml (e.g., presets/ollama/preset.yml)
// Directory structure takes precedence if both exist.
// Returns the loaded PresetDefinition or an error if not found or invalid.
func LoadPreset(name string) (*config.PresetDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("preset name cannot be empty")
	}

	// Search for preset file (directory structure takes precedence)
	var presetPath string
	var found bool
	var baseDir string

	for _, searchPath := range PresetSearchPaths() {
		// Try directory structure first: <name>/preset.yml
		candidatePath := filepath.Join(searchPath, name, "preset.yml")
		if _, err := os.Stat(candidatePath); err == nil {
			presetPath = candidatePath
			baseDir = filepath.Join(searchPath, name)
			found = true
			break
		}

		// Fallback to flat structure: <name>.yml
		candidatePath = filepath.Join(searchPath, name+".yml")
		if _, err := os.Stat(candidatePath); err == nil {
			presetPath = candidatePath
			baseDir = searchPath
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("preset '%s' not found in search paths: %v", name, PresetSearchPaths())
	}

	// Read and parse preset file
	data, err := os.ReadFile(presetPath) // #nosec G304 -- presetPath is validated through search paths
	if err != nil {
		return nil, fmt.Errorf("failed to read preset file '%s': %w", presetPath, err)
	}

	// Parse YAML
	var preset config.PresetDefinition
	if err := yaml.Unmarshal(data, &preset); err != nil {
		return nil, fmt.Errorf("failed to parse preset file '%s': %w", presetPath, err)
	}

	// Validate preset structure
	if preset.Name == "" {
		return nil, fmt.Errorf("preset file '%s' missing required field 'name'", presetPath)
	}

	if preset.Name != name {
		return nil, fmt.Errorf("preset file '%s' name mismatch: expected '%s', got '%s'", presetPath, name, preset.Name)
	}

	if len(preset.Steps) == 0 {
		return nil, fmt.Errorf("preset '%s' has no steps defined", name)
	}

	// Validate that preset steps don't contain other preset invocations (no nesting)
	for i, step := range preset.Steps {
		if step.Preset != nil {
			return nil, fmt.Errorf("preset '%s' step %d: presets cannot invoke other presets (nesting not supported)", name, i+1)
		}
	}

	// Store the base directory in the preset for relative path resolution
	preset.BaseDir = baseDir

	return &preset, nil
}

// PresetInfo contains summary information about a discovered preset.
type PresetInfo struct {
	Name        string
	Description string
	Version     string
	Path        string
	Source      string // "local", "user", "system"
}

// DiscoverAllPresets finds all available presets in the search paths.
// Returns a sorted list of PresetInfo structs.
func DiscoverAllPresets() ([]PresetInfo, error) {
	seen := make(map[string]bool)
	var presets []PresetInfo

	searchPaths := PresetSearchPaths()
	for i, searchPath := range searchPaths {
		// Determine source type
		source := "system"
		if i == 0 {
			source = "local"
		} else if strings.Contains(searchPath, ".mooncake") {
			source = "user"
		}

		// Check if directory exists
		if _, err := os.Stat(searchPath); os.IsNotExist(err) {
			continue
		}

		// Look for flat format: *.yml files
		matches, err := filepath.Glob(filepath.Join(searchPath, "*.yml"))
		if err == nil {
			for _, match := range matches {
				name := strings.TrimSuffix(filepath.Base(match), ".yml")
				if seen[name] {
					continue // Skip duplicates (higher priority already found)
				}

				// Try to load preset to get metadata
				if preset, loadErr := LoadPreset(name); loadErr == nil {
					presets = append(presets, PresetInfo{
						Name:        preset.Name,
						Description: preset.Description,
						Version:     preset.Version,
						Path:        match,
						Source:      source,
					})
					seen[name] = true
				}
			}
		}

		// Look for directory format: */preset.yml
		entries, err := os.ReadDir(searchPath)
		if err != nil {
			continue
		}

		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}

			name := entry.Name()
			if seen[name] {
				continue
			}

			presetFile := filepath.Join(searchPath, name, "preset.yml")
			if _, err := os.Stat(presetFile); err == nil {
				// Try to load preset to get metadata
				if preset, loadErr := LoadPreset(name); loadErr == nil {
					presets = append(presets, PresetInfo{
						Name:        preset.Name,
						Description: preset.Description,
						Version:     preset.Version,
						Path:        presetFile,
						Source:      source,
					})
					seen[name] = true
				}
			}
		}
	}

	// Sort by name
	sort.Slice(presets, func(i, j int) bool {
		return presets[i].Name < presets[j].Name
	})

	return presets, nil
}
