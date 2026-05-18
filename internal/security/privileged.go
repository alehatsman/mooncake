// Privileged is the spec-69 "I need root" command-exec primitive,
// surfaced on actions.Context as ctx.Privileged(). It wraps
// BecomeRunner with a smaller API tuned for the common case:
// "escalate to root if not already root, return CombinedOutput or
// error." Handlers stop constructing BecomeRunner directly so they
// can't forget to escalate the way pkg.upgrade and pkg.repo did.

package security

import (
	"bytes"
	"context"
	"os"
	"os/exec"
)

// PrivilegedRunner is the spec-69 implementation of
// actions.PrivilegedRunner. It defers all heavy lifting to
// BecomeRunner.
type PrivilegedRunner struct {
	// SudoPass is the password piped into `sudo -S` when escalation is
	// needed. Carries the same "empty = unsupported" semantics as
	// BecomeRunner.SudoPass — except when PasswordlessSudo is true.
	SudoPass string

	// PasswordlessSudo mirrors BecomeRunner.PasswordlessSudo: when
	// set, an empty SudoPass is allowed and commands run via `sudo -n`.
	// The `BecomeRunner(p)` conversion below relies on the field order
	// matching BecomeRunner.
	PasswordlessSudo bool
}

// becomeNeeded reports whether commands need to be wrapped in sudo to
// reach root. Mooncake-already-root paths (sudo mooncake apply, root
// shell) skip the sudo wrap and exec directly.
//
// Overridable for tests via the package-level becomeNeededFn hook.
var becomeNeededFn = func() bool { return os.Geteuid() != 0 }

// Run executes program+args under sudo when needed. Output is
// stdout+stderr concatenated, matching exec.Cmd.CombinedOutput so
// callers see sudo's error text without having to plumb a second
// pipe.
func (p PrivilegedRunner) Run(ctx context.Context, program string, args ...string) ([]byte, error) {
	cmd, err := p.command(ctx, program, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// RunWithInput runs the command with stdin piped through. When sudo
// escalation is needed, the password is prefixed onto stdin (matches
// BecomeRunner's exec stdin wiring).
func (p PrivilegedRunner) RunWithInput(ctx context.Context, stdin []byte, program string, args ...string) ([]byte, error) {
	cmd, err := p.command(ctx, program, args...)
	if err != nil {
		return nil, err
	}
	if becomeNeededFn() && p.SudoPass != "" {
		// BecomeRunner.Command already wired the password as stdin;
		// append the caller's stdin after it so sudo reads the
		// password first and forwards the rest to the program.
		var combined bytes.Buffer
		combined.WriteString(p.SudoPass)
		combined.WriteByte('\n')
		combined.Write(stdin)
		cmd.Stdin = &combined
	} else {
		// Either no escalation needed, or escalation via `sudo -n`
		// (PasswordlessSudo) — sudo doesn't read stdin in -n mode,
		// so the caller's stdin goes straight through to the program.
		cmd.Stdin = bytes.NewReader(stdin)
	}
	return cmd.CombinedOutput()
}

// command builds the *exec.Cmd, honoring the existing BecomeRunner
// validation (unsupported platform, missing password) so callers get
// a single error class.
func (p PrivilegedRunner) command(ctx context.Context, program string, args ...string) (*exec.Cmd, error) {
	become := becomeNeededFn()
	runner := BecomeRunner(p)
	cmd, err := runner.Command(become, program, args...)
	if err != nil {
		return nil, err
	}
	if ctx != nil {
		// BecomeRunner.Command returns an exec.Command(...) that
		// doesn't carry our caller context. Re-wrap so cancellation
		// propagates. Keep the existing Args/Stdin so the sudo
		// pre-wiring isn't lost.
		wrapped := exec.CommandContext(ctx, cmd.Path, cmd.Args[1:]...) //nolint:gosec // already validated by BecomeRunner
		wrapped.Stdin = cmd.Stdin
		wrapped.Env = cmd.Env
		return wrapped, nil
	}
	return cmd, nil
}
