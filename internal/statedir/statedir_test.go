package statedir

import (
	"errors"
	"os"
	"os/user"
	"path/filepath"
	"testing"
)

func TestDir_MooncakeHomeWins(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOONCAKE_HOME", tmp)
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error: %v", err)
	}
	if got != tmp {
		t.Errorf("Dir() = %q, want %q", got, tmp)
	}
}

func TestPath_JoinsUnderStateDir(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("MOONCAKE_HOME", tmp)
	got, err := Path("runs.jsonl")
	if err != nil {
		t.Fatalf("Path() error: %v", err)
	}
	if want := filepath.Join(tmp, "runs.jsonl"); got != want {
		t.Errorf("Path() = %q, want %q", got, want)
	}
}

// A redirected HOME is how most of the suite isolates itself already
// (internal/runlog, internal/ops). That must keep working — the guard
// is aimed at tests that redirect nothing.
func TestDir_RedirectedHomeIsAllowed(t *testing.T) {
	t.Setenv("MOONCAKE_HOME", "")
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)
	got, err := Dir()
	if err != nil {
		t.Fatalf("Dir() error with redirected HOME: %v", err)
	}
	if want := filepath.Join(tmp, ".mooncake"); got != want {
		t.Errorf("Dir() = %q, want %q", got, want)
	}
}

// The regression this package exists for: `go test ./...` appended real
// entries to the developer's own ~/.mooncake/runs.jsonl, so `mooncake
// history` was full of phantom runs from the test suite.
func TestDir_RefusesRealHomeUnderTest(t *testing.T) {
	u, err := user.Current()
	if err != nil || u.HomeDir == "" {
		t.Skip("no passwd home available to compare against")
	}
	t.Setenv("MOONCAKE_HOME", "")
	t.Setenv("HOME", u.HomeDir)

	if _, err := Dir(); !errors.Is(err, ErrTestIsolation) {
		t.Errorf("Dir() error = %v, want ErrTestIsolation — a test binary must not "+
			"resolve the real ~/.mooncake", err)
	}
}

func TestUnderTest_TrueInThisBinary(t *testing.T) {
	if !underTest() {
		t.Errorf("underTest() = false while running under `go test` (argv[0]=%q)", os.Args[0])
	}
}
