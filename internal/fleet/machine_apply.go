package fleet

import (
	"context"
	"fmt"
	"io"
	"path/filepath"
	"strings"
)

// MachinePhaseInput is the per-phase setup that RunMachineApply hands to
// ResolveMachinePhase. The phase index/total are tracked for banner text;
// the Phase itself carries everything else.
type MachinePhaseInput struct {
	Phase       MachinePhase
	PhaseNum    int
	TotalPhases int
}

// MachineRunResult is the structured outcome of RunMachineApply. The cmd
// layer translates this into cli.Exit / os.Exit at the boundary; internal
// (non-CLI) callers can branch on ExitCode directly.
//
// ExitCode semantics:
//
//	0   = every phase ran to "every peer ok"
//	1   = at least one phase failed (PhaseFailures lists them)
//	2   = a phase couldn't be resolved (peer not in peers.toml etc.)
//	130 = ^C cancellation before/between phases
type MachineRunResult struct {
	ExitCode      int
	Message       string   // user-facing; empty when ExitCode==0
	PhaseFailures []string // phase names that failed; empty on success
}

// ResolveMachinePhase turns one MachinePhase entry into the ApplyPhaseInput
// shape RunApplyPhase consumes: looks up the named peer in cfgPeers, builds
// the vars stack (machine-wide + phase-specific + CLI), merges tags, and
// pins the phase's banner heading.
//
// peerFilter, if non-nil, is AND'd with the manifest's peer selection: a
// phase whose peer fails the filter is signalled to the caller by an empty
// Peers slice in the returned ApplyPhaseInput. Pass nil to accept every
// manifest-pinned peer.
func ResolveMachinePhase(
	mInput MachinePhaseInput,
	cfgPeers []Peer,
	planDir string,
	cliVarsAbs []string,
	cliTags []string,
	stepNames []string,
	maxSync int64,
	parallel int,
	controllerID string,
	peerFilter func(Peer) bool,
) (ApplyPhaseInput, error) {
	var found *Peer
	for i := range cfgPeers {
		if cfgPeers[i].Name == mInput.Phase.Peer {
			found = &cfgPeers[i]
			break
		}
	}
	if found == nil {
		return ApplyPhaseInput{}, fmt.Errorf("phase %q: peer %q not in peers.toml", mInput.Phase.Name, mInput.Phase.Peer)
	}
	peers := []Peer{*found}
	if peerFilter != nil {
		filtered := peers[:0]
		for _, p := range peers {
			if peerFilter(p) {
				filtered = append(filtered, p)
			}
		}
		peers = filtered
	}

	// Vars: machine-wide (cliVarsAbs already contains shared+machine vars
	// from the caller's top-level resolution) + phase.Vars (already
	// resolved to absolute paths in LoadMachineManifest).
	varsAbs := make([]string, 0, len(cliVarsAbs)+len(mInput.Phase.Vars))
	varsAbs = append(varsAbs, cliVarsAbs...)
	varsAbs = append(varsAbs, mInput.Phase.Vars...)

	// Tags: phase.Tags ∪ cliTags. Both filter step selection; both applying
	// simultaneously is "show steps tagged with anything in the merged set"
	// (the planner's MatchesTags is a union match).
	tags := make([]string, 0, len(cliTags)+len(mInput.Phase.Tags))
	tags = append(tags, cliTags...)
	tags = append(tags, mInput.Phase.Tags...)

	// Skip non-agentd transports. In machine mode the manifest pins the
	// peer, so a non-agentd peer is operator misconfiguration — but the
	// same code-path handles it gracefully (Skipped banner).
	agentdPeers := make([]Peer, 0, len(peers))
	var skipped []Peer
	for _, p := range peers {
		if p.Transport != TransportAgentd {
			skipped = append(skipped, p)
			continue
		}
		agentdPeers = append(agentdPeers, p)
	}

	planAbs := mInput.Phase.Plan
	if !filepath.IsAbs(planAbs) {
		// LoadMachineManifest resolves Plan relative to the manifest dir, so
		// we shouldn't hit this branch — defensive.
		abs, err := filepath.Abs(planAbs)
		if err != nil {
			return ApplyPhaseInput{}, fmt.Errorf("phase %q: resolve plan: %w", mInput.Phase.Name, err)
		}
		planAbs = abs
	}

	banner := fmt.Sprintf(
		"phase %d/%d — %s · peer %s · plan %s",
		mInput.PhaseNum, mInput.TotalPhases, mInput.Phase.Name, mInput.Phase.Peer, planAbs,
	)

	return ApplyPhaseInput{
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

// RunMachineApply orchestrates ordered multi-phase apply for a machine
// whose machines/<name>/fleet.yml was found. Runs each phase sequentially,
// fail-fast: a phase that doesn't reach "every peer ok" stops subsequent
// phases. Returns the structured outcome; the cmd layer translates it to
// cli.Exit / os.Exit.
//
// peerFilter, if non-nil, is AND'd with each phase's manifest-pinned peer
// (same semantics as ResolveMachinePhase).
func RunMachineApply(
	applyCtx context.Context,
	w io.Writer,
	useColor bool,
	manifest *MachineManifest,
	machineName string,
	cfgPeers []Peer,
	planDir string,
	cliVarsAbs []string,
	cliTags []string,
	stepNames []string,
	maxSync int64,
	parallel int,
	controllerID string,
	peerFilter func(Peer) bool,
) MachineRunResult {
	total := len(manifest.Phases)
	fmt.Fprintf(w, "fleet apply %s: %d phase(s)\n", machineName, total)

	var phaseFailures []string
	for i, phase := range manifest.Phases {
		select {
		case <-applyCtx.Done():
			return MachineRunResult{
				ExitCode: 130,
				Message:  "fleet apply: interrupted before phase " + phase.Name,
			}
		default:
		}

		in, err := ResolveMachinePhase(
			MachinePhaseInput{Phase: phase, PhaseNum: i + 1, TotalPhases: total},
			cfgPeers, planDir, cliVarsAbs, cliTags, stepNames,
			maxSync, parallel, controllerID,
			peerFilter,
		)
		if err != nil {
			return MachineRunResult{
				ExitCode: 2,
				Message:  "fleet apply: " + err.Error(),
			}
		}

		if len(in.Peers) == 0 {
			// --peer filtered out the phase's peer. Don't treat as failure —
			// print a banner and continue, matching how --peer behaves in
			// single-phase mode.
			fmt.Fprintf(w, "phase %d/%d — %s · peer %s skipped by --peer\n",
				i+1, total, phase.Name, phase.Peer)
			continue
		}

		out := RunApplyPhase(applyCtx, w, useColor, in)
		if out.Terminal() {
			// Fail-fast: record the failure, stop subsequent phases.
			if len(out.FailedNames) > 0 {
				phaseFailures = append(phaseFailures, fmt.Sprintf("%s (peer %s)", phase.Name, strings.Join(out.FailedNames, ",")))
			} else {
				phaseFailures = append(phaseFailures, phase.Name)
			}
			break
		}
	}

	if len(phaseFailures) == 0 {
		fmt.Fprintf(w, "fleet apply %s: %d/%d phases ok\n", machineName, total, total)
		return MachineRunResult{ExitCode: 0}
	}
	return MachineRunResult{
		ExitCode:      1,
		Message:       "fleet apply " + machineName + ": failed phase(s): " + strings.Join(phaseFailures, "; "),
		PhaseFailures: phaseFailures,
	}
}
