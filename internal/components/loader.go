// Package components provides component loading and expansion functionality.
package components

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
)

// ComponentSearchPaths returns the ordered list of directories to search for components.
// Priority order (highest to lowest):
// 1. ./components/ (playbook directory)
// 2. ~/.mooncake/components/ (user components)
// 3. /usr/local/share/mooncake/components/ (local installation)
// 4. /usr/share/mooncake/components/ (system installation)
//
// Implemented as a package-level variable so tests can stub discovery
// to a hermetic path set. Production code calls it as a function;
// callers don't need to know it's a variable.
var ComponentSearchPaths = defaultComponentSearchPaths

func defaultComponentSearchPaths() []string {
	paths := []string{
		"./components",
	}

	// Add user home directory component path
	if home, err := os.UserHomeDir(); err == nil {
		paths = append(paths, filepath.Join(home, ".mooncake", "components"))
	}

	// Add system paths
	paths = append(paths,
		"/usr/local/share/mooncake/components",
		"/usr/share/mooncake/components",
	)

	return paths
}

// LoadComponent loads a component definition by name.
// It searches for components in two formats:
// 1. Flat: <name>.yml (e.g., components/ollama.yml)
// 2. Directory: <name>/component.yml (e.g., components/ollama/component.yml)
// Directory structure takes precedence if both exist.
// Returns the loaded ComponentDefinition or an error if not found or invalid.
func LoadComponent(name string) (*config.ComponentDefinition, error) {
	if name == "" {
		return nil, fmt.Errorf("component name cannot be empty")
	}

	// Search for component file (directory structure takes precedence)
	var componentPath string
	var found bool
	var baseDir string

	for _, searchPath := range ComponentSearchPaths() {
		// Try directory structure first: <name>/component.yml
		candidatePath := filepath.Join(searchPath, name, "component.yml")
		if _, err := os.Stat(candidatePath); err == nil {
			componentPath = candidatePath
			baseDir = filepath.Join(searchPath, name)
			found = true
			break
		}

		// Fallback to flat structure: <name>.yml
		candidatePath = filepath.Join(searchPath, name+".yml")
		if _, err := os.Stat(candidatePath); err == nil {
			componentPath = candidatePath
			baseDir = searchPath
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("component '%s' not found in search paths: %v", name, ComponentSearchPaths())
	}

	// Read and parse component file
	data, err := os.ReadFile(componentPath) // #nosec G304 -- componentPath is validated through search paths
	if err != nil {
		return nil, fmt.Errorf("failed to read component file '%s': %w", componentPath, err)
	}

	// Parse component (YAML or JSON, auto-detected from content)
	var component config.ComponentDefinition
	if err := config.DecodeAuto(data, &component); err != nil {
		return nil, fmt.Errorf("failed to parse component file '%s': %w", componentPath, err)
	}

	// Validate component structure
	if component.Name == "" {
		return nil, fmt.Errorf("component file '%s' missing required field 'name'", componentPath)
	}

	if component.Name != name {
		return nil, fmt.Errorf("component file '%s' name mismatch: expected '%s', got '%s'", componentPath, name, component.Name)
	}

	if len(component.Steps) == 0 {
		return nil, fmt.Errorf("component '%s' has no steps defined", name)
	}

	// Validate that component steps don't contain other component invocations (no nesting)
	for i, step := range component.Steps {
		if step.Use != "" {
			return nil, fmt.Errorf("component '%s' step %d: components cannot invoke other components (nesting not supported)", name, i+1)
		}
	}

	// Store the base directory in the component for relative path resolution
	component.BaseDir = baseDir

	return &component, nil
}

// LoadComponentFromPath loads a component definition from an explicit
// filesystem path. Used by spec-67 local-component (`use: ./foo.yml`) and
// remote-module dispatch, where the loader has already resolved the file
// without going through search paths.
//
// If the file omits `name:`, the filename stem is used so downstream code that
// expects a non-empty Name still works.
func LoadComponentFromPath(path string) (*config.ComponentDefinition, error) {
	if path == "" {
		return nil, fmt.Errorf("component path is empty")
	}
	data, err := os.ReadFile(path) // #nosec G304 -- path is validated by caller (executor resolves relative paths against the playbook dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("component not found: %s", path)
		}
		return nil, fmt.Errorf("read component %s: %w", path, err)
	}
	var component config.ComponentDefinition
	if err := config.DecodeAuto(data, &component); err != nil {
		return nil, fmt.Errorf("parse component %s: %w", path, err)
	}
	if component.Name == "" {
		component.Name = strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	}
	if len(component.Steps) == 0 {
		return nil, fmt.Errorf("component %s has no steps defined", path)
	}
	component.BaseDir = filepath.Dir(path)
	return &component, nil
}

// ComponentInfo contains summary information about a discovered component.
type ComponentInfo struct {
	Name        string
	Description string
	Version     string
	Path        string
	Source      string // "local", "user", "system"
}

// DiscoverAllComponents finds all available components in the search paths.
// Returns a sorted list of ComponentInfo structs.
func DiscoverAllComponents() ([]ComponentInfo, error) {
	seen := make(map[string]bool)
	var components []ComponentInfo

	searchPaths := ComponentSearchPaths()
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

				// Try to load component to get metadata
				if component, loadErr := LoadComponent(name); loadErr == nil {
					components = append(components, ComponentInfo{
						Name:        component.Name,
						Description: component.Description,
						Version:     component.Version,
						Path:        match,
						Source:      source,
					})
					seen[name] = true
				}
			}
		}

		// Look for directory format: */component.yml
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

			componentFile := filepath.Join(searchPath, name, "component.yml")
			if _, err := os.Stat(componentFile); err == nil {
				// Try to load component to get metadata
				if component, loadErr := LoadComponent(name); loadErr == nil {
					components = append(components, ComponentInfo{
						Name:        component.Name,
						Description: component.Description,
						Version:     component.Version,
						Path:        componentFile,
						Source:      source,
					})
					seen[name] = true
				}
			}
		}
	}

	// Sort by name
	sort.Slice(components, func(i, j int) bool {
		return components[i].Name < components[j].Name
	})

	return components, nil
}
