package security

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
)

// F005: BecomeRunner is the canonical "shell out under optional sudo
// elevation" primitive. Pre-fix the project had 6 distinct
// implementations of this pattern (effects.runSudo, package.runCmd,
// service handler ×6 inline sites, download inline, template inline)
// with INCONSISTENT validation:
//
//   - effects + 4-of-6 service sites validated IsBecomeSupported
//     before invoking sudo; the other 5 went straight to
//     exec.Command("sudo", ...) which fails with the OS-level
//     "executable file not found" on platforms without sudo.
//   - service sites validated empty SudoPass; the rest passed
//     bytes.NewBufferString(""+"\n") which makes sudo prompt-and-hang
//     when it has a TTY or fail with a cryptic "no password" otherwise.
//
// New code paths should build their *exec.Cmd via BecomeRunner so
// "become requested but unsupported" and "become requested but
// SudoPass empty" produce a single diagnosable error class.

// BecomeRunner carries the sudo-password state and applies the
// project-wide "shell out with optional `sudo -S`" policy.
//
// Zero value works for non-become calls: BecomeRunner{}.Command(false,
// "ls", "/").
type BecomeRunner struct {
	// SudoPass is the password piped to sudo's stdin via `sudo -S`.
	// Empty when the operator didn't supply one; callers requesting
	// become with an empty SudoPass get a clean error rather than a
	// hung sudo prompt — *unless* PasswordlessSudo is true, in which
	// case the command runs under `sudo -n` (sudo's "no password
	// prompt" mode, succeeds iff a NOPASSWD sudoers rule covers it).
	SudoPass string

	// PasswordlessSudo signals that the operator's sudo is configured
	// with NOPASSWD (typically via /etc/sudoers.d/<user>-nopasswd).
	// Set by the executor at run startup via a `sudo -n true` probe.
	// When true, Command builds `sudo -n <cmd>` even with an empty
	// SudoPass — sudo declines to prompt and either succeeds via the
	// NOPASSWD rule or fails fast with a clear "a password is
	// required" message instead of hanging.
	PasswordlessSudo bool
}

// ErrBecomeUnsupported is returned by BecomeRunner.Command when
// become=true on a platform without sudo (e.g. Windows).
var ErrBecomeUnsupported = errors.New("become not supported on this platform")

// ErrBecomeNoSudoPass is returned by BecomeRunner.Command when
// become=true but BecomeRunner.SudoPass is empty.
var ErrBecomeNoSudoPass = errors.New("become requested but no sudo password configured (use --sudo-pass or --ask-become-pass)")

// Command builds an *exec.Cmd for `program args...`. When become is
// true, it prepends `sudo -S` and pipes SudoPass to stdin.
//
// Returns ErrBecomeUnsupported / ErrBecomeNoSudoPass on the two
// validation failures; nil otherwise. The caller invokes
// cmd.Run() / cmd.CombinedOutput() / etc. as usual.
//
// For shell-style call sites that want to run a composed command
// string, pass `("sh", "-c", command)` — see effects.runSudo for an
// example of the wrapping.
func (r BecomeRunner) Command(become bool, program string, args ...string) (*exec.Cmd, error) {
	if !become {
		return exec.Command(program, args...), nil //nolint:gosec // provisioning tool runs user-defined programs
	}
	if !IsBecomeSupported() {
		return nil, fmt.Errorf("%w (GOOS=%s)", ErrBecomeUnsupported, runtime.GOOS)
	}
	if r.SudoPass == "" && !r.PasswordlessSudo {
		return nil, ErrBecomeNoSudoPass
	}
	// `sudo -n` runs non-interactively: succeeds under NOPASSWD,
	// fails immediately with a "password is required" diagnostic
	// otherwise. `sudo -S` reads the password from stdin. We use -n
	// when no password is configured (passwordless path) and -S when
	// one is — never both: -nS makes -S a no-op since sudo never reads
	// stdin in non-interactive mode.
	var sudoArgs []string
	if r.SudoPass == "" {
		sudoArgs = append([]string{"-n", program}, args...)
	} else {
		sudoArgs = append([]string{"-S", program}, args...)
	}
	cmd := exec.Command("sudo", sudoArgs...) //nolint:gosec // provisioning tool runs user-defined programs
	if r.SudoPass != "" {
		cmd.Stdin = bytes.NewBufferString(r.SudoPass + "\n")
	}
	return cmd, nil
}
