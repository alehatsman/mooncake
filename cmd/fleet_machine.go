package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"sync"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
	"github.com/urfave/cli/v2"
)

// applyPhaseInput is the per-phase input to runApplyPhase. fleetApplyAction
// in single-phase mode calls it once; in multi-phase mode (machine
// fleet.yml present) it calls it once per phase.
type applyPhaseInput struct {
	PlanAbs       string
	PlanDir       string
	Peers         []fleet.Peer // already filtered + agentd-only
	UnknownPeers  []string     // for warning banner; empty in machine mode
	SkippedPeers  []fleet.Peer // non-agentd; printed as skipped banners
	VarsAbs       []string
	Tags          []string
	StepNames     []string
	MaxSyncBytes  int64
	Parallel      int
	ControllerID  string
	BannerHeading string // "fleet apply: ..." or "phase N/M — <phase-name>"
}

// applyPhaseOutcome rolls up one phase's per-peer results into the four
// numbers the caller needs to decide fail-fast and exit code. Mirrors the
// switch inside fleetApplyAction's old aggregate block.
type applyPhaseOutcome struct {
	OK          int
	RunFailed   int
	Unreachable int
	FailedNames []string
	FirstErr    error
}

// terminal reports whether this phase ended in a state that should stop
// further phases in fail-fast mode (i.e. anything other than every peer
// reaching "success").
func (o applyPhaseOutcome) terminal() bool {
	return o.RunFailed > 0 || o.Unreachable > 0 || o.FirstErr != nil
}

// runApplyPhase drives one phase's worth of fleet apply: spins up the
// multiplexer + signal-aware event drain, fan-outs Apply across all peers
// in parallel (capped by Parallel), and returns the rolled-up outcome.
// It is the body of the original fleetApplyAction extracted so the
// multi-phase machine-apply path can call it once per phase with a fresh
// banner / vars / peer set.
//
// applyCtx is the shared cancellation context owned by fleetApplyAction
// (so a single ^C cancels every in-flight phase at once); each phase
// gets its own multiplexer + event channel.
func runApplyPhase(applyCtx context.Context, w io.Writer, useColor bool, in applyPhaseInput) applyPhaseOutcome {
	peerNames := make([]string, 0, len(in.Peers))
	for _, p := range in.Peers {
		peerNames = append(peerNames, p.Name)
	}
	mux := fleet.NewMultiplexer(w, peerNames, useColor)
	mux.Banner(in.BannerHeading)
	if len(in.UnknownPeers) > 0 {
		mux.Banner("warning: unknown peer name(s) in --peers filter: " + strings.Join(in.UnknownPeers, ", "))
	}
	for _, p := range in.SkippedPeers {
		mux.Banner(fmt.Sprintf("skipped %s: transport %q not supported (agentd only)", p.Name, p.Transport))
	}

	events := make(chan fleet.PeerEvent, 64*len(in.Peers))
	drained := make(chan struct{})
	go func() {
		mux.Drain(events)
		close(drained)
	}()

	var sem chan struct{}
	if in.Parallel > 0 {
		sem = make(chan struct{}, in.Parallel)
	}

	results := make([]fleet.ApplyResult, len(in.Peers))
	errs := make([]error, len(in.Peers))
	var wg sync.WaitGroup
	for i, p := range in.Peers {
		wg.Add(1)
		go func(i int, p fleet.Peer) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			client := transport.New(p.Name, p.Addr, p.Token)
			overlayVars := fleet.ResolveVarsFiles(in.PlanDir, p)
			peerVars := append(append([]string{}, overlayVars...), in.VarsAbs...)
			results[i], errs[i] = fleet.Apply(applyCtx, fleet.ApplyOptions{
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

	var out applyPhaseOutcome
	for i, r := range results {
		switch {
		case r.Status == "success":
			out.OK++
		case r.Status == "failed" || r.Status == "interrupted":
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

// machinePhaseInput is the per-phase setup that fleetApplyAction needs to
// hand to runApplyPhase when running in multi-phase mode. It resolves the
// peer, plan path, and vars stack from a MachinePhase plus the
// command-line-wide context.
type machinePhaseInput struct {
	Phase     fleet.MachinePhase
	PhaseNum  int
	TotalPhases int
}

// resolveMachinePhase turns one MachinePhase entry into the
// applyPhaseInput shape runApplyPhase consumes: looks up the named peer
// in peers.toml, builds the vars stack (machine-wide + phase-specific +
// CLI), merges tags, and pins the phase's banner heading.
//
// peerFilterGroups, if non-empty, is AND'd with the manifest's peer
// selection: a phase whose peer fails the filter is skipped (the caller
// recognises a "phase has zero selected peers" outcome as a skip, not a
// failure).
func resolveMachinePhase(
	mInput machinePhaseInput,
	cfgPeers []fleet.Peer,
	planDir string,
	cliVarsAbs []string,
	cliTags []string,
	stepNames []string,
	maxSync int64,
	parallel int,
	controllerID string,
	peerFilterGroups [][]filterTerm,
	osFor peerOSResolver,
) (applyPhaseInput, error) {
	var found *fleet.Peer
	for i := range cfgPeers {
		if cfgPeers[i].Name == mInput.Phase.Peer {
			found = &cfgPeers[i]
			break
		}
	}
	if found == nil {
		return applyPhaseInput{}, fmt.Errorf("phase %q: peer %q not in peers.toml", mInput.Phase.Name, mInput.Phase.Peer)
	}
	// Apply --peer-filter on the manifest-pinned peer. Phases whose peer
	// doesn't match are signaled to the caller by an empty Peers slice.
	peers := []fleet.Peer{*found}
	if len(peerFilterGroups) > 0 {
		filtered := peers[:0]
		for _, p := range peers {
			if peerMatchesFilters(p, peerFilterGroups, osFor) {
				filtered = append(filtered, p)
			}
		}
		peers = filtered
	}

	// Vars: machine-wide (cliVarsAbs already contains shared+machine
	// vars from fleetApplyAction's top-level resolution) + phase.Vars
	// (already resolved to absolute paths in LoadMachineManifest).
	varsAbs := make([]string, 0, len(cliVarsAbs)+len(mInput.Phase.Vars))
	varsAbs = append(varsAbs, cliVarsAbs...)
	varsAbs = append(varsAbs, mInput.Phase.Vars...)

	// Tags: phase.Tags ∪ cliTags. Both filter step selection; both
	// applying simultaneously is "show steps tagged with anything in the
	// merged set" (the planner's MatchesTags is a union match).
	tags := make([]string, 0, len(cliTags)+len(mInput.Phase.Tags))
	tags = append(tags, cliTags...)
	tags = append(tags, mInput.Phase.Tags...)

	// Skip non-agentd transports. In machine mode the manifest pins the
	// peer, so a non-agentd peer is an operator misconfiguration — but
	// the same code-path handles it gracefully (Skipped banner).
	agentdPeers := make([]fleet.Peer, 0, len(peers))
	var skipped []fleet.Peer
	for _, p := range peers {
		if p.Transport != fleet.TransportAgentd {
			skipped = append(skipped, p)
			continue
		}
		agentdPeers = append(agentdPeers, p)
	}

	planAbs := mInput.Phase.Plan
	if !filepath.IsAbs(planAbs) {
		// LoadMachineManifest resolves Plan relative to the manifest
		// dir, so we shouldn't hit this branch — defensive.
		abs, err := filepath.Abs(planAbs)
		if err != nil {
			return applyPhaseInput{}, fmt.Errorf("phase %q: resolve plan: %w", mInput.Phase.Name, err)
		}
		planAbs = abs
	}

	banner := fmt.Sprintf(
		"phase %d/%d — %s · peer %s · plan %s",
		mInput.PhaseNum, mInput.TotalPhases, mInput.Phase.Name, mInput.Phase.Peer, planAbs,
	)

	return applyPhaseInput{
		PlanAbs:       planAbs,
		PlanDir:       planDir,
		Peers:         agentdPeers,
		SkippedPeers:  skipped,
		VarsAbs:       varsAbs,
		Tags:          tags,
		StepNames:     stepNames,
		MaxSyncBytes:  maxSync,
		Parallel:      parallel,
		ControllerID:  controllerID,
		BannerHeading: banner,
	}, nil
}

// runMachineApply orchestrates ordered multi-phase apply for a machine
// whose `machines/<name>/fleet.yml` was found. Runs each phase
// sequentially, fail-fast: a phase that doesn't reach "every peer ok"
// stops subsequent phases. Returns the overall exit error (or nil).
//
// peerFilterGroups is the parsed --peer-filter from the CLI; it's AND'd
// with each phase's manifest-pinned peer. osFor is the lazy /v1/version
// probe cache (may be nil when no os= predicate is present).
func runMachineApply(
	applyCtx context.Context,
	w io.Writer,
	useColor bool,
	manifest *fleet.MachineManifest,
	machineName string,
	cfgPeers []fleet.Peer,
	planDir string,
	cliVarsAbs []string,
	cliTags []string,
	stepNames []string,
	maxSync int64,
	parallel int,
	controllerID string,
	peerFilterGroups [][]filterTerm,
	osFor peerOSResolver,
) error {
	total := len(manifest.Phases)
	fmt.Fprintf(w, "fleet apply %s: %d phase(s)\n", machineName, total)

	var phaseFailures []string
	for i, phase := range manifest.Phases {
		select {
		case <-applyCtx.Done():
			return cli.Exit("fleet apply: interrupted before phase "+phase.Name, 130)
		default:
		}

		in, err := resolveMachinePhase(
			machinePhaseInput{Phase: phase, PhaseNum: i + 1, TotalPhases: total},
			cfgPeers, planDir, cliVarsAbs, cliTags, stepNames,
			maxSync, parallel, controllerID,
			peerFilterGroups, osFor,
		)
		if err != nil {
			return cli.Exit("fleet apply: "+err.Error(), 2)
		}

		if len(in.Peers) == 0 {
			// --peer-filter filtered out the phase's peer. Don't treat
			// as failure — print a banner and continue, matching how
			// --peer-filter behaves in single-phase mode.
			fmt.Fprintf(w, "phase %d/%d — %s · peer %s skipped by --peer-filter\n",
				i+1, total, phase.Name, phase.Peer)
			continue
		}

		out := runApplyPhase(applyCtx, w, useColor, in)
		if out.terminal() {
			// Fail-fast: record the failure, stop subsequent phases.
			if len(out.FailedNames) > 0 {
				phaseFailures = append(phaseFailures, fmt.Sprintf("%s (peer %s)", phase.Name, strings.Join(out.FailedNames, ",")))
			} else {
				phaseFailures = append(phaseFailures, phase.Name)
			}
			break
		}
	}

	switch {
	case len(phaseFailures) == 0:
		fmt.Fprintf(w, "fleet apply %s: %d/%d phases ok\n", machineName, total, total)
		return nil
	default:
		return cli.Exit("fleet apply "+machineName+": failed phase(s): "+strings.Join(phaseFailures, "; "), 1)
	}
}
