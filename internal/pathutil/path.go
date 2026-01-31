package pathutil

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/alehatsman/mooncake/internal/template"
)

// PathExpander provides path expansion functionality
type PathExpander struct {
	renderer template.Renderer
}

// NewPathExpander creates a new PathExpander with the given template renderer
func NewPathExpander(renderer template.Renderer) *PathExpander {
	return &PathExpander{
		renderer: renderer,
	}
}

// ExpandPath expands a path with template variables and special prefixes (~, ., ./)
func (p *PathExpander) ExpandPath(originalPath string, currentDir string, context map[string]interface{}) (string, error) {
	// First, render any template variables in the path
	expandedPath, err := p.renderer.Render(originalPath, context)
	if err != nil {
		return "", err
	}

	expandedPath = strings.Trim(expandedPath, " ")

	// If path is empty, return it as-is
	if expandedPath == "" {
		return expandedPath, nil
	}

	// Handle home directory expansion ~/
	if strings.HasPrefix(expandedPath, "~/") {
		home := os.Getenv("HOME")
		expandedPath = home + expandedPath[1:]
		return expandedPath, nil
	}

	// If path is already absolute, return it
	if filepath.IsAbs(expandedPath) {
		return expandedPath, nil
	}

	// Handle relative paths starting with ../
	if strings.HasPrefix(expandedPath, "../") {
		expandedPath = filepath.Join(currentDir, expandedPath)
		return expandedPath, nil
	}

	// Handle current directory paths starting with ./
	if strings.HasPrefix(expandedPath, "./") {
		expandedPath = filepath.Join(currentDir, expandedPath)
		return expandedPath, nil
	}

	// Handle current directory paths starting with . (single dot)
	if strings.HasPrefix(expandedPath, ".") && len(expandedPath) > 1 && expandedPath[1] != '/' {
		expandedPath = filepath.Join(currentDir, expandedPath[1:])
		return expandedPath, nil
	}

	// For plain relative paths (no prefix), join with currentDir
	if !filepath.IsAbs(expandedPath) && currentDir != "" {
		expandedPath = filepath.Join(currentDir, expandedPath)
	}

	return expandedPath, nil
}

// ValidatePathWithinBase checks if targetPath is within baseDir (no path traversal escape)
// This is optional validation for security-sensitive operations
// Pass empty baseDir to skip validation
func ValidatePathWithinBase(targetPath string, baseDir string) error {
	if baseDir == "" {
		return nil // No validation requested
	}

	// Clean paths to resolve .. and . components
	cleanTarget := filepath.Clean(targetPath)
	cleanBase := filepath.Clean(baseDir)

	// Convert to absolute paths
	absTarget, err := filepath.Abs(cleanTarget)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for %s: %w", targetPath, err)
	}

	absBase, err := filepath.Abs(cleanBase)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path for base %s: %w", baseDir, err)
	}

	// Check if target is within base (or equal to it)
	// Use filepath.Rel to check if target is relative to base
	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil {
		return fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If the relative path starts with "..", it's outside the base directory
	if strings.HasPrefix(rel, "..") {
		return fmt.Errorf("path traversal detected: %s escapes base directory %s", targetPath, baseDir)
	}

	return nil
}

// SafeExpandPath is like ExpandPath but validates the result is within baseDir
// Pass empty baseDir to disable validation (same as ExpandPath)
func (p *PathExpander) SafeExpandPath(originalPath string, currentDir string, context map[string]interface{}, baseDir string) (string, error) {
	expandedPath, err := p.ExpandPath(originalPath, currentDir, context)
	if err != nil {
		return "", err
	}

	if err := ValidatePathWithinBase(expandedPath, baseDir); err != nil {
		return "", err
	}

	return expandedPath, nil
}

// GetDirectoryOfFile returns the directory containing the given file path
func GetDirectoryOfFile(path string) string {
	return filepath.Dir(path)
}
