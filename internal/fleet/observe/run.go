package observe

import (
	"context"
	"encoding/json"
	"errors"
	"math/rand"
	"os"
	"path"
	"sync"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// Options configures one fleet-observe fan-out.
type Options struct {
	// Peers is the resolved peer list (already filtered upstream).
	Peers []Peer

	// PlanBytes is the synthesized one-step plan from Synthesize.
	PlanBytes []byte

	// ControllerID is the UUID used to namespace the scratch scope.
	// Mirrors fleet apply's ControllerID; minted via fleet.EnsureControllerID.
	ControllerID string

	// Parallel caps the number of in-flight peers. 0 = unbounded.
	Parallel int
}

// Peer is one peer the runner needs to drive.
type Peer struct {
	Name   string
	Client *transport.Client
}

// PeerOutcome is one peer's final state after Observe. The Result map
// is the typed observation envelope captured from the peer's
// step.completed event — keyed by "found", "value", "as_of", "error".
type PeerOutcome struct {
	Peer       string         `json:"peer"`
	RunID      string         `json:"run_id,omitempty"`
	Status     string         `json:"status"` // "success" | "failed" | "unreachable" | "error"
	Result     map[string]any `json:"result,omitempty"`
	Error      string         `json:"error,omitempty"`
	DurationMs int64          `json:"duration_ms"`
}

// Observe runs the synthesized observation plan against every peer in
// opts.Peers in parallel (capped by opts.Parallel). Returns outcomes
// in peer-list order — slot[i] always corresponds to opts.Peers[i].
//
// Per-peer failures are reported via PeerOutcome.Status/Error rather
// than the function-level error; the function-level error is reserved
// for invalid options.
func Observe(ctx context.Context, opts Options) ([]PeerOutcome, error) {
	if len(opts.Peers) == 0 {
		return nil, errors.New("observe: no peers")
	}
	if len(opts.PlanBytes) == 0 {
		return nil, errors.New("observe: empty PlanBytes; call Synthesize first")
	}
	if opts.ControllerID == "" {
		return nil, errors.New("observe: ControllerID is required")
	}

	planPath, planSha, err := fleet.WriteStagedPlan(opts.PlanBytes, "mooncake-fleet-observe-*.yml", "observe")
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
			outcomes[i] = observeOne(ctx, p, planPath, planSha, opts.ControllerID)
		}()
	}
	wg.Wait()
	return outcomes, nil
}

// observeOne drives one peer's full observation cycle: GetVersion →
// Put scratch plan → Submit → Stream → capture the step.completed
// Result map. Mirrors exec.execOne but with capture instead of stream
// accumulation.
func observeOne(
	ctx context.Context,
	p Peer,
	planPath, planSha, controllerID string,
) PeerOutcome {
	out := PeerOutcome{Peer: p.Name, Status: "error"}
	start := time.Now()
	defer func() { out.DurationMs = time.Since(start).Milliseconds() }()

	ver, err := p.Client.GetVersion(ctx)
	if err != nil {
		out.Status, out.Error = "unreachable", oneLine(err.Error())
		return out
	}

	uid := newULID()
	scope := scopeFor(controllerID)
	relPath := uid + "/observe.yml"

	// Put imposes no default deadline of its own, so bound the upload here.
	// The scratch plan is a tiny synthesized one-step file, so 30s is
	// generous; ctx cancellation (SIGINT) still applies underneath.
	putCtx, putCancel := context.WithTimeout(ctx, 30*time.Second)
	perr := p.Client.Put(putCtx, scope, relPath, planPath, planSha)
	putCancel()
	if perr != nil {
		out.Error = oneLine(perr.Error())
		return out
	}

	peerPlanPath := path.Join(ver.SyncedRoot, scope, relPath)
	peerBaseDir := path.Join(ver.SyncedRoot, scope, uid)

	runID, err := p.Client.Submit(ctx, transport.SubmitRequest{
		PlanPath: peerPlanPath,
		BaseDir:  peerBaseDir,
	})
	if err != nil {
		out.Error = oneLine(err.Error())
		return out
	}
	out.RunID = runID

	sink := make(chan transport.Event, 32)
	streamErr := make(chan error, 1)
	go func() { streamErr <- p.Client.Stream(ctx, runID, sink) }()

	for {
		select {
		case <-ctx.Done():
			<-streamErr
			out.Error = oneLine(ctx.Err().Error())
			return out
		case ev := <-sink:
			capture(ev, &out)
		case err := <-streamErr:
			// Drain anything left in the buffer.
			drain(sink, &out)
			if err != nil && out.Status == "error" {
				out.Error = oneLine(err.Error())
			}
			if out.Status == "error" {
				// Stream closed but we never saw a terminal status. Fall
				// back to GetRun for the canonical status.
				if rec, rerr := p.Client.GetRun(context.WithoutCancel(ctx), runID); rerr == nil {
					switch rec.Status {
					case "success", "failed":
						out.Status = rec.Status
					default:
						out.Error = "run ended without terminal status: " + rec.Status
					}
				} else {
					out.Error = "missing terminal event and GetRun failed: " + oneLine(rerr.Error())
				}
			}
			return out
		}
	}
}

// capture pulls the observation Result map out of the per-step events
// and the terminal status out of run.completed.
func capture(ev transport.Event, out *PeerOutcome) {
	switch ev.Type {
	case "step.completed":
		var d struct {
			Result map[string]any `json:"result"`
		}
		if json.Unmarshal(ev.Data, &d) == nil && d.Result != nil {
			out.Result = d.Result
		}
	case "step.failed":
		var d struct {
			ErrorMessage string `json:"error_message"`
		}
		if json.Unmarshal(ev.Data, &d) == nil && out.Error == "" {
			out.Error = d.ErrorMessage
		}
	case "run.completed":
		var d struct {
			Success bool `json:"success"`
		}
		if json.Unmarshal(ev.Data, &d) == nil {
			if d.Success {
				out.Status = "success"
			} else {
				out.Status = "failed"
			}
		}
	}
}

func drain(sink <-chan transport.Event, out *PeerOutcome) {
	for {
		select {
		case ev := <-sink:
			capture(ev, out)
		default:
			return
		}
	}
}

// scopeFor returns the daemon-side scope for one observe invocation.
// Shape mirrors exec's: `observe/<controller-id>`.
func scopeFor(controllerID string) string {
	return "observe/" + controllerID
}

// newULID mints a ULID for the per-run scratch directory name.
func newULID() string {
	src := ulid.Monotonic(rand.New(rand.NewSource(time.Now().UnixNano())), 0) //nolint:gosec // not cryptographic
	return ulid.MustNew(ulid.Timestamp(time.Now()), src).String()
}

// oneLine collapses a multi-line error message into a single line so
// it fits in CLI / JSON output.
func oneLine(s string) string {
	for i := 0; i < len(s); i++ {
		if s[i] == '\n' || s[i] == '\r' {
			return s[:i] + " " + oneLine(s[i+1:])
		}
	}
	return s
}

// Avoid "imported and not used" if fleet ends up unused; the package
// is intentionally imported for downstream symmetry with exec.
var _ = fleet.TransportAgentd
