package agent

import (
	"context"
	"encoding/json"
	"io"
)

// Control-message types a driver (moongit) sends to a programmatic agent run
// on its stdin (#103). The run is spawned with --output-format json and no
// --auto-apply; the driver reads the NDJSON event stream on stdout and writes
// these one-per-line control messages back on stdin.
const (
	// ControlStop / ControlAbort cancel the run's context — the loop stops
	// at the next safe point with StopCanceled (#101). Both spellings are
	// accepted; "abort" mirrors the confirm gate's vocabulary.
	ControlStop  = "stop"
	ControlAbort = "abort"
	// ControlApprove / ControlReject answer a pending plan.awaiting_approval.
	ControlApprove = "approve"
	ControlReject  = "reject"
	// ControlEdit approves a plan the driver replaced: the run executes
	// Plan instead of the proposed one (mirrors the gate's "edit" outcome).
	ControlEdit = "edit"
)

// ControlMessage is one inbound NDJSON control message. Plan is only read for
// type=edit and carries the replacement plan (a JSON array of steps, the same
// shape the planner emits).
type ControlMessage struct {
	Type string          `json:"type"`
	Plan json.RawMessage `json:"plan,omitempty"`
}

// ControlChannel reads NDJSON ControlMessages from a reader (the agent's
// stdin) and routes them: stop/abort cancel the run; approve/reject/edit
// answer the pending confirm gate. It owns one read goroutine for the run's
// lifetime, started by NewControlChannel and stopped when the reader hits
// EOF (the driver closed stdin) or the run ctx is cancelled.
//
// This is the inbound half of the moongit integration; the outbound half is
// the existing --output-format json event stream on stdout. One channel
// solves both "stop the run" and "drive the human-gated approval".
type ControlChannel struct {
	cancel    context.CancelFunc
	approvals chan ConfirmResult
}

// NewControlChannel starts a goroutine reading control messages from r and
// returns the channel. cancel is the run ctx's CancelFunc — a stop/abort
// message invokes it. The goroutine exits on read error/EOF; pass a reader
// whose Read unblocks on ctx cancellation (or accept that the goroutine
// lingers harmlessly until stdin closes — it holds no locks and the process
// is exiting).
func NewControlChannel(r io.Reader, cancel context.CancelFunc) *ControlChannel {
	cc := &ControlChannel{
		cancel: cancel,
		// Buffered(1) so a deliver() never blocks the read loop even if the
		// approval arrives a hair before the gate enters its select — the
		// gate drains the buffer when it gets there. A second queued approval
		// (a misbehaving driver) is dropped by deliver's non-blocking send.
		approvals: make(chan ConfirmResult, 1),
	}
	go cc.readLoop(r)
	return cc
}

func (cc *ControlChannel) readLoop(r io.Reader) {
	dec := json.NewDecoder(r)
	for {
		var msg ControlMessage
		if err := dec.Decode(&msg); err != nil {
			return // EOF / closed stdin / malformed stream ends control input
		}
		switch msg.Type {
		case ControlStop, ControlAbort:
			cc.cancel()
			return // nothing meaningful follows a stop
		case ControlApprove:
			cc.deliver(ConfirmResult{Outcome: OutcomeApply})
		case ControlReject:
			cc.deliver(ConfirmResult{Outcome: OutcomeReject})
		case ControlEdit:
			cc.deliver(ConfirmResult{Outcome: OutcomeApply, PlanBytes: []byte(msg.Plan)})
		default:
			// Unknown type: ignore. Forward-compatible with a driver that
			// sends control verbs this build doesn't know yet.
		}
	}
}

// deliver hands an approval to a waiting gate without blocking the read loop.
// A non-blocking send into the buffered(1) channel: if the gate is waiting
// (or the buffer is empty) it lands; otherwise it's dropped, since a
// well-behaved driver only sends approve/reject/edit in response to a
// plan.awaiting_approval event.
func (cc *ControlChannel) deliver(res ConfirmResult) {
	select {
	case cc.approvals <- res:
	default:
	}
}

// Approver returns a RunOptions.Approver backed by this control channel.
// announce is called with the plan bytes when the gate is reached — the cmd
// layer uses it to emit the plan.awaiting_approval event — and then the
// approver blocks until a control message answers it or the run ctx is
// cancelled. A cancellation returns OutcomeAbort with ctx.Err() so RunLoop
// reports the stop (its post-gate ctx check maps that to StopCanceled).
func (cc *ControlChannel) Approver(announce func(planBytes []byte)) func(context.Context, []byte) (ConfirmResult, error) {
	return func(ctx context.Context, planBytes []byte) (ConfirmResult, error) {
		if announce != nil {
			announce(planBytes)
		}
		select {
		case res := <-cc.approvals:
			return res, nil
		case <-ctx.Done():
			return ConfirmResult{Outcome: OutcomeAbort}, ctx.Err()
		}
	}
}
