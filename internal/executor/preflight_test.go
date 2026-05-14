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
	if err := preflightPermissions(actions.PermissionSet{}, step); err != nil {
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
	err := preflightPermissions(actions.PermissionSet{Sudo: true}, step)
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
	if err := preflightPermissions(actions.PermissionSet{Sudo: true}, step); err != nil {
		t.Errorf("AsUser=root should satisfy Sudo preflight, got: %v", err)
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
	err := preflightPermissions(perms, step)
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
	if err := preflightPermissions(perms, step); err != nil {
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
	if err := preflightPermissions(perms, step); err != nil {
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
