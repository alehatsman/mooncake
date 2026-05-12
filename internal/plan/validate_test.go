package plan

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/alehatsman/mooncake/internal/facts"
)

// currentHost returns the HostFacts that ValidateForApply will compare
// against, so tests can build plans that match the local machine.
func currentHost() HostFacts {
	f := facts.Collect()
	return HostFacts{
		OsFamily:     f.OS,
		Arch:         f.Arch,
		DistroFamily: f.Distribution,
	}
}

// TestValidateForApply_HappyPath: matching facts, intact files, no age
// limit. Must succeed.
func TestValidateForApply_HappyPath(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "main.yml")
	if err := os.WriteFile(cfg, []byte("steps: []\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, err := HashInputFiles([]string{cfg})
	if err != nil {
		t.Fatal(err)
	}
	p := &Plan{
		Version:        "1.0",
		GeneratedAt:    time.Now(),
		GeneratedOn:    currentHost(),
		RootFile:       cfg,
		InputFiles:     []string{cfg},
		InputFilesHash: hash,
	}

	if err := ValidateForApply(p, ValidateOptions{}); err != nil {
		t.Errorf("happy path should pass, got %v", err)
	}
}

// TestValidateForApply_HostMismatch: deliberately wrong host facts.
// Must reject as host_mismatch unless AllowStale is set.
func TestValidateForApply_HostMismatch(t *testing.T) {
	p := &Plan{
		Version:     "1.0",
		GeneratedAt: time.Now(),
		GeneratedOn: HostFacts{OsFamily: "darwin", Arch: "arm64", DistroFamily: "macos"},
	}
	// On Linux/amd64 this will mismatch; on Darwin/arm64 it would
	// match. Skip if we happen to be on that combo (rare in CI).
	cur := facts.Collect()
	if cur.OS == "darwin" && cur.Arch == "arm64" {
		t.Skip("running on the same host as the fake plan facts")
	}

	err := ValidateForApply(p, ValidateOptions{})
	if err == nil {
		t.Fatal("expected stale error, got nil")
	}
	var se *StaleError
	if !errors.As(err, &se) {
		t.Fatalf("expected *StaleError, got %T: %v", err, err)
	}
	if se.Reason != StaleReasonHostMismatch {
		t.Errorf("reason = %v, want %v", se.Reason, StaleReasonHostMismatch)
	}

	// AllowStale should suppress the rejection.
	if err := ValidateForApply(p, ValidateOptions{AllowStale: true}); err != nil {
		t.Errorf("AllowStale should suppress mismatch, got %v", err)
	}
}

// TestValidateForApply_HashMismatch: content drift after plan time.
func TestValidateForApply_HashMismatch(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "main.yml")
	if err := os.WriteFile(cfg, []byte("original\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	hash, _ := HashInputFiles([]string{cfg})
	p := &Plan{
		Version:        "1.0",
		GeneratedAt:    time.Now(),
		GeneratedOn:    currentHost(),
		RootFile:       cfg,
		InputFiles:     []string{cfg},
		InputFilesHash: hash,
	}

	// Modify the file after the plan was built.
	if err := os.WriteFile(cfg, []byte("changed\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := ValidateForApply(p, ValidateOptions{})
	if err == nil {
		t.Fatal("expected stale error from hash mismatch")
	}
	var se *StaleError
	if !errors.As(err, &se) || se.Reason != StaleReasonHashMismatch {
		t.Errorf("reason = %v, want %v (err=%v)", se.Reason, StaleReasonHashMismatch, err)
	}
}

// TestValidateForApply_FileMissing: a referenced input file no longer
// exists at apply time.
func TestValidateForApply_FileMissing(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "main.yml")
	_ = os.WriteFile(cfg, []byte("steps: []\n"), 0o644)
	hash, _ := HashInputFiles([]string{cfg})
	p := &Plan{
		Version:        "1.0",
		GeneratedAt:    time.Now(),
		GeneratedOn:    currentHost(),
		RootFile:       cfg,
		InputFiles:     []string{cfg},
		InputFilesHash: hash,
	}
	if err := os.Remove(cfg); err != nil {
		t.Fatal(err)
	}

	err := ValidateForApply(p, ValidateOptions{})
	if err == nil {
		t.Fatal("expected stale error from missing file")
	}
	var se *StaleError
	if !errors.As(err, &se) || se.Reason != StaleReasonFileMissing {
		t.Errorf("reason = %v, want %v (err=%v)", se.Reason, StaleReasonFileMissing, err)
	}
}

// TestValidateForApply_AgeExceeded: plan older than MaxAge.
func TestValidateForApply_AgeExceeded(t *testing.T) {
	dir := t.TempDir()
	cfg := filepath.Join(dir, "main.yml")
	_ = os.WriteFile(cfg, []byte("steps: []\n"), 0o644)
	hash, _ := HashInputFiles([]string{cfg})
	p := &Plan{
		Version:        "1.0",
		GeneratedAt:    time.Now().Add(-2 * time.Hour),
		GeneratedOn:    currentHost(),
		RootFile:       cfg,
		InputFiles:     []string{cfg},
		InputFilesHash: hash,
	}

	err := ValidateForApply(p, ValidateOptions{MaxAge: 1 * time.Hour})
	if err == nil {
		t.Fatal("expected stale error from age limit")
	}
	var se *StaleError
	if !errors.As(err, &se) || se.Reason != StaleReasonAgeExceeded {
		t.Errorf("reason = %v, want %v (err=%v)", se.Reason, StaleReasonAgeExceeded, err)
	}

	// Zero MaxAge should disable the check.
	if err := ValidateForApply(p, ValidateOptions{}); err != nil {
		t.Errorf("zero MaxAge should disable age check, got %v", err)
	}
}

// TestHashInputFiles_Determinism: same content, different path order,
// same hash. Different content, different hash.
func TestHashInputFiles_Determinism(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a.yml")
	b := filepath.Join(dir, "b.yml")
	_ = os.WriteFile(a, []byte("AAA"), 0o644)
	_ = os.WriteFile(b, []byte("BBB"), 0o644)

	h1, err := HashInputFiles([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	h2, err := HashInputFiles([]string{b, a}) // reversed order
	if err != nil {
		t.Fatal(err)
	}
	if h1 != h2 {
		t.Errorf("hash should be order-independent: %s != %s", h1, h2)
	}

	// Different content → different hash.
	_ = os.WriteFile(a, []byte("XXX"), 0o644)
	h3, err := HashInputFiles([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	if h1 == h3 {
		t.Errorf("hash should change when content changes")
	}

	// Different path (same content) → different hash.
	c := filepath.Join(dir, "c.yml")
	_ = os.WriteFile(c, []byte("AAA"), 0o644) // same content as original a
	h4, _ := HashInputFiles([]string{c, b})
	if h1 == h4 {
		t.Errorf("hash should change when path changes even with same content")
	}
}

// Sanity: the runtime package is imported indirectly via facts; keep
// this reference so the test file compiles cleanly across platforms.
var _ = runtime.GOOS
