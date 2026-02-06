package registry

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultCacheDir returns the default cache directory for presets.
// Returns ~/.mooncake/cache/presets
func DefaultCacheDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".mooncake", "cache", "presets"), nil
}

// UserPresetsDir returns the user presets directory.
// Returns ~/.mooncake/presets
func UserPresetsDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get user home directory: %w", err)
	}

	return filepath.Join(home, ".mooncake", "presets"), nil
}

// InstallToUserDir installs a preset from the cache to the user presets directory.
// It copies (or symlinks on Unix) the preset files to make them available for use.
func InstallToUserDir(name string, cacheDir string, userDir string) error {
	// Ensure user presets directory exists
	if err := os.MkdirAll(userDir, 0755); err != nil {
		return fmt.Errorf("failed to create user presets directory: %w", err)
	}

	// Source: cache/<sha256>/<name>.yml or cache/<sha256>/<name>/
	// Target: ~/.mooncake/presets/<name>.yml or ~/.mooncake/presets/<name>/

	// Find the cached preset (try both flat and directory formats)
	var sourceDir string
	var found bool

	// Search all cache entries
	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() || entry.Name() == "manifest.json" {
			continue
		}

		cachePath := filepath.Join(cacheDir, entry.Name())

		// Check for flat format: <sha256>/<name>.yml
		flatPath := filepath.Join(cachePath, name+".yml")
		if _, statErr := os.Stat(flatPath); statErr == nil {
			sourceDir = cachePath
			found = true
			break
		}

		// Check for directory format: <sha256>/<name>/preset.yml
		dirPath := filepath.Join(cachePath, name, "preset.yml")
		if _, statErr := os.Stat(dirPath); statErr == nil {
			sourceDir = cachePath
			found = true
			break
		}
	}

	if !found {
		return fmt.Errorf("cached preset '%s' not found in cache directory", name)
	}

	// Determine format and copy accordingly
	flatSource := filepath.Join(sourceDir, name+".yml")
	dirSource := filepath.Join(sourceDir, name)

	if _, err := os.Stat(flatSource); err == nil {
		// Flat format: copy <name>.yml
		target := filepath.Join(userDir, name+".yml")
		if err := copyFile(flatSource, target); err != nil {
			return fmt.Errorf("failed to install flat preset: %w", err)
		}
	} else if _, err := os.Stat(dirSource); err == nil {
		// Directory format: copy entire directory
		target := filepath.Join(userDir, name)
		if err := copyDir(dirSource, target); err != nil {
			return fmt.Errorf("failed to install directory preset: %w", err)
		}
	} else {
		return fmt.Errorf("preset files not found in cache")
	}

	return nil
}

// copyFile copies a single file from src to dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) // #nosec G304 -- src is validated by caller
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, 0644) // #nosec G306 -- preset files are not sensitive
}

// copyDir recursively copies a directory from src to dst
func copyDir(src, dst string) error {
	// Get source directory info
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	// Create destination directory
	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	// Read source directory
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	// Copy each entry
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Recursively copy subdirectory
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}
