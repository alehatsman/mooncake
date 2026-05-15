// Package agentd implements the mooncake host daemon: a long-running process
// that exposes mooncake's kernel (facts, MCP tools, plan execution) over a
// local HTTP API. The daemon serves on a unix socket by default (gated by
// filesystem perms) and optionally on a TCP listener with bearer-token
// auth. On Windows where AF_UNIX is supported but uncommon, the daemon
// can also run TCP-only. See VISION.md §5 / §6.2.
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

	// SocketPath is the unix socket listener path. Empty disables the
	// unix listener (BindAddr is then required). On platforms that don't
	// support AF_UNIX in your environment, set SocketPath="" and use
	// BindAddr for TCP-only operation.
	SocketPath string

	StateDir string
	LogLevel string

	// BindAddr is the TCP listener address (e.g. "0.0.0.0:7878"). Empty
	// disables TCP. The TCP listener is bearer-auth-gated by Token; the
	// unix socket is not.
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

	// AdvertiseMDNS enables `_mooncake._tcp.local` Bonjour advertising
	// on the TCP bind address. Spec-45: lets `mooncake fleet discover`
	// find this daemon on the LAN without operator-maintained
	// peers.toml entries. Only meaningful when BindAddr != ""; the
	// advertise goroutine is a no-op on unix-socket-only daemons.
	AdvertiseMDNS bool

	// AdvertiseName overrides the mDNS instance name (TXT `hn=`). When
	// empty, defaults to os.Hostname() with `.local` stripped. Useful
	// on macOS where Bonjour aggressively renames instances on
	// collision ("desktop (2)").
	AdvertiseName string
}

// Default returns the platform-appropriate per-user (or per-system) config.
// Unix: XDG-aware paths. Windows: %LOCALAPPDATA%\Mooncake\. The bodies live
// in config_unix.go / config_windows.go behind build tags so this function
// is platform-neutral.
func Default(systemMode bool) (Config, error) {
	if systemMode {
		return systemModeDefaults(), nil
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

// SyncedRoot is the absolute path under which PUT /v1/files writes scope
// subtrees. Sibling of <state_dir>/runs/.
func (c Config) SyncedRoot() string {
	return filepath.Join(c.StateDir, "synced")
}

// Validate checks the config's internal consistency. The daemon needs at
// least one listener; permitted shapes are (a) unix-only, (b) TCP-only, or
// (c) both. TCP requires a token.
func (c Config) Validate() error {
	if c.SocketPath == "" && c.BindAddr == "" {
		return errors.New("at least one of socket_path or bind_addr must be set")
	}
	if c.StateDir == "" {
		return errors.New("state_dir is empty")
	}
	if c.SocketPath != "" && !filepath.IsAbs(c.SocketPath) {
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

// EnsureDirs makes the parent directories the daemon will write into. The
// socket dir is only created when SocketPath is set (TCP-only mode skips
// it entirely).
func (c Config) EnsureDirs() error {
	if c.SocketPath != "" {
		socketDir := filepath.Dir(c.SocketPath)
		if err := os.MkdirAll(socketDir, 0o700); err != nil {
			return fmt.Errorf("create socket dir %s: %w", socketDir, err)
		}
	}
	if err := os.MkdirAll(c.StateDir, 0o700); err != nil {
		return fmt.Errorf("create state dir %s: %w", c.StateDir, err)
	}
	if err := os.MkdirAll(c.SyncedRoot(), 0o700); err != nil {
		return fmt.Errorf("create synced dir %s: %w", c.SyncedRoot(), err)
	}
	return nil
}
