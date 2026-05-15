//go:build linux || darwin

package agentd

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

// Issue #12 regression tests: swapBinary must succeed even when src
// and dst live on different filesystems (EXDEV from os.Rename).
// Linux peers where /var/lib (state_dir) and /usr/local (install dir)
// live on separate partitions hit this on every `fleet upgrade`.

// TestSwapBinary_FastPathRenameSameFS exercises the original behavior:
// when os.Rename works (same filesystem), use it directly. No copy.
func TestSwapBinary_FastPathRenameSameFS(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "staged")
	dst := filepath.Join(dir, "installed")
	if err := os.WriteFile(src, []byte("new binary"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write src: %v", err)
	}

	if err := swapBinary(src, dst); err != nil {
		t.Fatalf("swapBinary: %v", err)
	}
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("src should be gone after rename, got err=%v", err)
	}
	got, err := os.ReadFile(dst) //nolint:gosec
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new binary" {
		t.Errorf("dst content = %q, want %q", got, "new binary")
	}
}

// TestSwapBinary_EXDEVFallbackCopiesThenRenames is the headline
// issue-12 regression. When os.Rename fails with EXDEV, swapBinary
// must copy src into a sibling-of-dst temp, atomic-rename it onto
// dst, and GC the staged source. The end state matches the same-fs
// case byte-for-byte from the daemon's perspective.
func TestSwapBinary_EXDEVFallbackCopiesThenRenames(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "staged")
	dst := filepath.Join(dir, "installed")
	if err := os.WriteFile(src, []byte("new binary"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write src: %v", err)
	}

	// Force the initial rename to look like EXDEV without an actual
	// cross-fs mount. The fallback's own rename uses os.Rename directly,
	// which works against the same temp dir.
	prev := renameFunc
	defer func() { renameFunc = prev }()
	renameFunc = func(_, _ string) error { return &os.PathError{Op: "rename", Err: syscall.EXDEV} }

	if err := swapBinary(src, dst); err != nil {
		t.Fatalf("swapBinary EXDEV fallback: %v", err)
	}

	// Source GC'd, dst present with new content, no stale .upgrade-tmp.
	if _, err := os.Stat(src); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("src should be GC'd after EXDEV fallback, got err=%v", err)
	}
	got, err := os.ReadFile(dst) //nolint:gosec
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != "new binary" {
		t.Errorf("dst content = %q, want %q", got, "new binary")
	}
	if _, err := os.Stat(dst + ".upgrade-tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("upgrade-tmp leaked after successful fallback: %v", err)
	}
}

// TestSwapBinary_EXDEVCopyFailureCleansUpTemp covers the negative
// path: if copyFile fails mid-fallback, the sibling temp must not be
// left dangling.
func TestSwapBinary_EXDEVCopyFailureCleansUpTemp(t *testing.T) {
	dir := t.TempDir()
	// Use a non-existent src so the copy step fails after the EXDEV
	// branch is entered. The fallback's `_ = os.Remove(tmp)` should
	// keep the dir clean.
	src := filepath.Join(dir, "absent")
	dst := filepath.Join(dir, "installed")

	prev := renameFunc
	defer func() { renameFunc = prev }()
	renameFunc = func(_, _ string) error { return &os.PathError{Op: "rename", Err: syscall.EXDEV} }

	err := swapBinary(src, dst)
	if err == nil {
		t.Fatal("expected error when copy source is missing")
	}
	if _, err := os.Stat(dst + ".upgrade-tmp"); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("upgrade-tmp must be cleaned up on copy failure: %v", err)
	}
}

// TestSwapBinary_NonEXDEVErrorReturnsDirectly ensures the fallback
// only triggers on EXDEV. Any other rename error (e.g. permission
// denied) returns immediately so the caller can surface the real
// failure.
func TestSwapBinary_NonEXDEVErrorReturnsDirectly(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "staged")
	dst := filepath.Join(dir, "installed")
	if err := os.WriteFile(src, []byte("x"), 0o755); err != nil { //nolint:gosec
		t.Fatalf("write src: %v", err)
	}

	prev := renameFunc
	defer func() { renameFunc = prev }()
	renameFunc = func(_, _ string) error { return &os.PathError{Op: "rename", Err: syscall.EPERM} }

	err := swapBinary(src, dst)
	if err == nil {
		t.Fatal("expected error to propagate")
	}
	if errors.Is(err, syscall.EXDEV) {
		t.Errorf("EPERM should not be treated as EXDEV: %v", err)
	}
	// The fallback must not have run — src is still there.
	if _, err := os.Stat(src); err != nil {
		t.Errorf("src should remain after non-EXDEV error, got err=%v", err)
	}
}
