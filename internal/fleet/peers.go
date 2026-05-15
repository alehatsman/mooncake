// Package fleet implements the controller side of personal-fleet
// management: peer config, sync, multiplexed log streaming, and the
// `mooncake fleet …` subcommands.
package fleet

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

// Transport identifies how the controller reaches a peer. Only `agentd`
// (HTTP+SSE over the bind addr) is implemented in PR3; `ssh` is reserved
// for spec-44 / spec-47.
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
//
// Roles is the spec-50 addition: a list of semantic labels for what this
// peer is for ("db", "primary", "edge"). Distinct from Tags by convention
// — Tags is free-form, Roles answers "what's this machine's job?". Both
// are matched by `--peer-filter` (`tag=` and `role=` respectively).
// Optional; existing peers.toml entries with no roles field load fine.
type Peer struct {
	Name      string    `toml:"name"`
	Addr      string    `toml:"addr"`
	Transport Transport `toml:"transport"`
	Token     string    `toml:"token,omitempty"`
	Tags      []string  `toml:"tags,omitempty"`
	Roles     []string  `toml:"roles,omitempty"`

	// SSH is an optional fallback transport spec used by `fleet doctor`
	// (and future diagnostic flows). Format: `user@host:port` — the
	// same shape `mooncake fleet bootstrap` accepts. Empty means no
	// SSH fallback is configured; doctor's --ssh flag becomes a no-op.
	//
	// The SSH transport is NEVER used to apply runs — it's strictly a
	// diagnostic channel. Keeping the field optional means existing
	// peers.toml files load unchanged.
	SSH string `toml:"ssh,omitempty"`
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
		return nil, fmt.Errorf("parse %s: %w", path, annotateTOMLParseError(data, err))
	}
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate %s: %w", path, err)
	}
	return &cfg, nil
}

// annotateTOMLParseError catches the common dotted-key vs
// array-of-tables mistake — `[peers.local]` instead of `[[peers]]` —
// and decorates the underlying TOML parser error with the right form.
// The default error ("cannot store a table in a slice") is technically
// correct but tells the user nothing about the schema. (MT-78)
func annotateTOMLParseError(data []byte, err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	// pelletier/go-toml/v2 surfaces "cannot store a table in a slice"
	// for [peers.X] when Peers is `[]Peer`. The file string is the
	// other reliable signal — users who write [peers.X] are mapping
	// from a different config language and benefit from seeing the
	// canonical form spelled out.
	if !strings.Contains(msg, "cannot store a table in a slice") &&
		!bytes.Contains(data, []byte("[peers.")) {
		return err
	}
	return fmt.Errorf(
		"%w (hint: peers.toml uses TOML array-of-tables `[[peers]]` with `name = \"...\"` inside, not `[peers.<name>]` dotted-key form — see `mooncake fleet --help`)",
		err,
	)
}

// Upsert inserts or replaces a peer by Name in peers.toml at path. The file
// is created if missing. Returns:
//
//   - added=true when no entry with that Name existed before;
//   - diff=non-empty when an existing entry was replaced (one human-readable
//     line per changed field, suitable for printing as a confirmation).
//
// Atomic on success: the file is written via temp+rename. Validates the
// whole config before persisting — if the new peer conflicts with an
// existing one in a way that breaks Validate, nothing is written.
func Upsert(path string, p Peer) (added bool, diff []string, err error) {
	cfg, err := LoadPeers(path)
	if err != nil {
		// LoadPeers returns nil error for "file missing" so this is a real
		// load failure.
		return false, nil, err
	}
	if err := p.Validate(); err != nil {
		return false, nil, err
	}

	idx := -1
	for i, existing := range cfg.Peers {
		if existing.Name == p.Name {
			idx = i
			break
		}
	}
	if idx == -1 {
		cfg.Peers = append(cfg.Peers, p)
		added = true
	} else {
		diff = peerDiff(cfg.Peers[idx], p)
		cfg.Peers[idx] = p
	}
	if err := SavePeers(path, cfg); err != nil {
		return added, diff, err
	}
	return added, diff, nil
}

func peerDiff(old, newer Peer) []string {
	var out []string
	if old.Addr != newer.Addr {
		out = append(out, fmt.Sprintf("addr: %s → %s", old.Addr, newer.Addr))
	}
	if old.Transport != newer.Transport {
		out = append(out, fmt.Sprintf("transport: %s → %s", old.Transport, newer.Transport))
	}
	if old.Token != newer.Token {
		out = append(out, "token: (rotated)")
	}
	if !stringSlicesEqual(old.Tags, newer.Tags) {
		out = append(out, fmt.Sprintf("tags: %v → %v", old.Tags, newer.Tags))
	}
	return out
}

func stringSlicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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

// userConfigDir is defined per-platform in path_unix.go and
// path_windows.go.
