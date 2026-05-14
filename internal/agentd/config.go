// Package agentd implements the mooncake host daemon: a long-running process
// that exposes mooncake's kernel (facts, MCP tools, plan execution) over a
// local unix-socket HTTP API. See VISION.md §5 / §6.2.
package agentd

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
)

// DefaultMaxSyncBytes is the default per-file size cap for PUT /v1/files.
const DefaultMaxSyncBytes int64 = 100 << 20 // 100 MiB

type Config struct {
	SystemMode bool
	SocketPath string
	StateDir   string
	LogLevel   string

	// BindAddr is the TCP listener address (e.g. "0.0.0.0:7878"). Empty
	// disables TCP — the daemon serves only over the unix socket. The TCP
	// listener is bearer-auth-gated by Token; the unix socket is not.
	BindAddr string

	// TokenPath is where the bearer token file lives. Read by
	// LoadOrCreateToken at startup. Used only when BindAddr != "".
	TokenPath string

	// Token is the actual bearer token used by the auth middleware. Loaded
	// from TokenPath by the caller (cmd/agentd.go) before calling New.
	// Empty token + non-empty BindAddr is a configuration error.
	Token string

	// MaxSyncBytes caps the body of a single PUT /v1/files request.
	// Defaults to DefaultMaxSyncBytes when zero.
	MaxSyncBytes int64
}

func Default(systemMode bool) (Config, error) {
	if systemMode {
		return Config{
			SystemMode:   true,
			SocketPath:   "/run/mooncake/agentd.sock",
			StateDir:     "/var/lib/mooncake/agentd",
			LogLevel:     "info",
			TokenPath:    "/etc/mooncake/agentd.token",
			MaxSyncBytes: DefaultMaxSyncBytes,
		}, nil
	}

	socketDir := userSocketDir()
	stateDir, err := userStateDir()
	if err != nil {
		return Config{}, err
	}
	configDir, err := userConfigDir()
	if err != nil {
		return Config{}, err
	}
	return Config{
		SocketPath:   filepath.Join(socketDir, "mooncake", "agentd.sock"),
		StateDir:     filepath.Join(stateDir, "mooncake", "agentd"),
		LogLevel:     "info",
		TokenPath:    filepath.Join(configDir, "mooncake", "agentd.token"),
		MaxSyncBytes: DefaultMaxSyncBytes,
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

// SyncedRoot is the absolute path under which PUT /v1/files writes scope
// subtrees. Sibling of <state_dir>/runs/.
func (c Config) SyncedRoot() string {
	return filepath.Join(c.StateDir, "synced")
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
	if c.BindAddr != "" {
		if _, _, err := net.SplitHostPort(c.BindAddr); err != nil {
			return fmt.Errorf("bind_addr %q invalid: %w", c.BindAddr, err)
		}
		if c.Token == "" {
			return errors.New("bind_addr is set but token is empty")
		}
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
	if err := os.MkdirAll(c.SyncedRoot(), 0o700); err != nil {
		return fmt.Errorf("create synced dir %s: %w", c.SyncedRoot(), err)
	}
	return nil
}
