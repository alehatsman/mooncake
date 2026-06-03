package install

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The running test binary is built for the host platform, so sniffing it
// must report the host's GOOS/GOARCH. This keeps the test portable across
// linux/darwin/windows runners without shipping cross-built fixtures.
func TestSniffBinaryPlatform_Self(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	gotOS, gotArch, err := sniffBinaryPlatform(exe)
	if err != nil {
		t.Fatalf("sniffBinaryPlatform(%q): %v", exe, err)
	}
	if gotOS != runtime.GOOS || gotArch != runtime.GOARCH {
		t.Errorf("got %s/%s, want host %s/%s", gotOS, gotArch, runtime.GOOS, runtime.GOARCH)
	}
}

func TestVerifyBinaryPlatform_SelfMatches(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	if err := VerifyBinaryPlatform(exe, runtime.GOOS, runtime.GOARCH); err != nil {
		t.Errorf("expected match for host platform, got: %v", err)
	}
}

func TestVerifyBinaryPlatform_MismatchIsActionable(t *testing.T) {
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	// Ask for an OS the host binary can't be, so it always mismatches.
	wantOS := "windows"
	if runtime.GOOS == "windows" {
		wantOS = "linux"
	}
	err = VerifyBinaryPlatform(exe, wantOS, runtime.GOARCH)
	if err == nil {
		t.Fatal("expected mismatch error, got nil")
	}
	// The message must steer the operator to --binary with a build hint.
	for _, want := range []string{"--binary", wantOS, "go build"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error missing %q: %v", want, err)
		}
	}
}

func TestSniffBinaryPlatform_UnrecognisedFormat(t *testing.T) {
	p := filepath.Join(t.TempDir(), "notabinary.txt")
	if err := os.WriteFile(p, []byte("just some text, not an executable\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, _, err := sniffBinaryPlatform(p); err == nil {
		t.Error("expected error for non-executable file, got nil")
	}
}
