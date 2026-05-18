package install

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
)

// LocalExecutor runs install primitives against the local machine —
// `os/exec` for Run, `os.WriteFile`/`io.Copy` for the file primitives.
// Used by `mooncake agentd bootstrap` (spec-70 §Design 3) to install
// agentd on the same box the mooncake binary is running on, without
// the self-loopback SSH round-trip.
type LocalExecutor struct{}

// NewLocalExecutor returns a zero-value LocalExecutor. The
// constructor exists for symmetry with NewSSHExecutor.
func NewLocalExecutor() *LocalExecutor { return &LocalExecutor{} }

// Run shells out to `sh -c cmd` and returns stdout/stderr/exit.
// Matches the SSH side's Run contract: only transport/spawn failures
// produce a non-nil err; a non-zero exit returns the code with err=nil.
func (LocalExecutor) Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	c := exec.CommandContext(ctx, "sh", "-c", cmd)
	var outBuf, errBuf bytes.Buffer
	c.Stdout = &outBuf
	c.Stderr = &errBuf
	runErr := c.Run()
	stdout = outBuf.String()
	stderr = errBuf.String()
	if ctx.Err() != nil {
		return stdout, stderr, 0, ctx.Err()
	}
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	// exec.ExitError carries the exit code from the child.
	var exitErr *exec.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitCode(), nil
	}
	return stdout, stderr, 0, fmt.Errorf("local run %q: %w", cmd, runErr)
}

// WriteFile writes data to path with mode via os.WriteFile.
// Equivalent to the SFTP side's WriteFile for in-memory content.
func (LocalExecutor) WriteFile(_ context.Context, path string, data []byte, mode fs.FileMode) error {
	if err := os.WriteFile(path, data, mode); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

// CopyLocalFile copies src to dest with mode. Streams through
// io.Copy so a multi-MB binary doesn't load fully into memory. The
// dest is opened with O_TRUNC so a stale partial doesn't leak through.
func (LocalExecutor) CopyLocalFile(_ context.Context, src, dest string, mode fs.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open src %s: %w", src, err)
	}
	defer func() { _ = in.Close() }()

	out, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, mode)
	if err != nil {
		return fmt.Errorf("open dest %s: %w", dest, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		_ = out.Close()
		_ = os.Remove(dest)
		return fmt.Errorf("copy %s → %s: %w", src, dest, err)
	}
	if err := out.Close(); err != nil {
		return fmt.Errorf("close %s: %w", dest, err)
	}
	// os.OpenFile applies the umask; Chmod ensures mode is exact (matches
	// SFTP.Chmod in the SSH path).
	if err := os.Chmod(dest, mode); err != nil {
		return fmt.Errorf("chmod %s: %w", dest, err)
	}
	return nil
}
