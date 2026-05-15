package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
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
			fleetExecCommand(),
			fleetPsCommand(),
			fleetWatchCommand(),
			fleetStatusCommand(),
			fleetDoctorCommand(),
			fleetLogsCommand(),
			fleetFactsCommand(),
			fleetBootstrapCommand(),
			fleetPairCommand(),
			fleetDiscoverCommand(),
			fleetInitCommand(),
			fleetUpgradeCommand(),
		},
	}
}

func fleetBootstrapCommand() *cli.Command {
	return &cli.Command{
		Name:      "bootstrap",
		Usage:     "Install mooncake on a remote box via SSH and register it as a peer",
		ArgsUsage: "<user@host>",
		Description: "Production bootstrap (spec-44 §88, 8-step sequence):\n" +
			" 1. SSH to <user@host> using ssh-agent or ~/.ssh/id_ed25519.\n" +
			" 2. Detect platform (linux+darwin+windows × amd64+arm64).\n" +
			" 3. Skip steps 4-6 if the same version is already installed and active.\n" +
			" 4. SFTP the mooncake binary; sudo-install to /usr/local/bin (linux/darwin)\n" +
			"    or Move-Item to %LOCALAPPDATA%\\Mooncake\\bin\\mooncake.exe (windows).\n" +
			" 5. Render + install a systemd unit (Linux), launchd plist (macOS),\n" +
			"    or Task Scheduler XML (Windows). Windows: also opens host firewall.\n" +
			" 6. Enable + start the service; wait for /v1/version reachable.\n" +
			" 7. Read the bearer token (sudo cat on linux/darwin, Get-Content on\n" +
			"    windows from %LOCALAPPDATA%\\Mooncake\\agentd.token).\n" +
			" 8. Upsert a [[peers]] entry in peers.toml.",
		Flags: []cli.Flag{
			&cli.IntFlag{Name: "port", Aliases: []string{"p"}, Usage: "SSH port", Value: 22},
			&cli.IntFlag{Name: "agentd-port", Usage: "agentd TCP port on remote", Value: 7878},
			&cli.StringFlag{Name: "name", Usage: "Peer name in peers.toml (default: hostname with dots → dashes)"},
			&cli.StringSliceFlag{Name: "tag", Usage: "Tag to attach to the peer (repeatable)"},
			&cli.StringFlag{Name: "binary", Usage: "Path to mooncake binary to upload (default: this process)"},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
			&cli.BoolFlag{
				Name:  "upgrade",
				Usage: "Replace an already-installed mooncake of a different version. " +
					"Without this, version mismatch on the target errors out.",
			},
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
		Target:            target,
		Name:              c.String("name"),
		Tags:              c.StringSlice("tag"),
		Port:              c.Int("agentd-port"),
		LocalBinary:       binPath,
		ControllerVersion: version,
		Upgrade:           c.Bool("upgrade"),
		Writer:            w,
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
				Name: "peer-filter",
				Usage: "Filter peers by `key=value` (v1: tag=<x>). " +
					"Commas within one flag = AND; repeating the flag = OR. " +
					"Intersected with --peers. Example: --peer-filter tag=os=darwin",
			},
			&cli.StringSliceFlag{
				Name: "step-filter",
				Usage: "Filter steps to run on each peer by `key=value` (v1: tag=<x>); " +
					"forwarded to daemon. Multiple values OR together. " +
					"Example: --step-filter tag=deploy",
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
//	    plan      = <plan-dir>/machines/main_pc/index.yml
//	    plan-dir  = $PWD (unless --plan-dir given)
//	    vars      = shared/variables.yml + machines/main_pc/vars.yml (if present)
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

	// Machine convention: bare name (no slash, no .yml) → look for a
	// machine layout under <plan-dir>/machines/<name>/. Two layouts are
	// recognised, checked in this order:
	//
	//  1. fleet.yml — multi-phase apply (spec for `mooncake apply <name>`
	//     against a Windows+WSL box that needs ordered host-then-guest
	//     phases). Dispatches to runMachineApply later.
	//  2. index.yml — single-phase apply (the original spec-48 machine
	//     convention; one peer named <name>, one entry plan).
	//
	// Each machine owns a directory so it can carry its own vars.yml /
	// fixtures alongside the entry plan.
	machine := ""
	var machineManifest *fleet.MachineManifest
	if !strings.ContainsAny(planArg, "/\\") && !strings.HasSuffix(planArg, ".yml") {
		root := planDir
		if root == "" {
			pwd, _ := os.Getwd()
			root = pwd
		}
		// First try the multi-phase manifest. A missing file falls
		// through to the index.yml single-phase check.
		manifestPath, found, lookupErr := fleet.LookupMachineManifest(root, planArg)
		if lookupErr != nil {
			return cli.Exit("fleet apply: "+lookupErr.Error(), 2)
		}
		if found {
			m, err := fleet.LoadMachineManifest(manifestPath)
			if err != nil {
				return cli.Exit("fleet apply: "+err.Error(), 2)
			}
			machine = planArg
			machineManifest = m
			if planDir == "" {
				planDir = root
			}
			// Set planArg to the manifest so error messages and the
			// planAbs derivation below have a sensible string. The
			// multi-phase path doesn't use planAbs directly; it uses
			// each phase's own Plan field.
			planArg = manifestPath
		} else {
			entry := filepath.Join(root, "machines", planArg, "index.yml")
			if st, err := os.Stat(entry); err == nil && !st.IsDir() {
				machine = planArg
				planArg = entry
				if planDir == "" {
					planDir = root
				}
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

	// --peer-filter applies on top of the --peers name filter.
	peerFilterGroups, err := parseFilterFlags(c.StringSlice("peer-filter"))
	if err != nil {
		return cli.Exit("fleet apply: "+err.Error(), 2)
	}
	if err := validatePeerFilterKeys(peerFilterGroups); err != nil {
		return cli.Exit("fleet apply: "+err.Error(), 2)
	}
	if len(peerFilterGroups) > 0 {
		// Spec-50 §Phase B: `os=` requires a /v1/version probe. Run that
		// pass only when at least one term needs it; cache results so a
		// single `fleet apply` doesn't re-probe the same peer for each
		// AND-group it appears in.
		var osFor peerOSResolver
		if peerFilterGroupsUseKey(peerFilterGroups, "os") {
			osFor = newPeerOSCache(c.Context, selected, c.App.Writer)
		}
		filtered := make([]fleet.Peer, 0, len(selected))
		for _, p := range selected {
			if peerMatchesFilters(p, peerFilterGroups, osFor) {
				filtered = append(filtered, p)
			}
		}
		if len(filtered) == 0 {
			return cli.Exit("fleet apply: --peer-filter selected 0 of "+strconv.Itoa(len(selected))+" peer(s); nothing to do", 1)
		}
		selected = filtered
	}

	// Resolve vars files relative to plan-dir, absolute on the controller.
	// When the machine convention is active, prepend conventional vars
	// files (shared/variables.yml + machines/<machine>/vars.yml) when
	// they exist on disk. The shared file goes first so per-machine
	// overrides win on key collision (later-wins).
	varsRel := c.StringSlice("vars-file")
	if machine != "" {
		conv := []string{
			filepath.Join("shared", "variables.yml"),
			filepath.Join("machines", machine, "vars.yml"),
		}
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
	tags, stepNames, err := extractStepFilter(c.StringSlice("step-filter"))
	if err != nil {
		return cli.Exit("fleet apply: "+err.Error(), 2)
	}

	w := c.App.Writer
	useColor := fleet.ShouldColor(w, c.Bool("no-color"))
	parallel := c.Int("parallel")

	// SIGINT: first → cancel & banner; second → hard exit. Owned at the
	// outer fleetApplyAction level so a single ^C cancels every in-flight
	// phase at once in machine mode.
	applyCtx, cancel := context.WithCancel(c.Context)
	defer cancel()
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			fmt.Fprintln(w, "⚠ ^C closes the log stream only — remote runs continue.")
			fmt.Fprintln(w, "  See `mooncake fleet logs <host>` to reattach.")
			cancel()
			select {
			case <-sigCh:
				os.Exit(130)
			case <-applyCtx.Done():
			}
		case <-applyCtx.Done():
		}
	}()

	// Multi-phase machine mode: dispatch to the manifest-driven
	// orchestrator. cfgPeers.Peers (not `selected`) is passed in so the
	// manifest can resolve any peer in peers.toml — `--peers`-driven
	// preselection doesn't make sense when the manifest is the
	// authoritative peer list.
	if machineManifest != nil {
		return runMachineApply(
			applyCtx, w, useColor,
			machineManifest, machine, cfgPeers.Peers,
			planDir, varsAbs, tags, stepNames,
			maxSync, parallel, controllerID,
			peerFilterGroups, nil, // osFor lazily allocated inside the helper if needed
		)
	}

	// Single-phase path: filter out non-agentd transports, run the
	// shared runApplyPhase helper that drives Apply across the selected
	// peer set.
	agentdPeers := make([]fleet.Peer, 0, len(selected))
	var skippedPeers []fleet.Peer
	for _, p := range selected {
		if p.Transport != fleet.TransportAgentd {
			skippedPeers = append(skippedPeers, p)
			continue
		}
		agentdPeers = append(agentdPeers, p)
	}
	if len(agentdPeers) == 0 {
		return cli.Exit("fleet apply: no agentd-transport peers selected", 1)
	}

	out := runApplyPhase(applyCtx, w, useColor, applyPhaseInput{
		PlanAbs:       planAbs,
		PlanDir:       planDir,
		Peers:         agentdPeers,
		UnknownPeers:  unknown,
		SkippedPeers:  skippedPeers,
		VarsAbs:       varsAbs,
		Tags:          tags,
		StepNames:     stepNames,
		MaxSyncBytes:  maxSync,
		Parallel:      parallel,
		ControllerID:  controllerID,
		BannerHeading: fmt.Sprintf("fleet apply: %s → %d peer(s)", planAbs, len(agentdPeers)),
	})

	switch {
	case out.Unreachable > 0:
		return cli.Exit("fleet apply: unreachable peer(s): "+strings.Join(out.FailedNames, ", "), 2)
	case out.RunFailed > 0:
		return cli.Exit("fleet apply: failed on peer(s): "+strings.Join(out.FailedNames, ", "), 1)
	}
	if out.FirstErr != nil {
		return errors.Join(out.FirstErr)
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

// filterTerm is a single `key=value` predicate inside a --peer-filter or
// --step-filter expression. v1 only accepts key="tag"; the parser is generic
// so the planned extension (spec-49 extended-filter-keys) lands as a
// validator change.
type filterTerm struct {
	key   string
	value string
}

// parseFilterFlags converts the raw string slice from cli.StringSlice into
// AND-groups. Each flag value contributes one group; commas inside a value
// separate AND-terms within that group. The peer/step must match every term
// in at least one group.
//
//	--peer-filter tag=a,tag=b                       → [[a, b]]            (a AND b)
//	--peer-filter tag=a --peer-filter tag=b         → [[a], [b]]          (a OR b)
//	--peer-filter tag=a,tag=b --peer-filter tag=c   → [[a, b], [c]]       ((a AND b) OR c)
func parseFilterFlags(args []string) ([][]filterTerm, error) {
	var groups [][]filterTerm
	for _, arg := range args {
		var group []filterTerm
		for _, raw := range strings.Split(arg, ",") {
			tok := strings.TrimSpace(raw)
			if tok == "" {
				continue
			}
			eq := strings.IndexByte(tok, '=')
			if eq < 0 {
				return nil, fmt.Errorf("invalid filter %q: expected key=value", tok)
			}
			key := strings.TrimSpace(tok[:eq])
			val := strings.TrimSpace(tok[eq+1:])
			if key == "" || val == "" {
				return nil, fmt.Errorf("invalid filter %q: key and value must be non-empty", tok)
			}
			group = append(group, filterTerm{key: key, value: val})
		}
		if len(group) > 0 {
			groups = append(groups, group)
		}
	}
	return groups, nil
}

// peerFilterKeys is the allowlist enforced by validatePeerFilterKeys. Kept
// as a slice so the error message can render the keys in a stable order
// (spec-50 §G3: the unknown-key error lists the valid keys explicitly).
var peerFilterKeys = []string{"tag", "name", "os", "role"}

// validatePeerFilterKeys rejects any --peer-filter key not in peerFilterKeys.
// Spec-50 extends the v1 allowlist (`tag`) with `name`, `os`, and `role`;
// the parser is already generic over keys, so this is a validator change
// only.
func validatePeerFilterKeys(groups [][]filterTerm) error {
	for _, g := range groups {
		for _, t := range g {
			if !isPeerFilterKey(t.key) {
				return fmt.Errorf(
					"unsupported --peer-filter key %q (valid: %s)",
					t.key, strings.Join(peerFilterKeys, ", "))
			}
		}
	}
	return nil
}

func isPeerFilterKey(k string) bool {
	for _, v := range peerFilterKeys {
		if k == v {
			return true
		}
	}
	return false
}

// peerOSResolver is the function peerMatchesFilters consults for `os=`
// predicates. Returns the GOOS string for p (e.g. "darwin", "linux") and
// ok=false when the daemon couldn't be probed. Spec-50 §G1: `os=` matches
// `runtime.GOOS` from the peer's `/v1/version` response, cached per
// `fleet apply` invocation by the caller.
type peerOSResolver func(p fleet.Peer) (string, bool)

// peerMatchesFilters reports whether p satisfies any of the AND-groups in
// groups. An empty groups slice matches everything.
//
// osFor is the resolver for `os=` predicates; pass nil when no `os=`
// term is present in any group (or to force every `os=` predicate to
// fail). When osFor returns ok=false, the `os=` predicate fails and the
// caller is responsible for surfacing a warning so the operator notices
// an unreachable peer dropped out of the selection.
func peerMatchesFilters(p fleet.Peer, groups [][]filterTerm, osFor peerOSResolver) bool {
	if len(groups) == 0 {
		return true
	}
	tagSet := make(map[string]struct{}, len(p.Tags))
	for _, t := range p.Tags {
		tagSet[t] = struct{}{}
	}
	roleSet := make(map[string]struct{}, len(p.Roles))
	for _, r := range p.Roles {
		roleSet[r] = struct{}{}
	}
	for _, g := range groups {
		all := true
		for _, t := range g {
			switch t.key {
			case "tag":
				if _, ok := tagSet[t.value]; !ok {
					all = false
				}
			case "name":
				if p.Name != t.value {
					all = false
				}
			case "role":
				if _, ok := roleSet[t.value]; !ok {
					all = false
				}
			case "os":
				// No resolver, or probe failed → predicate fails. The
				// caller logs the unreachable case so the operator
				// notices the peer dropped out.
				if osFor == nil {
					all = false
					break
				}
				peerOS, ok := osFor(p)
				if !ok || peerOS != t.value {
					all = false
				}
			default:
				// validatePeerFilterKeys should have rejected this; treat
				// defensively as a non-match rather than panic.
				all = false
			}
			if !all {
				break
			}
		}
		if all {
			return true
		}
	}
	return false
}

// peerFilterGroupsUseKey reports whether any term in groups uses key. Lets
// the caller skip the os-probe pass entirely when no `os=` predicate is
// present.
func peerFilterGroupsUseKey(groups [][]filterTerm, key string) bool {
	for _, g := range groups {
		for _, t := range g {
			if t.key == key {
				return true
			}
		}
	}
	return false
}

// stepFilterKeys is the allowlist for --step-filter. Spec-50 adds `name=`
// so operators can target a single step by name without editing the plan.
var stepFilterKeys = []string{"tag", "name"}

// extractStepFilter flattens --step-filter values into the (tags, names)
// pair the daemon's executor consumes. AND/OR within step-filter has no
// daemon-side analogue (the executor unions tags and unions names), so
// all values dedupe into flat per-key lists.
func extractStepFilter(args []string) (tags, names []string, err error) {
	tagSeen := make(map[string]struct{})
	nameSeen := make(map[string]struct{})
	for _, arg := range args {
		for _, raw := range strings.Split(arg, ",") {
			tok := strings.TrimSpace(raw)
			if tok == "" {
				continue
			}
			eq := strings.IndexByte(tok, '=')
			if eq < 0 {
				return nil, nil, fmt.Errorf("invalid --step-filter %q: expected key=value", tok)
			}
			key := strings.TrimSpace(tok[:eq])
			val := strings.TrimSpace(tok[eq+1:])
			if val == "" {
				return nil, nil, fmt.Errorf("invalid --step-filter %q: empty value", tok)
			}
			switch key {
			case "tag":
				if _, dup := tagSeen[val]; dup {
					continue
				}
				tagSeen[val] = struct{}{}
				tags = append(tags, val)
			case "name":
				if _, dup := nameSeen[val]; dup {
					continue
				}
				nameSeen[val] = struct{}{}
				names = append(names, val)
			default:
				return nil, nil, fmt.Errorf(
					"unsupported --step-filter key %q (valid: %s)",
					key, strings.Join(stepFilterKeys, ", "))
			}
		}
	}
	return tags, names, nil
}

// newPeerOSCache returns a peerOSResolver that probes each peer's
// /v1/version exactly once per `fleet apply` invocation and caches the
// reported OS. Probes run lazily — the first match attempt against a
// peer triggers its probe — so peers excluded by another AND-term in
// the same group never get probed at all.
//
// Unreachable peers (probe error, missing token, etc.) get a warning
// printed once to w and ok=false on every subsequent resolve. This
// surfaces dropped peers to the operator without spamming the log when
// multiple groups consult the same peer.
func newPeerOSCache(ctx context.Context, peers []fleet.Peer, w io.Writer) peerOSResolver {
	type cacheEntry struct {
		os string
		ok bool
	}
	cache := make(map[string]cacheEntry, len(peers))
	tokens := make(map[string]string, len(peers))
	addrs := make(map[string]string, len(peers))
	warned := make(map[string]struct{})
	for _, p := range peers {
		tokens[p.Name] = p.Token
		addrs[p.Name] = p.Addr
	}

	const probeTimeout = 2 * time.Second
	return func(p fleet.Peer) (string, bool) {
		if e, ok := cache[p.Name]; ok {
			return e.os, e.ok
		}
		var entry cacheEntry
		token := tokens[p.Name]
		addr := addrs[p.Name]
		if token == "" || addr == "" || p.Transport != fleet.TransportAgentd {
			entry.ok = false
		} else {
			probeCtx, cancel := context.WithTimeout(ctx, probeTimeout)
			defer cancel()
			cli := transport.New(p.Name, addr, token)
			v, err := cli.GetVersion(probeCtx)
			if err == nil && v.OS != "" {
				entry.os = v.OS
				entry.ok = true
			}
		}
		cache[p.Name] = entry
		if !entry.ok {
			if _, already := warned[p.Name]; !already {
				warned[p.Name] = struct{}{}
				fmt.Fprintf(w, "warning: peer %q: os= predicate could not be evaluated (peer unreachable or daemon predates spec-50)\n", p.Name)
			}
		}
		return entry.os, entry.ok
	}
}
