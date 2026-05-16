package file

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
)

// F026 regression. The pre-fix file/handler.go used os.ReadFile on the
// target path three times per Run (pre-snapshot, backup, post-snapshot)
// — peak RSS hit ~3× the target file size. snapshotFile + the
// io.Copy backup path replaced that with bounded buffers. These tests
// pin the bounded-buffer contract so a future refactor that reverts
// to os.ReadFile trips immediately.

// TestSnapshotFile_HeadCappedFullHashed verifies the head buffer is
// capped at snapshotMaxBytes while the SHA-256 still covers the full
// content and size is the true byte count.
func TestSnapshotFile_HeadCappedFullHashed(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.bin")

	// Generate a payload that's bigger than the head cap. Using a
	// repeating byte pattern so the test is hermetic — no crypto/rand
	// dependency, and the expected hash is deterministic.
	totalBytes := snapshotMaxBytes + 1024 // just past the cap
	payload := make([]byte, totalBytes)
	for i := range payload {
		payload[i] = byte(i % 256)
	}
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	head, size, sum, err := snapshotFile(path)
	if err != nil {
		t.Fatalf("snapshotFile: %v", err)
	}

	if size != int64(totalBytes) {
		t.Errorf("size = %d, want %d (full file size)", size, totalBytes)
	}
	if len(head) != snapshotMaxBytes {
		t.Errorf("len(head) = %d, want %d (capped)", len(head), snapshotMaxBytes)
	}
	if !bytes.Equal(head, payload[:snapshotMaxBytes]) {
		t.Error("head bytes do not match the file's leading snapshotMaxBytes")
	}
	wantSum := sha256.Sum256(payload)
	if sum != hex.EncodeToString(wantSum[:]) {
		t.Errorf("sum = %q, want full-file sha256 %q", sum, hex.EncodeToString(wantSum[:]))
	}
}

// TestSnapshotFile_SmallFileFullCapture: a file smaller than the cap
// produces head == file bytes, size == len(file), and a matching hash.
func TestSnapshotFile_SmallFileFullCapture(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "small.txt")
	payload := []byte("hello world\n")
	if err := os.WriteFile(path, payload, 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	head, size, sum, err := snapshotFile(path)
	if err != nil {
		t.Fatalf("snapshotFile: %v", err)
	}
	if size != int64(len(payload)) {
		t.Errorf("size = %d, want %d", size, len(payload))
	}
	if !bytes.Equal(head, payload) {
		t.Errorf("head = %q, want %q", head, payload)
	}
	wantSum := sha256.Sum256(payload)
	if sum != hex.EncodeToString(wantSum[:]) {
		t.Errorf("sum mismatch: got %q want %q", sum, hex.EncodeToString(wantSum[:]))
	}
}

// TestSnapshotFile_MissingFileReturnsError: an attempt to snapshot a
// path that doesn't exist returns an error rather than panicking or
// reporting bogus zero values. The handler's Run-mode pre-write probe
// treats this as "no before state" and proceeds with the create-path
// event — verified separately by the existing TestHandler_FileCreate
// suite.
func TestSnapshotFile_MissingFileReturnsError(t *testing.T) {
	head, size, sum, err := snapshotFile(filepath.Join(t.TempDir(), "absent.txt"))
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if head != nil || size != 0 || sum != "" {
		t.Errorf("error path should zero out head/size/sum; got (%v, %d, %q)", head, size, sum)
	}
}

// TestCopyFileStreaming_BoundedBuffer copies a payload larger than the
// 64 KB internal io.Copy buffer and verifies the destination contains
// the same bytes. Pre-fix the backup path called os.ReadFile +
// os.WriteFile (full RAM allocation); the new copyFileStreaming pins
// the streaming contract.
func TestCopyFileStreaming_BoundedBuffer(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.bin")
	dst := filepath.Join(dir, "dst.bin")

	payload := make([]byte, 1<<20) // 1 MB — comfortably past the 64 KB buf
	for i := range payload {
		payload[i] = byte(i & 0xff)
	}
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}

	if err := copyFileStreaming(src, dst, 0o600); err != nil {
		t.Fatalf("copyFileStreaming: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Error("destination bytes do not match source")
	}
	info, err := os.Stat(dst)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}
