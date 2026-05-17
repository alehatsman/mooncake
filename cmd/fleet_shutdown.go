package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetShutdownCommand defines `mooncake fleet shutdown`.
//
// Powers off one or more peers via agentd. Before each shutdown call,
// the command opportunistically fetches the peer's MAC and writes it
// to peers.toml so a follow-up `fleet up` has the address it needs
// for Wake-on-LAN. The MAC fetch failure is non-fatal — the shutdown
// still goes ahead if the operator's intent was just "power off".
func fleetShutdownCommand() *cli.Command {
	return &cli.Command{
		Name:      "shutdown",
		Usage:     "Power off one or more fleet peers via agentd",
		ArgsUsage: "<peer>...",
		Description: "Sends POST /v1/self/shutdown to each selected peer. The " +
			"daemon refuses with 409 runs_in_flight when an active run is in " +
			"progress; pass --force to override. Right before each shutdown, " +
			"the peer's MAC is read and stored in peers.toml so `fleet up` " +
			"can wake it later (skip with --no-mac-collect).",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers (in addition to positional args): repeat for UNION. Each value is a name, `key=value` filter, or `@k=v,k2=v2` AND-group.",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.BoolFlag{
				Name:  "force",
				Usage: "Tell the daemon to shut down even if a run is in flight",
			},
			&cli.BoolFlag{
				Name:  "no-mac-collect",
				Usage: "Skip the pre-shutdown MAC capture (peers.toml is left unchanged)",
			},
		},
		Action: fleetShutdownAction,
	}
}

func fleetShutdownAction(c *cli.Context) error {
	peersPath, cfg, err := loadFleetPeers(c)
	if err != nil {
		return err
	}

	selected, err := selectMutatingPeers(c, cfg.Peers, "shutdown")
	if err != nil {
		return err
	}

	w := c.App.Writer
	force := c.Bool("force")
	collectMAC := !c.Bool("no-mac-collect")

	type result struct {
		peer fleet.Peer
		err  error
		mac  string
		// macErr is non-nil when the pre-shutdown MAC capture failed.
		// We still proceed to the shutdown — losing WoL is a future-
		// inconvenience, not a reason to leave the host running.
		macErr error
	}
	results := make([]result, len(selected))
	var wg sync.WaitGroup
	for i, p := range selected {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			results[i].peer = p
			if collectMAC && p.MAC == "" {
				mac, err := fetchPeerMAC(c.Context, p, defaultMACTimeout)
				if err != nil {
					results[i].macErr = err
				} else {
					results[i].mac = mac
				}
			}
			results[i].err = doPeerShutdown(c.Context, p, force, defaultShutdownTimeout)
		}(i, p)
	}
	wg.Wait()

	// Persist any captured MACs serially — peers.toml is a single
	// file and SavePeers rewrites it atomically.
	for _, r := range results {
		if r.mac == "" {
			continue
		}
		if _, err := upsertPeerMAC(peersPath, r.peer.Name, r.mac); err != nil {
			fmt.Fprintf(w, "%s: warning: could not store mac %s: %s\n", r.peer.Name, r.mac, err)
		}
	}

	var firstErr error
	for _, r := range results {
		switch {
		case r.err != nil:
			fmt.Fprintf(w, "%s: shutdown failed: %s\n", r.peer.Name, oneLineErr(r.err.Error()))
			if firstErr == nil {
				firstErr = r.err
			}
		case r.macErr != nil:
			fmt.Fprintf(w, "%s: shutdown scheduled; warning: could not capture MAC (%s)\n",
				r.peer.Name, oneLineErr(r.macErr.Error()))
		case r.mac != "":
			fmt.Fprintf(w, "%s: shutdown scheduled; stored mac %s\n", r.peer.Name, r.mac)
		default:
			fmt.Fprintf(w, "%s: shutdown scheduled\n", r.peer.Name)
		}
	}
	if firstErr != nil {
		return cli.Exit("fleet shutdown: one or more peers failed", 1)
	}
	return nil
}

const defaultShutdownTimeout = 10 * time.Second

func doPeerShutdown(ctx context.Context, p fleet.Peer, force bool, timeout time.Duration) error {
	if p.Transport != fleet.TransportAgentd {
		return fmt.Errorf("transport %q does not support /v1/self/shutdown", p.Transport)
	}
	cli := transport.New(p.Name, p.Addr, p.Token)
	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	_, err := cli.Shutdown(pCtx, force)
	return err
}
