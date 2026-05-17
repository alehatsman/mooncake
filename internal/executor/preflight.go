package executor

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
)

// preflightPermissions checks a step's declared PermissionSet against
// the executor's runtime context before the step runs. Fails-fast on
// missing privileges or missing binaries so the user sees a clear
// error at the start of the run rather than a confusing partial run
// that EACCESs halfway through.
//
// Spec-22 §"Where Permissions surfaces":
//   - Sudo + non-root + empty AsUser → error
//   - RequiredBinaries missing on PATH → error
//   - Network is informational only (no enforcement)
//
// Spec-69 phase 4 extension:
//   - Sudo + non-root + AsUser set + no sudo password configured
//     → error at plan/dispatch time. Without this, the step plans
//     fine, then EACCES'd later at apply when the handler tried to
//     escalate. Now the operator hears about the missing password
//     before any side effects start.
//
// This function is called from dispatchRunner for every handler that
// implements actions.Permitter. Handlers without a Permitter
// implementation never trigger preflight — they get the legacy
// "runtime error" path until they opt in.
func preflightPermissions(perms actions.PermissionSet, step *config.Step, sudoPassConfigured bool) error {
	if step != nil && perms.Sudo && !runningElevated() {
		if step.AsUser == "" {
			return fmt.Errorf(
				"step %q requires elevated privileges (Sudo: true) but mooncake is not running as root and the step has no as_user; add as_user: root or run mooncake with sudo",
				stepLabel(step),
			)
		}
		if !sudoPassConfigured {
			return fmt.Errorf(
				"step %q requires elevated privileges (as_user: %s, Sudo: true) but no sudo password is configured; pass --sudo-pass / --sudo-pass-file / --ask-become-pass, or run mooncake as root",
				stepLabel(step), step.AsUser,
			)
		}
	}
	for _, bin := range perms.RequiredBinaries {
		if _, err := exec.LookPath(bin); err != nil {
			label := ""
			if step != nil {
				label = " in step " + stepLabel(step)
			}
			return fmt.Errorf(
				"required binary %q is not on PATH%s: %w",
				bin, label, err,
			)
		}
	}
	return nil
}

// runningElevated reports whether the current process has effective
// root privileges. POSIX uses euid==0; on Windows mooncake's spec-36
// design treats `run_as_admin` as a step-level declaration that the
// PowerShell wrapper enforces, so the daemon process itself doesn't
// need elevation. For Windows we return false unconditionally, which
// disables the Sudo preflight there; Windows-specific privilege
// checks are out of scope for spec-22 phase 3a.
func runningElevated() bool {
	// os.Geteuid returns -1 on Windows (no POSIX euid concept). We
	// treat -1 as "no privilege check applicable" → not elevated, but
	// the preflight rule with AsUser bypass means Windows callers
	// wanting elevation declare run_as_admin via the shell action
	// (spec-36) and aren't affected.
	euid := os.Geteuid()
	return euid == 0
}

// stepLabel produces a short human-readable identifier for a step,
// used in error messages. Falls back to the action type when Name is
// empty so error text stays useful for unnamed steps in tests/examples.
func stepLabel(step *config.Step) string {
	if step == nil {
		return "<nil>"
	}
	if step.Name != "" {
		return step.Name
	}
	return step.DetermineActionType()
}
