package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/urfave/cli/v2"
	"golang.org/x/term"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// Version is the binary's controller version, stamped at build time by
// goreleaser linker flags into cmd/mooncake.go's `version` and copied
// here from main() before Command() is invoked. Lives at the package
// level (not closed over Command()) so fleet subcommands deep in the
// tree can read it without threading the value through every action.
var Version = "dev"

// globalFleetFlags returns the four flags shared across all fleet subcommands.
// Hoisted to the parent `fleet` command so urfave/cli v2's context lineage
// walk makes them available from any subcommand action via c.String / c.Bool /
// c.Int without re-declaring them per-subcommand.
func globalFleetFlags() []cli.Flag {
	return []cli.Flag{
		&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
		&cli.BoolFlag{Name: "no-color", Usage: "Disable ANSI colors (also honors NO_COLOR env)"},
		&cli.BoolFlag{Name: "json", Usage: "Emit JSONL instead of formatted text"},
		&cli.IntFlag{Name: "parallel", Usage: "Max peers in flight (0 = unbounded)", Value: 0},
	}
}

func Command() *cli.Command {
	return &cli.Command{
		Name:  "fleet",
		Usage: "Manage and operate a personal fleet of mooncake peers (experimental)",
		Description: "Drive plans across machines you own: discover peers, " +
			"sync plan trees, apply with multiplexed logs. Peers are configured in " +
			"~/.config/mooncake/peers.toml. " +
			"Global flags (--peers-file, --no-color, --json, --parallel) apply to all " +
			"subcommands and must be placed before the subcommand name.",
		Flags: globalFleetFlags(),
		Subcommands: []*cli.Command{
			fleetApplyCommand(),
			fleetExecCommand(),
			fleetObserveCommand(),
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
			fleetShutdownCommand(),
			fleetUpCommand(),
			fleetMACRefreshCommand(),
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
			&cli.BoolFlag{
				Name: "upgrade",
				Usage: "Replace an already-installed mooncake of a different version. " +
					"Without this, version mismatch on the target errors out.",
			},
			&cli.BoolFlag{
				Name: "user",
				Usage: "Linux only: install the agentd as a user-scope systemd unit " +
					"running as the SSH user (binary in ~/.local/bin, unit in " +
					"~/.config/systemd/user/, token in ~/.config/mooncake/). " +
					"Default is a system-scope unit running as root. " +
					"Implies `loginctl enable-linger` so the unit survives logout.",
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

	// Empty when --binary is unset: the bootstrap layer resolves it lazily
	// from the ~/.mooncake/bin store (by detected target platform) right
	// before upload, falling back to a matching controller binary. Passing
	// it through empty keeps refresh-only runs working without a store.
	binPath := c.String("binary")

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
		ControllerVersion: Version,
		Upgrade:           c.Bool("upgrade"),
		AsUser:            c.Bool("user"),
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
			&cli.BoolFlag{
				Name:  "insecure-token-on-cmdline",
				Usage: "Opt-in to --token-via literal:<token>. The token is visible in shell history, ps output, and audit logs; prefer stdin or file:.",
			},
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
		fd := int(os.Stdin.Fd())
		if term.IsTerminal(fd) {
			raw, err := term.ReadPassword(fd)
			fmt.Fprintln(c.App.ErrWriter)
			if err != nil {
				return "", fmt.Errorf("read stdin: %w", err)
			}
			tok = strings.TrimSpace(string(raw))
		} else {
			if _, err := fmt.Fscanln(c.App.Reader, &tok); err != nil {
				return "", fmt.Errorf("read stdin: %w", err)
			}
			tok = strings.TrimSpace(tok)
		}
		if tok == "" {
			return "", errors.New("empty token from stdin")
		}
		return tok, nil
	case strings.HasPrefix(src, "file:"):
		path := strings.TrimPrefix(src, "file:")
		// F031(b): refuse to read a group/world-accessible token file.
		// Mirrors security.FilePasswordProvider's owner-only invariant
		// (post-F030, owner-only-via-bitmask rather than exact-0600).
		// A token in a 0644 file is functionally equivalent to a
		// world-readable password — same blast radius, same guard.
		info, statErr := os.Stat(path)
		if statErr != nil {
			return "", fmt.Errorf("stat token file %s: %w", path, statErr)
		}
		if info.Mode().Perm()&0o077 != 0 {
			return "", fmt.Errorf(
				"token file %s is group/world-accessible (mode %04o); chmod 600 or stricter",
				path, info.Mode().Perm(),
			)
		}
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
		// F031(a): require an explicit opt-in flag — mirrors the
		// --insecure-sudo-pass guard on --sudo-pass. A bearer token on
		// the command line lands in shell history, ps output, and
		// audit logs. Refuse silently-insecure usage.
		if !c.Bool("insecure-token-on-cmdline") {
			return "", errors.New(
				"--token-via literal:<token> requires --insecure-token-on-cmdline " +
					"(WARNING: token will be visible in shell history, ps output, and audit logs). " +
					"Prefer --token-via stdin or file:<path>.",
			)
		}
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
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name (`main_pc`), `key=value` filter (`tag=production`), or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml (or the machine name when invoked as `fleet apply <machine>`).",
			},
			&cli.Int64Flag{
				Name:  "max-sync-size",
				Usage: "Plan-dir cumulative size cap in bytes (default 100 MiB)",
				Value: 100 << 20,
			},
			&cli.StringFlag{
				Name: "plan-dir",
				Usage: "Root directory to sync to each peer (default: directory of the plan file). " +
					"Use this when the plan imports siblings via `../` — point at the repo root.",
			},
			&cli.StringSliceFlag{
				Name:  "vars-file",
				Usage: "Vars file to load (relative to plan-dir or absolute); may be repeated",
			},
			&cli.StringSliceFlag{
				Name: "step-filter",
				Usage: "Filter steps to run on each peer by `key=value` (v1: tag=<x>); " +
					"forwarded to daemon. Multiple values OR together. " +
					"Example: --step-filter tag=deploy",
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
//	    --peer    = main_pc (unless --peer given)
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

	// Load peers.toml. cmd owns the file path resolution; the orchestrator
	// consumes the resolved peer list.
	peersPath := c.String("peers-file")
	if peersPath == "" {
		var err error
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
		return cli.Exit(fleet.NoPeersConfiguredError(peersPath), 1)
	}

	// Fleet DX proposal-01: single unified --peer flag. When the machine
	// convention is invoked as `fleet apply <name>` with no --peer values,
	// the machine name becomes the default selector. We peek at planArg
	// here (the orchestrator re-detects the machine convention from the
	// same input, so the two views agree).
	planArg := c.Args().First()
	peerFlag := c.StringSlice("peer")
	if len(peerFlag) == 0 && isBareMachineArg(planArg) {
		peerFlag = []string{planArg}
	}

	// Spec-50 §Phase B: `os=` predicates require a /v1/version probe; build
	// the cache only when --peer references os=. cmd-side because the
	// resolver type and the transport-probe live next to filterTerm here.
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, cfgPeers.Peers, c.App.Writer)
	}

	sel, err := resolvePeers(cfgPeers.Peers, peerFlag, osFor)
	if err != nil {
		return cli.Exit("fleet apply: "+err.Error(), 2)
	}
	if len(sel.Matched) == 0 {
		return cli.Exit(fleet.NoPeersSelectedError(len(cfgPeers.Peers), sel.UnknownNames), 1)
	}

	// peerFilter adapts cmd-internal filterTerm/peerOSResolver into the
	// typeless predicate fleet.RunMachineApply consumes via the orchestrator.
	peerFilterGroups, _ := parsePeerFlags(peerFlag)
	var peerFilter func(fleet.Peer) bool
	if len(peerFilterGroups) > 0 {
		peerFilter = func(p fleet.Peer) bool {
			return peerMatchesFilters(p, peerFilterGroups, nil)
		}
	}

	stepTags, stepNames, err := extractStepFilter(c.StringSlice("step-filter"))
	if err != nil {
		return cli.Exit("fleet apply: "+err.Error(), 2)
	}

	cfg := &fleet.ApplyConfig{
		PlanArg:         planArg,
		PlanDirHint:     c.String("plan-dir"),
		PeersPath:       peersPath,
		SelectedPeers:   sel.Matched,
		UnknownPeers:    sel.UnknownNames,
		AllPeers:        cfgPeers.Peers,
		PeerFilter:      peerFilter,
		VarsFilesRel:    c.StringSlice("vars-file"),
		StepFilterTags:  stepTags,
		StepFilterNames: stepNames,
		MaxSyncBytes:    c.Int64("max-sync-size"),
		Parallel:        c.Int("parallel"),
		NoColor:         c.Bool("no-color"),
		Writer:          c.App.Writer,
	}

	// R2.1b: Orchestrator.Run returns a typed *KernelResult (the
	// fleet-scope kernel shape from vision/kernel.md). On failure with a
	// CLI exit code, err is a *fleet.ExitError carrying ExitCode +
	// Message; other errors flow through unchanged. The recap renderer
	// can consume result.Summary; we don't render anything here today
	// because RunApplyPhase already prints the per-peer ok/n line.
	_, err = fleet.NewOrchestrator(cfg).Run(c.Context)
	if err != nil {
		var exitErr *fleet.ExitError
		if errors.As(err, &exitErr) {
			return cli.Exit(exitErr.Message, exitErr.ExitCode)
		}
		return err
	}
	return nil
}

// isBareMachineArg reports whether planArg could be a bare machine name
// (the machine-convention shape: no slash, no .yml suffix). cmd uses this
// to decide whether to default --peer to the machine name; the orchestrator
// re-detects the convention from the same input.
func isBareMachineArg(planArg string) bool {
	return !strings.ContainsAny(planArg, "/\\") && !strings.HasSuffix(planArg, ".yml")
}

// filterTerm is a single `key=value` predicate inside a --peer or
// --step-filter expression. The parser is generic over keys; the
// allowlist is enforced by validatePeerFilterKeys.
type filterTerm struct {
	key   string
	value string
}

// peerFilterKeys is the allowlist enforced by validatePeerFilterKeys. Kept
// as a slice so the error message can render the keys in a stable order
// (spec-50 §G3: the unknown-key error lists the valid keys explicitly).
var peerFilterKeys = []string{"tag", "name", "os", "role"}

// validatePeerFilterKeys rejects any --peer key not in peerFilterKeys.
// Spec-50 extends the v1 allowlist (`tag`) with `name`, `os`, and `role`;
// the parser is already generic over keys, so this is a validator change
// only.
func validatePeerFilterKeys(groups [][]filterTerm) error {
	for _, g := range groups {
		for _, t := range g {
			if !isPeerFilterKey(t.key) {
				return fmt.Errorf(
					"unsupported --peer key %q (valid: %s)",
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
//
// Non-test code now drives the same decision off the raw --peer values
// via peerFlagsReferenceOSKey (avoids the parse step). This helper stays
// because fleet_filter_test.go pins the post-parse predicate semantics.
//
//nolint:unused // exercised by cmd/fleet_filter_test.go; lint runs with tests:false.
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
