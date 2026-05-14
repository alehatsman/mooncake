package main

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
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
			fleetStatusCommand(),
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
			"run via agentd, and stream [host]-prefixed log lines back. Peers " +
			"are processed in parallel; output is line-atomic per peer.",
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
			&cli.IntFlag{
				Name:  "parallel",
				Usage: "Maximum peers in flight at once (0 = unbounded, default)",
				Value: 0,
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable ANSI colors in the [host] prefix (also honors NO_COLOR env)",
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
// Peers are applied in parallel (capped by --parallel). One PeerEvent
// channel feeds a single Multiplexer that owns stdout; per-peer goroutines
// never write to the terminal directly, so output is line-atomic.
//
// SIGINT: first signal cancels the local apply context (closes SSE streams)
// and prints a banner explaining that remote runs continue. Second signal
// hard-exits with code 130.
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
	selected, unknown := selectPeers(cfgPeers.Peers, peersFilter)
	if len(selected) == 0 {
		msg := "fleet apply: no peers matched filter " + peersFilter
		if len(unknown) > 0 {
			msg += " (unknown: " + strings.Join(unknown, ", ") + ")"
		}
		return cli.Exit(msg, 1)
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

	// Filter out non-agentd transports up-front. Skipped peers are reported
	// after the orchestrator banner so the output order is consistent:
	// banner → unknown-peer warning → skips → per-peer events → summary.
	agentdPeers := make([]fleet.Peer, 0, len(selected))
	var skipped []fleet.Peer
	for _, p := range selected {
		if p.Transport != fleet.TransportAgentd {
			skipped = append(skipped, p)
			continue
		}
		agentdPeers = append(agentdPeers, p)
	}
	if len(agentdPeers) == 0 {
		return cli.Exit("fleet apply: no agentd-transport peers selected", 1)
	}

	peerNames := make([]string, 0, len(agentdPeers))
	for _, p := range agentdPeers {
		peerNames = append(peerNames, p.Name)
	}
	useColor := fleet.ShouldColor(w, c.Bool("no-color"))
	mux := fleet.NewMultiplexer(w, peerNames, useColor)
	mux.Banner(fmt.Sprintf("fleet apply: %s → %d peer(s)", planAbs, len(agentdPeers)))
	if len(unknown) > 0 {
		mux.Banner("warning: unknown peer name(s) in --peers filter: " + strings.Join(unknown, ", "))
	}
	for _, p := range skipped {
		mux.Banner(fmt.Sprintf("skipped %s: transport %q not supported (agentd only)", p.Name, p.Transport))
	}

	// One event channel feeds the single Multiplexer. Buffer ~64 events per
	// peer to keep producers off the critical path during bursty step.stdout
	// runs (e.g. packer.nvim parallel git clones).
	events := make(chan fleet.PeerEvent, 64*len(agentdPeers))
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	// SIGINT: first → cancel & banner; second → hard exit.
	applyCtx, cancel := context.WithCancel(c.Context)
	defer cancel()
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			mux.Banner("⚠ ^C closes the log stream only — remote runs continue.")
			mux.Banner("  See `mooncake fleet logs <host>` to reattach.")
			cancel()
			select {
			case <-sigCh:
				// Second signal: bypass the orderly shutdown.
				os.Exit(130)
			case <-applyCtx.Done():
				// Apply finished on its own after first ^C.
			}
		case <-applyCtx.Done():
		}
	}()

	// Optional semaphore. parallel <= 0 → unbounded.
	parallel := c.Int("parallel")
	var sem chan struct{}
	if parallel > 0 {
		sem = make(chan struct{}, parallel)
	}

	results := make([]fleet.ApplyResult, len(agentdPeers))
	errs := make([]error, len(agentdPeers))
	var wg sync.WaitGroup
	for i, p := range agentdPeers {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			client := transport.New(p.Name, p.Addr, p.Token)
			results[i], errs[i] = fleet.Apply(applyCtx, fleet.ApplyOptions{
				PeerName:     p.Name,
				Peer:         client,
				PlanDir:      planDir,
				PlanPath:     planAbs,
				VarsFiles:    varsAbs,
				Tags:         tags,
				ControllerID: controllerID,
				MaxSyncBytes: maxSync,
				Events:       events,
			})
		}(i, p)
	}
	wg.Wait()
	close(events)
	<-drained

	// Aggregate exit code:
	//   0 → every peer's run reached "success"
	//   1 → at least one peer ran but ended failed/interrupted
	//   2 → at least one peer was unreachable (sync/version/submit failed)
	ok, runFailed, unreachable := 0, 0, 0
	var failedPeers []string
	for i, r := range results {
		switch {
		case r.Status == "success":
			ok++
		case r.Status == "failed" || r.Status == "interrupted":
			runFailed++
			failedPeers = append(failedPeers, agentdPeers[i].Name)
		default:
			// Status == "" → never made it to a terminal SSE event.
			// errs[i] should be set in that case (transport/sync/submit).
			if errs[i] != nil {
				unreachable++
				failedPeers = append(failedPeers, agentdPeers[i].Name)
			}
		}
	}
	mux.Banner(fmt.Sprintf("fleet apply: %d/%d ok", ok, len(agentdPeers)))

	switch {
	case unreachable > 0:
		return cli.Exit("fleet apply: unreachable peer(s): "+strings.Join(failedPeers, ", "), 2)
	case runFailed > 0:
		return cli.Exit("fleet apply: failed on peer(s): "+strings.Join(failedPeers, ", "), 1)
	}
	// Belt + braces: surface a stray error that didn't map to either bucket.
	var firstErr error
	for _, e := range errs {
		if e != nil && !errors.Is(e, context.Canceled) {
			firstErr = e
			break
		}
	}
	if firstErr != nil {
		return errors.Join(firstErr)
	}
	return nil
}

// selectPeers filters peers by a comma-separated name list. An empty filter
// returns all peers and no unknowns. Names from the filter that don't match
// any peer are returned in `unknown` so callers can warn the user (typos in
// --peers used to silently no-op).
func selectPeers(peers []fleet.Peer, filter string) (matched []fleet.Peer, unknown []string) {
	if filter == "" {
		matched = make([]fleet.Peer, len(peers))
		copy(matched, peers)
		return matched, nil
	}
	known := make(map[string]struct{}, len(peers))
	for _, p := range peers {
		known[p.Name] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, raw := range strings.Split(filter, ",") {
		n := strings.TrimSpace(raw)
		if n == "" {
			continue
		}
		if _, dup := seen[n]; dup {
			continue
		}
		seen[n] = struct{}{}
		if _, ok := known[n]; !ok {
			unknown = append(unknown, n)
		}
	}
	for _, p := range peers {
		if _, ok := seen[p.Name]; ok {
			matched = append(matched, p)
		}
	}
	return matched, unknown
}
