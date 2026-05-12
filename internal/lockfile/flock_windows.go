//go:build windows

package lockfile

import "os"

// Windows: no flock available in the standard library. Trust the atomic
// rename and accept that two concurrent applies on the same lockfile may
// race. This is a v1 limitation; Spec 19 is Linux/macOS-only by default
// (see action Metadata.SupportedPlatforms).

func lockExclusive(_ *os.File) error { return nil }
func unlock(_ *os.File) error        { return nil }
