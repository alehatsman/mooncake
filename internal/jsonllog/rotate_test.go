package jsonllog

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Nothing bounded runs.jsonl / ops.jsonl before: a daily-use box reached
// 9.0 MB / 24,903 lines with no rotation and no cap (#176).
func TestAppend_RollsAtMaxBytes(t *testing.T) {
	orig := MaxBytes
	t.Cleanup(func() { MaxBytes = orig })
	MaxBytes = 256

	dir := t.TempDir()
	path := filepath.Join(dir, "runs.jsonl")

	// Each entry is well under MaxBytes; enough of them cross it.
	for i := 0; i < 40; i++ {
		if err := Append(path, map[string]any{"i": i, "pad": strings.Repeat("x", 20)}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	live, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat live log: %v", err)
	}
	if live.Size() >= MaxBytes {
		t.Errorf("live log is %d bytes, want < MaxBytes (%d) after a roll", live.Size(), MaxBytes)
	}

	roll, err := os.Stat(path + rollSuffix)
	if err != nil {
		t.Fatalf("expected a rolled generation at %s: %v", path+rollSuffix, err)
	}
	if roll.Size() == 0 {
		t.Error("rolled log is empty")
	}
}

// Only one generation is kept, so the pair is bounded at 2×MaxBytes
// no matter how long the box runs.
func TestAppend_KeepsExactlyOneRoll(t *testing.T) {
	orig := MaxBytes
	t.Cleanup(func() { MaxBytes = orig })
	MaxBytes = 128

	dir := t.TempDir()
	path := filepath.Join(dir, "runs.jsonl")
	for i := 0; i < 200; i++ {
		if err := Append(path, map[string]any{"i": i, "pad": strings.Repeat("y", 20)}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("readdir: %v", err)
	}
	if len(entries) != 2 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("got %d files %v, want exactly 2 (live + one roll)", len(entries), names)
	}
	if _, err := os.Stat(path + ".2"); !os.IsNotExist(err) {
		t.Error("a second-generation roll exists; only one is kept")
	}
}

// MaxBytes <= 0 disables rotation entirely.
func TestAppend_RotationDisabled(t *testing.T) {
	orig := MaxBytes
	t.Cleanup(func() { MaxBytes = orig })
	MaxBytes = 0

	dir := t.TempDir()
	path := filepath.Join(dir, "runs.jsonl")
	for i := 0; i < 50; i++ {
		if err := Append(path, map[string]any{"i": i}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	if _, err := os.Stat(path + rollSuffix); !os.IsNotExist(err) {
		t.Error("rotation happened with MaxBytes = 0")
	}
}

// A roll must not lose the entry that triggered it.
func TestAppend_RollPreservesTheTriggeringEntry(t *testing.T) {
	orig := MaxBytes
	t.Cleanup(func() { MaxBytes = orig })
	MaxBytes = 64

	dir := t.TempDir()
	path := filepath.Join(dir, "runs.jsonl")
	for i := 0; i < 20; i++ {
		if err := Append(path, map[string]any{"marker": i}); err != nil {
			t.Fatalf("append %d: %v", i, err)
		}
	}
	live, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read live log: %v", err)
	}
	if !strings.Contains(string(live), `"marker":19`) {
		t.Errorf("last entry missing from live log:\n%s", live)
	}
}
