package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// TestDetectBinaryTarget_RecognisesRunningTestBinary uses os.Executable
// (the test binary itself) as a known-good ELF or PE example. On Linux
// the test binary is an ELF amd64; on Windows it's PE amd64. Either
// way we expect detectBinaryTarget to identify it correctly.
func TestDetectBinaryTarget_RecognisesRunningTestBinary(t *testing.T) {
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		t.Skipf("detectBinaryTarget only handles ELF/PE; skipping on %s", runtime.GOOS)
	}
	self, err := os.Executable()
	if err != nil {
		t.Skipf("os.Executable not supported: %v", err)
	}
	gotOS, gotArch, err := detectBinaryTarget(self)
	if err != nil {
		t.Fatalf("detectBinaryTarget(%s): %v", self, err)
	}
	if gotOS != runtime.GOOS {
		t.Errorf("OS = %q, want %q (runtime.GOOS)", gotOS, runtime.GOOS)
	}
	if gotArch != runtime.GOARCH {
		t.Errorf("Arch = %q, want %q (runtime.GOARCH)", gotArch, runtime.GOARCH)
	}
}

// TestDetectBinaryTarget_RejectsGarbage covers the path where someone
// points --binary at a non-binary file (text doc, partial download, etc).
func TestDetectBinaryTarget_RejectsGarbage(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-binary")
	if err := os.WriteFile(path, []byte("this is not an executable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := detectBinaryTarget(path); err == nil {
		t.Fatal("want error for non-binary input, got nil")
	}
}
