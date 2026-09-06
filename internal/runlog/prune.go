package runlog

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// PruneOptions selects which entries survive a Prune. Both filters are
// optional; when both are set an entry must satisfy both to be kept.
type PruneOptions struct {
	// Keep is the number of newest entries to retain. <= 0 means no
	// count limit.
	Keep int

	// OlderThan drops entries whose timestamp is further in the past
	// than this duration. <= 0 means no age limit.
	OlderThan time.Duration

	// Now overrides the clock for age comparisons. Zero means
	// time.Now(). Tests set this.
	Now time.Time
}

// PruneResult reports what a Prune did, so the CLI can say something
// concrete instead of "done".
type PruneResult struct {
	Before  int   // entries in the log before pruning
	After   int   // entries retained
	Removed int   // Before - After
	Bytes   int64 // size of the rewritten log
}

// Prune rewrites runs.jsonl retaining only the entries that satisfy
// opts, and returns what changed.
//
// jsonllog rotation bounds the log's growth automatically; Prune is the
// explicit control on top of it, for an operator who wants the history
// short now rather than at the next roll (#176).
//
// The rewrite is atomic (temp file in the same directory, then rename),
// so an interrupted prune leaves the original log intact rather than a
// half-written one.
func Prune(opts PruneOptions) (PruneResult, error) {
	path, err := logPath()
	if err != nil {
		return PruneResult{}, err
	}

	entries, err := readAll()
	if err != nil {
		return PruneResult{}, err
	}
	res := PruneResult{Before: len(entries)}

	kept := filterEntries(entries, opts)
	res.After = len(kept)
	res.Removed = res.Before - res.After

	// Nothing to do — don't rewrite the file for a no-op.
	if res.Removed == 0 {
		if info, statErr := os.Stat(path); statErr == nil {
			res.Bytes = info.Size()
		}
		return res, nil
	}

	written, err := writeEntriesAtomic(path, kept)
	if err != nil {
		return PruneResult{}, err
	}
	res.Bytes = written
	return res, nil
}

// PreviewPrune reports what Prune would do without touching the log.
// Backs `mooncake history gc --dry-run`.
func PreviewPrune(opts PruneOptions) (PruneResult, error) {
	entries, err := readAll()
	if err != nil {
		return PruneResult{}, err
	}
	kept := filterEntries(entries, opts)
	res := PruneResult{
		Before:  len(entries),
		After:   len(kept),
		Removed: len(entries) - len(kept),
	}
	if path, pErr := logPath(); pErr == nil {
		if info, sErr := os.Stat(path); sErr == nil {
			res.Bytes = info.Size()
		}
	}
	return res, nil
}

// filterEntries applies the age filter then the count filter. Entries
// arrive oldest-first (readAll's order) and leave in that same order, so
// the rewritten log keeps the append-only shape every reader expects.
func filterEntries(entries []Entry, opts PruneOptions) []Entry {
	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	kept := entries
	if opts.OlderThan > 0 {
		cutoff := now.Add(-opts.OlderThan)
		kept = kept[:0:0] // fresh backing array; don't clobber entries
		for _, e := range entries {
			// A zero timestamp carries no age information. Keeping it is
			// the conservative choice: pruning is destructive, and an
			// entry we can't date is not evidence that it's old.
			if e.TS.IsZero() || e.TS.After(cutoff) {
				kept = append(kept, e)
			}
		}
	}
	if opts.Keep > 0 && len(kept) > opts.Keep {
		kept = kept[len(kept)-opts.Keep:]
	}
	return kept
}

// writeEntriesAtomic rewrites path with entries, one JSON object per
// line, via a same-directory temp file and a rename. Returns the byte
// size of the new file.
func writeEntriesAtomic(path string, entries []Entry) (int64, error) {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".runs.jsonl.tmp-*")
	if err != nil {
		return 0, fmt.Errorf("cannot create temp log: %w", err)
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // no-op once the rename succeeds

	enc := json.NewEncoder(tmp)
	for _, e := range entries {
		if err := enc.Encode(e); err != nil {
			tmp.Close()
			return 0, fmt.Errorf("cannot write log entry: %w", err)
		}
	}
	info, statErr := tmp.Stat()
	if err := tmp.Close(); err != nil {
		return 0, fmt.Errorf("cannot close temp log: %w", err)
	}
	if statErr != nil {
		return 0, fmt.Errorf("cannot size temp log: %w", statErr)
	}
	// Match the 0o600 the appender creates the log with; CreateTemp
	// already uses 0o600, but be explicit rather than rely on it.
	if err := os.Chmod(tmpName, 0o600); err != nil {
		return 0, fmt.Errorf("cannot chmod temp log: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return 0, fmt.Errorf("cannot replace log: %w", err)
	}
	return info.Size(), nil
}
