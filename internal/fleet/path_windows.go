//go:build windows

package fleet

import (
	"errors"
	"os"
	"path/filepath"
)

// userConfigDir returns the per-user config root on Windows:
// %LOCALAPPDATA% (with a fallback to %USERPROFILE%\AppData\Local). Not
// %APPDATA% — peers.toml + controller_id are host-specific, and roaming
// them across machines would cause silent collisions between fleets on
// AD-joined boxes. Matches the agentd convention.
//
// Note: this means Windows peers.toml lands at
// %LOCALAPPDATA%\Mooncake\peers.toml, not the legacy ~/.config path.
// Before this split the fleet package always returned ~/.config which
// produced a confusing C:\Users\<u>\.config\mooncake\peers.toml on
// Windows.
func userConfigDir() (string, error) {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d, nil
	}
	if up := os.Getenv("USERPROFILE"); up != "" {
		return filepath.Join(up, "AppData", "Local"), nil
	}
	return "", errors.New("locate %LOCALAPPDATA%: not set and %USERPROFILE% missing")
}
