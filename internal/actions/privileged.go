package actions

import (
	"context"
	"os/exec"
)

// PrivilegedRunner runs a command under root via sudo when mooncake is
// not already running as root. It is the command-exec sibling of
// Performer: handlers should call ctx.Privileged().Run(...) for any
// shell-out that needs root, instead of constructing exec.Command or
// security.BecomeRunner themselves.
//
// The default privilege target is root. Handlers that need to run as
// a specific non-root user (rare; mostly `as_user: <name>` on a
// shell action) should keep using security.BecomeRunner directly —
// this primitive is the "I need root" common path that covered the
// systemic bug class spec-69 surfaced.
//
// Errors:
//   - security.ErrBecomeUnsupported when mooncake is on a platform
//     without sudo (Windows).
//   - security.ErrBecomeNoSudoPass when mooncake is non-root and
//     no sudo password is configured.
//
// Both error classes surface from spec-22's plan-time preflight too,
// so a Sudo:true action without sudo configured fails at *plan*; the
// runtime errors above are the apply-time fallback if a handler is
// invoked outside the normal preflight path.
type PrivilegedRunner interface {
	// Run executes program with args under sudo (if needed) and
	// returns the combined stdout+stderr. Semantics match
	// exec.Cmd.CombinedOutput.
	Run(ctx context.Context, program string, args ...string) ([]byte, error)

	// RunWithInput runs the command with stdin piped in. Useful for
	// `apt-key add -` and similar. The sudo password is consumed
	// before the caller's stdin (matching security.BecomeRunner).
	RunWithInput(ctx context.Context, stdin []byte, program string, args ...string) ([]byte, error)

	// RunWithBecome is the per-call-conditional-escalation variant of
	// Run, added by spec-72 phase 2 to resolve Open Question §1.
	// `become=true` produces the same effect as Run (escalate if not
	// already root); `become=false` runs the command directly without
	// any sudo wrap. The pkg handler's brew-vs-apt branch is the
	// canonical caller: brew installs run as the operator's user
	// (Homebrew refuses sudo), apt installs need root.
	//
	// Chose an additive method over a breaking-change to Run because
	// the always-escalate caller pool is large (~20 handlers) and the
	// per-call-become pool is small (1–3 sites). Existing Run callers
	// stay unchanged; new opt-ins call RunWithBecome.
	RunWithBecome(ctx context.Context, become bool, program string, args ...string) ([]byte, error)

	// Command returns a configured *exec.Cmd for callers that need
	// access to per-subcommand exit codes (cmd.ProcessState), private
	// stderr buffers (cmd.Stderr = &buf), or other exec.Cmd state
	// the Run* methods hide. Returns a clean error when become=true
	// is requested on an unsupported platform or with no sudo password
	// configured — same sentinel-error class as the Run* methods.
	//
	// Added by spec-72 phase 2b to migrate the three remaining
	// hand-rolled BecomeRunner constructs (service/shared.{Become-
	// AwareCommand, writeFileWithSudo}, os_systemd.becomeRunner)
	// onto ctx.Privileged(). The interface stops being a pure "shell
	// out and read bytes" abstraction; that cost was deliberate —
	// preserving per-cp-vs-per-chmod diagnostics for operators reading
	// service install errors was judged more important than interface
	// purity. Callers that don't need *exec.Cmd state should prefer
	// Run / RunWithInput / RunWithBecome.
	Command(ctx context.Context, become bool, program string, args ...string) (*exec.Cmd, error)
}
