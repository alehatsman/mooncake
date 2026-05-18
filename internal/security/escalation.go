package security

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"
)

// EscalationReport captures the once-per-run answer to "can this
// process escalate to root, and if not, why not?". Computed by the
// executor at run startup via ProbeEscalation and stored on
// RunServices.Escalation. Consumed by *Privileged (this package)
// to decide the sudo wrap, by preflight (executor) for diagnostic
// error messages, and by future policy layers.
//
// Moved from internal/executor in spec-72's Layer C redesign so the
// escalation primitive (*Privileged) can live next to its inputs
// without forcing internal/actions to import internal/executor.
type EscalationReport struct {
	// Available is true iff a Sudo step can be expected to succeed
	// against this process. The union of every "available_*" Reason.
	Available bool

	// Reason explains the verdict. Stable across a run.
	Reason EscalationReason

	// Detail carries reason-specific extra info: the sudo binary path
	// (when found), the stderr from `sudo -n true` (when the probe
	// failed), the directive that's blocking (when we can detect it
	// from /proc/self/status). Free-form string, safe to include in
	// diagnostic output.
	Detail string
}

// EscalationReason names the seven verdicts ProbeEscalation can
// return. The "available_*" reasons all set Report.Available=true;
// the "blocked_*" reasons all set it false.
type EscalationReason int

const (
	// EscalationAvailableRoot — the current process already has
	// euid 0. Escalation is a no-op; sudo is not invoked.
	EscalationAvailableRoot EscalationReason = iota

	// EscalationAvailablePassword — the operator passed --sudo-pass /
	// --sudo-pass-file / --ask-become-pass. We trust the caller; the
	// probe doesn't validate the password against real sudoers because
	// that has side effects.
	EscalationAvailablePassword

	// EscalationAvailablePasswordless — `sudo -n true` succeeded, so a
	// NOPASSWD sudoers rule covers this user for at least the trivial
	// command. *Privileged uses `sudo -n <cmd>` on this path.
	EscalationAvailablePasswordless

	// EscalationBlockedNNP — /proc/self/status reports NoNewPrivs=1.
	// sudo refuses with "The 'no new privileges' flag is set, which
	// prevents sudo from running as root" regardless of sudoers; the
	// blocker is the systemd unit (or other ancestor) that set the
	// directive.
	EscalationBlockedNNP

	// EscalationBlockedSudoMissing — exec.LookPath("sudo") failed.
	// Either install sudo or run mooncake as root.
	EscalationBlockedSudoMissing

	// EscalationBlockedSudoersInsecure — `sudo -n true` failed with
	// stderr indicating bad ownership/mode on a sudoers file (matched
	// loosely on "owned by uid"). The fix is fchown/chmod on the
	// offending file, not anything mooncake controls.
	EscalationBlockedSudoersInsecure

	// EscalationBlockedProbeFailed — `sudo -n true` failed with no
	// recognizable diagnostic. Detail carries the raw stderr.
	EscalationBlockedProbeFailed
)

// String returns the stable lowercase-snake_case identifier for a
// reason, suitable for inclusion in error messages and structured
// logs. Mirrors the enum constant names without the package prefix.
func (r EscalationReason) String() string {
	switch r {
	case EscalationAvailableRoot:
		return "available_root"
	case EscalationAvailablePassword:
		return "available_password"
	case EscalationAvailablePasswordless:
		return "available_passwordless"
	case EscalationBlockedNNP:
		return "blocked_nnp"
	case EscalationBlockedSudoMissing:
		return "blocked_sudo_missing"
	case EscalationBlockedSudoersInsecure:
		return "blocked_sudoers_insecure"
	case EscalationBlockedProbeFailed:
		return "blocked_probe_failed"
	default:
		return "unknown"
	}
}

// Remediation returns a one-line operator-facing hint for the
// blocker, or "" for the available_* reasons.
func (r EscalationReason) Remediation() string {
	switch r {
	case EscalationBlockedNNP:
		return "drop NoNewPrivileges=true from the systemd unit (or equivalent ancestor hardening)"
	case EscalationBlockedSudoMissing:
		return "install sudo or run mooncake as root"
	case EscalationBlockedSudoersInsecure:
		return "fix file ownership/mode under /etc/sudoers.d/ (sudoers files must be owned by root:root)"
	case EscalationBlockedProbeFailed:
		return "check sudoers rules for this user; raw stderr above"
	default:
		return ""
	}
}

// Probe hooks. Real implementations defer to the runtime; tests
// override these to drive ProbeEscalation through every Reason
// branch without needing real sudo / a real /proc.
var (
	probeGeteuid        = func() int { return os.Geteuid() }
	probeReadSelfStatus = readSelfStatus
	probeLookPathSudo   = func() (string, error) { return exec.LookPath("sudo") }
	probeSudoNTrue      = runSudoNTrue
)

// sudoProbeTimeout caps the `sudo -n true` invocation so a
// misconfigured sudoers file can't stall executor startup.
const sudoProbeTimeout = 2 * time.Second

// ProbeEscalation runs the once-per-run sudo-availability probe in
// the order documented in spec-72 §1 and returns an EscalationReport
// suitable for caching on RunServices.Escalation.
//
//  1. euid 0          → available_root
//  2. sudoPass != ""  → available_password
//  3. /proc/self/status NoNewPrivs=1 (Linux) → blocked_nnp
//  4. no sudo on PATH → blocked_sudo_missing
//  5. sudo -n true:
//     success                     → available_passwordless
//     stderr ~ "owned by uid"     → blocked_sudoers_insecure
//     other failure               → blocked_probe_failed
//
// Steps 3 and 5 are gated by a 2s timeout and nil-ctx guard so the
// probe can't hang the run. The function is side-effect-free apart
// from the `sudo -n true` invocation, which by construction is a
// no-op (`true` exits 0) whenever it can be run at all.
func ProbeEscalation(ctx context.Context, sudoPass string) EscalationReport {
	if probeGeteuid() == 0 {
		return EscalationReport{Available: true, Reason: EscalationAvailableRoot}
	}
	if sudoPass != "" {
		return EscalationReport{Available: true, Reason: EscalationAvailablePassword}
	}
	if runtime.GOOS == "linux" {
		if nnp, detail := probeReadSelfStatus(); nnp {
			return EscalationReport{Reason: EscalationBlockedNNP, Detail: detail}
		}
	}
	sudoPath, err := probeLookPathSudo()
	if err != nil {
		return EscalationReport{Reason: EscalationBlockedSudoMissing, Detail: err.Error()}
	}
	if ctx == nil {
		ctx = context.Background()
	}
	probeCtx, cancel := context.WithTimeout(ctx, sudoProbeTimeout)
	defer cancel()
	stderr, err := probeSudoNTrue(probeCtx)
	if err == nil {
		return EscalationReport{Available: true, Reason: EscalationAvailablePasswordless, Detail: sudoPath}
	}
	if isSudoersInsecureStderr(stderr) {
		return EscalationReport{Reason: EscalationBlockedSudoersInsecure, Detail: strings.TrimSpace(stderr)}
	}
	return EscalationReport{Reason: EscalationBlockedProbeFailed, Detail: strings.TrimSpace(stderr)}
}

// isSudoersInsecureStderr matches the substring fingerprints sudo
// emits when refusing to read /etc/sudoers* because of bad
// ownership, mode, or writability. The exact wording varies across
// distros + sudo versions (Debian/Ubuntu, Fedora, Alpine's
// sudo-rs, etc.); we match on the load-bearing fragments rather
// than the full sentence so a distro that re-phrases "owned by uid
// X, should be 0" to "owner is not root" still trips the typed
// reason.
//
// Resolved spec-72 §Open-Questions §3: keeping the typed reason
// (BlockedSudoersInsecure → "fix file ownership/mode under
// /etc/sudoers.d/") is more useful to operators than collapsing
// into the generic BlockedProbeFailed bucket. Match heuristics
// are best-effort; novel distro wordings fall through to
// BlockedProbeFailed with the raw stderr in Detail, where the
// operator reads it directly.
func isSudoersInsecureStderr(stderr string) bool {
	for _, fingerprint := range []string{
		"owned by uid",      // Debian/Ubuntu/Fedora classic — "is owned by uid 1000, should be 0"
		"bad permissions",   // sudo's wording for mode mismatch — "/etc/sudoers.d/foo: bad permissions, should be mode 0440"
		"should be mode",    // alternate phrasing for the same mode-mismatch error
		"is world writable", // sudo refuses world-writable sudoers files outright
		"is not a regular",  // sudoers file is a symlink / device / directory
	} {
		if strings.Contains(stderr, fingerprint) {
			return true
		}
	}
	return false
}

// readSelfStatus parses /proc/self/status for the NoNewPrivs line.
// Returns (true, "NoNewPrivs=1 in /proc/self/status") when the
// directive is active; (false, "") otherwise or on read error
// (treating the file as unreadable as "not blocked" — we'd rather
// report blocked_probe_failed later from a real sudo failure than
// false-positive on a kernel without the field).
func readSelfStatus() (bool, string) {
	data, err := os.ReadFile("/proc/self/status")
	if err != nil {
		return false, ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "NoNewPrivs:") {
			if strings.TrimSpace(strings.TrimPrefix(line, "NoNewPrivs:")) == "1" {
				return true, "NoNewPrivs=1 in /proc/self/status"
			}
			return false, ""
		}
	}
	return false, ""
}

// runSudoNTrue invokes `sudo -n true` and returns its stderr +
// error. Split out as a package-level seam so escalation_test.go can
// drive the probe through its branches without needing a real sudo
// on the test host.
func runSudoNTrue(ctx context.Context) (string, error) {
	cmd := exec.CommandContext(ctx, "sudo", "-n", "true") //nolint:gosec // fixed args, no interpolation
	var stderr strings.Builder
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stderr.String(), err
}
