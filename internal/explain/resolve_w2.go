package explain

import (
	"errors"

	"github.com/alehatsman/mooncake/internal/ops"
	"github.com/alehatsman/mooncake/internal/runlog"
)

// resolveRun looks up a run by its r/<id> identifier. Linear scan over
// runs.jsonl — see spec-68 §"Source of truth" for the scaling note.
//
// Pre-wave-2 runlog entries don't carry RunID so they can't match here;
// the resolver returns not_found with a hint pointing at the legacy
// `mooncake history` command.
func resolveRun(noun string, opts Options) Result {
	entries, err := readRuns(opts)
	if err != nil {
		return notFound(noun, "could not read run log: "+err.Error(), nil)
	}
	for _, e := range entries {
		if e.RunID == noun {
			return Result{Kind: KindRun, Run: runPayloadFrom(e)}
		}
	}
	return notFound(noun, "no run with that id (pre-spec-68 runs have no run_id; try mooncake history)", nil)
}

// resolveOp looks up an op by its op/<id> identifier in ops.jsonl,
// then derives its Runs list by scanning runs.jsonl for entries whose
// OpID matches. The single-write-FK pattern keeps ops.jsonl additive
// — no row rewrites when a run lands later.
func resolveOp(noun string, opts Options) Result {
	entry, err := readOp(noun, opts)
	if err != nil {
		if errors.Is(err, ops.ErrNotFound) {
			return notFound(noun, "no op with that id", nil)
		}
		return notFound(noun, "could not read ops log: "+err.Error(), nil)
	}

	runIDs, _ := derivedRunIDs(noun, opts)
	return Result{
		Kind: KindOp,
		Op: &OpPayload{
			OpID:     entry.OpID,
			TS:       entry.TS,
			Command:  entry.Command,
			Args:     entry.Args,
			Actor:    entry.Actor,
			Parent:   entry.Parent,
			Config:   entry.Config,
			PlanOnly: entry.PlanOnly,
			Runs:     runIDs,
		},
	}
}

// resolveResource walks every run, filters Steps by Resource, and
// returns a newest-first history. Linear scan — fine for personal
// use; the spec leaves an indexed path for fleet-scale.
func resolveResource(noun string, opts Options) Result {
	entries, err := readRuns(opts)
	if err != nil {
		return notFound(noun, "could not read run log: "+err.Error(), nil)
	}

	var history []ResourceEvent
	for _, e := range entries {
		for _, s := range e.Steps {
			if s.Resource != noun {
				continue
			}
			history = append(history, ResourceEvent{
				RunID:      e.RunID,
				OpID:       e.OpID,
				TS:         e.TS,
				StepIndex:  s.Index,
				Action:     s.Action,
				Result:     s.Result,
				Reversible: s.Reversible,
			})
		}
	}

	// Reverse to newest-first. runs.jsonl is append-only, so the
	// underlying file order is oldest-first; emit the inverse so
	// agents see the most recent change as history[0].
	for i, j := 0, len(history)-1; i < j; i, j = i+1, j-1 {
		history[i], history[j] = history[j], history[i]
	}

	return Result{
		Kind: KindResource,
		Resource: &ResourcePayload{
			Resource: noun,
			History:  history,
		},
	}
}

// runPayloadFrom projects a runlog.Entry into the wire shape.
// Irreversible-step-count is the count of Steps whose Reversible flag
// is false — the operator-facing caveat from spec-68 §run payload.
func runPayloadFrom(e runlog.Entry) *RunPayload {
	steps := make([]RunStep, 0, len(e.Steps))
	irreversible := 0
	for _, s := range e.Steps {
		if !s.Reversible {
			irreversible++
		}
		steps = append(steps, RunStep{
			Index:      s.Index,
			Action:     s.Action,
			Resource:   s.Resource,
			Result:     s.Result,
			DurationMs: s.DurationMs,
			Reversible: s.Reversible,
		})
	}
	return &RunPayload{
		RunID:      e.RunID,
		OpID:       e.OpID,
		TS:         e.TS,
		Config:     e.Config,
		DurationMs: e.DurationMs,
		Totals: RunTotals{
			Changed: e.Changed,
			Ok:      e.Ok,
			Skipped: e.Skipped,
			Failed:  e.Failed,
		},
		Steps: steps,
		Caveats: RunCaveats{
			IrreversibleStepCount: irreversible,
		},
	}
}

func readRuns(opts Options) ([]runlog.Entry, error) {
	if opts.RunsReader != nil {
		return opts.RunsReader()
	}
	return runlog.ReadAll()
}

func readOp(opID string, opts Options) (ops.Entry, error) {
	if opts.OpsReader != nil {
		entries, err := opts.OpsReader()
		if err != nil {
			return ops.Entry{}, err
		}
		for _, e := range entries {
			if e.OpID == opID {
				return e, nil
			}
		}
		return ops.Entry{}, ops.ErrNotFound
	}
	return ops.Read(opID)
}

func derivedRunIDs(opID string, opts Options) ([]string, error) {
	entries, err := readRuns(opts)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.OpID == opID && e.RunID != "" {
			out = append(out, e.RunID)
		}
	}
	return out, nil
}
