package install

import (
	"context"
	"io/fs"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// SSHExecutor adapts a *transport.Session to Executor. The session
// methods (Run, WriteFile, Upload) already match the Executor shape;
// SSHExecutor is a thin pass-through.
//
// The orchestrator (install.Bootstrap, spec-70 §Design 2) drives the
// SSH path through this adapter so the same code that handles local
// installs handles remote installs.
type SSHExecutor struct {
	Session *transport.Session
}

// NewSSHExecutor wraps sess. Caller retains ownership of the session
// (Close is the caller's responsibility — SSHExecutor doesn't close
// the underlying session on its own).
func NewSSHExecutor(sess *transport.Session) *SSHExecutor {
	return &SSHExecutor{Session: sess}
}

// Run delegates to Session.Run.
func (s *SSHExecutor) Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	return s.Session.Run(ctx, cmd)
}

// WriteFile delegates to Session.WriteFile (SFTP).
func (s *SSHExecutor) WriteFile(ctx context.Context, path string, data []byte, mode fs.FileMode) error {
	return s.Session.WriteFile(ctx, path, data, mode)
}

// CopyLocalFile delegates to Session.Upload (SFTP).
func (s *SSHExecutor) CopyLocalFile(ctx context.Context, src, dest string, mode fs.FileMode) error {
	return s.Session.Upload(ctx, src, dest, mode)
}
