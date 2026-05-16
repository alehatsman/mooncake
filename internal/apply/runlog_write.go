package apply

import (
	"encoding/json"
	"time"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/runlog"
)

// writeEnrichedRunlog composes a spec-68-wave-2 runlog entry from the
// captureSubscriber's run totals + executor.RunCapture per-step
// records, and appends it to ~/.mooncake/runs.jsonl.
//
// Called only when the caller minted an op_id (apply.Config.OpID
// non-empty). Legacy callers (no OpID) fall through to the totals-only
// path written by logger.RunLogSubscriber — see runner.go for the
// branch.
//
// Lives in package apply rather than package logger because the
// per-step projection needs config.Step + actions.Reverser, neither
// of which logger can import (actions imports logger transitively).
func writeEnrichedRunlog(configBasename, opID, runID string, tail *captureSubscriber, capture *executor.RunCapture) {
	tail.mu.Lock()
	totals := tail.run
	tail.mu.Unlock()

	okCount := totals.SuccessSteps - totals.ChangedSteps
	if okCount < 0 {
		okCount = 0
	}

	entry := runlog.Entry{
		TS:         time.Now().UTC(),
		Config:     configBasename,
		Changed:    totals.ChangedSteps,
		Ok:         okCount,
		Skipped:    totals.SkippedSteps,
		Failed:     totals.FailedSteps,
		DurationMs: totals.DurationMs,
		RunID:      runID,
		OpID:       opID,
		Steps:      buildStepEntries(capture.Steps()),
	}
	_ = runlog.Append(entry)
}

// buildStepEntries projects executor StepRecords into the runlog
// shape: action verb, best-effort resource handle, result status,
// reversibility flag.
func buildStepEntries(records []executor.StepRecord) []runlog.StepEntry {
	if len(records) == 0 {
		return nil
	}
	out := make([]runlog.StepEntry, 0, len(records))
	for i, sr := range records {
		action := sr.Step.DetermineActionType()
		entry := runlog.StepEntry{
			Index:    i + 1,
			Action:   action,
			Resource: stepResource(sr.Step),
		}
		if sr.Result != nil {
			entry.Result = sr.Result.Status()
			entry.DurationMs = sr.Result.Duration.Milliseconds()
			if !sr.Result.StartTime.IsZero() {
				entry.StartTS = sr.Result.StartTime.UTC()
			}
			// Spec-68 wave 2.5: marshal the apply-time Diff captured by
			// the executor when the handler implements Differ. Stored as
			// RawMessage on the runlog so readers (`mooncake explain`)
			// decode on demand, and so runlog itself stays free of an
			// actions-package dependency. Marshal errors are swallowed
			// — the rest of the StepEntry is still useful.
			if sr.Result.AppliedDiff != nil {
				if raw, mErr := json.Marshal(sr.Result.AppliedDiff); mErr == nil {
					entry.Diff = raw
				}
			}
		} else {
			entry.Result = "ok"
		}
		if h, ok := actions.Get(action); ok {
			if _, reverser := h.(actions.Reverser); reverser {
				entry.Reversible = true
			}
		}
		out = append(out, entry)
	}
	return out
}

// stepResource synthesizes the canonical resource handle for a step,
// best-effort. Returns "" when the action doesn't carry an obvious
// single-resource identifier (vars / shell / observe / wait).
//
// The mapping mirrors the typed Diff.Resource the handler's Differ
// would emit; lifting it from step params here lets
// `explain file:/etc/...` find history entries even when Differ wasn't
// invoked in apply mode (executor.Result currently holds plan-time
// Diff only).
func stepResource(s config.Step) string {
	switch {
	case s.FileWrite != nil && s.FileWrite.Path != "":
		return "file:" + s.FileWrite.Path
	case s.FileTemplate != nil && s.FileTemplate.Dest != "":
		return "file:" + s.FileTemplate.Dest
	case s.FileCopy != nil && s.FileCopy.Dest != "":
		return "file:" + s.FileCopy.Dest
	case s.FileDownload != nil && s.FileDownload.Dest != "":
		return "file:" + s.FileDownload.Dest
	case s.FileUnarchive != nil && s.FileUnarchive.Dest != "":
		return "file:" + s.FileUnarchive.Dest
	case s.Pkg != nil:
		name := s.Pkg.Name
		if name == "" && len(s.Pkg.Names) > 0 {
			name = s.Pkg.Names[0]
		}
		if name == "" {
			return ""
		}
		if s.Pkg.Manager != "" {
			return "pkg:" + s.Pkg.Manager + "/" + name
		}
		return "pkg:" + name
	case s.OsUser != nil && s.OsUser.Name != "":
		return "user:" + s.OsUser.Name
	case s.OsService != nil && s.OsService.Name != "":
		return "service:" + s.OsService.Name
	}
	return ""
}
