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
func systemModeDefaults() Config {
	return Config{
		SystemMode:   true,
		SocketPath:   "/run/mooncake/agentd.sock",
		StateDir:     "/var/lib/mooncake/agentd",
		LogLevel:     "info",
		TokenPath:    "/etc/mooncake/agentd.token",
		MaxSyncBytes: DefaultMaxSyncBytes,
	}
}
