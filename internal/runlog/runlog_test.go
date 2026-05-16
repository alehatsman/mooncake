package runlog

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAppendAndLast(t *testing.T) {
	// Point the log at a temp dir for the test.
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// No history yet.
	if _, err := Last(); err != ErrNoHistory {
		t.Fatalf("expected ErrNoHistory, got %v", err)
	}

	e1 := Entry{
		TS:         time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC),
		Config:     "main.yml",
		Changed:    3,
		Ok:         10,
		Skipped:    1,
		Failed:     0,
		DurationMs: 5000,
	}
	if err := Append(e1); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := Last()
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if got.Config != e1.Config || got.Changed != e1.Changed {
		t.Errorf("Last returned wrong entry: %+v", got)
	}

	// A second entry becomes the new last.
	e2 := Entry{
		TS:         time.Date(2026, 1, 2, 10, 0, 0, 0, time.UTC),
		Config:     "arch.yml",
		Changed:    0,
		Ok:         74,
		Skipped:    3,
		Failed:     0,
		DurationMs: 18200,
	}
	if err := Append(e2); err != nil {
		t.Fatalf("Append second: %v", err)
	}

	got2, err := Last()
	if err != nil {
		t.Fatalf("Last after second append: %v", err)
	}
	if got2.Config != "arch.yml" {
		t.Errorf("expected arch.yml, got %s", got2.Config)
	}
}

func TestAppendCreatesDir(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	if err := Append(Entry{Config: "test.yml", TS: time.Now()}); err != nil {
		t.Fatalf("Append: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".mooncake", "runs.jsonl")); err != nil {
		t.Errorf("log file not created: %v", err)
	}
}

// seedRunLog appends n entries with distinct configs and predictable
// timestamps so Recent/At test ordering can be checked precisely.
func seedRunLog(t *testing.T, n int) {
	t.Helper()
	base := time.Date(2026, 1, 1, 10, 0, 0, 0, time.UTC)
	for i := 0; i < n; i++ {
		e := Entry{
			TS:         base.Add(time.Duration(i) * time.Hour),
			Config:     "run-" + string(rune('a'+i)) + ".yml",
			Changed:    i,
			Ok:         10 + i,
			DurationMs: int64(100 * (i + 1)),
		}
		if err := Append(e); err != nil {
			t.Fatalf("seed Append %d: %v", i, err)
		}
	}
}

func TestRecent_NewestFirst(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	seedRunLog(t, 5)

	got, err := Recent(3)
	if err != nil {
		t.Fatalf("Recent: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(got))
	}
	// run-e is the 5th seed (i=4, latest); run-d is i=3; run-c is i=2.
	want := []string{"run-e.yml", "run-d.yml", "run-c.yml"}
	for i, e := range got {
		if e.Config != want[i] {
			t.Errorf("Recent[%d].Config = %q, want %q", i, e.Config, want[i])
		}
	}
}

func TestRecent_AllWhenNZero(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	seedRunLog(t, 3)

	got, err := Recent(0)
	if err != nil {
		t.Fatalf("Recent(0): %v", err)
	}
	if len(got) != 3 {
		t.Errorf("Recent(0) should return all entries, got %d", len(got))
	}
}

func TestRecent_EmptyLogReturnsErrNoHistory(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if _, err := Recent(10); !errors.Is(err, ErrNoHistory) {
		t.Errorf("expected ErrNoHistory, got %v", err)
	}
}

func TestAt_NewestFirst(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	seedRunLog(t, 5)

	// At(1) is newest.
	got, err := At(1)
	if err != nil {
		t.Fatalf("At(1): %v", err)
	}
	if got.Config != "run-e.yml" {
		t.Errorf("At(1).Config = %q, want run-e.yml", got.Config)
	}

	// At(1) == Last() invariant.
	last, _ := Last()
	if got.Config != last.Config {
		t.Errorf("At(1) and Last must return the same entry")
	}

	got2, err := At(2)
	if err != nil {
		t.Fatalf("At(2): %v", err)
	}
	if got2.Config != "run-d.yml" {
		t.Errorf("At(2).Config = %q, want run-d.yml", got2.Config)
	}
}

func TestAt_OutOfRange(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	seedRunLog(t, 2)

	for _, i := range []int{0, -1, 99} {
		if _, err := At(i); !errors.Is(err, ErrIndexOutOfRange) {
			t.Errorf("At(%d): expected ErrIndexOutOfRange, got %v", i, err)
		}
	}
}

func TestAt_EmptyLog(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)
	if _, err := At(1); !errors.Is(err, ErrNoHistory) {
		t.Errorf("expected ErrNoHistory on empty log, got %v", err)
	}
}

// TestLegacyEntryStillDecodes verifies the wave-2 widening is purely
// additive: an entry written before RunID/OpID/Steps existed still
// parses, with the new fields at zero values.
func TestLegacyEntryStillDecodes(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Hand-write a legacy-shape line into runs.jsonl — no RunID,
	// no OpID, no Steps.
	path, err := logPath()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	legacy := `{"ts":"2026-05-17T00:00:00Z","config":"old.yml","changed":2,"ok":5,"skipped":1,"failed":0,"duration_ms":1234}` + "\n"
	if err := os.WriteFile(path, []byte(legacy), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("ReadAll = %d entries, want 1", len(entries))
	}
	got := entries[0]
	if got.Config != "old.yml" || got.Changed != 2 || got.DurationMs != 1234 {
		t.Errorf("legacy fields decoded wrong: %+v", got)
	}
	if got.RunID != "" || got.OpID != "" || got.Steps != nil {
		t.Errorf("wave-2 fields should be zero for legacy entry: %+v", got)
	}
}

// TestStepEntryRoundTrip verifies that a wave-2 entry with Steps
// round-trips through JSON intact.
func TestStepEntryRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	e := Entry{
		TS:         time.Date(2026, 5, 17, 0, 0, 0, 0, time.UTC),
		Config:     "x.yml",
		Changed:    1,
		Ok:         1,
		DurationMs: 100,
		RunID:      "r/01HVTEST",
		OpID:       "op/01HVTEST",
		Steps: []StepEntry{
			{Index: 1, Action: "file.write", Resource: "file:/tmp/x", Result: "changed", Reversible: true},
			{Index: 2, Action: "shell", Resource: "", Result: "ok"},
		},
	}
	if err := Append(e); err != nil {
		t.Fatalf("Append: %v", err)
	}
	got, err := Last()
	if err != nil {
		t.Fatalf("Last: %v", err)
	}
	if got.RunID != e.RunID || got.OpID != e.OpID {
		t.Errorf("ID roundtrip mismatch: got run=%q op=%q", got.RunID, got.OpID)
	}
	if len(got.Steps) != 2 || got.Steps[0].Resource != "file:/tmp/x" || !got.Steps[0].Reversible {
		t.Errorf("Steps roundtrip mismatch: %+v", got.Steps)
	}
}

// import-guard so the new test compiles without explicit deps beyond
// what the existing tests use.
var _ = errors.New
