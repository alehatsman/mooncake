package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/discovery"
	"github.com/urfave/cli/v2"
)

// fleetDiscoverCommand registers `mooncake fleet discover`.
//
// Scope (spec-45, simple slice): aggregate peer candidates from
// peers.toml and ~/.ssh/config, optionally probe each agentd peer with a
// /v1/version round-trip, and print a one-line-per-candidate table.
// mDNS discovery is a planned follow-up.
func fleetDiscoverCommand() *cli.Command {
	return &cli.Command{
		Name:  "discover",
		Usage: "List candidate peers from peers.toml and ~/.ssh/config",
		Description: "Aggregate fleet candidates from local sources without modifying state.\n" +
			"Useful as a smoke check before `mooncake fleet bootstrap` or to confirm\n" +
			"which boxes are visible to the controller right now. Each row reports the\n" +
			"source(s) that surfaced it and (for peers.toml entries) the result of a\n" +
			"/v1/version probe.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path (default ~/.config/mooncake/peers.toml)",
			},
			&cli.StringFlag{
				Name:  "ssh-config",
				Usage: "Override the ssh_config path (default ~/.ssh/config). Use '-' to skip the SSH source entirely.",
			},
			&cli.BoolFlag{
				Name:  "no-probe",
				Usage: "Skip the /v1/version round-trip for peers.toml entries (offline mode)",
			},
			&cli.DurationFlag{
				Name:  "probe-timeout",
				Value: 2 * time.Second,
				Usage: "Per-peer agentd probe timeout",
			},
			&cli.BoolFlag{
				Name:  "no-mdns",
				Usage: "Skip the mDNS browse for `_mooncake._tcp.local`. Useful on captive networks where multicast is unreliable, or when you want a pure peers.toml + ssh_config list.",
			},
			&cli.DurationFlag{
				Name:  "mdns-timeout",
				Value: 3 * time.Second,
				Usage: "mDNS browse wall-clock timeout. Responses arriving after the timeout are dropped.",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Emit candidates as a JSON array (one object per candidate) instead of the text table",
			},
		},
		Action: fleetDiscoverAction,
	}
}

func fleetDiscoverAction(c *cli.Context) error {
	probe := !c.Bool("no-probe")
	mdns := !c.Bool("no-mdns")
	opts := discovery.Options{
		PeersPath:     c.String("peers-file"),
		SSHConfigPath: c.String("ssh-config"),
		Probe:         &probe,
		ProbeTimeout:  c.Duration("probe-timeout"),
		MDNS:          &mdns,
		MDNSTimeout:   c.Duration("mdns-timeout"),
	}
	cands, err := discovery.Aggregate(c.Context, opts)
	if err != nil {
		return err
	}

	if c.Bool("json") {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(cands)
	}

	if len(cands) == 0 {
		fmt.Println("no peers configured in peers.toml; no usable hosts in ~/.ssh/config")
		fmt.Println("hint: `mooncake fleet bootstrap <user@host>` to add the first peer")
		return nil
	}

	renderDiscoverTable(os.Stdout, cands)
	return nil
}

// renderDiscoverTable lays out [SOURCE NAME ADDR STATUS] via tabwriter,
// mirroring fleet_status's layout so operators see a consistent shape
// across fleet subcommands.
//
// SOURCE collapses multiple-source candidates into a "+"-joined string
// ("peers.toml+ssh-config"), so dedup is visible at a glance.
//
// STATUS is the probe outcome for peers.toml entries; ssh-config-only
// entries say "ssh-only" (no agentd assumed).
func renderDiscoverTable(w io.Writer, cands []discovery.Candidate) {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "SOURCE\tNAME\tADDR\tSTATUS")
	for _, c := range cands {
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n",
			strings.Join(c.Sources, "+"),
			c.Name,
			displayAddr(c),
			discoverStatusCell(c),
		)
	}
	_ = tw.Flush()

	// Tail summary mirrors `fleet status`: per-source counts with a
	// reachability roll-up.
	var (
		peersN, sshN, both, agentdUp, probeFail int
	)
	for _, c := range cands {
		hasPeers := c.HasSource(discovery.SourcePeersTOML)
		hasSSH := c.HasSource(discovery.SourceSSHConfig)
		switch {
		case hasPeers && hasSSH:
			both++
		case hasPeers:
			peersN++
		case hasSSH:
			sshN++
		}
		if hasPeers && c.AgentdOK {
			agentdUp++
		}
		if hasPeers && !c.AgentdOK && c.ProbeError != "" {
			probeFail++
		}
	}
	parts := make([]string, 0, 4)
	parts = append(parts, fmt.Sprintf("%d candidate(s)", len(cands)))
	if peersN+both > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d agentd reachable", agentdUp, peersN+both))
	}
	if sshN > 0 {
		parts = append(parts, fmt.Sprintf("%d ssh-only", sshN))
	}
	if probeFail > 0 {
		parts = append(parts, fmt.Sprintf("%d probe failed", probeFail))
	}
	fmt.Fprintln(w, strings.Join(parts, ", "))
}

func displayAddr(c discovery.Candidate) string {
	if c.HasSource(discovery.SourcePeersTOML) {
		return c.Addr
	}
	// ssh-config-only: append :Port if set.
	if c.SSHPort > 0 {
		return fmt.Sprintf("%s:%d", c.Addr, c.SSHPort)
	}
	return c.Addr
}

func discoverStatusCell(c discovery.Candidate) string {
	if !c.HasSource(discovery.SourcePeersTOML) {
		return "ssh-only"
	}
	if c.AgentdOK {
		if c.AgentdVersion != "" {
			return "✓ agentd up (mooncake " + c.AgentdVersion + ")"
		}
		return "✓ agentd up"
	}
	if c.ProbeError != "" {
		return "✗ " + c.ProbeError
	}
	return "(probe skipped)"
}
