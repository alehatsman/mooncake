// Package install holds the platform-shaped primitives that put an
// agentd onto a target — render unit, place binary, enable+start,
// read token — abstracted over an Executor so the same orchestration
// drives both the SSH path (controller → remote box) and the local
// path (mooncake binary installing itself). Spec-70 §Design 1.
package install

import (
	"context"
	"io/fs"
	"strings"
)

// Executor runs install primitives against a target — local (os/exec)
// or remote (SSH session). Mirrors the surface area of
// transport.Session that the bootstrap orchestrator needs.
//
// Methods may be no-op idempotent at the call site, but Executor
// itself doesn't enforce idempotency — that's the orchestrator's job
// (ExistingInstall short-circuit, linger probe, etc.).
type Executor interface {
	// Run executes cmd via `sh -c cmd` semantics on the target.
	// stdout/stderr are captured. exitCode is the process exit; err
	// is non-nil only on transport / spawn failures (NOT on non-zero
	// exit). Mirrors transport.Session.Run exactly so SSHExecutor is
	// a trivial pass-through.
	Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error)

	// WriteFile writes data to path on the target with the given
	// mode. Atomicity is the caller's job — typically write to a tmp
	// path then `mv -f` via Run.
	WriteFile(ctx context.Context, path string, data []byte, mode fs.FileMode) error

	// CopyLocalFile copies src on the controller's filesystem to
	// dest on the target. SSH implementations stream via SFTP;
	// local implementations are io.Copy through the local FS.
	CopyLocalFile(ctx context.Context, src, dest string, mode fs.FileMode) error
}

// Sudoer wraps an Executor's Run with `sudo -n sh -c '<cmd>'` when
// escalation is needed. IsRoot=true (uid 0) and NoSudo=true (user-mode
// install) both bypass the wrap, so a single call site works for all
// three privilege shapes.
//
// WriteFile / CopyLocalFile aren't wrapped here — the caller stages
// into a sudo-writable path (/tmp/...) and uses Run("mv -f ...") for
// the final placement.
type Sudoer struct {
	Exec   Executor
	IsRoot bool
	NoSudo bool
}

// NewSudoer constructs a Sudoer over exec.
func NewSudoer(exec Executor, isRoot, noSudo bool) *Sudoer {
	return &Sudoer{Exec: exec, IsRoot: isRoot, NoSudo: noSudo}
}

// Run executes cmd, prefixed with `sudo -n sh -c '...'` unless IsRoot
// or NoSudo is set. -n makes a missing passwordless sudo fail fast
// rather than hanging on a prompt the SSH session can't satisfy.
func (s *Sudoer) Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	if s.IsRoot || s.NoSudo {
		return s.Exec.Run(ctx, cmd)
	}
	escaped := strings.ReplaceAll(cmd, "'", `'"'"'`)
	full := "sudo -n sh -c '" + escaped + "'"
	return s.Exec.Run(ctx, full)
}
