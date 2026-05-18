package executor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"time"

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
func preflightPermissions(perms actions.PermissionSet, step *config.Step, sudoAvailable bool) error {
	if step != nil && perms.Sudo && !runningElevated() {
		if step.AsUser == "" {
			return fmt.Errorf(
				"step %q requires elevated privileges (Sudo: true) but mooncake is not running as root and the step has no as_user; add as_user: root or run mooncake with sudo",
				stepLabel(step),
			)
		}
		if !sudoAvailable {
			return fmt.Errorf(
				"step %q requires elevated privileges (as_user: %s, Sudo: true) but no sudo password is configured and `sudo -n true` failed; configure a NOPASSWD sudoers rule for this user, pass --sudo-pass / --sudo-pass-file / --ask-become-pass, or run mooncake as root",
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

// detectPasswordlessSudo probes `sudo -n true` once to determine
// whether the current user can escalate without a password (i.e.
// covered by a NOPASSWD sudoers rule). Returns false on any failure
// — the executor then falls back to the legacy "require configured
// password" rule. The probe is short-timed (2s) so a misconfigured
// sudoers file can't stall startup.
//
// Called once at RunServices construction; the result is cached on
// Svc.PasswordlessSudo for the rest of the run.
func detectPasswordlessSudo(ctx context.Context) bool {
	// Already root → escalation is moot; report false so callers
	// take the no-escalation branch via runningElevated().
	if runningElevated() {
		return false
	}
	// Test/MCP callers occasionally pass a nil context (executor.Start
	// is invoked without a root context wired in). Fall back to
	// Background so context.WithTimeout doesn't panic on nil.
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, "sudo", "-n", "true") //nolint:gosec // fixed args, no interpolation
	if err := cmd.Run(); err != nil {
		return false
	}
	return true
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
