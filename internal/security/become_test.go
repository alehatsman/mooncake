package security

import (
	"errors"
	"runtime"
	"strings"
	"testing"
)

func TestBecomeRunner_NoBecomeBypassesSudo(t *testing.T) {
	r := BecomeRunner{SudoPass: "doesntmatter"}
	cmd, err := r.Command(false, "/bin/echo", "hi")
	if err != nil {
		t.Fatalf("non-become path should never error: %v", err)
	}
	if cmd.Path != "/bin/echo" {
		t.Errorf("Path = %q, want /bin/echo (no sudo wrapper)", cmd.Path)
	}
	if cmd.Stdin != nil {
		t.Errorf("non-become path should not have stdin wired: %v", cmd.Stdin)
	}
}

func TestBecomeRunner_BecomeWrapsWithSudo(t *testing.T) {
	if !IsBecomeSupported() {
		t.Skipf("become not supported on %s", runtime.GOOS)
	}
	r := BecomeRunner{SudoPass: "secret"}
	cmd, err := r.Command(true, "/usr/bin/apt-get", "install", "-y", "nginx")
	if err != nil {
		t.Fatalf("supported become with SudoPass should succeed: %v", err)
	}
	// Args[0] is the path under exec; what we care about is the rest.
	got := strings.Join(cmd.Args, " ")
	want := "sudo -S /usr/bin/apt-get install -y nginx"
	if got != want {
		t.Errorf("cmd.Args = %q, want %q", got, want)
	}
	if cmd.Stdin == nil {
		t.Error("expected stdin wired with sudo password")
	}
}

func TestBecomeRunner_EmptySudoPassRefuses(t *testing.T) {
	if !IsBecomeSupported() {
		t.Skipf("become not supported on %s", runtime.GOOS)
	}
	r := BecomeRunner{} // SudoPass: ""
	_, err := r.Command(true, "/usr/bin/apt-get", "install", "nginx")
	if !errors.Is(err, ErrBecomeNoSudoPass) {
		t.Errorf("expected ErrBecomeNoSudoPass, got %v", err)
	}
}

// On supported platforms (linux/darwin) this test path can't trigger
// ErrBecomeUnsupported. We can still assert the error contract by
// asserting the OTHER errors are NOT this one.
func TestBecomeRunner_NoBecomeNeverReturnsErrBecomeUnsupported(t *testing.T) {
	r := BecomeRunner{}
	_, err := r.Command(false, "/bin/echo", "hi")
	if errors.Is(err, ErrBecomeUnsupported) {
		t.Error("non-become path must not return ErrBecomeUnsupported")
	}
}
