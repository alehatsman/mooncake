package transport

// Thin shell-out wrapper around system `ssh` and `scp`. Used by
// `mooncake fleet bootstrap` to install agentd on a fresh box.
//
// TODO(spec-44 PR9): replace with native `golang.org/x/crypto/ssh` for
// proper auth + known-hosts handling without a forked subprocess. Today
// we trust the user's environment (ssh-agent, ~/.ssh/config,
// ~/.ssh/known_hosts) to do the right thing; shelling out gets us all
// of that for free at the cost of polish.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// SSHTarget is a parsed `user@host[:port]` (or alias from ~/.ssh/config).
// A bare hostname is allowed when the user prefers OS defaults.
type SSHTarget struct {
	User string // optional; "" defers to ssh's default
	Host string // required
	Port int    // optional; 0 → use system default (22 or ssh_config)
}

func (t SSHTarget) String() string {
	if t.User != "" {
		return t.User + "@" + t.Host
	}
	return t.Host
}

// ParseSSHTarget accepts `user@host`, `user@host:port`, or `host`. Port
// can also be provided via a separate flag — that's the caller's choice;
// when both are present the colon form wins.
func ParseSSHTarget(s string) (SSHTarget, error) {
	if s == "" {
		return SSHTarget{}, fmt.Errorf("empty SSH target")
	}
	var t SSHTarget
	if i := strings.IndexByte(s, '@'); i >= 0 {
		t.User = s[:i]
		s = s[i+1:]
	}
	if j := strings.LastIndexByte(s, ':'); j >= 0 {
		hostPart := s[:j]
		portPart := s[j+1:]
		port := 0
		if _, err := fmt.Sscanf(portPart, "%d", &port); err != nil || port <= 0 {
			return SSHTarget{}, fmt.Errorf("invalid port in %q: %v", s, err)
		}
		t.Host = hostPart
		t.Port = port
	} else {
		t.Host = s
	}
	if t.Host == "" {
		return SSHTarget{}, fmt.Errorf("missing host in %q", s)
	}
	return t, nil
}

// SSHRunner runs commands and copies files to an SSH target using the
// system `ssh`/`scp` binaries. Stateless — each call invokes a fresh
// subprocess.
type SSHRunner struct {
	Target SSHTarget
	// Verbose toggles -v on each invocation; useful for debugging auth.
	Verbose bool
}

func NewSSHRunner(target SSHTarget) *SSHRunner {
	return &SSHRunner{Target: target}
}

// sshArgs builds the leading "-p N -o ... user@host" arguments shared
// between ssh and scp. scp takes -P (capital) instead of -p; the caller
// patches that.
func (r *SSHRunner) sshArgs(uppercaseP bool) []string {
	var args []string
	if r.Target.Port > 0 {
		if uppercaseP {
			args = append(args, "-P", fmt.Sprintf("%d", r.Target.Port))
		} else {
			args = append(args, "-p", fmt.Sprintf("%d", r.Target.Port))
		}
	}
	// Reasonable defaults: don't hang on prompts, batch-mode auth, faster
	// connect.
	args = append(args,
		"-o", "BatchMode=yes",
		"-o", "ConnectTimeout=10",
	)
	if r.Verbose {
		args = append(args, "-v")
	}
	return args
}

// Run executes `cmd` on the remote and returns stdout/stderr separately.
// A non-zero exit code returns an error containing both.
func (r *SSHRunner) Run(ctx context.Context, cmd string) (stdout, stderr string, err error) {
	args := r.sshArgs(false)
	args = append(args, r.Target.String(), cmd)
	c := exec.CommandContext(ctx, "ssh", args...)
	var out, errBuf bytes.Buffer
	c.Stdout = &out
	c.Stderr = &errBuf
	runErr := c.Run()
	stdout = out.String()
	stderr = errBuf.String()
	if runErr != nil {
		return stdout, stderr, fmt.Errorf("ssh %s: %w (stderr: %s)",
			r.Target.String(), runErr, strings.TrimSpace(stderr))
	}
	return stdout, stderr, nil
}

// Upload copies localPath to remotePath via scp.
func (r *SSHRunner) Upload(ctx context.Context, localPath, remotePath string) error {
	if _, err := os.Stat(localPath); err != nil {
		return fmt.Errorf("scp: source %s: %w", localPath, err)
	}
	args := r.sshArgs(true)
	args = append(args,
		"-o", "ServerAliveInterval=30",
		localPath,
		fmt.Sprintf("%s:%s", r.Target.String(), remotePath),
	)
	c := exec.CommandContext(ctx, "scp", args...)
	var errBuf bytes.Buffer
	c.Stderr = &errBuf
	if err := c.Run(); err != nil {
		return fmt.Errorf("scp %s → %s:%s: %w (stderr: %s)",
			localPath, r.Target.String(), remotePath, err,
			strings.TrimSpace(errBuf.String()))
	}
	return nil
}
