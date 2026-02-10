//go:build windows

package assert

import (
	"fmt"
	"os"
)

// checkFileOwner checks if a file matches the expected owner (Windows stub).
func checkFileOwner(info os.FileInfo, expectedOwner string) (string, string, error) {
	return "", "", fmt.Errorf("owner assertion not supported on Windows")
}

// checkFileGroup checks if a file matches the expected group (Windows stub).
func checkFileGroup(info os.FileInfo, expectedGroup string) (string, string, error) {
	return "", "", fmt.Errorf("group assertion not supported on Windows")
}
