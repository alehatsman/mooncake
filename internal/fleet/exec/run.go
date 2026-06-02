package exec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// ExecOptions configures an Exec fan-out across N peers.
type ExecOptions struct {
	// Peers is the resolved peer list (already filtered through
	// --peers / --peer-filter on the controller side).
	Peers []ExecPeer

	// PlanBytes is the synthesized one-step plan. Build it once via
	// Synthesize and share the bytes across all peers — they all run
	// the same command.
	PlanBytes []byte

	// ControllerID is the UUID used to namespace the scratch scope.
	// Mirrors fleet apply's ControllerID; minted via fleet.EnsureControllerID.
	ControllerID string

	// Parallel caps the number of in-flight peers. 0 = unbounded.
	Parallel int

	// Events, when non-nil, receives one fleet.PeerEvent per streamed
	// SSE event plus control events at submit/disconnect/error
	// boundaries. The caller owns the channel and is responsible for
	// closing it after Exec returns. Ignored when nil.
	Events chan<- fleet.PeerEvent

	// CollectOutput, when true, accumulates stdout / stderr from
	// `step.stdout` / `step.stderr` events into the returned
	// PeerOutcome. Needed for --json mode. Multiplex mode leaves it
	// false to avoid per-peer string growth on long-running commands.
	CollectOutput bool

	// MaxOutputBytes bounds the accumulated stdout / stderr per peer.
	// Defaults to 1 MiB. Excess is dropped and the truncated bool is
	// set on the outcome.
	MaxOutputBytes int
}

// ExecPeer is a thin descriptor of one peer the orchestrator needs to
// drive. Lifted out of `cmd/` so this package stays callable from tests
// without dragging in the CLI layer.
type ExecPeer struct {
	Name   string
	Client *transport.Client
}

// PeerOutcome is one peer's final state after Exec.
type PeerOutcome struct {
	Peer            string `json:"peer"`
	RunID           string `json:"run_id,omitempty"`
	Status          string `json:"status"` // "success" / "failed" / "unreachable" / "error"
	ExitCode        int    `json:"exit_code"`
	Stdout          string `json:"stdout,omitempty"`
	Stderr          string `json:"stderr,omitempty"`
	StdoutTruncated bool   `json:"stdout_truncated,omitempty"`
	StderrTruncated bool   `json:"stderr_truncated,omitempty"`
	DurationMs      int64  `json:"duration_ms"`
	Error           string `json:"error,omitempty"` // non-empty when Status=="unreachable" or "error"
}

// Exec runs the synthesized plan against every peer in opts.Peers in
// parallel (capped by opts.Parallel), accumulates per-peer outcomes,
// and returns them in peer-list order (NOT completion order — that's
// what the streaming Events channel is for).
//
// Exec returns nil even when individual peers fail; the caller is
// responsible for translating the slice of outcomes into an exit code
// (see ExitCode helper).
func Exec(ctx context.Context, opts ExecOptions) ([]PeerOutcome, error) {
	if len(opts.Peers) == 0 {
		return nil, errors.New("exec: no peers")
	}
	if len(opts.PlanBytes) == 0 {
		return nil, errors.New("exec: empty PlanBytes; call Synthesize first")
	}
	if opts.ControllerID == "" {
		return nil, errors.New("exec: ControllerID is required")
	}

	maxBytes := opts.MaxOutputBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MiB
	}

	// Stage the synthesized plan to a local temp file once; every peer
	// uploads the same content.
	planPath, planSha, err := fleet.WriteStagedPlan(opts.PlanBytes, "mooncake-fleet-exec-*.yml", "exec")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(planPath) }()

	outcomes := make([]PeerOutcome, len(opts.Peers))
	limit := opts.Parallel
	if limit <= 0 || limit > len(opts.Peers) {
		limit = len(opts.Peers)
	}
	sem := make(chan struct{}, limit)
	var wg sync.WaitGroup
	for i, p := range opts.Peers {
		i, p := i, p
		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			outcomes[i] = execOne(ctx, p, planPath, planSha, opts.ControllerID, opts.Events, opts.CollectOutput, maxBytes)
		}()
	}
	wg.Wait()
	return outcomes, nil
}

// execOne drives one peer's full cycle: GetVersion → Put scratch plan
// → Submit → Stream. Always returns a PeerOutcome; errors are recorded
// on it rather than returned so the parallel fan-out can continue.
func execOne(
	ctx context.Context,
	p ExecPeer,
	planPath, planSha, controllerID string,
	events chan<- fleet.PeerEvent,
	collect bool,
	maxBytes int,
) PeerOutcome {
	out := PeerOutcome{Peer: p.Name, Status: "error"}
	start := time.Now()
	defer func() { out.DurationMs = time.Since(start).Milliseconds() }()

	emit := func(kind fleet.PeerEventKind, msg string, ev transport.Event) {
		if events == nil {
			return
		}
		events <- fleet.PeerEvent{Peer: p.Name, Kind: kind, Event: ev, Message: msg}
	}

	ver, err := p.Client.GetVersion(ctx)
	if err != nil {
		out.Status, out.Error = "unreachable", oneLine(err.Error())
		emit(fleet.KindError, "version probe: "+out.Error, transport.Event{})
		return out
	}

	uid := newULID()
	scope := scopeFor(controllerID)
	relPath := uid + "/exec.yml"

	// Put imposes no default deadline of its own, so bound the upload here.
	// The scratch plan is a tiny synthesized one-step file, so 30s is
	// generous; ctx cancellation (SIGINT) still applies underneath.
	putCtx, putCancel := context.WithTimeout(ctx, 30*time.Second)
	perr := p.Client.Put(putCtx, scope, relPath, planPath, planSha)
	putCancel()
	if perr != nil {
		out.Status, out.Error = "error", oneLine(perr.Error())
		emit(fleet.KindError, "put: "+out.Error, transport.Event{})
		return out
	}

	peerPlanPath := path.Join(ver.SyncedRoot, scope, relPath)
	peerBaseDir := path.Join(ver.SyncedRoot, scope, uid)

	runID, err := p.Client.Submit(ctx, transport.SubmitRequest{
		PlanPath: peerPlanPath,
		BaseDir:  peerBaseDir,
	})
	if err != nil {
		out.Status, out.Error = "error", oneLine(err.Error())
		emit(fleet.KindError, "submit: "+out.Error, transport.Event{})
		return out
	}
	out.RunID = runID
	emit(fleet.KindSubmit, "submitted run "+runID, transport.Event{})

	sink := make(chan transport.Event, 64)
	streamErr := make(chan error, 1)
	go func() { streamErr <- p.Client.Stream(ctx, runID, sink) }()

	var stdout, stderr strings.Builder
	appendBounded := func(b *strings.Builder, s string, truncated *bool) {
		if !collect || *truncated {
			return
		}
		// Reserve room for a trailing newline since we re-add one.
		remaining := maxBytes - b.Len()
		if remaining <= 0 {
			*truncated = true
			return
		}
		if len(s)+1 > remaining {
			b.WriteString(s[:remaining-1])
			*truncated = true
			return
		}
		b.WriteString(s)
		b.WriteByte('\n')
	}

	for {
		select {
		case <-ctx.Done():
			<-streamErr
			drainSink(sink, &out, &stdout, &stderr, appendBounded, events, p.Name)
			if out.Status == "error" {
				out.Status, out.Error = "error", oneLine(ctx.Err().Error())
			}
			out.Stdout, out.Stderr = stdout.String(), stderr.String()
			return out
		case ev := <-sink:
			handleEvent(ev, &out, &stdout, &stderr, appendBounded)
			emit(fleet.KindEvent, "", ev)
		case err := <-streamErr:
			drainSink(sink, &out, &stdout, &stderr, appendBounded, events, p.Name)
			if err != nil && out.Status == "error" {
				out.Status, out.Error = "error", oneLine(err.Error())
				emit(fleet.KindDisconnect, "", transport.Event{})
			}
			if out.Status == "error" {
				// Stream closed cleanly but we never saw a run.completed.
				// Reach back for the canonical status so we don't strand
				// the peer in "error".
				if rec, rerr := p.Client.GetRun(context.WithoutCancel(ctx), runID); rerr == nil {
					switch rec.Status {
					case "success", "failed":
						out.Status = rec.Status
					default:
						out.Status, out.Error = "error", "run ended without terminal status: "+rec.Status
					}
				} else {
					out.Status, out.Error = "error", "missing terminal event and GetRun failed: "+oneLine(rerr.Error())
				}
			}
			out.Stdout, out.Stderr = stdout.String(), stderr.String()
			// Stdout/StderrTruncated already set by appendBounded.
			return out
		}
	}
}

// handleEvent updates outcome state from one SSE event and accumulates
// stdout / stderr lines when collection is enabled.
func handleEvent(
	ev transport.Event,
	out *PeerOutcome,
	stdout, stderr *strings.Builder,
	appendBounded func(*strings.Builder, string, *bool),
) {
	switch ev.Type {
	case "step.stdout":
		appendBounded(stdout, extractLine(ev.Data), &out.StdoutTruncated)
	case "step.stderr":
		appendBounded(stderr, extractLine(ev.Data), &out.StderrTruncated)
	case "step.failed":
		var d struct {
			ExitCode int    `json:"exit_code"`
			Error    string `json:"error_message"`
		}
		_ = json.Unmarshal(ev.Data, &d)
		out.ExitCode = d.ExitCode
		if out.Error == "" {
			out.Error = d.Error
		}
	case "run.completed":
		var d struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(ev.Data, &d) == nil {
			if d.Success {
				out.Status = "success"
				// step.completed didn't carry exit_code today; success
				// implies the shell returned 0 (kernel only surfaces
				// non-zero through step.failed).
				out.ExitCode = 0
			} else {
				out.Status = "failed"
			}
		} else {
			out.Status = "failed"
		}
	}
}

func drainSink(
	sink <-chan transport.Event,
	out *PeerOutcome,
	stdout, stderr *strings.Builder,
	appendBounded func(*strings.Builder, string, *bool),
	events chan<- fleet.PeerEvent,
	peer string,
) {
	for {
		select {
		case ev := <-sink:
			handleEvent(ev, out, stdout, stderr, appendBounded)
			if events != nil {
				events <- fleet.PeerEvent{Peer: peer, Kind: fleet.KindEvent, Event: ev}
			}
		default:
			return
		}
	}
}

// scopeFor returns the daemon-side scope for one exec invocation.
// Shape: `exec/<controller-id>`. Stays within the daemon's
// max-two-segments / 128-byte-cap constraints (validateScope in
// internal/agentd/files_handler.go).
func scopeFor(controllerID string) string {
	return "exec/" + controllerID
}

// newULID returns a fresh ULID string. Each invocation gets its own
// per-peer scratch dir, so two concurrent `fleet exec` runs against
// the same peer don't collide.
func newULID() string {
	entropy := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) // #nosec G404 — non-crypto path
	return ulid.MustNew(ulid.Timestamp(time.Now()), entropy).String()
}

// extractLine pulls the "line" field out of a step.stdout / step.stderr
// event payload. Used to accumulate the per-peer captured output.
func extractLine(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m struct {
		Line string `json:"line"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return ""
	}
	return m.Line
}

// oneLine collapses a multi-line error into one suitable for inline UI.
// Mirrors fleet.oneLine but kept private here to avoid bleeding fleet's
// internals into exec callers.
func oneLine(s string) string {
	out := strings.ReplaceAll(s, "\n", " ")
	out = strings.ReplaceAll(out, "\t", " ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}

// ExitCode maps a per-peer outcome list to the controller's exit code,
// per spec-52 §"Exit-code aggregation":
//
//	0 — every peer succeeded
//	1 — at least one peer's run failed (non-zero exit)
//	2 — at least one peer was unreachable or the submit failed
//
// On a mix, the higher code wins. Callers should use this for the
// process exit code under both multiplex and --json modes.
func ExitCode(outs []PeerOutcome) int {
	rc := 0
	for _, o := range outs {
		switch o.Status {
		case "success":
			// no-op
		case "failed":
			if rc < 1 {
				rc = 1
			}
		case "unreachable", "error":
			if rc < 2 {
				rc = 2
			}
		}
	}
	return rc
}

// Summary builds the trailing one-line summary banner ("4/5 ok — failed
// on macbook (exit 2); unreachable: vps-1"). Returned without a newline.
func Summary(outs []PeerOutcome) string {
	if len(outs) == 0 {
		return "fleet exec: 0/0 ok"
	}
	ok := 0
	var failed, unreach []string
	for _, o := range outs {
		switch o.Status {
		case "success":
			ok++
		case "failed":
			failed = append(failed, fmt.Sprintf("%s (exit %d)", o.Peer, o.ExitCode))
		default:
			unreach = append(unreach, o.Peer)
		}
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "fleet exec: %d/%d ok", ok, len(outs))
	if len(failed) > 0 {
		fmt.Fprintf(&sb, " — failed on %s", strings.Join(failed, ", "))
	}
	if len(unreach) > 0 {
		if len(failed) > 0 {
			sb.WriteString("; unreachable: ")
		} else {
			sb.WriteString(" — unreachable: ")
		}
		sb.WriteString(strings.Join(unreach, ", "))
	}
	return sb.String()
}
