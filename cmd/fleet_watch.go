package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// fleetWatchCommand registers `mooncake fleet watch` (spec-53). Opens a
// long-lived multiplexed event stream across selected peers; new runs
// appear as they start, no restart needed. v1 architecture: per-peer
// goroutine that polls /v1/runs?status=running and attaches via the
// existing per-run SSE — no daemon-side change.
func fleetWatchCommand() *cli.Command {
	return &cli.Command{
		Name:        "watch",
		Usage:       "Stream live events from every peer; `tail -f` for the fleet",
		Description: "Subscribes to every selected peer's in-flight runs and surfaces events as they happen. New runs that start later appear without re-running the command. ^C exits cleanly; remote runs continue.",
		Flags: []cli.Flag{
			&cli.StringSliceFlag{
				Name:  "peer",
				Usage: "Select peers: repeat to UNION. Each value is a name, `key=value` filter (`tag=production`), or `@k=v,k2=v2` AND-group. Default: every peer in peers.toml.",
			},
			&cli.StringFlag{Name: "peers-file", Usage: "Override the peers.toml path"},
			&cli.DurationFlag{
				Name:  "poll-interval",
				Usage: "How often a peer is polled for new in-flight runs (jittered ±25%)",
				Value: 2 * time.Second,
			},
			&cli.BoolFlag{Name: "no-color", Usage: "Disable ANSI colors in the [peer] prefix"},
			&cli.BoolFlag{Name: "json", Usage: "Emit one JSONL record per event instead of multiplexed lines"},
		},
		Action: fleetWatchAction,
	}
}

func fleetWatchAction(c *cli.Context) error {
	peers, err := loadWatchPeers(c)
	if err != nil {
		return err
	}
	if c.Bool("json") {
		return runWatchJSON(c.Context, c.App.Writer, peers, c.Duration("poll-interval"))
	}
	return runWatchMultiplex(c.Context, c.App.Writer, peers, c.Duration("poll-interval"), c.Bool("no-color"))
}

// loadWatchPeers shares the peer selection pipeline with fleet apply /
// fleet ps / fleet exec. Non-agentd transports are skipped with a one-
// line warning to stderr (mirrors spec-46's `fleet logs --all` shape).
func loadWatchPeers(c *cli.Context) ([]fleet.Peer, error) {
	peersPath := c.String("peers-file")
	if peersPath == "" {
		p, err := fleet.DefaultPeersPath()
		if err != nil {
			return nil, err
		}
		peersPath = p
	}
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		return nil, err
	}
	if len(cfg.Peers) == 0 {
		return nil, cli.Exit("fleet watch: no peers configured. Run `mooncake fleet bootstrap` or edit "+peersPath, 1)
	}
	// Fleet DX proposal-01: single unified --peer flag.
	peerFlag := c.StringSlice("peer")
	var osFor peerOSResolver
	if peerFlagsReferenceOSKey(peerFlag) {
		osFor = newPeerOSCache(c.Context, cfg.Peers, c.App.Writer)
	}
	sel, err := resolvePeers(cfg.Peers, peerFlag, osFor)
	if err != nil {
		return nil, cli.Exit("fleet watch: "+err.Error(), 2)
	}
	selected, unknown := sel.Matched, sel.UnknownNames
	if len(unknown) > 0 {
		fmt.Fprintf(c.App.ErrWriter, "fleet watch: unknown peer name(s): %s\n", strings.Join(unknown, ", "))
	}
	if len(selected) == 0 {
		return nil, cli.Exit("fleet watch: --peer selected 0 peers", 1)
	}
	out := make([]fleet.Peer, 0, len(selected))
	var skipped []string
	for _, p := range selected {
		if p.Transport == fleet.TransportAgentd {
			out = append(out, p)
		} else {
			skipped = append(skipped, p.Name)
		}
	}
	if len(skipped) > 0 {
		fmt.Fprintf(c.App.ErrWriter, "fleet watch: skipping non-agentd peer(s): %s\n", strings.Join(skipped, ", "))
	}
	if len(out) == 0 {
		return nil, cli.Exit("fleet watch: no agentd peers after filters", 1)
	}
	return out, nil
}

// runWatchMultiplex drives the multiplexed output path. One goroutine
// per peer; events fan into a shared channel; fleet.Multiplexer drains
// to stdout with the standard [peer] prefix.
func runWatchMultiplex(ctx context.Context, w io.Writer, peers []fleet.Peer, pollInterval time.Duration, noColor bool) error {
	names := make([]string, len(peers))
	for i, p := range peers {
		names[i] = p.Name
	}
	useColor := fleet.ShouldColor(w, noColor)
	mux := fleet.NewMultiplexer(w, names, useColor)
	mux.Banner(fmt.Sprintf("fleet watch: %d peer(s)", len(peers)))

	events := make(chan fleet.PeerEvent, 64*len(peers))
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	watchCtx, cancel := installWatchSigint(ctx, mux)
	defer cancel()
	runPeerLoops(watchCtx, peers, pollInterval, events)
	close(events)
	<-drained

	mux.Banner("fleet watch: stopped.")
	return nil
}

// runWatchJSON drives the --json output path. No multiplexer; each
// event is marshalled to one JSON line. Control events (attach,
// disconnect, reattach, error) surface as {"kind":...} objects so
// downstream consumers can distinguish them from real SSE events.
func runWatchJSON(ctx context.Context, w io.Writer, peers []fleet.Peer, pollInterval time.Duration) error {
	events := make(chan fleet.PeerEvent, 64*len(peers))
	drained := make(chan struct{})
	go func() {
		emitJSONEvents(w, events)
		close(drained)
	}()

	watchCtx, cancel := installWatchSigint(ctx, nil)
	defer cancel()
	runPeerLoops(watchCtx, peers, pollInterval, events)
	close(events)
	<-drained

	return nil
}

// runPeerLoops spawns one watchOnePeer goroutine per peer and blocks
// until they all return (typically via ctx cancellation).
func runPeerLoops(ctx context.Context, peers []fleet.Peer, pollInterval time.Duration, events chan<- fleet.PeerEvent) {
	var wg sync.WaitGroup
	for _, p := range peers {
		p := p
		wg.Add(1)
		go func() {
			defer wg.Done()
			watchOnePeer(ctx, p, pollInterval, events)
		}()
	}
	wg.Wait()
}

// watchOnePeer is the per-peer state machine described in spec-53
// §"Per-peer state machine". POLLING: list runs every poll-interval;
// when a queued/running run appears that we haven't attached to yet,
// transition to ATTACHED. ATTACHED: stream that run's events until the
// SSE closes; then back to POLLING. On any error, back off (500ms →
// 8s, ±25% jitter) before retrying.
//
// Returns when ctx is canceled. Errors are inlined as PeerEvents so the
// command stays running on per-peer failure.
func watchOnePeer(ctx context.Context, peer fleet.Peer, pollInterval time.Duration, events chan<- fleet.PeerEvent) {
	client := transport.New(peer.Name, peer.Addr, peer.Token)
	attached := make(map[string]struct{})
	backoff := newBackoff(500*time.Millisecond, 8*time.Second)

	for {
		if ctx.Err() != nil {
			return
		}
		runID, err := pickInFlightRun(ctx, client, attached)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			emit(events, peer.Name, fleet.KindError, "list runs: "+oneLineErr(err.Error()))
			if !sleepWithCtx(ctx, backoff.next()) {
				return
			}
			continue
		}
		if runID == "" {
			backoff.reset()
			if !sleepWithCtx(ctx, jittered(pollInterval)) {
				return
			}
			continue
		}
		attached[runID] = struct{}{}
		backoff.reset()
		emit(events, peer.Name, fleet.KindSubmit, "attached to run "+runID)
		streamErr := streamWatchRunEvents(ctx, client, peer.Name, runID, events)
		if errors.Is(streamErr, context.Canceled) {
			return
		}
		if streamErr != nil {
			emit(events, peer.Name, fleet.KindDisconnect, oneLineErr(streamErr.Error()))
			if !sleepWithCtx(ctx, backoff.next()) {
				return
			}
		}
	}
}

// pickInFlightRun lists the peer's running runs and returns the ID of
// the first one we haven't already attached to. Empty string means
// nothing new — caller sleeps and re-polls.
func pickInFlightRun(ctx context.Context, client *transport.Client, attached map[string]struct{}) (string, error) {
	runs, err := client.ListRunsWith(ctx, transport.ListRunsOpts{Status: "running"})
	if err != nil {
		return "", err
	}
	for _, r := range runs {
		if _, seen := attached[r.ID]; !seen {
			return r.ID, nil
		}
	}
	return "", nil
}

// streamWatchRunEvents wraps Client.Stream, forwarding each SSE event to
// the events channel as a KindEvent. Returns nil on clean close.
func streamWatchRunEvents(ctx context.Context, client *transport.Client, peerName, runID string, events chan<- fleet.PeerEvent) error {
	sink := make(chan transport.Event, 64)
	errCh := make(chan error, 1)
	go func() { errCh <- client.Stream(ctx, runID, sink) }()
	for {
		select {
		case <-ctx.Done():
			<-errCh
			drainStreamSink(sink, peerName, events)
			return ctx.Err()
		case ev := <-sink:
			events <- fleet.PeerEvent{Peer: peerName, Kind: fleet.KindEvent, Event: ev}
		case err := <-errCh:
			drainStreamSink(sink, peerName, events)
			return err
		}
	}
}

// drainStreamSink flushes whatever's still buffered when the SSE goroutine
// finishes so we don't drop terminal events between the select branches.
func drainStreamSink(sink <-chan transport.Event, peer string, events chan<- fleet.PeerEvent) {
	for {
		select {
		case ev := <-sink:
			events <- fleet.PeerEvent{Peer: peer, Kind: fleet.KindEvent, Event: ev}
		default:
			return
		}
	}
}

// emit is a tiny helper to keep the per-peer goroutine readable. The
// events channel is never closed from inside the goroutine; the parent
// closes it once after wg.Wait().
func emit(events chan<- fleet.PeerEvent, peer string, kind fleet.PeerEventKind, msg string) {
	events <- fleet.PeerEvent{Peer: peer, Kind: kind, Message: msg}
}

// emitJSONEvents marshals each PeerEvent as a JSONL line. Mirrors the
// shape spelled out in spec-53 §"Output": KindEvent expands to a row
// with peer + run_id + seq + type + timestamp + data; control events
// surface as {kind: attached|disconnected|reattach|error, …}.
func emitJSONEvents(w io.Writer, events <-chan fleet.PeerEvent) {
	enc := json.NewEncoder(w)
	for ev := range events {
		var record map[string]any
		switch ev.Kind {
		case fleet.KindEvent:
			record = map[string]any{
				"kind":      "event",
				"peer":      ev.Peer,
				"seq":       ev.Event.Seq,
				"type":      ev.Event.Type,
				"timestamp": ev.Event.Timestamp,
				"data":      ev.Event.Data,
			}
		case fleet.KindSubmit:
			record = map[string]any{
				"kind":    "attached",
				"peer":    ev.Peer,
				"message": ev.Message,
			}
		case fleet.KindDisconnect:
			record = map[string]any{
				"kind":    "disconnected",
				"peer":    ev.Peer,
				"message": ev.Message,
			}
		case fleet.KindError:
			record = map[string]any{
				"kind":    "error",
				"peer":    ev.Peer,
				"message": ev.Message,
			}
		default:
			continue
		}
		_ = enc.Encode(record)
	}
}

// installWatchSigint mirrors fleet logs / fleet apply's two-^C pattern.
// First signal cancels the context (streams close, goroutines drain);
// second signal hard-exits 130 in case shutdown stalls.
func installWatchSigint(parent context.Context, mux *fleet.Multiplexer) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(parent)
	sigCh := make(chan os.Signal, 2)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	go func() {
		defer signal.Stop(sigCh)
		select {
		case <-sigCh:
			if mux != nil {
				mux.Banner("⚠ ^C closes the watch stream(s) — remote runs continue.")
			}
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

// backoff is a tiny exponential backoff helper bounded by max with
// ±25% jitter. Reset whenever a successful operation lands so a peer
// that flapped doesn't stay in the long-tail forever.
type backoff struct {
	base, cap, current time.Duration
}

func newBackoff(base, ceil time.Duration) *backoff {
	return &backoff{base: base, cap: ceil, current: base}
}

func (b *backoff) next() time.Duration {
	d := jittered(b.current)
	if b.current < b.cap {
		b.current *= 2
		if b.current > b.cap {
			b.current = b.cap
		}
	}
	return d
}

func (b *backoff) reset() { b.current = b.base }

// jittered returns d ±25%, useful for both backoff and the steady-
// state poll interval (so an N-peer fleet doesn't pulse the daemon
// in lockstep).
func jittered(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	delta := float64(d) * 0.25
	// math/rand is fine for jitter; not a security primitive.
	offset := (rand.Float64()*2 - 1) * delta // #nosec G404
	out := time.Duration(float64(d) + offset)
	if out < 0 {
		out = 0
	}
	return out
}

// sleepWithCtx returns false when ctx is canceled before d elapses,
// true on a clean sleep. Bare time.Sleep would block past cancellation,
// stranding the per-peer goroutine.
func sleepWithCtx(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return ctx.Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}
