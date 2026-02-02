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
// It searches for <name>.yml in the preset search paths.
// Returns the loaded PresetDefinition or an error if not found or invalid.
func LoadPreset(name string) (*config.PresetDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("preset name cannot be empty")
	}

	// Search for preset file
	fileName := name + ".yml"
	var presetPath string
	var found bool

	for _, searchPath := range PresetSearchPaths() {
		candidatePath := filepath.Join(searchPath, fileName)
		if _, err := os.Stat(candidatePath); err == nil {
			presetPath = candidatePath
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

	return preset, nil
}
