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
