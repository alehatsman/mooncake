package fleet

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path/filepath"
	"strings"

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

	// Writer receives one line per streamed event in `[peer] message`
	// format. A nil writer discards everything.
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
//  6. Stream events, writing `[peer] message` lines to opts.Writer until
//     the daemon closes the stream (run reached terminal state) or ctx is
//     canceled.
//
// Returns the per-peer result; the run's terminal status is reported via
// Result.Status when a run.completed event was seen. Returns the first
// non-nil error from any step; partial results are still populated where
// useful (e.g. Sync stats even when Submit fails).
func Apply(ctx context.Context, opts ApplyOptions) (ApplyResult, error) {
	result := ApplyResult{PeerName: opts.PeerName}

	if err := validateApplyPaths(opts); err != nil {
		return result, err
	}
	scope, err := ScopeFor(opts.ControllerID, opts.PlanDir)
	if err != nil {
		return result, err
	}

	entries, _, err := Walk(opts.PlanDir, opts.MaxSyncBytes)
	if err != nil {
		return result, fmt.Errorf("walk plan-dir: %w", err)
	}

	ver, err := opts.Peer.GetVersion(ctx)
	if err != nil {
		return result, err
	}

	syncStats, err := SyncTo(ctx, opts.Peer, entries, scope)
	result.Sync = syncStats
	if err != nil {
		return result, fmt.Errorf("sync: %w", err)
	}

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
		return result, err
	}
	result.RunID = runID

	sink := make(chan transport.Event, 64)
	streamErrCh := make(chan error, 1)
	go func() { streamErrCh <- opts.Peer.Stream(ctx, runID, sink) }()

	for {
		select {
		case <-ctx.Done():
			<-streamErrCh
			drainEvents(sink, opts, &result)
			return result, ctx.Err()
		case ev := <-sink:
			result.Events++
			if status, ok := terminalStatus(ev); ok {
				result.Status = status
			}
			if opts.Writer != nil {
				fmt.Fprintln(opts.Writer, formatEvent(opts.PeerName, ev))
			}
		case err := <-streamErrCh:
			// Stream goroutine finished (clean close OR error). Drain any
			// events still buffered in sink before returning — without
			// this, the select can pick streamErrCh before the last events
			// (commonly including run.completed) are read.
			drainEvents(sink, opts, &result)

			// If no run.completed event was seen (e.g. planner setup
			// failed before any events fired), reach back for the run
			// record so we can surface the real status + error. Without
			// this, Apply silently returns Status="" and the user has no
			// idea why their plan didn't run.
			if err == nil && result.Status == "" {
				if rec, rerr := opts.Peer.GetRun(context.WithoutCancel(ctx), runID); rerr == nil {
					result.Status = rec.Status
					if rec.Error != "" && opts.Writer != nil {
						fmt.Fprintf(opts.Writer, "[%s] ✗ run %s: %s\n",
							opts.PeerName, rec.Status, oneLine(rec.Error))
					}
					if rec.Status == "failed" || rec.Status == "interrupted" {
						return result, fmt.Errorf("run %s on %s: %s", rec.Status, opts.PeerName, oneLine(rec.Error))
					}
				}
			}
			return result, err
		}
	}
}

// oneLine collapses a multi-line error into a single-line summary suitable
// for the [peer] log prefix format. Newlines and indentation get squashed
// to spaces; the result is trimmed.
func oneLine(s string) string {
	if s == "" {
		return s
	}
	out := strings.ReplaceAll(s, "\n", " ")
	out = strings.ReplaceAll(out, "\t", " ")
	for strings.Contains(out, "  ") {
		out = strings.ReplaceAll(out, "  ", " ")
	}
	return strings.TrimSpace(out)
}

// drainEvents non-blockingly reads everything currently in sink, updating
// result and writing log lines. Used by Apply when the stream goroutine
// returns to flush any buffered events that arrived just before close.
func drainEvents(sink <-chan transport.Event, opts ApplyOptions, result *ApplyResult) {
	for {
		select {
		case ev := <-sink:
			result.Events++
			if status, ok := terminalStatus(ev); ok {
				result.Status = status
			}
			if opts.Writer != nil {
				fmt.Fprintln(opts.Writer, formatEvent(opts.PeerName, ev))
			}
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

// formatEvent renders one event as a single `[peer] message` line. The
// formatter is intentionally simple in PR5 — it covers run.* / step.*
// events with a human-friendly summary and falls back to a compact-JSON
// debug line for everything else. PR6 / spec-42 can iterate on visual
// polish (colors, padding, etc.).
func formatEvent(peer string, ev transport.Event) string {
	prefix := "[" + peer + "] "
	switch ev.Type {
	case "run.started":
		return prefix + "▶ run started"
	case "run.completed":
		return prefix + "✔ run complete " + summarizeRunCompleted(ev.Data)
	case "plan.loaded":
		return prefix + "plan loaded " + extractField(ev.Data, "step_count", "0") + " steps"
	case "step.started":
		return prefix + "  ▸ " + extractField(ev.Data, "name", "(unnamed step)")
	case "step.completed":
		return prefix + "    ✔ " + extractField(ev.Data, "name", "")
	case "step.skipped":
		return prefix + "    – " + extractField(ev.Data, "name", "") + " (skipped)"
	case "step.failed":
		return prefix + "    ✗ " + extractField(ev.Data, "name", "") + ": " +
			extractField(ev.Data, "error", "(no error message)")
	case "step.stdout", "step.stderr":
		txt := extractField(ev.Data, "line", "")
		if txt == "" {
			return prefix + ev.Type
		}
		return prefix + "      " + txt
	default:
		if len(ev.Data) > 0 {
			return prefix + string(ev.Type) + " " + string(ev.Data)
		}
		return prefix + string(ev.Type)
	}
}

func extractField(data json.RawMessage, key, fallback string) string {
	if len(data) == 0 {
		return fallback
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return fallback
	}
	raw, ok := m[key]
	if !ok {
		return fallback
	}
	// Strings: unquote. Anything else: stringify.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s
	}
	return string(raw)
}

func summarizeRunCompleted(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var m struct {
		Success      bool `json:"success"`
		TotalSteps   int  `json:"total_steps"`
		ChangedSteps int  `json:"changed_steps"`
		FailedSteps  int  `json:"failed_steps"`
		SkippedSteps int  `json:"skipped_steps"`
		DurationMs   int  `json:"duration_ms"`
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return string(data)
	}
	verdict := "success"
	if !m.Success {
		verdict = "failed"
	}
	return fmt.Sprintf("%s: %d/%d changed, %d failed, %d skipped (%dms)",
		verdict, m.ChangedSteps, m.TotalSteps, m.FailedSteps, m.SkippedSteps, m.DurationMs)
}
