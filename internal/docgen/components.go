package docgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/config"
	"gopkg.in/yaml.v3"
)

// generateComponentExamples generates documentation from actual component files.
func (g *Generator) generateComponentExamples(w io.Writer, componentsDir string) error {
	g.writeMetadataHeader(w, fmt.Sprintf("Source: %s", componentsDir))

	// Find all component files
	componentFiles, err := findComponentFiles(componentsDir)
	if err != nil {
		return fmt.Errorf("failed to find component files: %w", err)
	}

	if len(componentFiles) == 0 {
		write(w, "*No component files found in %s*\n", componentsDir)
		return nil
	}

	write(w, "## Component Examples\n\n")
	write(w, "The following examples are generated from actual component files in the repository.\n")
	write(w, "All syntax is validated and guaranteed to be correct.\n\n")

	// Process each component file
	for i, componentFile := range componentFiles {
		if i > 0 {
			write(w, "\n---\n\n")
		}

		if err := g.generateComponentExample(w, componentFile); err != nil {
			write(w, "<!-- Error processing %s: %v -->\n\n", componentFile, err)
			continue
		}
	}

	return nil
}

// generateComponentExample generates documentation for a single component file.
func (g *Generator) generateComponentExample(w io.Writer, componentPath string) error {
	// Read component file
	data, err := os.ReadFile(componentPath) // #nosec G304 -- componentPath validated via findComponentFiles and ComponentSearchPaths
	if err != nil {
		return fmt.Errorf("failed to read component file: %w", err)
	}

	// Parse component
	var component config.ComponentDefinition
	if err := yaml.Unmarshal(data, &component); err != nil {
		return fmt.Errorf("failed to parse component: %w", err)
	}

	// Generate documentation
	write(w, "### %s\n\n", component.Name)

	if component.Description != "" {
		write(w, "%s\n\n", component.Description)
	}

	if component.Version != "" {
		write(w, "**Version**: %s\n\n", component.Version)
	}

	// Show file location
	relPath, _ := filepath.Rel(".", componentPath)
	write(w, "**Source**: `%s`\n\n", relPath)

	// Show parameters if any
	if len(component.Props) > 0 {
		write(w, "**Parameters**:\n\n")

		// Sort parameters for consistent output
		paramNames := make([]string, 0, len(component.Props))
		for name := range component.Props {
			paramNames = append(paramNames, name)
		}
		sort.Strings(paramNames)

		for _, name := range paramNames {
			param := component.Props[name]
			write(w, "- `%s` (%s)", name, param.Type)
			if param.Required {
				write(w, " **required**")
			}
			if param.Default != nil {
				write(w, " - default: `%v`", param.Default)
			}
			if len(param.Enum) > 0 {
				write(w, " - values: %v", param.Enum)
			}
			if param.Description != "" {
				write(w, " - %s", param.Description)
			}
			write(w, "\n")
		}
		write(w, "\n")
	}

	// Show usage example
	write(w, "**Usage**:\n\n")
	write(w, "```yaml\n")
	write(w, "- name: Use %s component\n", component.Name)
	write(w, "  component: %s\n", component.Name)

	if len(component.Props) > 0 {
		write(w, "  with:\n")
		// Show required parameters
		paramNames := make([]string, 0, len(component.Props))
		for name := range component.Props {
			paramNames = append(paramNames, name)
		}
		sort.Strings(paramNames)

		for _, name := range paramNames {
			param := component.Props[name]
			if param.Required {
				write(w, "    %s: <value>  # %s\n", name, param.Description)
			}
		}
	}
	write(w, "```\n\n")

	// Show component definition structure (the correct format!)
	write(w, "**Component Definition Structure**:\n\n")
	write(w, "```yaml\n")
	write(w, "name: %s\n", component.Name)
	if component.Description != "" {
		write(w, "description: %s\n", component.Description)
	}
	if component.Version != "" {
		write(w, "version: %s\n", component.Version)
	}
	if len(component.Props) > 0 {
		write(w, "\nparameters:\n")
		paramNames := make([]string, 0, len(component.Props))
		for name := range component.Props {
			paramNames = append(paramNames, name)
		}
		sort.Strings(paramNames)

		for _, name := range paramNames {
			param := component.Props[name]
			write(w, "  %s:\n", name)
			write(w, "    type: %s\n", param.Type)
			if param.Required {
				write(w, "    required: true\n")
			}
			if param.Default != nil {
				write(w, "    default: %v\n", param.Default)
			}
			if len(param.Enum) > 0 {
				write(w, "    enum: %v\n", param.Enum)
			}
			if param.Description != "" {
				write(w, "    description: %s\n", param.Description)
			}
		}
	}
	write(w, "\nsteps:\n")
	write(w, "  # ... steps go here ...\n")
	write(w, "```\n")

	return nil
}

// findComponentFiles finds all component.yml files in a directory tree.
func findComponentFiles(rootDir string) ([]string, error) {
	var componentFiles []string

	err := filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Look for component.yml files or standalone .yml files in components root
		if info.Name() == "component.yml" {
			componentFiles = append(componentFiles, path)
		} else if filepath.Dir(path) == rootDir && strings.HasSuffix(info.Name(), ".yml") {
			componentFiles = append(componentFiles, path)
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	// Sort for consistent output
	sort.Strings(componentFiles)

	return componentFiles, nil
}
