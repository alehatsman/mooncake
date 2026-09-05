// Package runlog persists a compact record of every mooncake run to
// ~/.mooncake/runs.jsonl and provides a reader for the most recent entry.
package runlog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/jsonllog"

	"github.com/alehatsman/mooncake/internal/statedir"
)

// ErrNoHistory is returned by Last/Recent/At when the log file is absent
// or contains no valid entries.
var ErrNoHistory = errors.New("no run history")

// ErrIndexOutOfRange is returned by At when the 1-based newest-first index
// falls outside the available entries.
var ErrIndexOutOfRange = errors.New("history index out of range")

// Entry is a single run record written to the log.
//
// Schema is additive: fields below the legacy totals (RunID, OpID,
// Steps) were added with spec-68 wave 2. Pre-wave-2 readers ignore
// unknown fields; pre-wave-2 entries decode with the new fields at
// their zero values, which `explain` treats as "no detail available
// for this entry."
type Entry struct {
	TS         time.Time `json:"ts"`
	Config     string    `json:"config"`
	Changed    int       `json:"changed"`
	Ok         int       `json:"ok"`
	Skipped    int       `json:"skipped"`
	Failed     int       `json:"failed"`
	DurationMs int64     `json:"duration_ms"`

	// Spec-68 wave 2 additions: run/op identity + per-step detail.
	RunID string      `json:"run_id,omitempty"`
	OpID  string      `json:"op_id,omitempty"`
	Steps []StepEntry `json:"steps,omitempty"`
}

// StepEntry is one row in Entry.Steps — the post-apply record of a
// single step. Best-effort: Resource is synthesized from common step
// params (path / name / dest / unit) per-action.
//
// Spec-68 wave 2.5 widens this with StartTS (per-step start time,
// previously only DurationMs was carried) and Diff (the typed
// actions.Diff captured at apply time when the handler implements
// Differ — see executor.Result.AppliedDiff). Diff is stored as
// json.RawMessage so the runlog package stays free of an actions
// dependency; readers (explain, history) decode on demand.
//
// Pre-spec-68 entries decode with these fields at zero values; both
// carry omitempty so old + new shapes coexist on the wire.
type StepEntry struct {
	Index      int       `json:"index"`
	Action     string    `json:"action"`
	Resource   string    `json:"resource,omitempty"`
	Result     string    `json:"result"` // changed | ok | skipped | failed
	StartTS    time.Time `json:"start_ts,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Reversible bool      `json:"reversible,omitempty"`
	// Reverted is true when this step's mutation was undone by a
	// transaction rollback's LIFO Reverse() pass (F054 / spec-30).
	// The original action ran (Result = "changed") then the inverse
	// undid it; both facts are surfaced to `mooncake history` so
	// the operator can see "X ran, then was rolled back" without
	// stitching two log lines. Reversible can be true here too —
	// they're orthogonal: Reversible says "the handler declared
	// Reverse"; Reverted says "we actually called it on this run".
	Reverted bool            `json:"reverted,omitempty"`
	Diff     json.RawMessage `json:"diff,omitempty"`
	// Error and ExitCode are set only for failed/cancelled steps. Without
	// them a failed record answers "which step" but not "why", which is
	// half of what a post-mortem needs. Error is capped by the writer —
	// the untruncated text (plus full stdout/stderr) rides the step.failed
	// event; this copy exists so `runs.jsonl` alone stays diagnosable.
	Error    string `json:"error,omitempty"`
	ExitCode int    `json:"exit_code,omitempty"`
}

// logPath returns the path to the run log file under the state dir
// (~/.mooncake, or $MOONCAKE_HOME). Errors when a test binary would
// write to the developer's own history — see statedir.ErrTestIsolation.
func logPath() (string, error) {
	return statedir.Path("runs.jsonl")
}

// Append writes e as a single JSON line to ~/.mooncake/runs.jsonl.
// The directory is created if it does not exist. Write failures are
// returned but callers should treat them as best-effort.
func Append(e Entry) error {
	path, err := logPath()
	if err != nil {
		return err
	}
	return jsonllog.Append(path, e)
}

// Last returns the most recent entry in the log.
// Returns ErrNoHistory when the file does not exist or contains no valid lines.
func Last() (Entry, error) {
	entries, err := readAll()
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, ErrNoHistory
	}
	return entries[len(entries)-1], nil
}

// Recent returns up to n most-recent entries, ordered NEWEST-FIRST.
// Returns ErrNoHistory when the log is missing or empty. n <= 0 yields
// the full history.
func Recent(n int) ([]Entry, error) {
	entries, err := readAll()
	if err != nil {
		return nil, err
	}
	if len(entries) == 0 {
		return nil, ErrNoHistory
	}
	reversed := make([]Entry, len(entries))
	for i, e := range entries {
		reversed[len(entries)-1-i] = e
	}
	if n > 0 && n < len(reversed) {
		reversed = reversed[:n]
	}
	return reversed, nil
}

// At returns the entry at the given 1-based newest-first index
// (At(1) == Last()). Returns ErrIndexOutOfRange when index < 1 or
// index > total entries. Returns ErrNoHistory when the log is missing
// or empty regardless of index.
func At(index int) (Entry, error) {
	entries, err := readAll()
	if err != nil {
		return Entry{}, err
	}
	if len(entries) == 0 {
		return Entry{}, ErrNoHistory
	}
	if index < 1 || index > len(entries) {
		return Entry{}, ErrIndexOutOfRange
	}
	return entries[len(entries)-index], nil
}

// ReadAll parses runs.jsonl into a slice in OLDEST-FIRST order (the
// file's native order — entries are appended). Invalid lines are
// silently skipped so a single garbled write doesn't poison the log.
// A missing file returns (nil, nil), not ErrNoHistory — callers
// decide whether emptiness is meaningful.
//
// Exported in spec-68 wave 2 so the `mooncake explain r/<id>`
// resolver can scan the full log directly without going through
// the newest-first wrappers.
func ReadAll() ([]Entry, error) {
	return readAll()
}

// readAll is the internal implementation used by Last / Recent / At.
// Kept unexported so refactors of the read path stay private.
func readAll() ([]Entry, error) {
	path, err := logPath()
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path) // #nosec G304 -- path derived from os.UserHomeDir
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("cannot open run log: %w", err)
	}
	defer func() { _ = f.Close() }()

	var out []Entry
	scanner := bufio.NewScanner(f)
	// Default token size is 64KB which is plenty for a one-line summary;
	// raise the cap just in case a Config path is unusually long.
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var e Entry
		if json.Unmarshal(line, &e) == nil {
			out = append(out, e)
		}
	}
	return out, scanner.Err()
}
