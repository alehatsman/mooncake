package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetLogsCommand defines `mooncake fleet logs`.
//
// Two surfaces (spec-46):
//
//	mooncake fleet logs <peer>           # attach to latest run on peer
//	mooncake fleet logs <peer> <run_id>  # attach to a specific run id
//	mooncake fleet logs --all            # multiplex latest runs across all peers
//
// Out of scope for v1 (deferred per top-5 priorities): --follow, --since.
func fleetLogsCommand() *cli.Command {
	return &cli.Command{
		Name:      "logs",
		Usage:     "Attach to a peer's run logs (latest in-flight or terminal)",
		ArgsUsage: "<peer> [run_id] | --all",
		Description: "Streams a peer's most recent run via SSE. Picks the most " +
			"recent in-flight run if one exists; otherwise the most recent " +
			"terminal run (which replays its events and closes). With --all, " +
			"opens one stream per peer in peers.toml and multiplexes their " +
			"events into a single `[host]`-prefixed stdout stream.",
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:  "all",
				Usage: "Attach to every peer's latest run, multiplexed",
			},
			&cli.StringFlag{
				Name:  "peers",
				Usage: "Comma-separated peer names to target (only with --all)",
			},
			&cli.StringFlag{
				Name:  "peers-file",
				Usage: "Override the peers.toml path",
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable ANSI colors in the [host] prefix (also honors NO_COLOR env)",
			},
		},
		Action: fleetLogsAction,
	}
}

func fleetLogsAction(c *cli.Context) error {
	all := c.Bool("all")
	args := c.Args().Slice()

	if all && len(args) > 0 {
		return cli.Exit("fleet logs: --all takes no positional args", 2)
	}
	if !all && (len(args) < 1 || len(args) > 2) {
		return cli.Exit("fleet logs: expected <peer> [run_id], or --all", 2)
	}

	peersPath := c.String("peers-file")
	if peersPath == "" {
		var err error
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
		return cli.Exit("fleet logs: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}

	w := c.App.Writer

	// Single-peer mode: validate the named peer exists, then stream its one run.
	if !all {
		peerName := args[0]
		runID := ""
		if len(args) == 2 {
			runID = args[1]
		}
		peer, ok := findPeer(cfg.Peers, peerName)
		if !ok {
			return cli.Exit(fmt.Sprintf("fleet logs: peer %q not found in %s", peerName, peersPath), 1)
		}
		if peer.Transport != fleet.TransportAgentd {
			return cli.Exit(fmt.Sprintf("fleet logs: peer %q transport %q is not agentd (only agentd peers stream live logs)", peer.Name, peer.Transport), 1)
		}
		return streamPeers(c.Context, w, []fleet.Peer{peer}, map[string]string{peer.Name: runID}, c.Bool("no-color"))
	}

	// --all: resolve peer filter (subset by --peers if given) and stream every selected agentd peer.
	selected, unknown := selectPeers(cfg.Peers, c.String("peers"))
	if len(selected) == 0 {
		return cli.Exit("fleet logs: no peers matched filter", 1)
	}
	agentdPeers := make([]fleet.Peer, 0, len(selected))
	for _, p := range selected {
		if p.Transport == fleet.TransportAgentd {
			agentdPeers = append(agentdPeers, p)
		}
	}
	if len(agentdPeers) == 0 {
		return cli.Exit("fleet logs: no agentd-transport peers selected", 1)
	}
	if len(unknown) > 0 {
		fmt.Fprintln(c.App.ErrWriter, "warning: unknown peer name(s): "+strings.Join(unknown, ", "))
	}
	// runIDs nil/empty for every peer → resolve latest per-peer.
	return streamPeers(c.Context, w, agentdPeers, nil, c.Bool("no-color"))
}

// streamPeers opens one SSE stream per peer in parallel and feeds the events
// into a single Multiplexer. runIDs is an optional name→run_id override; if
// a peer is absent from the map (or the entry is ""), resolveLatestRun is
// called for that peer.
//
// First ^C cancels the local context (mirrors apply's two-^C pattern); a
// second ^C hard-exits. Remote runs continue regardless — same caveat the
// apply banner explains.
func streamPeers(ctx context.Context, w io.Writer, peers []fleet.Peer, runIDs map[string]string, noColor bool) error {
	peerNames := make([]string, 0, len(peers))
	for _, p := range peers {
		peerNames = append(peerNames, p.Name)
	}
	useColor := fleet.ShouldColor(w, noColor)
	mux := fleet.NewMultiplexer(w, peerNames, useColor)
	mux.Banner(fmt.Sprintf("fleet logs: %d peer(s)", len(peers)))

	events := make(chan fleet.PeerEvent, 64*len(peers))
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	streamCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(sigCh)
	go func() {
		select {
		case <-sigCh:
			mux.Banner("⚠ ^C closes the log stream(s) only — remote runs continue.")
			cancel()
			select {
			case <-sigCh:
				os.Exit(130)
			case <-streamCtx.Done():
			}
		case <-streamCtx.Done():
		}
	}()

	errs := make([]error, len(peers))
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			runID := ""
			if runIDs != nil {
				runID = runIDs[p.Name]
			}
			errs[i] = streamOnePeer(streamCtx, p, runID, events)
		}(i, p)
	}
	wg.Wait()
	close(events)
	<-drained

	for _, e := range errs {
		if e != nil && !errors.Is(e, context.Canceled) {
			return cli.Exit("fleet logs: "+oneLineErr(e.Error()), 1)
		}
	}
	return nil
}

// streamOnePeer drives one peer's logs cycle: latest-run resolution (if
// runID empty) → SSE stream → emit into events. Mirrors apply.Apply's
// emit-control-events-on-error pattern so the multiplexer renders peer
// failures inline with peer events.
func streamOnePeer(ctx context.Context, peer fleet.Peer, runID string, events chan<- fleet.PeerEvent) error {
	client := transport.New(peer.Name, peer.Addr, peer.Token)

	if runID == "" {
		rid, err := resolveLatestRun(ctx, client)
		if err != nil {
			events <- fleet.PeerEvent{Peer: peer.Name, Kind: fleet.KindError, Message: "resolve latest run: " + oneLineErr(err.Error())}
			return err
		}
		runID = rid
	}
	events <- fleet.PeerEvent{Peer: peer.Name, Kind: fleet.KindSubmit, Message: "attached to run " + runID}

	sink := make(chan transport.Event, 64)
	streamErrCh := make(chan error, 1)
	go func() { streamErrCh <- client.Stream(ctx, runID, sink) }()

	for {
		select {
		case <-ctx.Done():
			<-streamErrCh
			drainPeerEvents(sink, peer.Name, events)
			return ctx.Err()
		case ev := <-sink:
			events <- fleet.PeerEvent{Peer: peer.Name, Kind: fleet.KindEvent, Event: ev}
		case err := <-streamErrCh:
			drainPeerEvents(sink, peer.Name, events)
			if err != nil {
				events <- fleet.PeerEvent{Peer: peer.Name, Kind: fleet.KindError, Message: "stream: " + oneLineErr(err.Error())}
			}
			return err
		}
	}
}

func drainPeerEvents(sink <-chan transport.Event, peerName string, events chan<- fleet.PeerEvent) {
	for {
		select {
		case ev := <-sink:
			events <- fleet.PeerEvent{Peer: peerName, Kind: fleet.KindEvent, Event: ev}
		default:
			return
		}
	}
}

// resolveLatestRun returns the run-id the user almost certainly wants: an
// in-flight run if one exists, otherwise the most recent terminal run.
// Returns an error only when ListRuns fails or the peer has no runs at all
// — both are user-actionable.
func resolveLatestRun(ctx context.Context, client *transport.Client) (string, error) {
	runs, err := client.ListRuns(ctx, 5)
	if err != nil {
		return "", err
	}
	if len(runs) == 0 {
		return "", errors.New("no runs recorded on this peer")
	}
	for _, r := range runs {
		if isRunInFlight(r.Status) {
			return r.ID, nil
		}
	}
	return runs[0].ID, nil
}

// isRunInFlight knows the daemon's run-status vocabulary. "queued" /
// "running" are pre-terminal; everything else is terminal.
//
// Source of truth: internal/agentd/store.go. Keep this in lockstep when
// the daemon adds new statuses.
func isRunInFlight(status string) bool {
	switch status {
	case "queued", "running":
		return true
	}
	return false
}

// findPeer is the by-name lookup the single-peer `fleet logs <peer>` form
// needs. Distinct from selectPeers (which is comma-separated and tolerant
// of unknown names — wrong shape for "I named one peer and it must exist").
func findPeer(peers []fleet.Peer, name string) (fleet.Peer, bool) {
	for _, p := range peers {
		if p.Name == name {
			return p, true
		}
	}
	return fleet.Peer{}, false
}

