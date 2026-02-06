package registry

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// SourceType represents the type of preset source.
type SourceType string

const (
	SourceTypeURL  SourceType = "url"
	SourceTypeGit  SourceType = "git"
	SourceTypePath SourceType = "path"
)

// DetectSourceType determines the type of source based on the input string.
func DetectSourceType(source string) SourceType {
	// Check for URL (http:// or https://)
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		return SourceTypeURL
	}

	// Check for git (ends with .git or contains github.com/gitlab.com)
	if strings.HasSuffix(source, ".git") ||
		strings.Contains(source, "github.com") ||
		strings.Contains(source, "gitlab.com") {
		return SourceTypeGit
	}

	// Default to path (local file system)
	return SourceTypePath
}

// FetchSource downloads or copies a preset from the source to the cache directory.
// Returns the path to the cached preset directory.
func FetchSource(source string, sourceType SourceType, cacheDir string, sha256hash string) (string, error) {
	targetDir := filepath.Join(cacheDir, sha256hash)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create cache directory: %w", err)
	}

	switch sourceType {
	case SourceTypeURL:
		return fetchFromURL(source, targetDir)
	case SourceTypeGit:
		return fetchFromGit(source, targetDir)
	case SourceTypePath:
		return fetchFromPath(source, targetDir)
	default:
		return "", fmt.Errorf("unsupported source type: %s", sourceType)
	}
}

// fetchFromURL downloads a preset file from a URL.
func fetchFromURL(url string, targetDir string) (string, error) {
	// Download file
	resp, err := http.Get(url) // #nosec G107 -- URL is provided by user
	if err != nil {
		return "", fmt.Errorf("failed to download from URL: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download: HTTP %d", resp.StatusCode)
	}

	// Determine filename from URL
	filename := filepath.Base(url)
	if !strings.HasSuffix(filename, ".yml") && !strings.HasSuffix(filename, ".yaml") {
		filename += ".yml"
	}

	targetPath := filepath.Join(targetDir, filename)

	// Create target file
	outFile, err := os.Create(targetPath) // #nosec G304 -- targetPath is controlled
	if err != nil {
		return "", fmt.Errorf("failed to create target file: %w", err)
	}
	defer outFile.Close()

	// Copy content
	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return "", fmt.Errorf("failed to write file: %w", err)
	}

	return targetDir, nil
}

// fetchFromGit clones a git repository (currently a stub - requires git executable).
func fetchFromGit(gitURL string, targetDir string) (string, error) {
	// TODO: Implement git clone functionality
	// For v1, this can shell out to git command
	// Example: git clone --depth 1 <gitURL> <targetDir>
	return "", fmt.Errorf("git sources not yet implemented (coming in registry v2)")
}

// fetchFromPath copies a preset from a local file system path.
func fetchFromPath(source string, targetDir string) (string, error) {
	// Resolve absolute path
	absPath, err := filepath.Abs(source)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	// Check if source exists
	info, err := os.Stat(absPath)
	if err != nil {
		return "", fmt.Errorf("source path does not exist: %w", err)
	}

	if info.IsDir() {
		// Copy entire directory
		return targetDir, copyDirContents(absPath, targetDir)
	}

	// Copy single file
	filename := filepath.Base(absPath)
	targetPath := filepath.Join(targetDir, filename)

	data, err := os.ReadFile(absPath) // #nosec G304 -- absPath is validated
	if err != nil {
		return "", fmt.Errorf("failed to read source file: %w", err)
	}

	if err := os.WriteFile(targetPath, data, 0644); err != nil { // #nosec G306 -- preset files are not sensitive
		return "", fmt.Errorf("failed to write target file: %w", err)
	}

	return targetDir, nil
}

// copyDirContents copies the contents of src directory to dst directory
func copyDirContents(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			// Create subdirectory
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return fmt.Errorf("failed to create subdirectory: %w", err)
			}
			// Recursively copy contents
			if err := copyDirContents(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			// Copy file
			data, err := os.ReadFile(srcPath) // #nosec G304 -- srcPath is controlled
			if err != nil {
				return fmt.Errorf("failed to read file: %w", err)
			}
			if err := os.WriteFile(dstPath, data, 0644); err != nil { // #nosec G306 -- preset files are not sensitive
				return fmt.Errorf("failed to write file: %w", err)
			}
		}
	}

	return nil
}
