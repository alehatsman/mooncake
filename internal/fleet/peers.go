// Package fleet implements the controller side of personal-fleet
// management: peer config, sync, multiplexed log streaming, and the
// `mooncake fleet …` subcommands.
package fleet

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
)

// Transport identifies how the controller reaches a peer. Only `agentd`
// (HTTP+SSE over the bind addr) is implemented in PR3; `ssh` is reserved
// for spec-40 / spec-43.
type Transport string

const (
	TransportAgentd Transport = "agentd"
	TransportSSH    Transport = "ssh"
)

// Peer is one entry under `[[peers]]` in peers.toml.
//
// Validation rules:
//   - Name: 1..64 chars, only `[a-zA-Z0-9._-]`. Used as the `[host]` prefix
//     in multiplexed logs and as a path segment in sync scope keys.
//   - Addr: `host:port` parseable by net.SplitHostPort.
//   - Transport: "agentd" or "ssh"; defaults to "agentd" when empty.
//   - Token: required when Transport == "agentd".
type Peer struct {
	Name      string    `toml:"name"`
	Addr      string    `toml:"addr"`
	Transport Transport `toml:"transport"`
	Token     string    `toml:"token,omitempty"`
	Tags      []string  `toml:"tags,omitempty"`
}

// Config is the top-level shape of peers.toml.
type Config struct {
	Peers []Peer `toml:"peers"`
}

// nameRune reports whether c is allowed in a peer name.
func nameRune(c byte) bool {
	switch {
	case c >= 'a' && c <= 'z',
		c >= 'A' && c <= 'Z',
		c >= '0' && c <= '9',
		c == '.', c == '_', c == '-':
		return true
	}
	return false
}

// Validate returns the first error in p, or nil. Defaults the Transport to
// agentd when empty.
func (p *Peer) Validate() error {
	if p.Name == "" {
		return errors.New("peer: name is empty")
	}
	if len(p.Name) > 64 {
		return fmt.Errorf("peer %q: name exceeds 64 chars", p.Name)
	}
	for i := 0; i < len(p.Name); i++ {
		if !nameRune(p.Name[i]) {
			return fmt.Errorf("peer %q: name contains invalid char %q", p.Name, p.Name[i])
		}
	}
	if p.Transport == "" {
		p.Transport = TransportAgentd
	}
	switch p.Transport {
	case TransportAgentd:
		if p.Addr == "" {
			return fmt.Errorf("peer %q: addr is required for agentd transport", p.Name)
		}
		if _, _, err := net.SplitHostPort(p.Addr); err != nil {
			return fmt.Errorf("peer %q: addr %q invalid: %w", p.Name, p.Addr, err)
		}
		if p.Token == "" {
			return fmt.Errorf("peer %q: token is required for agentd transport", p.Name)
		}
	case TransportSSH:
		if p.Addr == "" {
			return fmt.Errorf("peer %q: addr is required for ssh transport", p.Name)
		}
	default:
		return fmt.Errorf("peer %q: unknown transport %q", p.Name, p.Transport)
	}
	return nil
}

// Validate returns the first error across all peers, or nil. Duplicate names
// are rejected; downstream code uses Name as a unique key.
func (c *Config) Validate() error {
	seen := make(map[string]struct{}, len(c.Peers))
	for i := range c.Peers {
		p := &c.Peers[i]
		if err := p.Validate(); err != nil {
			return err
		}
		if _, dup := seen[p.Name]; dup {
			return fmt.Errorf("duplicate peer name %q", p.Name)
		}
		seen[p.Name] = struct{}{}
	}
	return nil
}

// DefaultPeersPath is `$XDG_CONFIG_HOME/mooncake/peers.toml`, defaulting to
// `~/.config/mooncake/peers.toml` when XDG_CONFIG_HOME is unset.
func DefaultPeersPath() (string, error) {
	dir, err := userConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mooncake", "peers.toml"), nil
}

// LoadPeers reads and parses peers.toml at the given path. A missing file
// returns an empty Config (not an error) — a fresh user is the default
// state.
func LoadPeers(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &cfg, nil
}

// SavePeers atomically writes cfg to path. The parent dir is created with
// mode 0700; the file is written 0600. Validates before writing — there is
// no way to persist an invalid Config.
func SavePeers(path string, cfg *Config) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	data, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create peers dir: %w", err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write tmp: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename: %w", err)
	}
	return nil
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
