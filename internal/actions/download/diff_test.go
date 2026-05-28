package download

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/actions/testutil"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"

	filehandler "github.com/alehatsman/mooncake/internal/actions/file"
)

func diffTestCtx(t *testing.T) *executor.ExecutionContext {
	t.Helper()
	mock := testutil.NewMockContext()
	tmpl, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatalf("renderer: %v", err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template:       tmpl,
			Evaluator:      mock.Evaluator(),
			Logger:         mock.Log,
			EventPublisher: mock.Publisher,
			PathUtil:       pathutil.NewPathExpander(tmpl),
			Mode:           actions.ModePlan,
		},
		Scope:         executor.NewVariableScope(),
		CurrentStepID: mock.CurrentStepID,
		CurrentDir:    "/tmp",
	}
}

// sha256 of "hello\n" as a useful, real value the tests can compare
// against. Easy to recompute: `printf 'hello\n' | sha256sum`.
const helloSha256 = "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03"

// TestDiff_Download_CreateWhenAbsent — Dest missing → OpCreate.
// Independent of whether checksum is set.
func TestDiff_Download_CreateWhenAbsent(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")

	step := &config.Step{FileDownload: &config.Download{
		URL: "https://example.com/x", Dest: dest, Mode: "0644",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpCreate {
		t.Errorf("Operation = %q, want create", d.Operation)
	}
}

// TestDiff_Download_NoopOnChecksumMatch — Dest already exists and
// holds bytes whose SHA256 matches the declared Checksum. That's the
// idempotent "already downloaded" case — OpNoop.
func TestDiff_Download_NoopOnChecksumMatch(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(dest, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileDownload: &config.Download{
		URL:      "https://example.com/x",
		Dest:     dest,
		Mode:     "0644",
		Checksum: helloSha256,
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (Dest bytes match declared checksum)", d.Operation)
	}
}

// TestDiff_Download_UpdateOnChecksumMismatch — Dest exists but its
// content doesn't match the declared checksum. The download will
// happen; OpUpdate.
func TestDiff_Download_UpdateOnChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(dest, []byte("different\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileDownload: &config.Download{
		URL:      "https://example.com/x",
		Dest:     dest,
		Mode:     "0644",
		Checksum: helloSha256,
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (Dest content != checksum)", d.Operation)
	}
}

// TestDiff_Download_UpdateWhenNoChecksum — Dest exists but the user
// didn't declare a checksum. We can't predict noop without one, so
// OpUpdate (conservative). The runtime path produces actual
// changed=false via its own checks if applicable.
func TestDiff_Download_UpdateWhenNoChecksum(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(dest, []byte("any bytes\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileDownload: &config.Download{
		URL:  "https://example.com/x",
		Dest: dest,
		Mode: "0644",
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (no checksum → can't predict noop)", d.Operation)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 != "" {
		t.Errorf("After.Sha256 = %q, want empty (no checksum declared)", after.Sha256)
	}
}

// TestDiff_Download_MD5ChecksumNotUsedAsSha256 — MD5 is 32 hex chars;
// not a valid sha256. After.Sha256 must NOT be populated from an MD5
// checksum, otherwise we'd report misleading "predicted post-state
// hash" values that don't match the FileSnapshot.Sha256 contract
// (sha256-typed by name).
func TestDiff_Download_MD5ChecksumNotUsedAsSha256(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(dest, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	const md5OfHello = "b1946ac92492d2347c6235b4d2611184" // md5sum of "hello\n"
	step := &config.Step{FileDownload: &config.Download{
		URL: "https://example.com/x", Dest: dest, Mode: "0644",
		Checksum: md5OfHello,
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	after := d.After.(*filehandler.FileSnapshot)
	if after.Sha256 != "" {
		t.Errorf("After.Sha256 = %q, want empty (MD5 must not be passed through as sha256)", after.Sha256)
	}
	// Operation falls back to update — we can't predict noop without
	// a sha256.
	if d.Operation != actions.OpUpdate {
		t.Errorf("Operation = %q, want update (MD5 doesn't enable noop prediction)", d.Operation)
	}
}

// TestDiff_Download_ChecksumNormalizedToLower — User writes
// "DEADBEEF..." in upper case; the predicted Sha256 should still
// match a lower-case Sha256 in Before. Lock in the case-fold.
func TestDiff_Download_ChecksumNormalizedToLower(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "out.bin")
	if err := os.WriteFile(dest, []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	step := &config.Step{FileDownload: &config.Download{
		URL: "https://example.com/x", Dest: dest, Mode: "0644",
		Checksum: strings.ToUpper(helloSha256),
	}}
	d, err := (Handler{}).Diff(diffTestCtx(t), step)
	if err != nil {
		t.Fatalf("Diff: %v", err)
	}
	if d.Operation != actions.OpNoop {
		t.Errorf("Operation = %q, want noop (uppercase checksum should still match)", d.Operation)
	}
}

func TestDiff_Download_NilStep(t *testing.T) {
	if _, err := (Handler{}).Diff(diffTestCtx(t), nil); err == nil {
		t.Error("Diff(nil) should return an error")
	}
	if _, err := (Handler{}).Diff(diffTestCtx(t), &config.Step{}); err == nil {
		t.Error("Diff(empty step) should return an error")
	}
}

func TestDiff_Download_RegisteredAsDiffer(t *testing.T) {
	if !actions.IsDiffer(&Handler{}) {
		t.Error("*Handler should satisfy actions.Differ")
	}
}
