//go:build windows

package security

import (
	"fmt"
	"os"
)

func checkFileOwnership(_ os.FileInfo) error {
	return fmt.Errorf("file ownership verification not supported on Windows")
}
