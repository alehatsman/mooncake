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
	cmd, err := p.commandWith(ctx, becomeNeededFn(), program, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// RunWithBecome is the per-call-conditional-escalation variant of
// Run (spec-72 phase 2). `become=true` matches Run exactly; with
// `become=false` we skip the sudo wrap and exec the program
// directly. Callers in mixed always/never escalation contexts
// (notably pkg.runCmd's brew-vs-apt branch) use this in place of
// constructing BecomeRunner directly so the spec-72 lint rule's
// goal — ctx.Privileged() as the only escalation constructor —
// holds.
func (p PrivilegedRunner) RunWithBecome(ctx context.Context, become bool, program string, args ...string) ([]byte, error) {
	cmd, err := p.commandWith(ctx, become && becomeNeededFn(), program, args...)
	if err != nil {
		return nil, err
	}
	return cmd.CombinedOutput()
}

// RunWithInput runs the command with stdin piped through. When sudo
// escalation is needed, the password is prefixed onto stdin (matches
// BecomeRunner's exec stdin wiring).
func (p PrivilegedRunner) RunWithInput(ctx context.Context, stdin []byte, program string, args ...string) ([]byte, error) {
	cmd, err := p.commandWith(ctx, becomeNeededFn(), program, args...)
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

// Command returns a configured *exec.Cmd for callers that need
// access to per-subcommand exit codes (cmd.ProcessState), private
// stderr buffers (cmd.Stderr = &buf), or other exec.Cmd state the
// Run* methods hide. Returns the same sentinel error class as the
// Run* methods when become is requested on an unsupported platform
// or with no sudo password configured.
//
// Spec-72 phase 2b: the three remaining hand-rolled
// security.BecomeRunner constructs (service/shared, os_systemd) use
// this to centralize escalation under ctx.Privileged() while keeping
// their per-subcommand diagnostic shape.
func (p PrivilegedRunner) Command(ctx context.Context, become bool, program string, args ...string) (*exec.Cmd, error) {
	// becomeNeededFn() gates the actual sudo wrap: callers may
	// request become=true unconditionally (write-with-sudo helpers)
	// but if mooncake is already root, no wrap is needed. This
	// matches Run's behavior so a caller switching between
	// Run / Command / RunWithBecome doesn't get different sudo
	// semantics for the same input.
	return p.commandWith(ctx, become && becomeNeededFn(), program, args...)
}

// commandWith builds the *exec.Cmd, honoring the existing
// BecomeRunner validation (unsupported platform, missing password)
// so callers get a single error class.
func (p PrivilegedRunner) commandWith(ctx context.Context, become bool, program string, args ...string) (*exec.Cmd, error) {
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
