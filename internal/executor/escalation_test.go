package executor

import (
	"context"
	"errors"
	"runtime"
	"strings"
	"testing"
)

// withProbeHooks replaces the probe seams for the duration of the
// test and restores them on cleanup. Centralizes the boilerplate so
// the test bodies read as a (setup, assert) pair.
func withProbeHooks(t *testing.T, euid int, nnp bool, nnpDetail string, sudoPath string, sudoLookErr error, sudoStderr string, sudoRunErr error) {
	t.Helper()
	origGeteuid := probeGeteuid
	origReadSelfStatus := probeReadSelfStatus
	origLookPathSudo := probeLookPathSudo
	origSudoNTrue := probeSudoNTrue
	t.Cleanup(func() {
		probeGeteuid = origGeteuid
		probeReadSelfStatus = origReadSelfStatus
		probeLookPathSudo = origLookPathSudo
		probeSudoNTrue = origSudoNTrue
	})
	probeGeteuid = func() int { return euid }
	probeReadSelfStatus = func() (bool, string) { return nnp, nnpDetail }
	probeLookPathSudo = func() (string, error) { return sudoPath, sudoLookErr }
	probeSudoNTrue = func(context.Context) (string, error) { return sudoStderr, sudoRunErr }
}

// TestProbeEscalation_AvailableRoot — euid 0 short-circuits before
// any sudo probe. No sudo binary, no /proc read, no probe call.
func TestProbeEscalation_AvailableRoot(t *testing.T) {
	withProbeHooks(t, 0, false, "", "", errors.New("LookPath should not be called"), "", nil)
	got := ProbeEscalation(context.Background(), "")
	if !got.Available {
		t.Fatalf("Available = false, want true (euid 0)")
	}
	if got.Reason != EscalationAvailableRoot {
		t.Errorf("Reason = %s, want available_root", got.Reason)
	}
}

// TestProbeEscalation_AvailablePassword — operator-supplied SudoPass
// short-circuits the live probe; we trust the caller and don't
// validate the password (which would have side effects).
func TestProbeEscalation_AvailablePassword(t *testing.T) {
	withProbeHooks(t, 1000, false, "", "", errors.New("LookPath should not be called"), "", nil)
	got := ProbeEscalation(context.Background(), "secret")
	if !got.Available {
		t.Fatalf("Available = false, want true (SudoPass set)")
	}
	if got.Reason != EscalationAvailablePassword {
		t.Errorf("Reason = %s, want available_password", got.Reason)
	}
}

// TestProbeEscalation_AvailablePasswordless — non-root, no password,
// no NNP, sudo present, `sudo -n true` exits 0 → passwordless.
// Detail carries the sudo binary path for diagnostic surfaces.
func TestProbeEscalation_AvailablePasswordless(t *testing.T) {
	withProbeHooks(t, 1000, false, "", "/usr/bin/sudo", nil, "", nil)
	got := ProbeEscalation(context.Background(), "")
	if !got.Available {
		t.Fatalf("Available = false, want true (NOPASSWD probe succeeded)")
	}
	if got.Reason != EscalationAvailablePasswordless {
		t.Errorf("Reason = %s, want available_passwordless", got.Reason)
	}
	if got.Detail != "/usr/bin/sudo" {
		t.Errorf("Detail = %q, want %q", got.Detail, "/usr/bin/sudo")
	}
}

// TestProbeEscalation_BlockedNNP — Linux-only branch: the
// /proc/self/status probe returning NoNewPrivs=1 short-circuits with
// the typed reason and a human-readable detail. On non-Linux the
// probe is skipped, so we skip the test (would fall through to the
// sudo branch and report a different reason).
func TestProbeEscalation_BlockedNNP(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("NNP probe only fires on Linux")
	}
	withProbeHooks(t, 1000, true, "NoNewPrivs=1 in /proc/self/status", "", errors.New("LookPath should not be called"), "", nil)
	got := ProbeEscalation(context.Background(), "")
	if got.Available {
		t.Fatalf("Available = true, want false (NNP set)")
	}
	if got.Reason != EscalationBlockedNNP {
		t.Errorf("Reason = %s, want blocked_nnp", got.Reason)
	}
	if !strings.Contains(got.Detail, "NoNewPrivs") {
		t.Errorf("Detail = %q, want NoNewPrivs reference", got.Detail)
	}
}

// TestProbeEscalation_BlockedSudoMissing — sudo not on PATH. The
// LookPath error is surfaced verbatim in Detail so the operator can
// see exactly what was attempted (e.g. "exec: sudo: executable file
// not found in $PATH").
func TestProbeEscalation_BlockedSudoMissing(t *testing.T) {
	withProbeHooks(t, 1000, false, "", "", errors.New(`exec: "sudo": executable file not found in $PATH`), "", nil)
	got := ProbeEscalation(context.Background(), "")
	if got.Available {
		t.Fatalf("Available = true, want false (sudo missing)")
	}
	if got.Reason != EscalationBlockedSudoMissing {
		t.Errorf("Reason = %s, want blocked_sudo_missing", got.Reason)
	}
	if !strings.Contains(got.Detail, "sudo") {
		t.Errorf("Detail = %q, want sudo-related diagnostic", got.Detail)
	}
}

// TestProbeEscalation_BlockedSudoersInsecure — sudo exits non-zero
// with stderr matching the "owned by uid" hint, which sudo emits
// when /etc/sudoers.d/<file> has the wrong ownership. The hint is
// loose-match (sudo's wording varies by distro/version) so the test
// pins the substring rather than the full line.
func TestProbeEscalation_BlockedSudoersInsecure(t *testing.T) {
	stderr := "sudo: /etc/sudoers.d/alehnopasswd is owned by uid 1000, should be 0\n"
	withProbeHooks(t, 1000, false, "", "/usr/bin/sudo", nil, stderr, errors.New("exit status 1"))
	got := ProbeEscalation(context.Background(), "")
	if got.Available {
		t.Fatalf("Available = true, want false (insecure sudoers)")
	}
	if got.Reason != EscalationBlockedSudoersInsecure {
		t.Errorf("Reason = %s, want blocked_sudoers_insecure", got.Reason)
	}
	if !strings.Contains(got.Detail, "owned by uid") {
		t.Errorf("Detail = %q, want stderr substring 'owned by uid'", got.Detail)
	}
}

// TestProbeEscalation_BlockedProbeFailed — sudo exits non-zero with
// stderr that doesn't match a more-specific reason (typical case:
// "a password is required"). The raw stderr lands in Detail so the
// operator gets the diagnostic without us having to enumerate every
// possible sudo error.
func TestProbeEscalation_BlockedProbeFailed(t *testing.T) {
	stderr := "sudo: a password is required\n"
	withProbeHooks(t, 1000, false, "", "/usr/bin/sudo", nil, stderr, errors.New("exit status 1"))
	got := ProbeEscalation(context.Background(), "")
	if got.Available {
		t.Fatalf("Available = true, want false (probe failed)")
	}
	if got.Reason != EscalationBlockedProbeFailed {
		t.Errorf("Reason = %s, want blocked_probe_failed", got.Reason)
	}
	if !strings.Contains(got.Detail, "password is required") {
		t.Errorf("Detail = %q, want stderr passthrough", got.Detail)
	}
}

// TestEscalationReason_String_StableNames pins the lowercase-
// snake_case identifiers; they're embedded in error messages and
// structured logs, so changes here are a breaking change for
// downstream parsers / alert rules.
func TestEscalationReason_String_StableNames(t *testing.T) {
	cases := map[EscalationReason]string{
		EscalationAvailableRoot:          "available_root",
		EscalationAvailablePassword:      "available_password",
		EscalationAvailablePasswordless:  "available_passwordless",
		EscalationBlockedNNP:             "blocked_nnp",
		EscalationBlockedSudoMissing:     "blocked_sudo_missing",
		EscalationBlockedSudoersInsecure: "blocked_sudoers_insecure",
		EscalationBlockedProbeFailed:     "blocked_probe_failed",
	}
	for r, want := range cases {
		if got := r.String(); got != want {
			t.Errorf("EscalationReason(%d).String() = %q, want %q", r, got, want)
		}
	}
}

// TestEscalationReason_Remediation_BlockedReasonsHaveHints — every
// blocked_* reason carries an operator-facing remediation; every
// available_* reason returns the empty string. The actual wording
// is the implementation's call (deliberately not pinned here) but
// the presence/absence contract is part of phase 3's diagnostic
// pipeline and must hold from phase 1 onward.
func TestEscalationReason_Remediation_BlockedReasonsHaveHints(t *testing.T) {
	for _, r := range []EscalationReason{
		EscalationAvailableRoot,
		EscalationAvailablePassword,
		EscalationAvailablePasswordless,
	} {
		if got := r.Remediation(); got != "" {
			t.Errorf("Remediation(%s) = %q, want empty for available_* reasons", r, got)
		}
	}
	for _, r := range []EscalationReason{
		EscalationBlockedNNP,
		EscalationBlockedSudoMissing,
		EscalationBlockedSudoersInsecure,
		EscalationBlockedProbeFailed,
	} {
		if got := r.Remediation(); got == "" {
			t.Errorf("Remediation(%s) = empty, want non-empty hint for blocked_* reasons", r)
		}
	}
}
