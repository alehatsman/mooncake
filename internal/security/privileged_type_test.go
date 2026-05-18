package security

import (
	"context"
	"strings"
	"testing"
)

// withAlreadyIs replaces the alreadyIs hook for the duration of a
// test. Lets us drive the "already root" / "already named user"
// branches without actually running as that user.
func withAlreadyIs(t *testing.T, fn func(string) bool) {
	t.Helper()
	orig := alreadyIs
	t.Cleanup(func() { alreadyIs = orig })
	alreadyIs = fn
}

// TestPrivileged_NoAsUser_NoSudo — empty AsUser means "run as the
// current process." The returned command must not be wrapped in
// sudo regardless of escalation availability.
func TestPrivileged_NoAsUser_NoSudo(t *testing.T) {
	p := &Privileged{Escalation: EscalationReport{Available: true, Reason: EscalationAvailableRoot}}
	cmd, err := p.Command(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if got := cmd.Args[0]; strings.HasSuffix(got, "sudo") {
		t.Errorf("Args[0] = %q, want a non-sudo program (empty AsUser)", got)
	}
}

// TestPrivileged_AsUserRoot_NonRoot_PasswordlessSudo — AsUser:root
// from non-root with NOPASSWD probe success wraps in `sudo -n`.
func TestPrivileged_AsUserRoot_NonRoot_PasswordlessSudo(t *testing.T) {
	withAlreadyIs(t, func(string) bool { return false })
	p := &Privileged{
		AsUser:     "root",
		Escalation: EscalationReport{Available: true, Reason: EscalationAvailablePasswordless},
	}
	cmd, err := p.Command(context.Background(), "apt-get", "update")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{"sudo", "-n", "apt-get", "update"}
	if !equalSlices(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
	if cmd.Stdin != nil {
		t.Error("Stdin should be nil for passwordless sudo")
	}
}

// TestPrivileged_AsUserRoot_NonRoot_PasswordSudo — AsUser:root with
// SudoPass set wraps in `sudo -S` and pipes the password.
func TestPrivileged_AsUserRoot_NonRoot_PasswordSudo(t *testing.T) {
	withAlreadyIs(t, func(string) bool { return false })
	p := &Privileged{
		SudoPass:   "secret",
		AsUser:     "root",
		Escalation: EscalationReport{Available: true, Reason: EscalationAvailablePassword},
	}
	cmd, err := p.Command(context.Background(), "apt-get", "update")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{"sudo", "-S", "apt-get", "update"}
	if !equalSlices(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
	if cmd.Stdin == nil {
		t.Error("Stdin should be wired for password sudo")
	}
}

// TestPrivileged_AsUserRoot_AlreadyRoot_NoSudo — euid 0 with
// AsUser:root short-circuits the sudo wrap. Mooncake-already-root
// presets need this to work in containers that don't ship sudo.
func TestPrivileged_AsUserRoot_AlreadyRoot_NoSudo(t *testing.T) {
	withAlreadyIs(t, func(target string) bool { return target == "root" || target == "0" })
	p := &Privileged{
		AsUser:     "root",
		Escalation: EscalationReport{Available: true, Reason: EscalationAvailableRoot},
	}
	cmd, err := p.Command(context.Background(), "apt-get", "update")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if strings.HasSuffix(cmd.Args[0], "sudo") {
		t.Errorf("Args[0] = %q, want direct exec (already root)", cmd.Args[0])
	}
}

// TestPrivileged_AsUserNamed_NonRoot_PasswordSudo — AsUser:postgres
// wraps in `sudo -u postgres -S`. This is the named-user feature
// Layer C unlocks for every handler, not just command/shell.
func TestPrivileged_AsUserNamed_NonRoot_PasswordSudo(t *testing.T) {
	withAlreadyIs(t, func(string) bool { return false })
	p := &Privileged{
		SudoPass:   "secret",
		AsUser:     "postgres",
		Escalation: EscalationReport{Available: true, Reason: EscalationAvailablePassword},
	}
	cmd, err := p.Command(context.Background(), "psql", "-c", "SELECT 1")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	want := []string{"sudo", "-S", "-u", "postgres", "psql", "-c", "SELECT 1"}
	if !equalSlices(cmd.Args, want) {
		t.Errorf("Args = %v, want %v", cmd.Args, want)
	}
}

// TestPrivileged_AsUserRoot_NoEscalationAvailable_ErrSentinel — when
// AsUser is set but Escalation is unavailable and no SudoPass is
// configured, the primitive returns ErrBecomeNoSudoPass — same
// sentinel BecomeRunner has always returned, so handlers that
// errors.Is against it keep working without changes.
func TestPrivileged_AsUserRoot_NoEscalationAvailable_ErrSentinel(t *testing.T) {
	withAlreadyIs(t, func(string) bool { return false })
	p := &Privileged{
		AsUser:     "root",
		Escalation: EscalationReport{Available: false, Reason: EscalationBlockedNNP, Detail: "NoNewPrivs=1"},
	}
	_, err := p.Command(context.Background(), "apt-get", "update")
	if err == nil {
		t.Fatal("Command returned nil error; want ErrBecomeNoSudoPass")
	}
	if err != ErrBecomeNoSudoPass {
		t.Errorf("err = %v, want ErrBecomeNoSudoPass", err)
	}
}

// TestPrivileged_Run_BypassesSudoEntirelyForEmptyAsUser — pkg.runCmd's
// brew-vs-apt question lives entirely in the step's AsUser now: if
// the step doesn't declare as_user, even an always-escalating
// helper executes directly. Regression guard against accidentally
// re-introducing implicit-escalation-on-non-root.
func TestPrivileged_Run_BypassesSudoEntirelyForEmptyAsUser(t *testing.T) {
	withAlreadyIs(t, func(string) bool { return false })
	p := &Privileged{
		// No AsUser set — even though sudo is fully available.
		Escalation: EscalationReport{Available: true, Reason: EscalationAvailablePasswordless},
	}
	cmd, err := p.Command(context.Background(), "echo", "hi")
	if err != nil {
		t.Fatalf("Command: %v", err)
	}
	if strings.HasSuffix(cmd.Args[0], "sudo") {
		t.Errorf("Args[0] = %q, want direct exec (empty AsUser means no escalation regardless of report)", cmd.Args[0])
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
