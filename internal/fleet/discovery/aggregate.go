package discovery

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// Options tunes Aggregate. All fields are optional; the zero value yields
// the default behavior (load peers.toml + ~/.ssh/config from their
// standard locations, probe agentd peers in parallel with a short
// timeout).
type Options struct {
	// PeersPath overrides the peers.toml location. Empty → fleet.DefaultPeersPath.
	PeersPath string

	// SSHConfigPath overrides the ssh_config location. Empty →
	// DefaultSSHConfigPath() (~/.ssh/config). Set to "-" to skip the
	// SSH source entirely.
	SSHConfigPath string

	// Probe controls whether agentd peers get a /v1/version round-trip.
	// Pointer so the zero value (nil) is distinguishable from explicit
	// false; nil means "probe by default."
	Probe *bool

	// ProbeTimeout per-peer; default 2 seconds. Aggregate runs probes
	// in parallel so the wall-clock cost is one timeout, not N.
	ProbeTimeout time.Duration
}

// Aggregate loads candidates from peers.toml and ~/.ssh/config, dedupes
// by Name, and optionally probes each agentd peer for /v1/version. The
// returned slice is sorted by Name for stable output.
//
// Two sources of dedup overlap are common: a host configured in
// peers.toml AND also listed in ~/.ssh/config (because the operator
// SSHes in for ad-hoc work). The peers.toml entry is authoritative;
// merged Sources list both so the CLI can surface "also bootstrappable
// via SSH" if needed.
func Aggregate(ctx context.Context, opts Options) ([]Candidate, error) {
	peersPath := opts.PeersPath
	if peersPath == "" {
		p, err := fleet.DefaultPeersPath()
		if err != nil {
			return nil, fmt.Errorf("resolve peers.toml path: %w", err)
		}
		peersPath = p
	}

	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return nil, fmt.Errorf("load peers.toml: %w", err)
	}

	peerCands := candidatesFromPeers(cfg.Peers)

	var sshCands []Candidate
	if opts.SSHConfigPath != "-" {
		sshPath := opts.SSHConfigPath
		if sshPath == "" {
			sshPath = DefaultSSHConfigPath()
		}
		if sshPath != "" {
			sshCands, err = ParseSSHConfig(sshPath)
			if err != nil {
				return nil, err
			}
		}
	}

	merged := merge(peerCands, sshCands)

	probe := true
	if opts.Probe != nil {
		probe = *opts.Probe
	}
	if probe {
		probeTimeout := opts.ProbeTimeout
		if probeTimeout <= 0 {
			probeTimeout = 2 * time.Second
		}
		probeAgentd(ctx, merged, cfg.Peers, probeTimeout)
	}

	sort.Slice(merged, func(i, j int) bool { return merged[i].Name < merged[j].Name })
	return merged, nil
}

// candidatesFromPeers maps peers.toml entries into Candidate form.
// Tokens are intentionally not copied into the Candidate — the probe path
// looks them up by name when it needs to authenticate.
func candidatesFromPeers(peers []fleet.Peer) []Candidate {
	out := make([]Candidate, 0, len(peers))
	for _, p := range peers {
		out = append(out, Candidate{
			Name:    p.Name,
			Addr:    p.Addr,
			Sources: []string{SourcePeersTOML},
			Tags:    append([]string(nil), p.Tags...),
		})
	}
	return out
}

// merge joins peer-sourced and ssh-sourced candidate lists. Peer entries
// win the slot; an SSH entry whose Name matches an existing peer merges
// its Sources/SSHUser/SSHPort into the peer entry. Unmatched SSH entries
// are appended at the end.
func merge(peerCands, sshCands []Candidate) []Candidate {
	byName := make(map[string]int, len(peerCands)+len(sshCands))
	out := make([]Candidate, 0, len(peerCands)+len(sshCands))

	for _, c := range peerCands {
		out = append(out, c)
		byName[c.Name] = len(out) - 1
	}

	for _, s := range sshCands {
		if idx, ok := byName[s.Name]; ok {
			out[idx].Sources = append(out[idx].Sources, SourceSSHConfig)
			if out[idx].SSHUser == "" {
				out[idx].SSHUser = s.SSHUser
			}
			if out[idx].SSHPort == 0 {
				out[idx].SSHPort = s.SSHPort
			}
			continue
		}
		out = append(out, s)
		byName[s.Name] = len(out) - 1
	}
	return out
}

// probeAgentd updates each peers.toml-sourced candidate in cands with the
// result of a /v1/version probe. Mutates in place. Probes run in
// parallel; per-probe deadline is timeout, overall wall-clock is also
// bounded by timeout.
func probeAgentd(ctx context.Context, cands []Candidate, peers []fleet.Peer, timeout time.Duration) {
	tokenByName := make(map[string]string, len(peers))
	addrByName := make(map[string]string, len(peers))
	for _, p := range peers {
		tokenByName[p.Name] = p.Token
		addrByName[p.Name] = p.Addr
	}

	var wg sync.WaitGroup
	for i := range cands {
		if !cands[i].HasSource(SourcePeersTOML) {
			continue
		}
		token, ok := tokenByName[cands[i].Name]
		if !ok || token == "" {
			// Configured but no token — can't auth a probe. Surface as a
			// probe failure rather than silently skipping.
			cands[i].ProbeError = "no bearer token in peers.toml"
			continue
		}
		wg.Add(1)
		go func(i int, addr, token string) {
			defer wg.Done()
			probeCtx, cancel := context.WithTimeout(ctx, timeout)
			defer cancel()
			cli := transport.New(cands[i].Name, addr, token)
			v, err := cli.GetVersion(probeCtx)
			if err != nil {
				cands[i].ProbeError = shortErr(err)
				return
			}
			cands[i].AgentdOK = true
			cands[i].AgentdVersion = v.Version
		}(i, addrByName[cands[i].Name], token)
	}
	wg.Wait()
}

// shortErr collapses a verbose transport error into a one-line reason
// suitable for the discover table. Falls back to err.Error() when it's
// already short.
func shortErr(err error) string {
	msg := err.Error()
	if len(msg) > 80 {
		msg = msg[:77] + "..."
	}
	return msg
}
