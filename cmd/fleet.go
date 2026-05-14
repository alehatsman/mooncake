package main

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
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
			"run via agentd, and stream multiplexed [host] log lines back. " +
			"In PR3 this command resolves peers + plan-dir and prints a plan; " +
			"real sync + run land in PR4 and PR5.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "peers",
				Usage: "Comma-separated list of peer names to target (default: all in peers.toml)",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
		},
		Action: fleetApplyAction,
	}
}

// fleetApplyAction in PR3 is a skeleton: it resolves the controller ID,
// loads peers.toml, filters by --peers, resolves the plan-dir, and prints
// what it WOULD do. Real sync + submit + stream lands in PR4/PR5.
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
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return err
	}
	if len(cfg.Peers) == 0 {
		return cli.Exit("fleet apply: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	selected := selectPeers(cfg.Peers, c.String("peers"))
	if len(selected) == 0 {
		return cli.Exit("fleet apply: no peers matched --peers filter", 1)
	}

	// PR3 stops here. PR4 wires the transport; PR5 wires sync+submit+stream.
	w := c.App.Writer
	fmt.Fprintf(w, "controller_id: %s\n", controllerID)
	fmt.Fprintf(w, "plan_dir:      %s\n", planDir)
	fmt.Fprintf(w, "plan_file:     %s\n", planAbs)
	fmt.Fprintf(w, "peers (%d):\n", len(selected))
	for _, p := range selected {
		fmt.Fprintf(w, "  - %s (%s, %s)\n", p.Name, p.Transport, p.Addr)
	}
	fmt.Fprintln(w, "\n[PR3 skeleton — sync + apply land in PR4/PR5]")
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
