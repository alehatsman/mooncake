package fleet

// peerstate.go persists "last successful contact" per peer between
// `fleet status` invocations, so the user can tell at a glance whether
// an unreachable peer was healthy an hour ago or never. The state lives
// under the user state dir (XDG_STATE_HOME on Unix / %LOCALAPPDATA% on
// Windows) — one tiny JSON per peer to keep concurrent updates from
// stepping on each other.
//
// Failure to read or write the state file is non-fatal: status probing
// has to keep working on a fresh box (no state yet) and on a read-only
// home directory. Errors propagate to the caller only when explicitly
// requested via the Err* helpers.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PeerState is the disk representation of what we know about a peer
// from past probes. The JSON shape is intentionally small and append-
// only — new fields can be added without invalidating existing files.
type PeerState struct {
	// LastSeenAt is the UTC time of the most recent successful
	// `/v1/version` response. Zero ↔ "never seen this peer succeed".
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`

	// LastAddr is the host:port that returned the most recent success.
	// Used to detect peers that moved (DHCP churn, port re-bind) so
	// the "21h ago at <old addr>" hint stays honest.
	LastAddr string `json:"last_addr,omitempty"`

	// LastMooncake is the agentd version string of the last success.
	// Surfaces in the unreachable footnote so an upgrade-broke-it
	// scenario is visible without grepping logs.
	LastMooncake string `json:"last_mooncake,omitempty"`
}

// PeerStateDir returns the directory holding per-peer state files.
// Exposed so tests and `fleet doctor` can poke at it. Returns an error
// only if the user's state root can't be resolved; missing directory
// itself is fine — callers handle ENOENT explicitly.
func PeerStateDir() (string, error) {
	root, err := userStateDir()
	if err != nil {
		return "", err
	}
	// On Unix-like OSes the state root is shared across apps
	// ($XDG_STATE_HOME); we nest under mooncake/. On Windows the root
	// is already mooncake-specific, but adding the extra segment is
	// harmless and keeps the path shape identical for tooling that
	// resolves it generically.
	return filepath.Join(root, "mooncake", "peers"), nil
}

// peerStatePath returns the absolute path of the state file for the
// named peer. Sanitises the name so a peer called "../etc" can't
// escape the directory; this is paranoid (peers.toml is user-edited)
// but cheap.
func peerStatePath(name string) (string, error) {
	dir, err := PeerStateDir()
	if err != nil {
		return "", err
	}
	clean := sanitisePeerName(name)
	if clean == "" {
		return "", fmt.Errorf("peer state: name %q reduces to empty after sanitisation", name)
	}
	return filepath.Join(dir, clean+".json"), nil
}

// sanitisePeerName replaces anything outside [A-Za-z0-9._-] with "_"
// so a hostile/odd peer name can't traverse out of PeerStateDir or
// trip filesystem quirks (colons on Windows, slashes everywhere).
func sanitisePeerName(name string) string {
	var b strings.Builder
	for _, r := range name {
		switch {
		case r >= 'A' && r <= 'Z',
			r >= 'a' && r <= 'z',
			r >= '0' && r <= '9',
			r == '.', r == '_', r == '-':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	out := strings.Trim(b.String(), ".")
	return out
}

// LoadPeerState reads the persisted state for `name`. A missing file
// returns a zero-value PeerState and nil — equivalent to "no prior
// contact". Other errors (corrupted JSON, permission denied) surface
// to the caller so they can decide whether to ignore or escalate; in
// `Probe` we ignore them so a poisoned state file doesn't break
// status.
func LoadPeerState(name string) (PeerState, error) {
	path, err := peerStatePath(name)
	if err != nil {
		return PeerState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return PeerState{}, nil
		}
		return PeerState{}, fmt.Errorf("read %s: %w", path, err)
	}
	var ps PeerState
	if err := json.Unmarshal(data, &ps); err != nil {
		return PeerState{}, fmt.Errorf("parse %s: %w", path, err)
	}
	return ps, nil
}

// SavePeerState atomically writes the state for `name`. Atomic via
// write-temp-then-rename, so a crashed controller can't leave a
// half-written JSON that breaks future reads.
func SavePeerState(name string, ps PeerState) error {
	path, err := peerStatePath(name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	data, err := json.MarshalIndent(ps, "", "  ")
	if err != nil {
		return fmt.Errorf("encode peer state: %w", err)
	}
	// O_TRUNC on the tmpfile so a leftover from a prior crash doesn't
	// poison the new write.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("rename %s -> %s: %w", tmp, path, err)
	}
	return nil
}
