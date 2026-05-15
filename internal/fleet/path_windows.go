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

// userStateDir returns the per-user state root on Windows. Windows
// doesn't have an XDG equivalent for state-vs-config; both live under
// %LOCALAPPDATA%. We tuck state under a `Mooncake\State\` subdirectory
// so it's easy to wipe without touching peers.toml.
func userStateDir() (string, error) {
	cfg, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(cfg, "Mooncake", "State"), nil
}
