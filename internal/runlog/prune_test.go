package runlog

import (
	"os"
	"testing"
	"time"
)

// seedEntries appends n entries spaced one day apart, oldest first,
// ending at `newest`.
func seedEntries(t *testing.T, n int, newest time.Time) {
	t.Helper()
	for i := n - 1; i >= 0; i-- {
		e := Entry{
			TS:     newest.AddDate(0, 0, -i),
			Config: "main.yml",
			Ok:     i,
		}
		if err := Append(e); err != nil {
			t.Fatalf("seed append: %v", err)
		}
	}
}

// `mooncake history gc --keep N` retains the N newest runs and drops the
// rest. Nothing bounded the log before this existed (#176).
func TestPrune_KeepsNewest(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 20, now)

	res, err := Prune(PruneOptions{Keep: 5, Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Before != 20 || res.After != 5 || res.Removed != 15 {
		t.Fatalf("got before=%d after=%d removed=%d, want 20/5/15",
			res.Before, res.After, res.Removed)
	}

	// The survivors must be the NEWEST five, still oldest-first on disk.
	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 5 {
		t.Fatalf("got %d entries on disk, want 5", len(entries))
	}
	if !entries[len(entries)-1].TS.Equal(now) {
		t.Errorf("newest surviving entry is %v, want %v", entries[len(entries)-1].TS, now)
	}
	for i := 1; i < len(entries); i++ {
		if entries[i].TS.Before(entries[i-1].TS) {
			t.Fatalf("entries are not oldest-first after prune: %v then %v",
				entries[i-1].TS, entries[i].TS)
		}
	}
}

// --older-than drops by age rather than count.
func TestPrune_DropsByAge(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 30, now) // one per day, 30 days back

	res, err := Prune(PruneOptions{OlderThan: 7 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.After != 7 {
		t.Errorf("kept %d entries, want 7 (the last week)", res.After)
	}
}

// Both filters compose as AND: an entry must be recent enough AND within
// the newest N.
func TestPrune_FiltersCompose(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 30, now)

	res, err := Prune(PruneOptions{Keep: 3, OlderThan: 10 * 24 * time.Hour, Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.After != 3 {
		t.Errorf("kept %d, want 3 (the tighter of the two limits)", res.After)
	}
}

// An entry with no timestamp carries no age evidence. Pruning is
// destructive, so an undatable entry is kept rather than guessed at.
func TestPrune_KeepsUndatedEntries(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	if err := Append(Entry{Config: "no-timestamp.yml"}); err != nil {
		t.Fatalf("append: %v", err)
	}
	seedEntries(t, 3, now)

	res, err := Prune(PruneOptions{OlderThan: time.Hour, Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	entries, _ := ReadAll()
	found := false
	for _, e := range entries {
		if e.Config == "no-timestamp.yml" {
			found = true
		}
	}
	if !found {
		t.Errorf("undated entry was pruned; kept %d of %d", res.After, res.Before)
	}
}

// A prune that removes nothing must not rewrite the file.
func TestPrune_NoOpLeavesFileAlone(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 3, now)

	path, err := logPath()
	if err != nil {
		t.Fatalf("logPath: %v", err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	res, err := Prune(PruneOptions{Keep: 10, Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Removed != 0 {
		t.Fatalf("removed %d, want 0", res.Removed)
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !before.ModTime().Equal(after.ModTime()) {
		t.Error("no-op prune rewrote the log")
	}
	if res.Bytes != after.Size() {
		t.Errorf("reported %d bytes, file is %d", res.Bytes, after.Size())
	}
}

// PreviewPrune answers the same question without touching the log.
func TestPreviewPrune_DoesNotWrite(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 20, now)

	res, err := PreviewPrune(PruneOptions{Keep: 5, Now: now})
	if err != nil {
		t.Fatalf("PreviewPrune: %v", err)
	}
	if res.Removed != 15 || res.After != 5 {
		t.Errorf("preview says removed=%d after=%d, want 15/5", res.Removed, res.After)
	}

	entries, _ := ReadAll()
	if len(entries) != 20 {
		t.Errorf("preview mutated the log: %d entries remain, want 20", len(entries))
	}
}

// Zero filters mean "keep everything" — gc must never empty the log by
// accident when invoked with explicit zeros.
func TestPrune_NoFiltersKeepsEverything(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	now := time.Date(2026, 9, 7, 12, 0, 0, 0, time.UTC)
	seedEntries(t, 12, now)

	res, err := Prune(PruneOptions{Now: now})
	if err != nil {
		t.Fatalf("Prune: %v", err)
	}
	if res.Removed != 0 || res.After != 12 {
		t.Errorf("removed=%d after=%d, want 0/12", res.Removed, res.After)
	}
}
