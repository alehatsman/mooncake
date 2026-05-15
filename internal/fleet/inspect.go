package fleet

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// State summarises a peer's overall health in one of a few words. Used as
// the value of the STATE column in `fleet status` and as the dispatch
// signal for color + exit-code logic.
type State string

const (
	// StateOK means the peer responded to all probes and has no
	// in-flight or failed run.
	StateOK State = "ok"
	// StateRunning means the peer is reachable AND has a run in flight
	// (runs_running > 0 OR the latest run record is non-terminal).
	StateRunning State = "running"
	// StateFailed means the peer is reachable AND the latest terminal
	// run failed or was interrupted.
	StateFailed State = "failed"
	// StateUnreachable means at least one of the three probes errored
	// (network, auth, timeout). The peer is opaque to the controller.
	StateUnreachable State = "unreachable"
)

// Status is one row of `fleet status` output, serialisable to JSON for
// the --json flag. Fields are intentionally string-typed where the table
// needs a presentational dash; the JSON consumer can re-derive structure.
type Status struct {
	// Name is the peer's name from peers.toml.
	Name string `json:"name"`
	// Addr is the peer's `host:port`.
	Addr string `json:"addr"`
	// State is one of the State constants above. Drives exit-code logic
	// and color, but no longer rendered as a single column — see
	// Accessible / Running for the table-facing booleans.
	State State `json:"state"`

	// Accessible reports whether agentd responded to the gating
	// /v1/version probe. False ↔ State == StateUnreachable.
	Accessible bool `json:"accessible"`
	// Running reports whether the peer has a run in flight at the time
	// of the probe (RunsRunning > 0 or the latest record is
	// non-terminal). Always false when Accessible is false.
	Running bool `json:"running"`

	// OS is the peer's reported os + version (e.g. "ubuntu 24.04",
	// "darwin 14.4"). Empty if the facts probe failed.
	OS string `json:"os,omitempty"`
	// Arch is the peer's reported architecture (e.g. "amd64",
	// "arm64"). Empty if the facts probe failed.
	Arch string `json:"arch,omitempty"`

	// Mooncake is the agentd version string from /v1/version. Empty
	// when the version probe failed.
	Mooncake string `json:"mooncake,omitempty"`
	// QueueDepth is the number of queued (not-yet-started) runs on
	// the peer. -1 when the version probe failed (distinguishes
	// "no queue" from "didn't ask").
	QueueDepth int `json:"queue_depth"`
	// RunsRunning is the number of in-flight runs on the peer.
	RunsRunning int `json:"runs_running"`

	// LastRunStatus is the status of the most recent run record,
	// or empty if no runs exist.
	LastRunStatus string `json:"last_run_status,omitempty"`
	// LastRunAge is a humanised time-since-finished string ("2m ago",
	// "18h ago", "in flight"). Empty when the peer has no runs.
	LastRunAge string `json:"last_run_age,omitempty"`

	// Error is the first probe error encountered; only populated when
	// State == StateUnreachable.
	Error string `json:"error,omitempty"`

	// ProbeDuration is how long the slowest of the three probes took.
	// Useful for diagnosing slow peers and surfaced in --json.
	ProbeDuration time.Duration `json:"probe_duration_ns"`

	// LastSeenAt is the controller's persisted timestamp of the most
	// recent successful version probe for this peer. Zero when there
	// is no prior contact on this controller. The table renderer
	// surfaces "last seen Xh ago" in the per-peer footnote when the
	// current probe failed AND LastSeenAt is non-zero, so the user
	// can tell "freshly broken" apart from "never worked".
	LastSeenAt time.Time `json:"last_seen_at,omitempty"`
}

// Probe runs the three status GETs against peer in parallel and returns
// one Status row. The Status is always populated — even on full failure
// we return a row with State=Unreachable and an Error message, so
// callers can render every configured peer instead of dropping silent
// failures.
//
// timeout bounds each individual GET (and the overall probe by
// extension). Pass 0 for a sensible default (3s).
func Probe(ctx context.Context, name, addr, token string, timeout time.Duration) Status {
	if timeout <= 0 {
		timeout = 3 * time.Second
	}
	pCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := transport.New(name, addr, token)
	out := Status{Name: name, Addr: addr, QueueDepth: -1, RunsRunning: -1}

	// Seed last-seen from persisted state. Best-effort: a missing or
	// poisoned state file is not a probe failure — we just leave
	// LastSeenAt zero and continue.
	if prior, err := LoadPeerState(name); err == nil {
		out.LastSeenAt = prior.LastSeenAt
	}

	type result struct {
		ver   *transport.Version
		verErr error
		runs  []transport.RunRecord
		runsErr error
		facts map[string]any
		factsErr error
	}
	var r result
	start := time.Now()

	var wg sync.WaitGroup
	wg.Add(3)
	go func() { defer wg.Done(); r.ver, r.verErr = client.GetVersion(pCtx) }()
	go func() { defer wg.Done(); r.runs, r.runsErr = client.ListRuns(pCtx, 1) }()
	go func() { defer wg.Done(); r.facts, r.factsErr = client.GetFacts(pCtx) }()
	wg.Wait()
	out.ProbeDuration = time.Since(start)

	// Treat the version probe as the gating call: if it fails, the peer is
	// effectively offline as far as the controller is concerned. ListRuns
	// and GetFacts failures degrade individual columns but don't blank the
	// whole row.
	if r.verErr != nil {
		out.State = StateUnreachable
		out.Error = r.verErr.Error()
		return out
	}
	out.Mooncake = r.ver.Version
	out.QueueDepth = r.ver.QueueDepth
	out.RunsRunning = r.ver.RunsRunning

	// Facts → OS + arch. Best-effort: if the call failed, leave the
	// columns empty but DON'T flip State to unreachable.
	if r.factsErr == nil {
		out.OS = describeOS(r.facts)
		if a, _ := r.facts["arch"].(string); a != "" {
			out.Arch = a
		}
	}

	// Runs → last run status + age. If the call failed, leave the
	// LAST RUN column empty.
	if r.runsErr == nil && len(r.runs) > 0 {
		last := r.runs[0]
		out.LastRunStatus = last.Status
		out.LastRunAge = humanRunAge(last)
	}

	// State precedence: running > failed > ok.
	switch {
	case r.ver.RunsRunning > 0:
		out.State = StateRunning
	case isNonTerminal(out.LastRunStatus):
		out.State = StateRunning
	case isTerminalFailure(out.LastRunStatus):
		out.State = StateFailed
	default:
		out.State = StateOK
	}
	out.Accessible = true
	out.Running = out.State == StateRunning

	// Reachable → persist a fresh contact stamp. We capture the moment
	// the version probe completed (not Probe entry) so the timestamp
	// matches the data it summarises. Best-effort: a write failure
	// (read-only home, ENOSPC) shouldn't fail the probe.
	now := time.Now().UTC()
	out.LastSeenAt = now
	_ = SavePeerState(name, PeerState{
		LastSeenAt:   now,
		LastAddr:     addr,
		LastMooncake: r.ver.Version,
	})
	return out
}

// ProbeAll runs Probe across peers in parallel, capped by maxParallel
// (0 = unbounded). Order of the result matches the input order so the
// caller can preserve peers.toml ordering in the table.
func ProbeAll(ctx context.Context, peers []Peer, timeout time.Duration, maxParallel int) []Status {
	out := make([]Status, len(peers))
	var sem chan struct{}
	if maxParallel > 0 {
		sem = make(chan struct{}, maxParallel)
	}
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func(i int, p Peer) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			out[i] = Probe(ctx, p.Name, p.Addr, p.Token, timeout)
		}(i, p)
	}
	wg.Wait()
	return out
}

// describeOS combines a facts map's "os" + "os_version" into the single
// "ubuntu 24.04" / "darwin 14.4" / "windows 10.0" string used in the
// table. Returns the OS name alone when no version is reported (e.g.
// agentd built from a different facts revision).
func describeOS(facts map[string]any) string {
	os, _ := facts["os"].(string)
	ver, _ := facts["os_version"].(string)
	switch {
	case os == "" && ver == "":
		return ""
	case ver == "":
		return os
	case os == "":
		return ver
	default:
		return os + " " + ver
	}
}

// humanRunAge picks the most-relevant timestamp on a run (finished, else
// started, else queued) and formats time-since as "2m ago" / "18h ago".
// Returns "in flight" when no terminal timestamp exists; callers display
// this only when the run is non-terminal.
func humanRunAge(r transport.RunRecord) string {
	if !isTerminal(r.Status) {
		return "in flight"
	}
	ts := pickTimestamp(r.FinishedAt, r.StartedAt, r.QueuedAt)
	if ts.IsZero() {
		return ""
	}
	return humanDuration(time.Since(ts)) + " ago"
}

func pickTimestamp(stamps ...string) time.Time {
	for _, s := range stamps {
		if s == "" {
			continue
		}
		if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
			return t
		}
		if t, err := time.Parse(time.RFC3339, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// humanDuration is a deliberately rough time-since formatter. We don't
// need millisecond precision in a status table; we want short, glanceable
// labels: "5s", "2m", "1h", "3d", "2w".
func humanDuration(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	default:
		return fmt.Sprintf("%dw", int(d.Hours()/(24*7)))
	}
}

func isTerminal(s string) bool {
	switch s {
	case "success", "failed", "interrupted":
		return true
	}
	return false
}

func isNonTerminal(s string) bool { return s != "" && !isTerminal(s) }

func isTerminalFailure(s string) bool {
	return s == "failed" || s == "interrupted"
}

// FetchOpts configures a FetchRuns / FetchRunsAll call (spec-54).
type FetchOpts struct {
	// Statuses, if non-empty, restricts results. Multi-value triggers
	// one daemon call per status (the daemon supports a single
	// `?status=` query param). Empty = all statuses.
	Statuses []string
	// LimitPerPeer caps per-peer records. <=0 lets the daemon decide.
	LimitPerPeer int
	// Timeout bounds each per-peer call.
	Timeout time.Duration
	// MaxParallel caps in-flight peer probes. 0 = unbounded.
	MaxParallel int
}

// PeerRuns is the result of FetchRuns for one peer, mirroring the
// PeerEvent shape so unreachable peers surface alongside data.
type PeerRuns struct {
	Name  string
	Runs  []transport.RunRecord
	Error error
}

// FetchRuns asks one peer for its run records, honoring the spec-54
// status/limit filters. Multi-status invocations issue one daemon
// request per status and merge client-side, sorted newest-first by
// pickTimestamp().
func FetchRuns(ctx context.Context, peer Peer, opts FetchOpts) ([]transport.RunRecord, error) {
	client := transport.New(peer.Name, peer.Addr, peer.Token)
	statuses := opts.Statuses
	if len(statuses) == 0 {
		statuses = []string{""}
	}
	probeCtx := ctx
	if opts.Timeout > 0 {
		var cancel context.CancelFunc
		probeCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
		defer cancel()
	}
	var all []transport.RunRecord
	for _, s := range statuses {
		runs, err := client.ListRunsWith(probeCtx, transport.ListRunsOpts{
			Status: s,
			Limit:  opts.LimitPerPeer,
		})
		if err != nil {
			return nil, fmt.Errorf("list runs (status=%q): %w", s, err)
		}
		all = append(all, runs...)
	}
	if len(statuses) > 1 {
		// One-call-per-status concat: sort merged result newest-first
		// using the same pickTimestamp() recipe as humanRunAge.
		sortRunsNewestFirst(all)
	}
	return all, nil
}

// FetchRunsAll fans FetchRuns out across peers in parallel, capped by
// opts.MaxParallel. Returns one PeerRuns per input peer in input order
// (unreachable peers carry Error != nil rather than being dropped).
func FetchRunsAll(ctx context.Context, peers []Peer, opts FetchOpts) []PeerRuns {
	out := make([]PeerRuns, len(peers))
	var sem chan struct{}
	if opts.MaxParallel > 0 {
		sem = make(chan struct{}, opts.MaxParallel)
	}
	var wg sync.WaitGroup
	for i, p := range peers {
		wg.Add(1)
		go func(i int, p Peer) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			runs, err := FetchRuns(ctx, p, opts)
			out[i] = PeerRuns{Name: p.Name, Runs: runs, Error: err}
		}(i, p)
	}
	wg.Wait()
	return out
}

// sortRunsNewestFirst sorts a merged multi-status slice by best
// available timestamp (finished > started > queued), most-recent first.
// Stable so within-status order from the daemon is preserved on ties.
func sortRunsNewestFirst(runs []transport.RunRecord) {
	// Decorate with the pick timestamp once to avoid re-parsing inside Less.
	type stamped struct {
		ts time.Time
		i  int
	}
	stamps := make([]stamped, len(runs))
	for i, r := range runs {
		stamps[i] = stamped{ts: pickTimestamp(r.FinishedAt, r.StartedAt, r.QueuedAt), i: i}
	}
	// Simple insertion sort — N is small (typically < 50), avoids the
	// stdlib sort overhead and keeps the algorithm easy to reason about.
	for i := 1; i < len(stamps); i++ {
		for j := i; j > 0 && stamps[j].ts.After(stamps[j-1].ts); j-- {
			stamps[j], stamps[j-1] = stamps[j-1], stamps[j]
		}
	}
	sorted := make([]transport.RunRecord, len(runs))
	for i, s := range stamps {
		sorted[i] = runs[s.i]
	}
	copy(runs, sorted)
}
