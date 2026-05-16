package ops

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewOpID_PrefixAndShape(t *testing.T) {
	id := NewOpID()
	if !strings.HasPrefix(id, "op/") {
		t.Fatalf("op id %q missing op/ prefix", id)
	}
	if len(id) != len("op/")+26 {
		t.Fatalf("op id %q has length %d, want 29 (op/ + 26-char ULID)", id, len(id))
	}
}

func TestNewRunID_PrefixAndShape(t *testing.T) {
	id := NewRunID()
	if !strings.HasPrefix(id, "r/") {
		t.Fatalf("run id %q missing r/ prefix", id)
	}
	if len(id) != len("r/")+26 {
		t.Fatalf("run id %q has length %d, want 28 (r/ + 26-char ULID)", id, len(id))
	}
}

func TestNewOpID_Unique(t *testing.T) {
	a := NewOpID()
	b := NewOpID()
	if a == b {
		t.Fatalf("two consecutive op ids are identical: %q", a)
	}
}

func TestAppendReadRoundTrip(t *testing.T) {
	withTempHome(t)

	entry := Entry{
		TS:       time.Now().UTC().Truncate(time.Second),
		OpID:     NewOpID(),
		Command:  "apply",
		Args:     []string{"--config", "x.yml"},
		Actor:    "user:test",
		Config:   "x.yml",
		PlanOnly: false,
	}
	if err := Append(entry); err != nil {
		t.Fatalf("Append: %v", err)
	}

	got, err := Read(entry.OpID)
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	if got.OpID != entry.OpID || got.Command != entry.Command {
		t.Errorf("roundtrip mismatch: got %+v", got)
	}
}

func TestRead_NotFound(t *testing.T) {
	withTempHome(t)
	if _, err := Read("op/01HVNOPE"); err == nil {
		t.Fatal("expected ErrNotFound, got nil")
	} else if err != ErrNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestReadAll_MissingFileReturnsEmpty(t *testing.T) {
	withTempHome(t)
	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll on missing file: %v", err)
	}
	if entries != nil {
		t.Errorf("ReadAll on missing file = %d entries, want nil", len(entries))
	}
}

func TestReadAll_SkipsGarbledLines(t *testing.T) {
	withTempHome(t)
	path, _ := LogPath()
	_ = os.MkdirAll(filepath.Dir(path), 0o700)
	good := `{"ts":"2026-05-17T00:00:00Z","op_id":"op/01HV1","command":"apply"}`
	garbage := `not json at all`
	missingPrefix := `{"ts":"2026-05-17T00:00:01Z","op_id":"NOPREFIX","command":"x"}`
	body := good + "\n" + garbage + "\n" + missingPrefix + "\n"
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}

	entries, err := ReadAll()
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	if len(entries) != 1 || entries[0].OpID != "op/01HV1" {
		t.Errorf("ReadAll = %+v, want [op/01HV1]", entries)
	}
}

// withTempHome redirects $HOME to a per-test temp dir so Append/Read
// hit a fresh ops.jsonl without touching the real one.
func withTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}
