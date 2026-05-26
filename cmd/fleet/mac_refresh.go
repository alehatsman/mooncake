package fleet

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetMACRefreshCommand defines `mooncake fleet mac-refresh`.
//
// Reads each selected peer's MAC via GET /v1/self/mac and writes it
// back to peers.toml. Used to backfill the new `mac` field on peers
// that were bootstrapped before this command existed, or to refresh
// after a NIC change. Side-effects peers.toml only on success — a
// peer that fails to respond leaves its existing entry untouched.
func fleetMACRefreshCommand() *cli.Command {
	return &cli.Command{
		Name:      "mac-refresh",
		Usage:     "Read each peer's MAC over agentd and store it in peers.toml",
		ArgsUsage: "[<peer>...]",
		Description: "Contacts each selected peer's /v1/self/mac endpoint and " +
			"upserts the returned MAC into peers.toml. Required as a one-time " +
			"backfill before `fleet up` can wake a peer over Wake-on-LAN; " +
			"`fleet shutdown` runs this implicitly right before powering off.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers (in addition to positional args): repeat for UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group.",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
		},
		Action: fleetMACRefreshAction,
	}
}

func fleetMACRefreshAction(c *cli.Context) error {
	peersPath, cfg, err := loadFleetPeers(c)
	if err != nil {
		return err
	}

	selected, err := selectMutatingPeers(c, cfg.Peers, "mac-refresh")
	if err != nil {
		return err
	}

	w := c.App.Writer
	type result struct {
		peer fleet.Peer
		mac  string
		err  error
	}
	results := make([]result, len(selected))
	var wg sync.WaitGroup
	for i, p := range selected {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			results[i].peer = p
			mac, err := fetchPeerMAC(c.Context, p, defaultMACTimeout)
			results[i].mac = mac
			results[i].err = err
		}(i, p)
	}
	wg.Wait()

	var firstErr error
	for _, r := range results {
		if r.err != nil {
			fmt.Fprintf(w, "%s: error: %s\n", r.peer.Name, oneLineErr(r.err.Error()))
			if firstErr == nil {
				firstErr = r.err
			}
			continue
		}
		updated, err := upsertPeerMAC(peersPath, r.peer.Name, r.mac)
		if err != nil {
			fmt.Fprintf(w, "%s: error writing peers.toml: %s\n", r.peer.Name, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		switch {
		case updated == "":
			fmt.Fprintf(w, "%s: mac already %s\n", r.peer.Name, r.mac)
		default:
			fmt.Fprintf(w, "%s: mac %s → %s\n", r.peer.Name, updated, r.mac)
		}
	}

	if firstErr != nil {
		return cli.Exit("fleet mac-refresh: one or more peers failed", 1)
	}
	return nil
}

// defaultMACTimeout caps a single /v1/self/mac probe. Matches the
// facts probe budget — the response is a tiny JSON, latency is purely
// network.
const defaultMACTimeout = 3 * time.Second

// fetchPeerMAC issues the agentd /v1/self/mac call against one peer.
// Refuses non-agentd transports — the SSH fallback isn't yet wired
// for this endpoint and the operator probably wants a clear refusal
// over a silent skip.
func fetchPeerMAC(ctx context.Context, p fleet.Peer, timeout time.Duration) (string, error) {
	if p.Transport != fleet.TransportAgentd {
		return "", fmt.Errorf("transport %q does not support /v1/self/mac", p.Transport)
	}
	cli := transport.New(p.Name, p.Addr, p.Token)
	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	resp, err := cli.GetMAC(pCtx)
	if err != nil {
		return "", err
	}
	return resp.MAC, nil
}

// upsertPeerMAC reads peers.toml, sets the MAC for the named peer,
// and writes back. Returns the previous MAC (or "" when unchanged or
// never set). Errors propagate from fleet.LoadPeers/SavePeers.
func upsertPeerMAC(peersPath, name, mac string) (previous string, err error) {
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return "", err
	}
	norm, err := fleet.NormalizeMAC(mac)
	if err != nil {
		return "", fmt.Errorf("normalize %q: %w", mac, err)
	}
	for i := range cfg.Peers {
		if cfg.Peers[i].Name != name {
			continue
		}
		if cfg.Peers[i].MAC == norm {
			return "", nil
		}
		previous = cfg.Peers[i].MAC
		cfg.Peers[i].MAC = norm
		return previous, fleet.SavePeers(peersPath, cfg)
	}
	return "", fmt.Errorf("peer %q not found in %s", name, peersPath)
}

// loadFleetPeers resolves the peers.toml path (from --peers-file or
// the default) and loads its contents. Refuses an empty file —
// fan-out commands with zero peers have no meaningful behavior.
func loadFleetPeers(c *cli.Context) (string, *fleet.Config, error) {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		var err error
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return "", nil, err
		}
	}
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return "", nil, err
	}
	if len(cfg.Peers) == 0 {
		return "", nil, cli.Exit(fleet.NoPeersConfiguredError(peersPath), 1)
	}
	return peersPath, cfg, nil
}

// selectMutatingPeers resolves --peer flags + positional args into a
// concrete list, and refuses an empty result. The "mutating" name is
// load-bearing: shutdown / up / mac-refresh all change real state on
// the peer or in peers.toml, so we refuse the "no selector → every
// peer" default that fleet apply uses. The operator must spell out
// which hosts.
func selectMutatingPeers(c *cli.Context, peers []fleet.Peer, action string) ([]fleet.Peer, error) {
	peerFlag := c.StringSlice("peer")
	// Positional args are bare peer names (no filter syntax).
	peerFlag = append(peerFlag, c.Args().Slice()...)
	if len(peerFlag) == 0 {
		return nil, cli.Exit(fmt.Sprintf(
			"fleet %s: at least one peer must be specified (positional or --peer)",
			action,
		), 2)
	}
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, peers, c.App.ErrWriter)
	}
	sel, err := resolvePeers(peers, peerFlag, osFor)
	if err != nil {
		return nil, cli.Exit("fleet "+action+": "+err.Error(), 2)
	}
	if len(sel.UnknownNames) > 0 {
		fmt.Fprintln(c.App.ErrWriter, "warning: unknown peer name(s): "+strings.Join(sel.UnknownNames, ", "))
	}
	if len(sel.Matched) == 0 {
		return nil, cli.Exit(fleet.NoPeersSelectedError(len(peers), sel.UnknownNames), 1)
	}
	return sel.Matched, nil
}
