package tool

import (
	"fmt"
	"os"
	"path/filepath"
)

// StoreRoot returns the mooncake tools install root, honoring
// $XDG_DATA_HOME with the canonical fallback. The path is created
// lazily by the install pipeline; this function does not touch the
// filesystem.
func StoreRoot() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "mooncake", "tools"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve $HOME for store root: %w", err)
	}
	return filepath.Join(home, ".local", "share", "mooncake", "tools"), nil
}

// InstallDir returns the install path for a (name, version) tuple,
// rooted at StoreRoot. URL-based backends own this layout; mise has its
// own dir and we don't relocate.
func InstallDir(name, version string) (string, error) {
	root, err := StoreRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, name, version), nil
}

// installDirIsPopulated returns true if dir exists and contains at least
// one entry. Used for cheap idempotency without re-verifying checksums.
func installDirIsPopulated(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return false, nil
		}
		return false, err
	}
	return len(entries) > 0, nil
}

// locateInInstallDir is the shared Locate body for URL-based backends.
// Returns ("", nil) if the install dir is empty (not installed), or the
// absolute bin path joining installDir + bin when populated. The empty
// installDir case (e.g. caller hasn't computed it yet) also reports
// "not installed".
//
// MT-60: when bin is unset, instead of returning the install dir
// (which is not executable), scan the dir for executable files at the
// top level. If exactly one is present, return it — this is the
// common github-release bare-binary case where authors forget to
// declare `bin:` and `asset:` is the single binary. If the install
// dir is ambiguous (0 or 2+ executables), fall back to returning the
// install dir so the existing "you need bin:" failure mode is
// preserved instead of guessing wrong.
func locateInInstallDir(bin, installDir string) (string, error) {
	if installDir == "" {
		return "", nil
	}
	populated, err := installDirIsPopulated(installDir)
	if err != nil {
		return "", err
	}
	if !populated {
		return "", nil
	}
	if bin == "" {
		if auto, ok, err := singleExecutableIn(installDir); err == nil && ok {
			return auto, nil
		}
		return installDir, nil
	}
	return filepath.Join(installDir, bin), nil
}

// singleExecutableIn returns the absolute path of the single
// executable file at the top of dir, ok=true, when there's exactly
// one. Otherwise ok=false (zero or two-plus candidates). Symlinks and
// subdirectories are ignored; user/group/other execute bits are
// honored.
func singleExecutableIn(dir string) (string, bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false, err
	}
	var matches []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		info, infoErr := e.Info()
		if infoErr != nil {
			continue
		}
		if info.Mode()&0o111 == 0 {
			continue
		}
		matches = append(matches, filepath.Join(dir, e.Name()))
		if len(matches) > 1 {
			return "", false, nil // ambiguous; fall back to dir
		}
	}
	if len(matches) == 1 {
		return matches[0], true, nil
	}
	return "", false, nil
}
