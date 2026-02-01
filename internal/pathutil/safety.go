// Package pathutil provides path manipulation and safety validation utilities.
package pathutil

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ValidateRemovalPath validates that a path is safe to remove.
// Returns an error if the path is dangerous (empty, root, system directories).
func ValidateRemovalPath(path string) error {
	if path == "" {
		return fmt.Errorf("refusing to remove empty path")
	}

	// Clean and get absolute path
	absPath, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	// Dangerous paths that should never be removed
	dangerous := []string{
		"/",
		"/bin",
		"/sbin",
		"/usr",
		"/usr/bin",
		"/usr/sbin",
		"/etc",
		"/boot",
		"/sys",
		"/proc",
		"/dev",
		"C:\\",
		"C:\\Windows",
		"C:\\Windows\\System32",
		"C:\\Program Files",
	}

	for _, dangerousPath := range dangerous {
		// Check exact match
		if absPath == dangerousPath {
			return fmt.Errorf("refusing to remove system path: %s", absPath)
		}
		// Check if it's a cleaned version
		if filepath.Clean(absPath) == filepath.Clean(dangerousPath) {
			return fmt.Errorf("refusing to remove system path: %s", absPath)
		}
	}

	return nil
}

// ValidateNoPathTraversal validates that a path doesn't contain path traversal sequences.
// This prevents attacks like extracting files to ../../../etc/passwd.
func ValidateNoPathTraversal(path string) error {
	// Clean the path to resolve any . or .. components
	cleanPath := filepath.Clean(path)

	// Check if the clean path tries to go up directories
	if strings.HasPrefix(cleanPath, "..") {
		return fmt.Errorf("path traversal detected: path attempts to escape base directory")
	}

	// Check for absolute paths when relative is expected
	if filepath.IsAbs(cleanPath) {
		return fmt.Errorf("absolute path not allowed in this context: %s", path)
	}

	// Additional check for .. anywhere in the path
	parts := strings.Split(filepath.ToSlash(cleanPath), "/")
	for _, part := range parts {
		if part == ".." {
			return fmt.Errorf("path traversal detected: .. found in path")
		}
	}

	return nil
}

// SafeJoin joins path elements and validates the result stays within the base directory.
// This is similar to filepath.Join but with traversal protection.
func SafeJoin(base string, elem ...string) (string, error) {
	// Join all elements
	joined := filepath.Join(append([]string{base}, elem...)...)

	// Get absolute paths for comparison
	absBase, err := filepath.Abs(base)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base path: %w", err)
	}

	absJoined, err := filepath.Abs(joined)
	if err != nil {
		return "", fmt.Errorf("failed to resolve joined path: %w", err)
	}

	// Verify the joined path is within base directory
	relPath, err := filepath.Rel(absBase, absJoined)
	if err != nil {
		return "", fmt.Errorf("failed to compute relative path: %w", err)
	}

	// If relative path starts with .., it's outside base
	if strings.HasPrefix(relPath, "..") {
		return "", fmt.Errorf("path traversal detected: result would be outside base directory")
	}

	return joined, nil
}
