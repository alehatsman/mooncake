package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// ApplyOptions configures one peer's apply cycle: sync the plan-dir, submit
// a run, stream events. See Apply.
type ApplyOptions struct {
	// PeerName is the display label used in the `[peer]` log prefix.
	PeerName string

	// Peer is the transport client to drive.
	Peer *transport.Client

	// PlanDir is the absolute path on the controller of the directory
	// being synced. Used to compute the sync scope.
	PlanDir string

	// PlanPath is the absolute path on the controller of the top-level
	// YAML. Must be inside PlanDir.
	PlanPath string

	// VarsFiles are absolute paths on the controller; each must also be
	// inside PlanDir (so they get synced as part of the tree).
	VarsFiles []string

	// Tags are forwarded to the daemon's run-submit request.
	Tags []string

	// ControllerID is the UUID minted by EnsureControllerID — drives the
	// scope key. Caller is responsible for providing it.
	ControllerID string

	// MaxSyncBytes caps the cumulative plan-dir size enforced by Walk. The
	// daemon enforces a separate per-file cap.
	MaxSyncBytes int64

	// Events, when non-nil, receives one PeerEvent per streamed SSE event
	// plus control events at sync/submit/disconnect/error boundaries. The
	// caller (typically a Multiplexer) owns the channel and is responsible
	// for closing it after every concurrent producer has returned. When
	// Events is set, Writer is ignored.
	Events chan<- PeerEvent

	// Writer is the legacy single-peer rendering path: Apply writes
	// `[peer] message` lines directly. Used by tests and callers that
	// don't want the multiplexer indirection. Ignored when Events is set.
	// A nil writer (with nil Events) discards rendered output.
	Writer io.Writer
}

// ApplyResult is the outcome of one peer's apply cycle.
type ApplyResult struct {
	PeerName string
	RunID    string
	Sync     SyncStats
	Status   string // run terminal status; empty if Apply errored before submit
	Events   int    // number of events streamed
}

// Apply runs the full sync → submit → stream cycle against one peer.
//
// Steps:
//  1. Walk PlanDir, enforcing MaxSyncBytes.
//  2. GetVersion to learn the peer's SyncedRoot.
//  3. SyncTo with HEAD-skip + PUT loop.
//  4. Translate PlanPath / VarsFiles / PlanDir to peer-side absolute paths
//     under the synced scope.
//  5. Submit the run.
//  6. Stream events, emitting them via Events or Writer until the daemon
//     closes the stream (run reached terminal state) or ctx is canceled.
//
// Returns the per-peer result; the run's terminal status is reported via
// Result.Status when a run.completed event was seen. Returns the first
// non-nil error from any step; partial results are still populated where
// useful (e.g. Sync stats even when Submit fails).
func Apply(ctx context.Context, opts ApplyOptions) (ApplyResult, error) {
	result := ApplyResult{PeerName: opts.PeerName}
	emit := makeEmitter(opts)

	if err := validateApplyPaths(opts); err != nil {
		return result, err
	}
	scope, err := ScopeFor(opts.ControllerID, opts.PlanDir)
	if err != nil {
		return result, err
	}

	entries, _, err := Walk(opts.PlanDir, opts.MaxSyncBytes)
	if err != nil {
		err = fmt.Errorf("walk plan-dir: %w", err)
		emit(PeerEvent{Kind: KindError, Message: err.Error()})
		return result, err
	}

	ver, err := opts.Peer.GetVersion(ctx)
	if err != nil {
		emit(PeerEvent{Kind: KindError, Message: "version probe: " + oneLine(err.Error())})
		return result, err
	}

	syncStats, err := SyncTo(ctx, opts.Peer, entries, scope)
	result.Sync = syncStats
	if err != nil {
		err = fmt.Errorf("sync: %w", err)
		emit(PeerEvent{Kind: KindError, Message: oneLine(err.Error())})
		return result, err
	}
	emit(PeerEvent{Kind: KindSync, Message: fmt.Sprintf(
		"sync: %d uploaded, %d skipped (%d bytes total)",
		syncStats.Put, syncStats.Skipped, syncStats.BytesTotal)})

	// Translate controller-side absolute paths to peer-side absolute paths
	// under the synced scope.
	planRel, err := filepath.Rel(opts.PlanDir, opts.PlanPath)
	if err != nil {
		return result, fmt.Errorf("rel plan_path: %w", err)
	}
	peerPlanPath := PeerPath(ver.SyncedRoot, scope, filepath.ToSlash(planRel))

	var peerVars []string
	for _, vp := range opts.VarsFiles {
		varRel, err := filepath.Rel(opts.PlanDir, vp)
		if err != nil {
			return result, fmt.Errorf("rel vars %s: %w", vp, err)
		}
		peerVars = append(peerVars, PeerPath(ver.SyncedRoot, scope, filepath.ToSlash(varRel)))
	}
	peerBase := PeerPath(ver.SyncedRoot, scope, "")

	runID, err := opts.Peer.Submit(ctx, transport.SubmitRequest{
		PlanPath:  peerPlanPath,
		VarsFiles: peerVars,
		Tags:      opts.Tags,
		BaseDir:   peerBase,
	})
	if err != nil {
		emit(PeerEvent{Kind: KindError, Message: "submit: " + oneLine(err.Error())})
		return result, err
	}
	result.RunID = runID
	emit(PeerEvent{Kind: KindSubmit, Message: "submitted run " + runID})

	sink := make(chan transport.Event, 64)
	streamErrCh := make(chan error, 1)
	go func() { streamErrCh <- opts.Peer.Stream(ctx, runID, sink) }()

	for {
		select {
		case <-ctx.Done():
			<-streamErrCh
			drainEvents(sink, emit, &result)
			return result, ctx.Err()
		case ev := <-sink:
			result.Events++
			if status, ok := terminalStatus(ev); ok {
				result.Status = status
			}
			emit(PeerEvent{Kind: KindEvent, Event: ev})
		case err := <-streamErrCh:
			// Stream goroutine finished (clean close OR error). Drain any
			// events still buffered in sink before returning — without
			// this, the select can pick streamErrCh before the last events
			// (commonly including run.completed) are read.
			drainEvents(sink, emit, &result)

			// If no run.completed event was seen (e.g. planner setup
			// failed before any events fired), reach back for the run
			// record so we can surface the real status + error. Without
			// this, Apply silently returns Status="" and the user has no
			// idea why their plan didn't run.
			if err == nil && result.Status == "" {
				if rec, rerr := opts.Peer.GetRun(context.WithoutCancel(ctx), runID); rerr == nil {
					result.Status = rec.Status
					if rec.Error != "" {
						emit(PeerEvent{Kind: KindError, Message: fmt.Sprintf("run %s: %s", rec.Status, oneLine(rec.Error))})
					}
					if rec.Status == "failed" || rec.Status == "interrupted" {
						return result, fmt.Errorf("run %s on %s: %s", rec.Status, opts.PeerName, oneLine(rec.Error))
					}
				}
			}
			// Stream ended without a terminal status and without an error:
			// honest signal that the SSE connection dropped mid-run. The
			// daemon-side run may still be alive; we just can't see it.
			if err == nil && result.Status == "" {
				emit(PeerEvent{Kind: KindDisconnect})
			}
			return result, err
		}
	}
}

// emitter is a thin abstraction over the two output paths: PeerEvent
// channel (multi-peer multiplexer) and direct Writer (single-peer / tests).
// The peer name is pre-filled so callers just pass {Kind, Event, Message}.
type emitter func(PeerEvent)

func makeEmitter(opts ApplyOptions) emitter {
	if opts.Events != nil {
		peer := opts.PeerName
		ch := opts.Events
		return func(ev PeerEvent) {
			ev.Peer = peer
			ch <- ev
		}
	}
	if opts.Writer == nil {
		return func(PeerEvent) {}
	}
	// Single-peer Writer path: render inline with an unpadded prefix. The
	// peer name appears in the same `[name]` format the multiplexer uses,
	// so downstream consumers (tests, grep) see consistent output regardless
	// of which code path produced it.
	peer := opts.PeerName
	w := opts.Writer
	return func(ev PeerEvent) {
		var line string
		switch ev.Kind {
		case KindEvent:
			line = "[" + peer + "] " + formatEvent(ev.Event)
		case KindSync, KindSubmit:
			line = "[" + peer + "] " + ev.Message
		case KindDisconnect:
			line = "[" + peer + "] *** disconnected ***"
		case KindError:
			line = "[" + peer + "] ✗ " + ev.Message
		default:
			return
		}
		fmt.Fprintln(w, line)
	}
}

// drainEvents non-blockingly reads everything currently in sink, updating
// result and emitting. Used when the stream goroutine returns, to flush any
// buffered events that arrived just before close.
func drainEvents(sink <-chan transport.Event, emit emitter, result *ApplyResult) {
	for {
		select {
		case ev := <-sink:
			result.Events++
			if status, ok := terminalStatus(ev); ok {
				result.Status = status
			}
			emit(PeerEvent{Kind: KindEvent, Event: ev})
		default:
			return
		}
	}
}

// validateApplyPaths checks that PlanPath and each VarsFile live inside
// PlanDir. Without this guard a vars file outside the synced tree wouldn't
// be uploaded but would be referenced — confusing failure at run time.
func validateApplyPaths(opts ApplyOptions) error {
	if opts.PlanDir == "" {
		return errors.New("apply: PlanDir is empty")
	}
	if opts.PlanPath == "" {
		return errors.New("apply: PlanPath is empty")
	}
	if !filepath.IsAbs(opts.PlanDir) {
		return errors.New("apply: PlanDir must be absolute")
	}
	if !filepath.IsAbs(opts.PlanPath) {
		return errors.New("apply: PlanPath must be absolute")
	}
	if !isUnderDir(opts.PlanDir, opts.PlanPath) {
		return fmt.Errorf("apply: PlanPath %s is outside PlanDir %s", opts.PlanPath, opts.PlanDir)
	}
	for _, vp := range opts.VarsFiles {
		if !filepath.IsAbs(vp) {
			return fmt.Errorf("apply: vars file %s must be absolute", vp)
		}
		if !isUnderDir(opts.PlanDir, vp) {
			return fmt.Errorf("apply: vars file %s is outside PlanDir %s", vp, opts.PlanDir)
		}
	}
	return nil
}

func isUnderDir(dir, p string) bool {
	rel, err := filepath.Rel(dir, p)
	if err != nil {
		return false
	}
	if rel == "." {
		return false // p == dir; not a "child"
	}
	if rel == ".." {
		return false
	}
	// Anything beginning with "../" escapes the dir.
	if len(rel) >= 3 && rel[:3] == ".."+string(filepath.Separator) {
		return false
	}
	return true
}

// terminalStatus reports whether ev is a run-completion event and, if so,
// derives the status string from its `success` flag.
func terminalStatus(ev transport.Event) (string, bool) {
	if ev.Type != "run.completed" {
		return "", false
	}
	var data struct {
		Success bool `json:"success"`
	}
	if err := json.Unmarshal(ev.Data, &data); err != nil {
		return "completed", true
	}
	if data.Success {
		return "success", true
	}
	return "failed", true
}
