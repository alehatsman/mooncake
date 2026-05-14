// Package discovery aggregates peer candidates from multiple sources for
// `mooncake fleet discover`. v1 covers two sources:
//
//   - peers.toml (already-configured peers)
//   - ~/.ssh/config (named hosts the operator already trusts over SSH)
//
// mDNS discovery is a planned follow-up (spec-45 PR12 full scope); the
// shape of Candidate is forward-compatible with it.
package discovery

// Source labels for Candidate.Source. Stable strings, used both in text
// output and in --json mode, so external consumers can switch on them.
const (
	SourcePeersTOML = "peers.toml"
	SourceSSHConfig = "ssh-config"
	SourceMDNS      = "mdns" // reserved for future mDNS discovery
)

// Candidate is one discovered fleet peer or potential peer. The same
// machine may appear from multiple sources (e.g. configured in peers.toml
// AND listed in ~/.ssh/config); Aggregate merges them into a single entry
// with merged provenance.
type Candidate struct {
	// Name is the operator-visible identifier. From peers.toml this is the
	// [[peers]] name; from ssh-config it's the Host alias.
	Name string `json:"name"`

	// Addr is host:port for agentd-reachable peers, or host[:port] for
	// SSH-only candidates. Empty if the source didn't supply one.
	Addr string `json:"addr"`

	// Sources lists every source that surfaced this candidate, in
	// stable order: peers.toml first (most trusted), ssh-config next,
	// mdns last. Two-element slices indicate dedup matches.
	Sources []string `json:"sources"`

	// AgentdOK is true when an agentd /v1/version probe succeeded for
	// this candidate. Only meaningful when the candidate has agentd
	// transport (i.e. came from peers.toml). False/zero for ssh-only
	// entries.
	AgentdOK bool `json:"agentd_ok"`

	// AgentdVersion is the agentd version string from /v1/version when
	// the probe succeeded. Empty otherwise.
	AgentdVersion string `json:"agentd_version,omitempty"`

	// ProbeError, when non-empty, is the (short) reason an agentd probe
	// failed. Only set when Sources contains peers.toml and the probe
	// was attempted. Excluded from --json when empty so SSH-only
	// candidates don't carry a spurious "" field.
	ProbeError string `json:"probe_error,omitempty"`

	// SSHUser and SSHPort come from ~/.ssh/config (User / Port
	// directives) when this candidate has an ssh-config source. Empty
	// when the operator omitted them (User defaults to current; Port
	// defaults to 22).
	SSHUser string `json:"ssh_user,omitempty"`
	SSHPort int    `json:"ssh_port,omitempty"`

	// Tags are the peers.toml tags for already-configured peers. Empty
	// for ssh-config-only candidates (the convention has no tag concept
	// over there).
	Tags []string `json:"tags,omitempty"`
}

// HasSource reports whether s is one of c.Sources. Convenience for the CLI
// renderer and dedup logic.
func (c *Candidate) HasSource(s string) bool {
	for _, x := range c.Sources {
		if x == s {
			return true
		}
	}
	return false
}
