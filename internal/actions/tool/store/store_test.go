package store

import (
	"os"
	"path/filepath"
	"testing"
)

// MT-60: when bin is unset and the install dir contains exactly one
// executable file, LocateInInstallDir should auto-resolve to that
// file — the common github-release bare-binary case. Before this
// fix the function returned the install dir, which is not executable
// and produces "Is a directory" when invoked.
func TestLocateInInstallDir_BinUnset_SingleExecutableAutoResolves(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "jq")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\necho jq\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	got, err := LocateInInstallDir("", dir)
	if err != nil {
		t.Fatalf("LocateInInstallDir: %v", err)
	}
	if got != binPath {
		t.Errorf("expected auto-resolve to %q, got %q", binPath, got)
	}
}

// MT-60: when bin is unset and the install dir contains multiple
// executables (e.g. a binary + a helper script), don't guess — fall
// back to returning the install dir so the existing "needs bin:"
// failure mode preserves operator intent.
func TestLocateInInstallDir_BinUnset_MultipleExecutablesFallBackToDir(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"jq", "jq-helper"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}

	got, err := LocateInInstallDir("", dir)
	if err != nil {
		t.Fatalf("LocateInInstallDir: %v", err)
	}
	if got != dir {
		t.Errorf("expected fallback to install dir %q, got %q", dir, got)
	}
}

// MT-60: non-executable files don't count toward the auto-resolve
// candidate set. A README plus a single executable still
// auto-resolves to the executable.
func TestLocateInInstallDir_BinUnset_IgnoresNonExecutables(t *testing.T) {
	dir := t.TempDir()
	binPath := filepath.Join(dir, "jq")
	if err := os.WriteFile(binPath, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("docs\n"), 0o644); err != nil {
		t.Fatalf("write readme: %v", err)
	}

	got, err := LocateInInstallDir("", dir)
	if err != nil {
		t.Fatalf("LocateInInstallDir: %v", err)
	}
	if got != binPath {
		t.Errorf("expected auto-resolve to %q (README ignored), got %q", binPath, got)
	}
}

// MT-60: explicit bin: still wins. Auto-resolution must not override
// what the user asked for, even if it would have picked something
// different.
func TestLocateInInstallDir_BinSet_ExplicitWins(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "jq"), []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write bin: %v", err)
	}

	got, err := LocateInInstallDir("jq", dir)
	if err != nil {
		t.Fatalf("LocateInInstallDir: %v", err)
	}
	want := filepath.Join(dir, "jq")
	if got != want {
		t.Errorf("expected explicit bin to win: want %q, got %q", want, got)
	}
}
