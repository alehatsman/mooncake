package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			fleetBootstrapCommand(),
			fleetPairCommand(),
		},
	}
}

func fleetBootstrapCommand() *cli.Command {
	return &cli.Command{
		Name:      "bootstrap",
		Usage:     "Install mooncake on a remote box via SSH and register it as a peer",
		ArgsUsage: "<user@host>",
		Description: "Minimal bootstrap (spec-44/43 PR9-11 cut down to the essentials):\n" +
			" - SCPs the local mooncake binary to the target via system ssh/scp.\n" +
			" - Starts agentd in the foreground via nohup (no systemd/launchd yet).\n" +
			" - Reads the freshly minted token and upserts a [[peers]] entry.\n" +
			" - Verifies the bind addr is reachable from this machine.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Usage: "SSH port", Value: 22},
			&cli.IntFlag{Name: "agentd-port", Usage: "agentd TCP port on remote", Value: 7878},
			&cli.StringFlag{Name: "name", Usage: "Peer name in peers.toml (default: hostname with dots → dashes)"},
			&cli.StringSliceFlag{Name: "tag", Usage: "Tag to attach to the peer (repeatable)"},
			&cli.StringFlag{Name: "binary", Usage: "Path to mooncake binary to upload (default: this process)"},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
		},
		Action: fleetBootstrapAction,
	}
}

func fleetBootstrapAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("fleet bootstrap: exactly one <user@host> argument required", 2)
	}
	target, err := transport.ParseSSHTarget(c.Args().First())
	if err != nil {
		return cli.Exit(err.Error(), 2)
	}
	// Allow --port to override unless the target string already specified one.
	if target.Port == 0 {
		target.Port = c.Int("port")
	}

	binPath := c.String("binary")
	if binPath == "" {
		binPath, err = fleet.EnsureLocalBinaryPath()
		if err != nil {
			return err
		}
	}

	peersPath := c.String("peers-file")
	if peersPath == "" {
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return err
		}
	}

	w := c.App.Writer
	res, err := fleet.Bootstrap(c.Context, fleet.BootstrapOptions{
		Target:      target,
		Name:        c.String("name"),
		Tags:        c.StringSlice("tag"),
		Port:        c.Int("agentd-port"),
		LocalBinary: binPath,
		Writer:      w,
	})
	if err != nil {
		return err
	}
	added, diff, err := fleet.Upsert(peersPath, res.Peer)
	if err != nil {
		return fmt.Errorf("update peers.toml: %w", err)
	}
	if added {
		fmt.Fprintf(w, "wrote new peer %q to %s\n", res.Peer.Name, peersPath)
	} else {
		fmt.Fprintf(w, "updated peer %q in %s\n", res.Peer.Name, peersPath)
		for _, line := range diff {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	fmt.Fprintf(w, "✓ %s is in your fleet. Try: mooncake fleet apply <plan.yml>\n", res.Peer.Name)
	return nil
}

func fleetPairCommand() *cli.Command {
	return &cli.Command{
		Name:      "pair",
		Usage:     "Register an already-running agentd as a fleet peer",
		ArgsUsage: "<host:port>",
		Description: "Use this when agentd is already installed and running on the target. " +
			"The token is read from --token-via (stdin|file:<path>|literal:<tok>). " +
			"For a fresh box without mooncake installed, use `fleet bootstrap` instead.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "name", Usage: "Peer name in peers.toml (default: host portion of addr)"},
			&cli.StringSliceFlag{Name: "tag", Usage: "Tag to attach to the peer (repeatable)"},
			&cli.StringFlag{
				Name:  "token-via",
				Usage: "Where to read the bearer token from: stdin | file:<path> | literal:<token>",
				Value: "stdin",
			},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
		},
		Action: fleetPairAction,
	}
}

func fleetPairAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("fleet pair: exactly one <host:port> argument required", 2)
	}
	addr := c.Args().First()
	if _, _, err := net.SplitHostPort(addr); err != nil {
		return cli.Exit(fmt.Sprintf("invalid addr %q: %v", addr, err), 2)
	}

	token, err := readToken(c, c.String("token-via"))
	if err != nil {
		return err
	}

	name := c.String("name")
	if name == "" {
		host, _, _ := net.SplitHostPort(addr)
		name = strings.ReplaceAll(host, ".", "-")
	}

	peersPath := c.String("peers-file")
	if peersPath == "" {
		peersPath, err = fleet.DefaultPeersPath()
		if err != nil {
			return err
		}
	}

	peer := fleet.Peer{
		Name:      name,
		Addr:      addr,
		Transport: fleet.TransportAgentd,
		Token:     token,
		Tags:      c.StringSlice("tag"),
	}

	// Verify the token before persisting — refuse to write a bogus entry.
	client := transport.New(name, addr, token)
	ctx, cancel := context.WithTimeout(c.Context, 5*time.Second)
	defer cancel()
	if _, err := client.GetVersion(ctx); err != nil {
		return fmt.Errorf("verify peer at %s: %w", addr, err)
	}

	w := c.App.Writer
	added, diff, err := fleet.Upsert(peersPath, peer)
	if err != nil {
		return fmt.Errorf("update peers.toml: %w", err)
	}
	if added {
		fmt.Fprintf(w, "✓ wrote new peer %q to %s\n", name, peersPath)
	} else {
		fmt.Fprintf(w, "✓ updated peer %q in %s\n", name, peersPath)
		for _, line := range diff {
			fmt.Fprintf(w, "  %s\n", line)
		}
	}
	return nil
}

func readToken(c *cli.Context, src string) (string, error) {
	switch {
	case src == "stdin":
		fmt.Fprint(c.App.ErrWriter, "Paste bearer token: ")
		var tok string
		if _, err := fmt.Fscanln(c.App.Reader, &tok); err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		tok = strings.TrimSpace(tok)
		if tok == "" {
			return "", errors.New("empty token from stdin")
		}
		return tok, nil
	case strings.HasPrefix(src, "file:"):
		path := strings.TrimPrefix(src, "file:")
		data, err := os.ReadFile(path)
		if err != nil {
			return "", fmt.Errorf("read token file %s: %w", path, err)
		}
		tok := strings.TrimSpace(string(data))
		if tok == "" {
			return "", fmt.Errorf("token file %s is empty", path)
		}
		return tok, nil
	case strings.HasPrefix(src, "literal:"):
		tok := strings.TrimSpace(strings.TrimPrefix(src, "literal:"))
		if tok == "" {
			return "", errors.New("literal: token is empty")
		}
		return tok, nil
	default:
		return "", fmt.Errorf("unknown --token-via source %q (use stdin | file:<path> | literal:<token>)", src)
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
			&cli.StringFlag{
				Name:  "plan-dir",
				Usage: "Root directory to sync to each peer (default: directory of the plan file). " +
					"Use this when the plan imports siblings via `../` — point at the repo root.",
			},
			&cli.StringSliceFlag{
				Name:  "vars-file",
				Usage: "Vars file to load (relative to plan-dir or absolute); may be repeated",
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
// Argument forms:
//
//	mooncake fleet apply path/to/config.yml          → plan file path
//	mooncake fleet apply main_pc                     → "machine convention":
//	    plan      = <plan-dir>/entries/main_pc.yml
//	    plan-dir  = $PWD (unless --plan-dir given)
//	    vars      = variables.yml + vars/main_pc.yml (if present)
//	    --peers   = main_pc (unless --peers given)
//
// PR5 processes peers serially. PR6 will parallelize with multiplexed
// output and ^C handling.
func fleetApplyAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return cli.Exit("fleet apply: exactly one plan-file-or-machine argument required", 2)
	}
	planArg := c.Args().First()

	// Resolve plan-dir first: explicit flag, else PWD if the arg matches
	// the machine convention, else the directory of the plan file.
	planDir := c.String("plan-dir")
	if planDir != "" {
		var err error
		planDir, err = filepath.Abs(planDir)
		if err != nil {
			return fmt.Errorf("resolve plan-dir: %w", err)
		}
	}

	// Machine convention: bare name (no slash, no .yml) and
	// <plan-dir>/entries/<name>.yml exists.
	machine := ""
	if !strings.ContainsAny(planArg, "/\\") && !strings.HasSuffix(planArg, ".yml") {
		root := planDir
		if root == "" {
			pwd, _ := os.Getwd()
			root = pwd
		}
		entry := filepath.Join(root, "entries", planArg+".yml")
		if st, err := os.Stat(entry); err == nil && !st.IsDir() {
			machine = planArg
			planArg = entry
			if planDir == "" {
				planDir = root
			}
		}
	}

	planAbs, err := filepath.Abs(planArg)
	if err != nil {
		return fmt.Errorf("resolve plan path: %w", err)
	}
	if planDir == "" {
		planDir = filepath.Dir(planAbs)
	}

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

	// Peer filter: explicit --peers wins; otherwise if the machine
	// convention fired, default to that single peer.
	peersFilter := c.String("peers")
	if peersFilter == "" && machine != "" {
		peersFilter = machine
	}
	selected := selectPeers(cfgPeers.Peers, peersFilter)
	if len(selected) == 0 {
		return cli.Exit("fleet apply: no peers matched filter "+peersFilter, 1)
	}

	// Resolve vars files relative to plan-dir, absolute on the controller.
	// When the machine convention is active, prepend conventional vars files
	// (variables.yml + vars/<machine>.yml) when they exist on disk.
	varsRel := c.StringSlice("vars-file")
	if machine != "" {
		conv := []string{"variables.yml", filepath.Join("vars", machine+".yml")}
		// Prepend so explicit --vars-file overrides (later wins on key collision).
		merged := make([]string, 0, len(conv)+len(varsRel))
		for _, p := range conv {
			abs := filepath.Join(planDir, p)
			if _, err := os.Stat(abs); err == nil {
				merged = append(merged, p)
			}
		}
		merged = append(merged, varsRel...)
		varsRel = merged
	}
	var varsAbs []string
	for _, v := range varsRel {
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
			// Apply already prints `[peer] ✗ run failed: …` when the
			// daemon-side record carries an error; don't double-log here.
			// Only echo the error if it doesn't already say "run failed".
			if !strings.Contains(err.Error(), "run failed") &&
				!strings.Contains(err.Error(), "run interrupted") {
				fmt.Fprintf(w, "[%s] apply error: %v\n", p.Name, err)
			}
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
