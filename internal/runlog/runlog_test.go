package runlog

import (
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
