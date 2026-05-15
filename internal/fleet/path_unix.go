//go:build !windows

package fleet

import (
	"fmt"
	"os"
	"path/filepath"
)

// userConfigDir returns the per-user config root where peers.toml and
// controller_id live. Honors XDG_CONFIG_HOME; falls back to ~/.config.
//
// Mirrors agentd's per-platform helper. Kept independent (rather than
// imported) so the fleet package doesn't take a dependency on agentd
// just for this lookup.
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

// userStateDir returns the per-user state root where the controller
// keeps mutable, machine-local artefacts (last-seen tracking, future
// run caches, etc.). Honors XDG_STATE_HOME; falls back to
// ~/.local/state. Mirrors agentd's helper of the same name.
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
