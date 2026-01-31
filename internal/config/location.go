package config

import (
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// Position represents a line and column position in a source file
type Position struct {
	Line   int
	Column int
}

// LocationMap tracks YAML source positions for validation error reporting
type LocationMap struct {
	locations map[string]Position
}

// NewLocationMap creates a new LocationMap
func NewLocationMap() *LocationMap {
	return &LocationMap{
		locations: make(map[string]Position),
	}
}

// Set stores a position for a given JSON pointer path
func (lm *LocationMap) Set(path string, line, column int) {
	lm.locations[path] = Position{Line: line, Column: column}
}

// Get retrieves the position for a given JSON pointer path
// Returns zero Position if not found
func (lm *LocationMap) Get(path string) Position {
	return lm.locations[path]
}

// GetOrDefault retrieves the position for a given JSON pointer path
// Returns the default position if not found
func (lm *LocationMap) GetOrDefault(path string, defaultPos Position) Position {
	if pos, ok := lm.locations[path]; ok {
		return pos
	}
	return defaultPos
}

// buildLocationMap traverses a yaml.Node tree and builds a map of JSON pointers to source positions
func buildLocationMap(node *yaml.Node) *LocationMap {
	lm := NewLocationMap()
	buildLocationMapRecursive(node, "", lm)
	return lm
}

// buildLocationMapRecursive recursively traverses the yaml.Node tree
func buildLocationMapRecursive(node *yaml.Node, path string, lm *LocationMap) {
	if node == nil {
		return
	}

	// Store the position for this node
	if path != "" {
		lm.Set(path, node.Line, node.Column)
	}

	switch node.Kind {
	case yaml.DocumentNode:
		// Document node wraps the actual content
		if len(node.Content) > 0 {
			buildLocationMapRecursive(node.Content[0], path, lm)
		}

	case yaml.SequenceNode:
		// Array/list node
		for i, child := range node.Content {
			childPath := formatArrayPath(path, i)
			buildLocationMapRecursive(child, childPath, lm)
		}

	case yaml.MappingNode:
		// Object/map node - Content alternates between keys and values
		for i := 0; i < len(node.Content); i += 2 {
			if i+1 >= len(node.Content) {
				break
			}

			keyNode := node.Content[i]
			valueNode := node.Content[i+1]

			// Get the field name from the key node
			fieldName := keyNode.Value

			// Build the child path
			childPath := formatObjectPath(path, fieldName)

			// Store position for the field (use key node position as it's more accurate)
			lm.Set(childPath, keyNode.Line, keyNode.Column)

			// Recursively process the value
			buildLocationMapRecursive(valueNode, childPath, lm)
		}

	case yaml.ScalarNode, yaml.AliasNode:
		// Leaf nodes - position already stored above
		// No further traversal needed
	}
}

// formatArrayPath formats a JSON pointer path for array elements
// e.g., "" + 0 -> "/0", "/steps" + 1 -> "/steps/1"
func formatArrayPath(parentPath string, index int) string {
	if parentPath == "" {
		return fmt.Sprintf("/%d", index)
	}
	return fmt.Sprintf("%s/%d", parentPath, index)
}

// formatObjectPath formats a JSON pointer path for object properties
// e.g., "" + "steps" -> "/steps", "/steps/0" + "name" -> "/steps/0/name"
func formatObjectPath(parentPath string, fieldName string) string {
	// Escape special characters in JSON pointer
	escapedFieldName := escapeJSONPointer(fieldName)

	if parentPath == "" {
		return fmt.Sprintf("/%s", escapedFieldName)
	}
	return fmt.Sprintf("%s/%s", parentPath, escapedFieldName)
}

// escapeJSONPointer escapes special characters in JSON pointer tokens
// Per RFC 6901: ~ must be escaped as ~0, / must be escaped as ~1
func escapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~", "~0")
	s = strings.ReplaceAll(s, "/", "~1")
	return s
}

// unescapeJSONPointer unescapes JSON pointer tokens
func unescapeJSONPointer(s string) string {
	s = strings.ReplaceAll(s, "~1", "/")
	s = strings.ReplaceAll(s, "~0", "~")
	return s
}
