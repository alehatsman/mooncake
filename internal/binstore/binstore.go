// Package binstore addresses the local mooncake binary cache at
// ~/.mooncake/bin. The store holds one binary per target platform
// (mooncake_<goos>_<goarch>, .exe for windows) so `fleet bootstrap` can
// ship the right artefact to any peer without a published release and
// without defaulting to the controller's own (possibly wrong-platform)
// executable. `mooncake selfbuild` populates it.
package binstore

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/statedir"
)

// Dir returns the binary store directory: ~/.mooncake/bin. $MOONCAKE_HOME
// overrides the ~/.mooncake base (its bin/ subdir) so tests and
// non-standard layouts can redirect it.
func Dir() (string, error) {
	return statedir.Path("bin")
}

// BinaryName returns the store filename for a (goos, goarch) build:
// mooncake_<goos>_<goarch>, with .exe appended for windows.
func BinaryName(goos, goarch string) string {
	name := fmt.Sprintf("mooncake_%s_%s", goos, goarch)
	if goos == "windows" {
		name += ".exe"
	}
	return name
}

// Path returns the absolute store path for a (goos, goarch) build (whether
// or not the file exists).
func Path(goos, goarch string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, BinaryName(goos, goarch)), nil
}

// Lookup returns the store path for (goos, goarch) and whether a regular
// file exists there.
func Lookup(goos, goarch string) (path string, found bool, err error) {
	p, err := Path(goos, goarch)
	if err != nil {
		return "", false, err
	}
	fi, err := os.Stat(p)
	if err != nil {
		if os.IsNotExist(err) {
			return p, false, nil
		}
		return p, false, err
	}
	return p, fi.Mode().IsRegular(), nil
}
