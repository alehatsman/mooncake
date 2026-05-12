// Package agentd implements the mooncake host daemon: a long-running process
// that exposes mooncake's kernel (facts, MCP tools, plan execution) over a
// local unix-socket HTTP API. See VISION.md §5 / §6.2.
package agentd

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	SystemMode bool
	SocketPath string
	StateDir   string
	LogLevel   string
}

func Default(systemMode bool) (Config, error) {
	if systemMode {
		return Config{
			SystemMode: true,
			SocketPath: "/run/mooncake/agentd.sock",
			StateDir:   "/var/lib/mooncake/agentd",
			LogLevel:   "info",
		}, nil
	}

	socketDir := userSocketDir()
	stateDir, err := userStateDir()
	if err != nil {
		return Config{}, err
	}
	return Config{
		SocketPath: filepath.Join(socketDir, "mooncake", "agentd.sock"),
		StateDir:   filepath.Join(stateDir, "mooncake", "agentd"),
		LogLevel:   "info",
	}, nil
}

func userSocketDir() string {
	if d := os.Getenv("XDG_RUNTIME_DIR"); d != "" {
		return d
	}
	return fmt.Sprintf("/tmp/mooncake-%d", os.Getuid())
}

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

func (c Config) Validate() error {
	if c.SocketPath == "" {
		return errors.New("socket_path is empty")
	}
	if c.StateDir == "" {
		return errors.New("state_dir is empty")
	}
	if !filepath.IsAbs(c.SocketPath) {
		return fmt.Errorf("socket_path must be absolute: %s", c.SocketPath)
	}
	if !filepath.IsAbs(c.StateDir) {
		return fmt.Errorf("state_dir must be absolute: %s", c.StateDir)
	}
	return nil
}

func (c Config) EnsureDirs() error {
	socketDir := filepath.Dir(c.SocketPath)
	if err := os.MkdirAll(socketDir, 0o700); err != nil {
		return fmt.Errorf("create socket dir %s: %w", socketDir, err)
	}
	if err := os.MkdirAll(c.StateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir %s: %w", c.StateDir, err)
	}
	return nil
}
