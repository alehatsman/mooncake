package security

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
)

// Privileged is the spec-72 Layer C escalation primitive. One value
// is constructed per-step by executor.dispatchRunner with the step's
// AsUser bound in; handlers reach it via ctx.Privileged() and call
// Run / Command without ever reading step.AsUser or threading a
// `become bool` through helper signatures. The bound AsUser is the
// single source of truth for whether (and as whom) this shell-out
// escalates.
//
// AsUser semantics:
//
//	""                         → run as the current process (no sudo)
//	"root" || "0"              → sudo to root (or no-op when already root)
//	"<other>"                  → sudo -u <other> (or no-op when already <other>)
//
// The "already that user" short-circuit matches config.Step.ShouldBecome's
// invariant: minimal containers (ubuntu:24.04, alpine:3.21) that don't
// ship sudo can still run as_user:root presets when mooncake is invoked
// as root.
//
// Errors flow from the underlying BecomeRunner sentinels
// (ErrBecomeUnsupported on platforms without sudo,
// ErrBecomeNoSudoPass when escalation is needed but neither a sudo
// password nor a NOPASSWD rule is available). Wrap with errors.Is at
// call sites that want to map them to a user-facing diagnostic.
type Privileged struct {
	// SudoPass is the operator-supplied password piped to sudo via
	// `sudo -S`. Empty when the operator didn't supply one; the probe
	// (Escalation) tells us whether escalation can still succeed via
	// a NOPASSWD rule.
	SudoPass string

	// Escalation is the once-per-run report from ProbeEscalation,
	// carried on RunServices and snapshotted into each per-step
	// Privileged. Drives the `sudo -n` vs `sudo -S` choice and the
	// "blocked because NoNewPrivileges" diagnostic.
	Escalation EscalationReport

	// AsUser is the step's declared target identity. Bound by
	// executor.dispatchRunner from step.AsUser before calling Run on
	// the handler. Unbound (empty) means "do not escalate"; see the
	// type comment for the full table.
	AsUser string
}

// Run executes program+args under the bound AsUser and returns
// combined stdout+stderr (exec.Cmd.CombinedOutput shape). The
// "common shape" entry point — the vast majority of handlers want
// this.
func (p *Privileged) Run(ctx context.Context, program string, args ...string) ([]byte, error) {
	cmd, err := p.Command(ctx, program, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// Command returns a configured *exec.Cmd for callers that need
// per-subcommand state (cmd.ProcessState, private stderr buffers,
// staged stdin). service/shared.writeFileWithSudo's cp + chmod
// sequence and os_systemd.runSystemctl's stderr buffering are the
// canonical callers.
//
// The returned *exec.Cmd already has sudo's stdin password buffer
// wired when SudoPass is non-empty; callers wanting to send their
// own stdin must concatenate it after, matching the wiring in
// PrivilegedRunner.RunWithInput.
func (p *Privileged) Command(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	wrap, err := p.shouldWrap()
	if err != nil {
		return nil, err
	}
	if wrap == nil {
		// No sudo: exec directly as the current user.
		return commandContext(ctx, program, args...), nil
	}
	// Sudo prefix: build "[sudo, -S|-n, [-u <name>,] program, args...]"
	sudoArgs := append(wrap, program)
	sudoArgs = append(sudoArgs, args...)
	cmd := commandContext(ctx, "sudo", sudoArgs...)
	if p.SudoPass != "" {
		cmd.Stdin = bytes.NewBufferString(p.SudoPass + "\n")
	}
	return cmd, nil
}

// shouldWrap returns the sudo argument prefix when this call needs
// escalation, nil when it can exec directly, and an error when
// escalation is needed but blocked (NNP / sudo missing / no
// password and no NOPASSWD). The returned slice always ends with
// the password-mode flag (-S or -n) so the caller can append
// `[program, args...]` directly.
func (p *Privileged) shouldWrap() ([]string, error) {
	target := normalizeAsUser(p.AsUser)
	if target == "" {
		return nil, nil
	}
	if alreadyIs(target) {
		return nil, nil
	}
	if !IsBecomeSupported() {
		return nil, fmt.Errorf("%w (GOOS=%s)", ErrBecomeUnsupported, runtime.GOOS)
	}
	if !p.Escalation.Available && p.SudoPass == "" {
		// Neither a sudo password configured nor a passwordless probe
		// success — escalation cannot proceed. Same sentinel
		// BecomeRunner has always returned so callers that errors.Is
		// against it keep working.
		return nil, ErrBecomeNoSudoPass
	}
	// `sudo -S` reads the password from stdin; `sudo -n` runs
	// non-interactively (succeeds under NOPASSWD, fails fast
	// otherwise). Pick the right one based on whether the operator
	// supplied a password.
	mode := "-n"
	if p.SudoPass != "" {
		mode = "-S"
	}
	prefix := []string{mode}
	if target != "root" && target != "0" {
		prefix = append(prefix, "-u", target)
	}
	return prefix, nil
}

// normalizeAsUser canonicalises numeric uid 0 to "root" so the
// already-target check below can compare against a single name.
// Whitespace is trimmed because YAML round-trips occasionally
// preserve trailing spaces in scalar values.
func normalizeAsUser(s string) string {
	s = trimSpace(s)
	return s
}

// alreadyIs reports whether the current process already runs as the
// target user (in which case sudo is a no-op and we skip the wrap).
// Today only the "root" and "0" cases are checked via Geteuid;
// per-name resolution (looking up the named user's uid and
// comparing against euid) is a follow-up the primitive can grow
// without disturbing call sites.
var alreadyIs = func(target string) bool {
	if target == "root" || target == "0" {
		return os.Geteuid() == 0
	}
	return false
}

func commandContext(ctx context.Context, program string, args ...string) *exec.Cmd {
	if ctx == nil {
		return exec.Command(program, args...) //nolint:gosec // provisioning tool runs user-defined programs
	}
	return exec.CommandContext(ctx, program, args...) //nolint:gosec // provisioning tool runs user-defined programs
}

// trimSpace mirrors strings.TrimSpace without importing strings —
// keeps the privileged_type.go import set small. The string is
// short enough that a manual scan is fine.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(b byte) bool { return b == ' ' || b == '\t' || b == '\n' || b == '\r' }

// Compile-time check: the empty-string and numeric-zero AsUser
// constants stay in sync. Strictly informational — strconv is
// imported so this is cheap.
var _ = strconv.Itoa(0)

// ErrPrivilegedUnsupportedTarget is reserved for a future "named
// user not resolvable" diagnostic. Currently unused; declared so
// callers can build switch errors.Is statements against it without
// the sentinel disappearing later.
var ErrPrivilegedUnsupportedTarget = errors.New("privileged: named user target not resolvable on this platform")
