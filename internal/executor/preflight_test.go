package executor

import (
	"os"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// TestPreflight_NoPermsRequired_PassesCleanly — empty PermissionSet is
// the default for handlers that don't implement Permitter. Preflight
// must let those through without complaint.
func TestPreflight_NoPermsRequired_PassesCleanly(t *testing.T) {
	step := &config.Step{Name: "noop"}
	if err := preflightPermissions(actions.PermissionSet{}, step, true); err != nil {
		t.Errorf("empty PermissionSet should pass preflight, got: %v", err)
	}
}

// TestPreflight_SudoNeeded_NonRoot_NoAsUser_Fails locks in the
// fail-fast contract: a step that declares Sudo, running as non-root,
// with no as_user, must error out at preflight with a clear message.
// Without this, the user sees an EACCES halfway through a multi-step
// run and has to deduce the cause from "permission denied".
//
// Skipped when actually running as root (e.g. in some CI images),
// since the precondition can't be set up.
func TestPreflight_SudoNeeded_NonRoot_NoAsUser_Fails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root euid")
	}
	step := &config.Step{Name: "writes-etc"}
	err := preflightPermissions(actions.PermissionSet{Sudo: true}, step, true)
	if err == nil {
		t.Fatal("want preflight error for Sudo+non-root+no-AsUser, got nil")
	}
	if !strings.Contains(err.Error(), "requires elevated privileges") {
		t.Errorf("error message = %q, want substring 'requires elevated privileges'", err)
	}
	if !strings.Contains(err.Error(), "writes-etc") {
		t.Errorf("error message = %q, want step name 'writes-etc'", err)
	}
	if !strings.Contains(err.Error(), "as_user") {
		t.Errorf("error message = %q, want hint about 'as_user'", err)
	}
}

// TestPreflight_SudoNeeded_AsUserSet_Passes — when the step declares
// as_user, the preflight should treat that as the user's intent to
// switch context (mooncake delegates the actual switch to whatever
// runs the step). No preflight error.
func TestPreflight_SudoNeeded_AsUserSet_Passes(t *testing.T) {
	step := &config.Step{Name: "writes-etc", AsUser: "root"}
	if err := preflightPermissions(actions.PermissionSet{Sudo: true}, step, true); err != nil {
		t.Errorf("AsUser=root + sudo password configured should satisfy preflight, got: %v", err)
	}
}

// TestPreflight_SudoNeeded_AsUserSet_NoSudoPass_Fails locks in the
// spec-69 phase 4 extension: a step that declares Sudo + has as_user
// set but runs against a mooncake invocation with no sudo password
// configured fails at preflight, not at apply-time EACCES.
//
// Without this check the step planned cleanly and only blew up when
// the handler actually tried to escalate — exactly what bit the
// dotfiles pkg.upgrade migration on 2026-05-17 ("permission denied"
// on /var/lib/dpkg/lock-frontend halfway through an apply).
func TestPreflight_SudoNeeded_AsUserSet_NoSudoPass_Fails(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root euid")
	}
	step := &config.Step{Name: "writes-etc", AsUser: "root"}
	err := preflightPermissions(actions.PermissionSet{Sudo: true}, step, false)
	if err == nil {
		t.Fatal("want preflight error for Sudo+AsUser+no-sudo-pass, got nil")
	}
	if !strings.Contains(err.Error(), "no sudo password is configured") {
		t.Errorf("error message = %q, want substring 'no sudo password is configured'", err)
	}
	if !strings.Contains(err.Error(), "--sudo-pass") {
		t.Errorf("error message = %q, want flag hint '--sudo-pass'", err)
	}
}

// TestPreflight_MissingBinary_Fails verifies RequiredBinaries enforcement.
// A binary that's almost certainly not on PATH (made-up name) must
// trigger a preflight error referencing the missing binary by name AND
// the step it came from — so users can fix the right thing.
func TestPreflight_MissingBinary_Fails(t *testing.T) {
	step := &config.Step{Name: "needs-rare-tool"}
	perms := actions.PermissionSet{
		RequiredBinaries: []string{"this-binary-definitely-does-not-exist-xyz-12345"},
	}
	err := preflightPermissions(perms, step, true)
	if err == nil {
		t.Fatal("want preflight error for missing binary, got nil")
	}
	if !strings.Contains(err.Error(), "this-binary-definitely-does-not-exist-xyz-12345") {
		t.Errorf("error message = %q, want missing binary name", err)
	}
	if !strings.Contains(err.Error(), "needs-rare-tool") {
		t.Errorf("error message = %q, want step name 'needs-rare-tool'", err)
	}
}

// TestPreflight_PresentBinary_Passes — `sh` is on every POSIX system
// where these tests run. Sanity check that LookPath success is treated
// as pass.
func TestPreflight_PresentBinary_Passes(t *testing.T) {
	step := &config.Step{Name: "needs-sh"}
	perms := actions.PermissionSet{RequiredBinaries: []string{"sh"}}
	if err := preflightPermissions(perms, step, true); err != nil {
		t.Errorf("preflight on present binary 'sh' failed: %v", err)
	}
}

// TestPreflight_NetworkIsInformational — Network flag MUST NOT fail
// preflight today. It's reserved for a future policy DSL. If this
// test ever fails it means someone added enforcement without updating
// the contract, and existing handlers that declare Network would
// suddenly break.
func TestPreflight_NetworkIsInformational(t *testing.T) {
	step := &config.Step{Name: "talks-to-internet"}
	perms := actions.PermissionSet{Network: true}
	if err := preflightPermissions(perms, step, true); err != nil {
		t.Errorf("Network: true should be informational; preflight failed: %v", err)
	}
}

// TestRunningElevated_MatchesEuid — runningElevated() is the privilege
// oracle used by the Sudo check. Lock in that it agrees with os.Geteuid()
// on the test platform so the privilege check stays predictable.
func TestRunningElevated_MatchesEuid(t *testing.T) {
	want := os.Geteuid() == 0
	if got := runningElevated(); got != want {
		t.Errorf("runningElevated() = %v, want %v (euid=%d)", got, want, os.Geteuid())
	}
}

// TestPreflight_PasswordlessSudoSatisfiesSudoAvailable pins the
// rename of the third arg from `sudoPassConfigured` to `sudoAvailable`:
// the executor now feeds it the union of (SudoPass set) or (NOPASSWD
// probe succeeded), so a NOPASSWD operator with no --sudo-pass flag
// passes preflight. Without this contract, switching the agentd from
// root system unit to a user unit + NOPASSWD sudoers entry would
// still fail at "no sudo password is configured" because preflight
// would only honor explicit --sudo-pass.
func TestPreflight_PasswordlessSudoSatisfiesSudoAvailable(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("test requires non-root euid")
	}
	step := &config.Step{Name: "apt-upgrade", AsUser: "root"}
	if err := preflightPermissions(actions.PermissionSet{Sudo: true}, step, true); err != nil {
		t.Errorf("sudoAvailable=true (e.g. via NOPASSWD probe) should pass preflight, got: %v", err)
	}
}

// TestDetectPasswordlessSudo_NilContext_NoPanic — the executor calls
// detectPasswordlessSudo at RunServices construction; some callers
// (MCP, in-process apply) pass a nil context all the way through.
// The probe must defend against that rather than panicking inside
// context.WithTimeout.
func TestDetectPasswordlessSudo_NilContext_NoPanic(t *testing.T) {
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("detectPasswordlessSudo panicked on nil context: %v", r)
		}
	}()
	_ = detectPasswordlessSudo(nil) //nolint:staticcheck // nil ctx is the case under test
}
