package effects

// F026 regression: CopyFile streams src → dest via io.Copy so peak RAM
// stays bounded regardless of source file size. Pre-F026 the copy
// action did os.ReadFile(src) + WriteFile(dest, content) which loaded
// the entire source into a single []byte (+ a second copy when
// WriteFile's idempotency check called os.ReadFile on existing dest).
// Two tests cover the contract:
//
//   - Round-trip: a multi-MB file copies byte-identically.
//   - Idempotency: a second CopyFile with identical src+mode returns
//     AlreadyOk without re-writing.

import (
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
)

func TestCopyFile_StreamsLargeFileByteIdentical(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dest := filepath.Join(dir, "dest.bin")

	// 4 MB random payload — larger than any conceivable buffered-read
	// cap but small enough to keep the test fast.
	payload := make([]byte, 4*1024*1024)
	if _, err := rand.Read(payload); err != nil {
		t.Fatalf("rand.Read: %v", err)
	}
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "")
	eff := p.CopyFile(src, dest, 0o644, actions.PerformerOpts{})
	if eff.Err != nil {
		t.Fatalf("CopyFile: %v", eff.Err)
	}
	if !eff.Performed {
		t.Errorf("Performed = false, want true on first copy")
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("dest size = %d, want %d", len(got), len(payload))
	}
	// Spot-check byte equality without dumping 4 MB on mismatch.
	for i := 0; i < len(payload); i += len(payload) / 16 {
		if got[i] != payload[i] {
			t.Fatalf("byte %d differs: got %#x want %#x", i, got[i], payload[i])
		}
	}
}

func TestCopyFile_IdempotentOnIdenticalContentAndMode(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")

	if err := os.WriteFile(src, []byte("same"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(dest, []byte("same"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	p := NewPerformer(func() actions.Mode { return actions.ModeApply }, "")
	eff := p.CopyFile(src, dest, 0o644, actions.PerformerOpts{})
	if eff.Err != nil {
		t.Fatalf("CopyFile: %v", eff.Err)
	}
	if !eff.AlreadyOk {
		t.Errorf("AlreadyOk = false, want true when src/dest match (got reason %q)", eff.Reason)
	}
	if eff.Performed {
		t.Errorf("Performed = true on idempotent path; should be false")
	}
}

func TestCopyFile_PlanModePredictsChangeWithoutMutating(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.txt")
	dest := filepath.Join(dir, "dest.txt")

	if err := os.WriteFile(src, []byte("new"), 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	if err := os.WriteFile(dest, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed dest: %v", err)
	}

	p := NewPerformer(func() actions.Mode { return actions.ModePlan }, "")
	eff := p.CopyFile(src, dest, 0o644, actions.PerformerOpts{})
	if eff.Err != nil {
		t.Fatalf("CopyFile: %v", eff.Err)
	}
	if !eff.WouldChange {
		t.Errorf("WouldChange = false, want true (dest content differs)")
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Errorf("dest mutated in plan mode: got %q, want %q", got, "old")
	}
}
