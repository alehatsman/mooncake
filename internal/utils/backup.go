package utils

import (
	"fmt"
	"io"
	"os"
	"time"
)

// CreateBackup creates a timestamped backup of a file.
// Returns the backup path and any error encountered.
func CreateBackup(path string) (backupPath string, err error) {
	// Check if source file exists
	if _, statErr := os.Stat(path); os.IsNotExist(statErr) {
		return "", fmt.Errorf("source file does not exist: %s", path)
	}

	// Generate backup path with timestamp
	timestamp := time.Now().Format("20060102-150405")
	backupPath = fmt.Sprintf("%s.%s.bak", path, timestamp)

	// Copy file to backup location
	// #nosec G304 -- File path from user config is intentional functionality
	src, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("failed to open source file: %w", err)
	}
	defer func() {
		if closeErr := src.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close source file: %w", closeErr)
		}
	}()

	// Get source file info for permissions
	srcInfo, err := src.Stat()
	if err != nil {
		return "", fmt.Errorf("failed to stat source file: %w", err)
	}

	// Create backup file with same permissions
	// #nosec G304 -- Backup path is derived from user config path, intentional functionality
	dst, err := os.OpenFile(backupPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return "", fmt.Errorf("failed to create backup file: %w", err)
	}

	// Copy contents
	if _, copyErr := io.Copy(dst, src); copyErr != nil {
		_ = dst.Close() // Ignore close error in error path
		_ = os.Remove(backupPath) // Clean up partial backup
		return "", fmt.Errorf("failed to copy file contents: %w", copyErr)
	}

	// Close destination and check for errors (important for writes)
	if closeErr := dst.Close(); closeErr != nil {
		return "", fmt.Errorf("failed to close backup file: %w", closeErr)
	}

	return backupPath, nil
}
