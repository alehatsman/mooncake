// Package presets provides preset loading and expansion functionality.
package presets

import (
	"fmt"
	"os"
	"path/filepath"

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
	var wrapper struct {
		Preset config.PresetDefinition `yaml:"preset"`
	}
	if err := yaml.Unmarshal(data, &wrapper); err != nil {
		return nil, fmt.Errorf("failed to parse preset file '%s': %w", presetPath, err)
	}

	preset := &wrapper.Preset

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

	return preset, nil
}
