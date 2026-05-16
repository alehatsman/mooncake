package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
)

// ApplyConfig holds the inputs for an Orchestrator.Run() — everything
// fleetApplyAction used to derive from cli.Context, plus the
// already-resolved peer selection (cmd-side parses --peer with its
// filterTerm/peerOSResolver types; the result is the SelectedPeers /
// UnknownPeers / PeerFilter fields below).
//
// PeerFilter is the cmd-side adapter used by the machine-manifest path
// (RunMachineApply takes a typeless predicate). nil = accept-all.
type ApplyConfig struct {
	// Plan inputs
	PlanArg     string // bare arg (machine name or plan file path)
	PlanDirHint string // --plan-dir; empty = derive from plan file
	PeersPath   string // --peers-file; empty = DefaultPeersPath()

	// Filter inputs (cmd resolves --peer + --vars-file + --step-filter; the
	// orchestrator just consumes the results)
	SelectedPeers   []Peer // ResolvePeers output, post --peer
	UnknownPeers    []string
	AllPeers        []Peer          // peers.toml full set, for machine-mode dispatch
	PeerFilter      func(Peer) bool // nil = accept-all
	VarsFilesRel    []string        // --vars-file values, raw (relative or absolute)
	StepFilterTags  []string        // resolved tags from --step-filter
	StepFilterNames []string        // resolved names from --step-filter

	// Execution knobs
	MaxSyncBytes int64
	Parallel     int
	NoColor      bool

	// Sinks
	Writer io.Writer

	// CWD override for machine-convention detection. Empty = os.Getwd().
	// Reserved for tests; cmd-side leaves it empty.
	Pwd string
}

// ApplyRunResult is the structured outcome of Orchestrator.Run. The cmd
// layer translates ExitCode != 0 into cli.Exit / os.Exit at the boundary;
// Err (when non-nil) is the wrapped runtime error from RunApplyPhase that
// the legacy CLI surface returned via errors.Join.
type ApplyRunResult struct {
	ExitCode int
	Message  string
	Err      error
}

// Orchestrator is the kernel-side entry point for `fleet apply` and
// `fleet apply <machine>`. Frontends (CLI, future MCP, agent loop) build
// an ApplyConfig and call Run().
type Orchestrator struct {
	cfg *ApplyConfig
}

// NewOrchestrator constructs an Orchestrator around the given config.
// cfg must not be nil; cfg is not deep-copied (caller must not mutate
// after the call).
func NewOrchestrator(cfg *ApplyConfig) *Orchestrator {
	return &Orchestrator{cfg: cfg}
}

// Run drives the full apply cycle: resolves plan-arg/machine convention,
// computes vars stack, installs SIGINT handling, and dispatches to
// RunApplyPhase (single-phase) or RunMachineApply (multi-phase machine
// manifest). Returns the structured result; the cmd boundary maps
// ExitCode/Message to cli.Exit and Err to errors.Join.
func (o *Orchestrator) Run(ctx context.Context) ApplyRunResult {
	planAbs, planDir, machine, machineManifest, res := o.resolvePlan()
	if res.ExitCode != 0 || res.Err != nil {
		return res
	}

	controllerID, err := EnsureControllerID()
	if err != nil {
		return ApplyRunResult{Err: fmt.Errorf("controller id: %w", err)}
	}

	varsAbs := o.resolveVars(planDir, machine)

	w := o.cfg.Writer
	useColor := ShouldColor(w, o.cfg.NoColor)

	applyCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	o.installSignalHandler(applyCtx, cancel, w)

	if machineManifest != nil {
		mr := RunMachineApply(
			applyCtx, w, useColor,
			machineManifest, machine, o.cfg.AllPeers,
			planDir, varsAbs, o.cfg.StepFilterTags, o.cfg.StepFilterNames,
			o.cfg.MaxSyncBytes, o.cfg.Parallel, controllerID,
			o.cfg.PeerFilter,
		)
		return ApplyRunResult{ExitCode: mr.ExitCode, Message: mr.Message}
	}

	return o.runSinglePhase(applyCtx, w, useColor, planAbs, planDir, varsAbs, controllerID)
}

// resolvePlan resolves PlanArg into (planAbs, planDir, machine name,
// machineManifest). Detects the machine convention (bare name → look
// for machines/<name>/{fleet.yml, index.yml}). Returns a non-zero
// ApplyRunResult on resolution failure.
func (o *Orchestrator) resolvePlan() (planAbs, planDir, machine string, manifest *MachineManifest, res ApplyRunResult) {
	planArg := o.cfg.PlanArg
	planDir = o.cfg.PlanDirHint
	if planDir != "" {
		abs, err := filepath.Abs(planDir)
		if err != nil {
			res = ApplyRunResult{Err: fmt.Errorf("resolve plan-dir: %w", err)}
			return
		}
		planDir = abs
	}

	if !strings.ContainsAny(planArg, "/\\") && !strings.HasSuffix(planArg, ".yml") {
		root := planDir
		if root == "" {
			if o.cfg.Pwd != "" {
				root = o.cfg.Pwd
			} else {
				pwd, _ := os.Getwd()
				root = pwd
			}
		}
		manifestPath, found, lookupErr := LookupMachineManifest(root, planArg)
		if lookupErr != nil {
			res = ApplyRunResult{ExitCode: 2, Message: "fleet apply: " + lookupErr.Error()}
			return
		}
		if found {
			m, err := LoadMachineManifest(manifestPath)
			if err != nil {
				res = ApplyRunResult{ExitCode: 2, Message: "fleet apply: " + err.Error()}
				return
			}
			machine = planArg
			manifest = m
			if planDir == "" {
				planDir = root
			}
			planArg = manifestPath
		} else {
			entry := filepath.Join(root, "machines", planArg, "index.yml")
			if st, err := os.Stat(entry); err == nil && !st.IsDir() {
				machine = planArg
				planArg = entry
				if planDir == "" {
					planDir = root
				}
			}
		}
	}

	abs, err := filepath.Abs(planArg)
	if err != nil {
		res = ApplyRunResult{Err: fmt.Errorf("resolve plan path: %w", err)}
		return
	}
	planAbs = abs
	if planDir == "" {
		planDir = filepath.Dir(planAbs)
	}
	return
}

// resolveVars merges --vars-file inputs with the machine-convention
// conventional vars (shared/variables.yml + machines/<machine>/vars.yml
// when they exist on disk). The shared/machine files prepend so explicit
// --vars-file overrides win on key collision (later-wins).
func (o *Orchestrator) resolveVars(planDir, machine string) []string {
	varsRel := o.cfg.VarsFilesRel
	if machine != "" {
		conv := []string{
			filepath.Join("shared", "variables.yml"),
			filepath.Join("machines", machine, "vars.yml"),
		}
		merged := make([]string, 0, len(conv)+len(varsRel))
		for _, p := range conv {
			abs := filepath.Join(planDir, p)
			if _, err := os.Stat(abs); err == nil {
				merged = append(merged, p)
			}
		}
		merged = append(merged, varsRel...)
		varsRel = merged
	}
	out := make([]string, 0, len(varsRel))
	for _, v := range varsRel {
		if !filepath.IsAbs(v) {
			v = filepath.Join(planDir, v)
		}
		out = append(out, filepath.Clean(v))
	}
	return out
}

// installSignalHandler wires SIGINT/SIGTERM to cancel applyCtx and print
// a banner; a second signal hard-exits 130. Same shape fleetApplyAction
// owned previously — moved verbatim so the goroutine topology is
// byte-identical.
func (o *Orchestrator) installSignalHandler(applyCtx context.Context, cancel context.CancelFunc, w io.Writer) {
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
			case <-applyCtx.Done():
			}
		case <-applyCtx.Done():
		}
	}()
}

// runSinglePhase handles the non-machine-manifest path: split SelectedPeers
// into agentd-transport and skipped, bail out if none remain, run one
// phase via RunApplyPhase, translate outcome to ApplyRunResult.
func (o *Orchestrator) runSinglePhase(applyCtx context.Context, w io.Writer, useColor bool, planAbs, planDir string, varsAbs []string, controllerID string) ApplyRunResult {
	agentdPeers := make([]Peer, 0, len(o.cfg.SelectedPeers))
	var skippedPeers []Peer
	for _, p := range o.cfg.SelectedPeers {
		if p.Transport != TransportAgentd {
			skippedPeers = append(skippedPeers, p)
			continue
		}
		agentdPeers = append(agentdPeers, p)
	}
	if len(agentdPeers) == 0 {
		return ApplyRunResult{ExitCode: 1, Message: "fleet apply: no agentd-transport peers selected"}
	}

	out := RunApplyPhase(applyCtx, w, useColor, ApplyPhaseInput{
		PlanAbs:       planAbs,
		PlanDir:       planDir,
		Peers:         agentdPeers,
		UnknownPeers:  o.cfg.UnknownPeers,
		SkippedPeers:  skippedPeers,
		VarsAbs:       varsAbs,
		Tags:          o.cfg.StepFilterTags,
		StepNames:     o.cfg.StepFilterNames,
		MaxSyncBytes:  o.cfg.MaxSyncBytes,
		Parallel:      o.cfg.Parallel,
		ControllerID:  controllerID,
		BannerHeading: fmt.Sprintf("fleet apply: %s → %d peer(s)", planAbs, len(agentdPeers)),
	})

	switch {
	case out.Unreachable > 0:
		return ApplyRunResult{ExitCode: 2, Message: "fleet apply: unreachable peer(s): " + strings.Join(out.FailedNames, ", ")}
	case out.RunFailed > 0:
		return ApplyRunResult{ExitCode: 1, Message: "fleet apply: failed on peer(s): " + strings.Join(out.FailedNames, ", ")}
	}
	if out.FirstErr != nil {
		return ApplyRunResult{Err: errors.Join(out.FirstErr)}
	}
	return ApplyRunResult{}
}

// NoPeersConfiguredError formats the message fleetApplyAction used to emit
// when peers.toml exists but is empty. Exported for cmd-side use so the
// path-specific message stays consistent.
func NoPeersConfiguredError(peersPath string) string {
	return "fleet apply: no peers configured. Run `mooncake fleet bootstrap` or edit " + peersPath
}

// NoPeersSelectedError formats the message cmd-side returns when --peer
// matched zero peers out of `total`. Exported for cmd-side use.
func NoPeersSelectedError(total int, unknown []string) string {
	msg := "fleet apply: --peer selected 0 of " + strconv.Itoa(total) + " peer(s); nothing to do"
	if len(unknown) > 0 {
		msg += " (unknown: " + strings.Join(unknown, ", ") + ")"
	}
	return msg
}
