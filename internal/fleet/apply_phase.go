package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// ApplyPhaseInput is the per-phase input to RunApplyPhase. cmd/fleet.go's
// fleetApplyAction in single-phase mode constructs it once; in multi-phase
// mode (machine fleet.yml present) RunMachineApply constructs one per
// phase via ResolveMachinePhase.
type ApplyPhaseInput struct {
	PlanAbs       string
	PlanDir       string
	Peers         []Peer // already filtered + agentd-only
	UnknownPeers  []string
	SkippedPeers  []Peer // non-agentd; printed as skipped banners
	VarsAbs       []string
	Tags          []string
	StepNames     []string
	MaxSyncBytes  int64
	Parallel      int
	ControllerID  string
	BannerHeading string // "fleet apply: ..." or "phase N/M — <phase-name>"
}

// ApplyPhaseOutcome rolls up one phase's per-peer results into the four
// numbers the caller needs to decide fail-fast and exit code. Mirrors the
// switch inside fleetApplyAction's aggregate block.
//
// PerPeer carries the typed per-peer ApplyResults (R2.1c) for callers
// that want to compose fleet.KernelResult.Peers without re-running
// Apply. Ordered to match ApplyPhaseInput.Peers; len(PerPeer) ==
// len(input.Peers) post-RunApplyPhase. Nil entries indicate a peer that
// errored before Apply could return (shouldn't happen — Apply always
// returns a value).
type ApplyPhaseOutcome struct {
	OK          int
	RunFailed   int
	Unreachable int
	FailedNames []string
	FirstErr    error
	PerPeer     []ApplyResult
}

// Terminal reports whether this phase ended in a state that should stop
// further phases in fail-fast mode (i.e. anything other than every peer
// reaching "success").
func (o ApplyPhaseOutcome) Terminal() bool {
	return o.RunFailed > 0 || o.Unreachable > 0 || o.FirstErr != nil
}

// RunApplyPhase drives one phase's worth of fleet apply: spins up the
// multiplexer + signal-aware event drain, fan-outs Apply across all peers
// in parallel (capped by Parallel), and returns the rolled-up outcome.
//
// applyCtx is the shared cancellation context owned by the caller (so a
// single ^C cancels every in-flight phase at once); each phase gets its
// own multiplexer + event channel.
func RunApplyPhase(applyCtx context.Context, w io.Writer, useColor bool, in ApplyPhaseInput) ApplyPhaseOutcome {
	peerNames := make([]string, 0, len(in.Peers))
	for _, p := range in.Peers {
		peerNames = append(peerNames, p.Name)
	}
	mux := NewMultiplexer(w, peerNames, useColor)
	mux.Banner(in.BannerHeading)
	if len(in.UnknownPeers) > 0 {
		mux.Banner("warning: unknown peer name(s) in --peer: " + strings.Join(in.UnknownPeers, ", "))
	}
	for _, p := range in.SkippedPeers {
		mux.Banner(fmt.Sprintf("skipped %s: transport %q not supported (agentd only)", p.Name, p.Transport))
	}

	events := make(chan PeerEvent, 64*len(in.Peers))
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	var sem chan struct{}
	if in.Parallel > 0 {
		sem = make(chan struct{}, in.Parallel)
	}

	results := make([]ApplyResult, len(in.Peers))
	errs := make([]error, len(in.Peers))
	var wg sync.WaitGroup
	for i, p := range in.Peers {
		wg.Add(1)
		go func(i int, p Peer) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			client := transport.New(p.Name, p.Addr, p.Token)
			overlayVars := ResolveVarsFiles(in.PlanDir, p)
			peerVars := append(append([]string{}, overlayVars...), in.VarsAbs...)
			results[i], errs[i] = Apply(applyCtx, ApplyOptions{
				PeerName:     p.Name,
				Peer:         client,
				PlanDir:      in.PlanDir,
				PlanPath:     in.PlanAbs,
				VarsFiles:    peerVars,
				Tags:         in.Tags,
				Names:        in.StepNames,
				ControllerID: in.ControllerID,
				MaxSyncBytes: in.MaxSyncBytes,
				Events:       events,
			})
		}(i, p)
	}
	wg.Wait()
	close(events)
	<-drained

	out := ApplyPhaseOutcome{PerPeer: results}
	for i, r := range results {
		switch r.Status {
		case "success":
			out.OK++
		case "failed", "interrupted":
			out.RunFailed++
			out.FailedNames = append(out.FailedNames, in.Peers[i].Name)
		default:
			if errs[i] != nil {
				out.Unreachable++
				out.FailedNames = append(out.FailedNames, in.Peers[i].Name)
			}
		}
	}
	mux.Banner(fmt.Sprintf("fleet apply: %d/%d ok", out.OK, len(in.Peers)))

	for _, e := range errs {
		if e != nil && !errors.Is(e, context.Canceled) {
			out.FirstErr = e
			break
		}
	}
	return out
}
