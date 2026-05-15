package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/exec"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetExecCommand registers `mooncake fleet exec` (spec-52). The
// shape mirrors `fleet apply` where overlap makes sense; new flags
// (`--env`, `--cwd`, `--timeout`, `--become`, `--shell`) map onto
// existing fields on the kernel's ShellAction and Step types.
func fleetExecCommand() *cli.Command {
	return &cli.Command{
		Name:      "exec",
		Usage:     "Run an ad-hoc shell command on one or more fleet peers",
		ArgsUsage: "'<command>' | -- <command...>",
		Description: "Fan-out a single shell command across selected peers. The " +
			"command runs through the kernel's `shell` action — `$VAR` is " +
			"expanded by the peer's shell, NOT the controller's. " +
			"Use `--` to separate flags from a multi-arg command form; the " +
			"args are joined with single spaces before dispatch. " +
			"For --json output one JSONL record per peer is emitted; pair " +
			"with jq for scripting.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name, `key=value` filter (`tag=production`), or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml.",
			},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
			&cli.IntFlag{Name: "parallel", Usage: "Max peers in flight (0 = unbounded)", Value: 0},
			&cli.StringSliceFlag{Name: "env", Usage: "KEY=VAL forwarded to the shell step (repeatable)"},
			&cli.StringFlag{Name: "cwd", Usage: "Working directory on the peer"},
			&cli.StringFlag{Name: "timeout", Usage: "Per-peer wall clock (e.g. 30s, 2m); enforced by the kernel"},
			&cli.BoolFlag{
				Name:  "become",
				Usage: "Run with sudo on Unix peers (maps to as_user: root). " +
					"--ask-become-pass is deferred to spec-47.",
			},
			&cli.StringFlag{
				Name:  "shell",
				Usage: "Override the default interpreter (bash, zsh, pwsh, powershell, cmd, ...)",
			},
			&cli.BoolFlag{Name: "no-color", Usage: "Disable ANSI colors in the [peer] prefix"},
			&cli.BoolFlag{Name: "json", Usage: "Emit one JSONL record per peer instead of multiplexed lines"},
		},
		Action: fleetExecAction,
	}
}

// fleetExecAction parses argv, resolves peers, synthesizes the plan,
// and dispatches to the multiplex or JSON driver.
func fleetExecAction(c *cli.Context) error {
	cmdString, err := joinCommandArgs(c.Args().Slice())
	if err != nil {
		return cli.Exit("fleet exec: "+err.Error(), 2)
	}

	envMap, err := parseEnvFlags(c.StringSlice("env"))
	if err != nil {
		return cli.Exit("fleet exec: "+err.Error(), 2)
	}

	planBytes, err := exec.Synthesize(exec.SynthOptions{
		Cmd:         cmdString,
		Interpreter: c.String("shell"),
		Env:         envMap,
		Cwd:         c.String("cwd"),
		Timeout:     c.String("timeout"),
		Become:      c.Bool("become"),
	})
	if err != nil {
		return cli.Exit("fleet exec: "+err.Error(), 2)
	}

	peers, err := resolveExecPeers(c)
	if err != nil {
		return err
	}

	controllerID, err := fleet.EnsureControllerID()
	if err != nil {
		return fmt.Errorf("controller id: %w", err)
	}

	w := c.App.Writer
	if c.Bool("json") {
		return runExecJSON(c.Context, runExecInputs{
			Peers:        peers,
			PlanBytes:    planBytes,
			ControllerID: controllerID,
			Parallel:     c.Int("parallel"),
			Writer:       w,
		})
	}
	return runExecMultiplex(c.Context, runExecInputs{
		Peers:        peers,
		PlanBytes:    planBytes,
		ControllerID: controllerID,
		Parallel:     c.Int("parallel"),
		Writer:       w,
		UseColor:     fleet.ShouldColor(w, c.Bool("no-color")),
		CommandLine:  cmdString,
	})
}

// runExecInputs is the resolved set of parameters that both driver
// implementations consume. Lifted into one type so the two functions
// don't drift on argument order.
type runExecInputs struct {
	Peers        []execPeerEntry
	PlanBytes    []byte
	ControllerID string
	Parallel     int
	Writer       io.Writer
	UseColor     bool
	CommandLine  string // banner string; only used by multiplex mode
}

// execPeerEntry is the cmd-layer descriptor; one of these per agentd
// peer that survived selection + filter + transport gating.
type execPeerEntry struct {
	Peer   fleet.Peer
	Client *transport.Client
}

// resolveExecPeers loads peers.toml, applies --peer, and rejects
// non-agentd transports with a banner. Same selector pipeline as
// fleet apply.
func resolveExecPeers(c *cli.Context) ([]execPeerEntry, error) {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		p, err := fleet.DefaultPeersPath()
		if err != nil {
			return nil, err
		}
		peersPath = p
	}
	cfgPeers, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return nil, err
	}
	if len(cfgPeers.Peers) == 0 {
		return nil, cli.Exit("fleet exec: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	// Fleet DX proposal-01: single unified --peer flag.
	peerFlag := c.StringSlice("peer")
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, cfgPeers.Peers, c.App.Writer)
	}
	sel, err := resolvePeers(cfgPeers.Peers, peerFlag, osFor)
	if err != nil {
		return nil, cli.Exit("fleet exec: "+err.Error(), 2)
	}
	selected, unknown := sel.Matched, sel.UnknownNames
	if len(selected) == 0 {
		msg := "fleet exec: --peer selected 0 of " + strconv.Itoa(len(cfgPeers.Peers)) + " peer(s)"
		if len(unknown) > 0 {
			msg += " (unknown: " + strings.Join(unknown, ", ") + ")"
		}
		return nil, cli.Exit(msg, 1)
	}

	// Skip non-agentd transports with a banner. v1: SSH-bootstrap peers
	// are not supported as an ongoing operating channel.
	var entries []execPeerEntry
	var skipped []string
	for _, p := range selected {
		if p.Transport != fleet.TransportAgentd {
			skipped = append(skipped, p.Name)
			continue
		}
		entries = append(entries, execPeerEntry{Peer: p, Client: transport.New(p.Name, p.Addr, p.Token)})
	}
	if len(skipped) > 0 {
		fmt.Fprintf(c.App.Writer, "fleet exec: skipped non-agentd peers (transport unsupported): %s\n",
			strings.Join(skipped, ", "))
	}
	if len(entries) == 0 {
		return nil, cli.Exit("fleet exec: no agentd peers remain after filters", 1)
	}
	return entries, nil
}

// runExecMultiplex is the default output path. Spawns a Multiplexer,
// drives `exec.Exec`, and prints a summary banner + exits with the
// aggregate code.
func runExecMultiplex(ctx context.Context, in runExecInputs) error {
	peerNames := make([]string, len(in.Peers))
	execPeers := make([]exec.ExecPeer, len(in.Peers))
	for i, p := range in.Peers {
		peerNames[i] = p.Peer.Name
		execPeers[i] = exec.ExecPeer{Name: p.Peer.Name, Client: p.Client}
	}
	mux := fleet.NewMultiplexer(in.Writer, peerNames, in.UseColor)

	mux.Banner(fmt.Sprintf("fleet exec: %d peer(s), command = %q", len(in.Peers), in.CommandLine))

	events := make(chan fleet.PeerEvent, 64)
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	execCtx, cancel := installSigintCancel(ctx, in.Writer)
	defer cancel()

	outs, runErr := exec.Exec(execCtx, exec.ExecOptions{
		Peers:        execPeers,
		PlanBytes:    in.PlanBytes,
		ControllerID: in.ControllerID,
		Parallel:     in.Parallel,
		Events:       events,
	})
	close(events)
	<-drained

	mux.Banner(exec.Summary(outs))

	if runErr != nil {
		return cli.Exit(runErr, 2)
	}
	if code := exec.ExitCode(outs); code != 0 {
		return cli.Exit("", code)
	}
	return nil
}

// runExecJSON emits one JSONL record per peer in input order to
// stdout. No multiplexer.
func runExecJSON(ctx context.Context, in runExecInputs) error {
	execPeers := make([]exec.ExecPeer, len(in.Peers))
	for i, p := range in.Peers {
		execPeers[i] = exec.ExecPeer{Name: p.Peer.Name, Client: p.Client}
	}

	execCtx, cancel := installSigintCancel(ctx, in.Writer)
	defer cancel()

	outs, runErr := exec.Exec(execCtx, exec.ExecOptions{
		Peers:         execPeers,
		PlanBytes:     in.PlanBytes,
		ControllerID:  in.ControllerID,
		Parallel:      in.Parallel,
		CollectOutput: true,
		// Events: nil → CollectOutput accumulates everything; we serialize after.
	})

	enc := json.NewEncoder(in.Writer)
	for _, o := range outs {
		_ = enc.Encode(o)
	}

	if runErr != nil {
		return cli.Exit(runErr, 2)
	}
	if code := exec.ExitCode(outs); code != 0 {
		return cli.Exit("", code)
	}
	return nil
}

// joinCommandArgs collapses CLI args into the single shell-cmd string
// expected by ShellAction.Cmd. Empty arg lists are rejected.
//
// Two argument forms, per spec-52 §CLI shape:
//   - Single-string form (one arg): pass it through verbatim.
//   - `--`-separator form: urfave/cli yields each remaining arg as a
//     separate slice entry; join them with single spaces.
func joinCommandArgs(args []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("missing command. Examples:\n" +
			"    mooncake fleet exec 'systemctl is-active sshd'\n" +
			"    mooncake fleet exec -- df -h /")
	}
	if len(args) == 1 {
		return args[0], nil
	}
	return strings.Join(args, " "), nil
}

// parseEnvFlags turns a slice of KEY=VAL strings into a map. Returns
// an error on missing '=' or empty KEY. Empty value is allowed.
func parseEnvFlags(items []string) (map[string]string, error) {
	if len(items) == 0 {
		return nil, nil
	}
	out := make(map[string]string, len(items))
	for _, it := range items {
		eq := strings.IndexByte(it, '=')
		if eq <= 0 {
			return nil, fmt.Errorf("--env %q: expected KEY=VAL with non-empty KEY", it)
		}
		out[it[:eq]] = it[eq+1:]
	}
	return out, nil
}

// installSigintCancel mirrors fleet apply's SIGINT handler shape:
// first ^C cancels the local context and prints a banner; second ^C
// hard-exits 130. Returns (ctx, cancel) — caller defers cancel().
func installSigintCancel(parent context.Context, w io.Writer) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigCh)
		select {
		case <-sigCh:
			fmt.Fprintln(w, "⚠ ^C closes the log stream only — remote runs continue.")
			fmt.Fprintln(w, "  See `mooncake fleet logs <host>` to reattach.")
			cancel()
			select {
			case <-sigCh:
				os.Exit(130)
			case <-ctx.Done():
			}
		case <-ctx.Done():
		}
	}()
	return ctx, cancel
}

