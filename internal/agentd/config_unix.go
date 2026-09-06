//go:build !windows

package agentd

import (
	"fmt"
	"os"
	"path/filepath"
)

// userSocketDir returns the per-user directory where the agentd unix
// socket should live. Honors XDG_RUNTIME_DIR; falls back to
// /tmp/mooncake-<uid> when the XDG var is unset (e.g. headless / cron).
func userSocketDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return fmt.Sprintf("/tmp/mooncake-%d", os.Getuid())
}

// userStateDir returns the per-user state directory (runs, synced, ...).
// Honors XDG_STATE_HOME, falls back to ~/.local/state.
func userStateDir() (string, error) {
	if d := os.Getenv("XDG_STATE_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".local", "state"), nil
}

// userConfigDir returns the per-user config directory (token file).
// Honors XDG_CONFIG_HOME, falls back to ~/.config.
func userConfigDir() (string, error) {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locate home dir: %w", err)
	}
	return filepath.Join(home, ".config"), nil
}

// systemModeDefaults returns the system-mode (root) defaults for this
// platform. Embedded into Default() when --system is set.
//
// The socket lives under systemRunDir, which differs per OS (/run on Linux
// and the BSDs, /var/run on macOS) — see rundir_*.go. StateDir and TokenPath
// need no such split: /var and /etc are on the writable data volume on macOS
// too, so MkdirAll creates them on demand there just as it does on Linux.
func systemModeDefaults() Config {
	return Config{
		SystemMode:   true,
		SocketPath:   filepath.Join(systemRunDir, "mooncake", "agentd.sock"),
		StateDir:     "/var/lib/mooncake/agentd",
		LogLevel:     "info",
		TokenPath:    "/etc/mooncake/agentd.token",
		MaxSyncBytes: DefaultMaxSyncBytes,
	}
}
