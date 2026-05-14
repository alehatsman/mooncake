package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

func fleetCommand() *cli.Command {
	return &cli.Command{
		Name:  "fleet",
		Usage: "Manage and operate a personal fleet of mooncake peers (experimental)",
		Description: "Drive plans across machines you own: discover peers, " +
			"sync plan trees, apply with multiplexed logs. Peers are configured in " +
			"~/.config/mooncake/peers.toml.",
		Subcommands: []*cli.Command{
			fleetApplyCommand(),
		},
	}
}

func fleetApplyCommand() *cli.Command {
	return &cli.Command{
		Name:      "apply",
		Usage:     "Apply a plan to one or more fleet peers",
		ArgsUsage: "<plan.yml>",
		Description: "Sync the plan's directory to each selected peer, submit a " +
			"run via agentd, and stream [host]-prefixed log lines back. PR5 " +
			"processes peers serially; PR6 adds parallel multiplexing.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "peers",
				Usage: "Comma-separated list of peer names to target (default: all in peers.toml)",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.Int64Flag{
				Name:  "max-sync-size",
				Usage: "Plan-dir cumulative size cap in bytes (default 100 MiB)",
				Value: 100 << 20,
			},
			&cli.StringSliceFlag{
				Name:  "vars-file",
				Usage: "Vars file to load (relative to plan-dir); may be repeated",
			},
			&cli.StringSliceFlag{
				Name:  "tag",
				Usage: "Forward this tag to the daemon's run-submit (filters steps); may be repeated",
			},
		},
		Action: fleetApplyAction,
	}
}

// fleetApplyAction runs the full apply cycle: resolve controller id +
// plan-dir, sync the plan-dir to each selected peer's <state_dir>/synced/,
// submit a run, and stream [host]-prefixed events to stdout.
//
// PR5 processes peers serially. PR6 will parallelize with multiplexed
// output and ^C handling.
func fleetApplyAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("fleet apply: exactly one plan file argument required", 2)
	}
	planArg := c.Args().First()
	planAbs, err := filepath.Abs(planArg)
	if err != nil {
		return fmt.Errorf("resolve plan path: %w", err)
	}
	planDir := filepath.Dir(planAbs)

	controllerID, err := fleet.EnsureControllerID()
	if err != nil {
		return fmt.Errorf("controller id: %w", err)
	}

	peersPath := c.String("peers-file")
	if peersPath == "" {
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return err
		}
	}
	cfgPeers, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return err
	}
	if len(cfgPeers.Peers) == 0 {
		return cli.Exit("fleet apply: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	selected := selectPeers(cfgPeers.Peers, c.String("peers"))
	if len(selected) == 0 {
		return cli.Exit("fleet apply: no peers matched --peers filter", 1)
	}

	// Resolve vars files relative to plan-dir, absolute on the controller.
	var varsAbs []string
	for _, v := range c.StringSlice("vars-file") {
		if !filepath.IsAbs(v) {
			v = filepath.Join(planDir, v)
		}
		varsAbs = append(varsAbs, filepath.Clean(v))
	}

	maxSync := c.Int64("max-sync-size")
	tags := c.StringSlice("tag")

	w := c.App.Writer
	fmt.Fprintf(w, "fleet apply: %s → %d peer(s)\n", planAbs, len(selected))

	var firstErr error
	var failedPeers []string
	results := make([]fleet.ApplyResult, 0, len(selected))
	for _, p := range selected {
		if p.Transport != fleet.TransportAgentd {
			fmt.Fprintf(w, "[%s] skipped: transport %q not supported in PR5 (agentd only)\n", p.Name, p.Transport)
			continue
		}
		client := transport.New(p.Name, p.Addr, p.Token)
		res, err := fleet.Apply(c.Context, fleet.ApplyOptions{
			PeerName:     p.Name,
			Peer:         client,
			PlanDir:      planDir,
			PlanPath:     planAbs,
			VarsFiles:    varsAbs,
			Tags:         tags,
			ControllerID: controllerID,
			MaxSyncBytes: maxSync,
			Writer:       w,
		})
		results = append(results, res)
		if err != nil {
			fmt.Fprintf(w, "[%s] apply error: %v\n", p.Name, err)
			failedPeers = append(failedPeers, p.Name)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		fmt.Fprintf(w, "[%s] sync: %d uploaded, %d skipped (%d bytes total)\n",
			p.Name, res.Sync.Put, res.Sync.Skipped, res.Sync.BytesTotal)
		if res.Status != "" && res.Status != "success" {
			failedPeers = append(failedPeers, p.Name)
		}
	}

	// Summary line.
	ok := 0
	for _, r := range results {
		if r.Status == "success" {
			ok++
		}
	}
	fmt.Fprintf(w, "fleet apply: %d/%d ok\n", ok, len(selected))

	if len(failedPeers) > 0 {
		return cli.Exit(
			"fleet apply: failed on peer(s): "+strings.Join(failedPeers, ", "), 1)
	}
	if firstErr != nil {
		// Belt + braces: shouldn't normally happen because err → failedPeers.
		return errors.Join(firstErr)
	}
	return nil
}

// selectPeers filters peers by a comma-separated name list. An empty filter
// returns all peers. Names not present in the config are silently skipped;
// callers should check the result length.
func selectPeers(peers []fleet.Peer, filter string) []fleet.Peer {
	if filter == "" {
		out := make([]fleet.Peer, len(peers))
		copy(out, peers)
		return out
	}
	wanted := make(map[string]struct{})
	for _, n := range strings.Split(filter, ",") {
		n = strings.TrimSpace(n)
		if n != "" {
			wanted[n] = struct{}{}
		}
	}
	var out []fleet.Peer
	for _, p := range peers {
		if _, ok := wanted[p.Name]; ok {
			out = append(out, p)
		}
	}
	return out
}
