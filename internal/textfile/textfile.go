// Package textfile holds shared file-IO helpers used by the text.*
// action handlers (text.line, text.patch.ini, …). The helpers are
// intentionally small and string-shaped (matching the in-place
// editing model those handlers use) — the byte-oriented helpers in
// text.patch.{yaml,json} have a different signature and stay local.
package textfile

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
)

// DefaultMode is the file mode used for newly created files when the
// existing file doesn't dictate one. Matches the per-handler
// defaultFileMode constants that used to live in each text package.
const DefaultMode os.FileMode = 0o644

// ReadOriginal returns the file's current content as a string,
// whether the file existed, and its mode. A non-existent file is not
// an error — Mode comes back as DefaultMode so callers can use it
// for writing the file fresh. `label` is the action name used in
// error prefixes ("text.line", "text.patch.ini") so callers don't
// have to re-wrap with their own prefix.
func ReadOriginal(path, label string) (content string, exists bool, mode os.FileMode, err error) {
	info, statErr := os.Stat(path)
	if errors.Is(statErr, fs.ErrNotExist) {
		return "", false, DefaultMode, nil
	}
	if statErr != nil {
		return "", false, 0, fmt.Errorf("%s: stat %s: %w", label, path, statErr)
	}
	if info.IsDir() {
		return "", false, 0, fmt.Errorf("%s: %s is a directory", label, path)
	}
	// #nosec G304 -- file path from user config.
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false, 0, fmt.Errorf("%s: read %s: %w", label, path, readErr)
	}
	return string(data), true, info.Mode().Perm(), nil
}
