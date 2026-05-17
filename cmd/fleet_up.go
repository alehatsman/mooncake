package main

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
	"github.com/alehatsman/mooncake/internal/fleet/wol"
)

// fleetUpCommand defines `mooncake fleet up`.
//
// Sends a Wake-on-LAN magic packet to each selected peer's MAC (read
// from peers.toml). By default, waits for each peer's agentd to start
// responding on /v1/version before returning — pass --no-wait for
// fire-and-forget. Peers without a stored MAC fail with a hint to
// run `fleet mac-refresh` first.
func fleetUpCommand() *cli.Command {
	return &cli.Command{
		Name:      "up",
		Usage:     "Wake one or more peers via Wake-on-LAN",
		ArgsUsage: "<peer>...",
		Description: "Sends a WoL magic packet to each peer's stored MAC " +
			"(see `fleet mac-refresh`). The controller must be on the same " +
			"broadcast domain as the target NIC — magic packets are link- " +
			"layer features and do not cross routers without explicit " +
			"directed-broadcast configuration. After sending, polls /v1/version " +
			"until the peer responds (use --no-wait to skip).",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers (in addition to positional args): repeat for UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group.",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.StringFlag{
				Name: "broadcast",
				Usage: "UDP broadcast address for the magic packet (default " +
					wol.DefaultBroadcast + "). Use a directed broadcast like " +
					"10.0.0.255:9 for routed LANs.",
			},
			&cli.BoolFlag{
				Name:  "no-wait",
				Usage: "Return immediately after sending the magic packet (skip the /v1/version reachability check)",
			},
			&cli.DurationFlag{
				Name:  "wait-timeout",
				Usage: "Maximum time to wait for each peer to come back online",
				Value: 2 * time.Minute,
			},
		},
		Action: fleetUpAction,
	}
}

func fleetUpAction(c *cli.Context) error {
	_, cfg, err := loadFleetPeers(c)
	if err != nil {
		return err
	}

	selected, err := selectMutatingPeers(c, cfg.Peers, "up")
	if err != nil {
		return err
	}

	// Up-front check: every selected peer must have a MAC. We refuse
	// the whole operation rather than partial-success, because the
	// operator's mental model is "wake N machines" and silently
	// dropping ones without a MAC is the kind of bug that surfaces
	// at 2am.
	var missing []string
	for _, p := range selected {
		if p.MAC == "" {
			missing = append(missing, p.Name)
		}
	}
	if len(missing) > 0 {
		return cli.Exit(fmt.Sprintf(
			"fleet up: peer(s) have no stored MAC: %v\n"+
				"  Run `mooncake fleet mac-refresh %s` while those peers are online,\n"+
				"  or `mooncake fleet shutdown` will auto-collect on next power-off.",
			missing, joinNames(missing),
		), 1)
	}

	w := c.App.Writer
	broadcast := c.String("broadcast")
	noWait := c.Bool("no-wait")
	waitTimeout := c.Duration("wait-timeout")

	type result struct {
		peer    fleet.Peer
		sendErr error
		// woke is true when the post-send /v1/version probe succeeded
		// before waitTimeout. Only meaningful when noWait is false.
		woke bool
		// wakeLatency is the time from packet-send to first
		// successful /v1/version. Zero if we didn't wait or the peer
		// never came back.
		wakeLatency time.Duration
	}
	results := make([]result, len(selected))
	var wg sync.WaitGroup
	for i, p := range selected {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			results[i].peer = p
			mac, err := net.ParseMAC(p.MAC)
			if err != nil {
				results[i].sendErr = fmt.Errorf("parse stored mac %q: %w", p.MAC, err)
				return
			}
			start := time.Now()
			if err := wol.Send(mac, broadcast); err != nil {
				results[i].sendErr = err
				return
			}
			if noWait {
				return
			}
			ok := waitForPeerVersion(c.Context, p, waitTimeout)
			results[i].woke = ok
			if ok {
				results[i].wakeLatency = time.Since(start).Truncate(time.Second)
			}
		}(i, p)
	}
	wg.Wait()

	var firstErr error
	for _, r := range results {
		switch {
		case r.sendErr != nil:
			fmt.Fprintf(w, "%s: send failed: %s\n", r.peer.Name, oneLineErr(r.sendErr.Error()))
			if firstErr == nil {
				firstErr = r.sendErr
			}
		case noWait:
			fmt.Fprintf(w, "%s: magic packet sent\n", r.peer.Name)
		case r.woke:
			fmt.Fprintf(w, "%s: woke in %s\n", r.peer.Name, r.wakeLatency)
		default:
			fmt.Fprintf(w, "%s: magic packet sent; did not respond within %s\n",
				r.peer.Name, waitTimeout)
			if firstErr == nil {
				firstErr = fmt.Errorf("peer %s did not respond within %s", r.peer.Name, waitTimeout)
			}
		}
	}
	if firstErr != nil {
		return cli.Exit("fleet up: one or more peers failed", 1)
	}
	return nil
}

// waitForPeerVersion polls /v1/version until the peer responds or
// timeout elapses. Returns true on first success. Uses a short
// per-probe timeout so unreachable peers don't burn the whole budget
// on a single TCP connect.
func waitForPeerVersion(ctx context.Context, p fleet.Peer, timeout time.Duration) bool {
	if p.Transport != fleet.TransportAgentd {
		return false
	}
	deadline := time.Now().Add(timeout)
	const probeInterval = 2 * time.Second
	const perProbeTimeout = 3 * time.Second
	cli := transport.New(p.Name, p.Addr, p.Token)
	for {
		probeCtx, cancel := context.WithTimeout(ctx, perProbeTimeout)
		_, err := cli.GetVersion(probeCtx)
		cancel()
		if err == nil {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(probeInterval):
		}
	}
}

// joinNames formats a list of peer names for the mac-refresh hint
// (space-separated, ready to paste as positional args).
func joinNames(names []string) string {
	out := ""
	for i, n := range names {
		if i > 0 {
			out += " "
		}
		out += n
	}
	return out
}
