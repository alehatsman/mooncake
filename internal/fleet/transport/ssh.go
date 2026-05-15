// Package transport implements the controller-side network drivers used by
// `mooncake fleet`. This file is the native SSH driver: `mooncake fleet
// bootstrap` uses it to install agentd on a fresh box that has no daemon
// yet.
//
// Earlier the bootstrap shelled out to the system `ssh` and `scp` binaries.
// That worked for development but pulled in the user's full ssh environment
// implicitly — ssh-agent, ~/.ssh/config, known_hosts — without any
// programmatic control. This package replaces the shell-out path with
// golang.org/x/crypto/ssh + pkg/sftp so the driver:
//
//   - Decides auth order explicitly (agent → ed25519 → rsa).
//   - Verifies host keys against known_hosts itself rather than relying on
//     the system ssh client to do it (and to interactively prompt).
//   - Exposes a small typed API the bootstrap orchestrator can call without
//     parsing subprocess output.
package transport

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/agent"
	"golang.org/x/crypto/ssh/knownhosts"
)

// SSHTarget is a parsed `user@host[:port]` (or alias from ~/.ssh/config).
// A bare hostname is allowed when the user prefers OS defaults.
type SSHTarget struct {
	User string // optional; "" defers to current OS user
	Host string // required
	Port int    // optional; 0 → 22
}

// String renders the target as `user@host` (port omitted; the connection
// dial logic uses Port separately). Useful for log lines.
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

// ConnectOptions tweaks the dial / auth / host-key behavior. Zero-value
// works for the common case (use ssh-agent or default key files; verify
// against ~/.ssh/known_hosts; 10s connect timeout).
type ConnectOptions struct {
	// User overrides SSHTarget.User. Useful when the target was parsed
	// from a bare hostname and the user lives in a flag.
	User string

	// Timeout caps the TCP dial. 0 → 10s.
	Timeout time.Duration

	// KnownHostsFile overrides ~/.ssh/known_hosts. Empty → default.
	KnownHostsFile string

	// InsecureSkipHostKey disables host-key verification entirely. Test-only
	// — refused in production callers.
	InsecureSkipHostKey bool

	// IdentityFiles lists private key paths to try in order if the agent is
	// not available. Empty → ~/.ssh/id_ed25519 then ~/.ssh/id_rsa.
	IdentityFiles []string
}

// Session is one authenticated SSH connection to a peer. A single Session
// can run many commands and SFTP uploads; close it with Close().
type Session struct {
	client *ssh.Client
	target SSHTarget

	// sftp is lazily initialized on first Upload/WriteFile call. SFTP is a
	// separate subsystem under the same SSH connection; opening it eagerly
	// would waste a channel for sessions that only Run() commands.
	sftpClient *sftp.Client
}

// Connect dials target, authenticates, and verifies the host key. Returns
// a Session ready for Run / Upload / WriteFile.
//
// Auth chain (per spec-44 §279):
//  1. SSH_AUTH_SOCK ssh-agent if set and reachable.
//  2. opts.IdentityFiles (or default ~/.ssh/id_ed25519 then ~/.ssh/id_rsa).
//  3. Fail with a clear message — no password prompt.
//
// Host-key verification uses ~/.ssh/known_hosts unless InsecureSkipHostKey.
// An unknown host returns an error containing the offered key fingerprint
// so the caller (CLI) can decide whether to TOFU-add it.
func Connect(ctx context.Context, target SSHTarget, opts ConnectOptions) (*Session, error) {
	user := opts.User
	if user == "" {
		user = target.User
	}
	if user == "" {
		// Default to current OS user — same behavior as system ssh.
		if u := os.Getenv("USER"); u != "" {
			user = u
		} else {
			return nil, errors.New("ssh connect: no user set (pass --user or include user@ in target)")
		}
	}
	port := target.Port
	if port == 0 {
		port = 22
	}
	addr := fmt.Sprintf("%s:%d", target.Host, port)

	authMethods, err := buildAuthMethods(opts.IdentityFiles)
	if err != nil {
		return nil, err
	}
	if len(authMethods) == 0 {
		return nil, errors.New("ssh connect: no auth methods available (start ssh-agent or place ~/.ssh/id_ed25519)")
	}

	hostKeyCallback, err := buildHostKeyCallback(opts.KnownHostsFile, opts.InsecureSkipHostKey)
	if err != nil {
		return nil, err
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 10 * time.Second
	}

	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            authMethods,
		HostKeyCallback: hostKeyCallback,
		Timeout:         timeout,
	}

	// Honor ctx during the TCP dial. Once the SSH handshake is in progress,
	// the ClientConfig.Timeout takes over.
	var d net.Dialer
	d.Timeout = timeout
	conn, err := d.DialContext(ctx, "tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ssh dial %s: %w", addr, err)
	}
	sshConn, chans, reqs, err := ssh.NewClientConn(conn, addr, cfg)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("ssh handshake %s: %w", addr, err)
	}
	client := ssh.NewClient(sshConn, chans, reqs)
	return &Session{client: client, target: target}, nil
}

// Close releases the SSH connection (and the SFTP subsystem channel if it
// was opened). Safe to call multiple times.
func (s *Session) Close() error {
	if s == nil {
		return nil
	}
	if s.sftpClient != nil {
		_ = s.sftpClient.Close()
		s.sftpClient = nil
	}
	if s.client != nil {
		err := s.client.Close()
		s.client = nil
		return err
	}
	return nil
}

// Run executes cmd on the remote and returns stdout, stderr, and the exit
// code separately. A non-nil err signals a transport-level failure (couldn't
// open the channel, the connection dropped). A non-zero exitCode is *not*
// an error — callers want to inspect it themselves (e.g. `command -v
// mooncake` deliberately returns 1).
func (s *Session) Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	ch, err := s.client.NewSession()
	if err != nil {
		return "", "", 0, fmt.Errorf("ssh session: %w", err)
	}
	defer func() { _ = ch.Close() }()

	var outBuf, errBuf bytes.Buffer
	ch.Stdout = &outBuf
	ch.Stderr = &errBuf

	// ssh.Session has no native context support. Cancel via Close on
	// ctx.Done — the running Wait() will return with an error we
	// translate to ctx.Err().
	done := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ch.Close()
		case <-done:
		}
	}()

	runErr := ch.Run(cmd)
	close(done)
	stdout = outBuf.String()
	stderr = errBuf.String()

	if ctx.Err() != nil {
		return stdout, stderr, 0, ctx.Err()
	}
	if runErr == nil {
		return stdout, stderr, 0, nil
	}
	// ssh.ExitError carries the exit code from the remote.
	var exitErr *ssh.ExitError
	if errors.As(runErr, &exitErr) {
		return stdout, stderr, exitErr.ExitStatus(), nil
	}
	// Any other error means the channel itself failed — connectivity, not
	// a non-zero exit. Surface it.
	return stdout, stderr, 0, fmt.Errorf("ssh run %q: %w", cmd, runErr)
}

// Upload SFTPs localPath to remotePath. The remote file is created with
// the given mode (typically 0755 for binaries, 0644 for configs). Parent
// directory must already exist — caller's responsibility, since deciding
// whether to create with what ownership is too policy-specific to bury here.
func (s *Session) Upload(ctx context.Context, localPath, remotePath string, mode os.FileMode) error {
	if err := s.openSFTP(); err != nil {
		return err
	}
	local, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("open local %s: %w", localPath, err)
	}
	defer func() { _ = local.Close() }()

	return s.writeRemote(ctx, remotePath, local, mode)
}

// WriteFile uploads body to remotePath via SFTP. Equivalent to Upload for
// in-memory content. Useful for config files generated in Go (systemd
// units, launchd plists) without a controller-side temp file.
func (s *Session) WriteFile(ctx context.Context, remotePath string, body []byte, mode os.FileMode) error {
	if err := s.openSFTP(); err != nil {
		return err
	}
	return s.writeRemote(ctx, remotePath, bytes.NewReader(body), mode)
}

// writeRemote is the shared SFTP write path: create + truncate, copy from
// src, chmod. Cancellation honors ctx by closing the remote handle (the
// in-flight io.Copy will return).
func (s *Session) writeRemote(ctx context.Context, remotePath string, src io.Reader, mode os.FileMode) error {
	dst, err := s.sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("sftp create %s: %w", remotePath, err)
	}
	closed := false
	closeOnce := func() error {
		if closed {
			return nil
		}
		closed = true
		return dst.Close()
	}
	defer func() { _ = closeOnce() }()

	// Cancel mid-copy: closing the SFTP file handle aborts the in-flight
	// io.Copy on the remote side.
	cancelDone := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = closeOnce()
		case <-cancelDone:
		}
	}()
	_, copyErr := io.Copy(dst, src)
	close(cancelDone)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if copyErr != nil {
		return fmt.Errorf("sftp write %s: %w", remotePath, copyErr)
	}
	if err := dst.Chmod(mode); err != nil {
		// Chmod may legitimately fail on filesystems that don't support
		// modes — surface clearly so caller can decide whether to bail.
		return fmt.Errorf("sftp chmod %s: %w", remotePath, err)
	}
	return closeOnce()
}

// openSFTP lazily opens the SFTP subsystem on this session. Reused across
// Upload / WriteFile calls so the per-call cost is just the file open, not
// a new SSH channel.
func (s *Session) openSFTP() error {
	if s.sftpClient != nil {
		return nil
	}
	c, err := sftp.NewClient(s.client)
	if err != nil {
		return fmt.Errorf("open sftp subsystem: %w", err)
	}
	s.sftpClient = c
	return nil
}

// DetectPlatform runs an OS probe on the remote and normalises the
// output to (os, arch) where os ∈ {linux, darwin, windows} and
// arch ∈ {amd64, arm64}. Returns a clear error for unsupported
// combinations so the caller surfaces the gap rather than silently
// downloading the wrong binary.
//
// Strategy: try `uname -s && uname -m` first (Linux/macOS). If that
// fails (non-zero exit OR an error trying to fork — Windows OpenSSH
// with PowerShell as the default shell does not have a `uname`),
// fall back to a PowerShell probe that emits the same two-line shape
// using $PSVersionTable + $env:PROCESSOR_ARCHITECTURE.
func (s *Session) DetectPlatform(ctx context.Context) (osName, arch string, err error) {
	out, _, code, runErr := s.Run(ctx, "uname -s && uname -m")
	if runErr == nil && code == 0 {
		return parseUnameOutput(out)
	}
	// uname failed — could be Windows (no uname) or a genuinely
	// broken remote. Try the PowerShell probe; if it also fails,
	// surface a combined error.
	winOut, _, winCode, winErr := s.Run(ctx,
		`powershell -NoProfile -Command "Write-Output 'Windows'; Write-Output $env:PROCESSOR_ARCHITECTURE"`)
	if winErr != nil || winCode != 0 {
		return "", "", fmt.Errorf("detect platform: uname failed (code=%d) and powershell probe failed (code=%d): %w",
			code, winCode, runErr)
	}
	return parseWindowsProbe(winOut)
}

// parseUnameOutput is the pure-function side of DetectPlatform on
// Unix-like remotes. Exposed as a package-private helper so tests can
// drive it without an SSH server.
//
// Expected input: two lines — `uname -s` (kernel) followed by `uname -m`
// (machine). Both case-insensitive on match.
func parseUnameOutput(out string) (osName, arch string, err error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("uname output: expected 2 lines, got %d: %q", len(lines), out)
	}
	kernel := strings.ToLower(strings.TrimSpace(lines[0]))
	machine := strings.ToLower(strings.TrimSpace(lines[1]))

	switch kernel {
	case "linux":
		osName = "linux"
	case "darwin":
		osName = "darwin"
	default:
		return "", "", fmt.Errorf("unsupported OS %q (only linux/darwin/windows)", kernel)
	}

	switch machine {
	case "x86_64", "amd64":
		arch = "amd64"
	case "arm64", "aarch64":
		arch = "arm64"
	default:
		return "", "", fmt.Errorf("unsupported arch %q (only amd64/arm64)", machine)
	}
	return osName, arch, nil
}

// parseWindowsProbe is the pure-function side of the PowerShell
// fallback DetectPlatform takes when uname fails.
//
// Expected input: two lines — the literal "Windows" then the
// $PROCESSOR_ARCHITECTURE value (AMD64 / ARM64 / X86 / IA64).
func parseWindowsProbe(out string) (osName, arch string, err error) {
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) < 2 {
		return "", "", fmt.Errorf("windows probe: expected 2 lines, got %d: %q", len(lines), out)
	}
	kernel := strings.ToLower(strings.TrimSpace(lines[0]))
	machine := strings.ToLower(strings.TrimSpace(lines[1]))
	if kernel != "windows" {
		return "", "", fmt.Errorf("windows probe: first line %q is not 'Windows'", kernel)
	}
	switch machine {
	case "amd64", "x86_64":
		return "windows", "amd64", nil
	case "arm64", "aarch64":
		return "windows", "arm64", nil
	default:
		return "", "", fmt.Errorf("windows probe: unsupported arch %q (only amd64/arm64)", machine)
	}
}

// --- auth + host key helpers ---

// buildAuthMethods constructs the ordered list of ssh.AuthMethod callbacks
// per the spec-44 §279 chain: agent first, then identity files. The order
// matters: ssh.ClientConfig tries them in sequence and the agent should win
// for users who configured one.
func buildAuthMethods(identityFiles []string) ([]ssh.AuthMethod, error) {
	var methods []ssh.AuthMethod

	if sock := os.Getenv("SSH_AUTH_SOCK"); sock != "" {
		if conn, err := net.Dial("unix", sock); err == nil {
			methods = append(methods, ssh.PublicKeysCallback(agent.NewClient(conn).Signers))
		}
		// If the agent socket is set but unreachable we silently fall back to
		// key files — matches ssh's behavior, and avoids a confusing error
		// when the agent died between sessions.
	}

	keys := identityFiles
	if len(keys) == 0 {
		home, _ := os.UserHomeDir()
		keys = []string{
			filepath.Join(home, ".ssh", "id_ed25519"),
			filepath.Join(home, ".ssh", "id_rsa"),
		}
	}
	for _, path := range keys {
		b, err := os.ReadFile(path)
		if err != nil {
			continue // missing key file is fine; try the next one
		}
		signer, err := ssh.ParsePrivateKey(b)
		if err != nil {
			// A malformed or passphrase-protected key is *not* fine —
			// surface explicitly. Tools like ssh-keygen produce passphrase-
			// protected keys by default and the user needs to know we
			// won't prompt.
			return nil, fmt.Errorf("parse private key %s: %w (passphrase-protected keys not supported; use ssh-agent)", path, err)
		}
		methods = append(methods, ssh.PublicKeys(signer))
	}
	return methods, nil
}

// buildHostKeyCallback resolves the known_hosts verifier. When
// InsecureSkipHostKey is set (test-only), returns a permissive callback;
// otherwise wraps golang.org/x/crypto/ssh/knownhosts and renders an
// actionable error on mismatch.
func buildHostKeyCallback(knownHostsFile string, insecure bool) (ssh.HostKeyCallback, error) {
	if insecure {
		return ssh.InsecureIgnoreHostKey(), nil // #nosec G106 -- gated by InsecureSkipHostKey flag
	}
	path := knownHostsFile
	if path == "" {
		home, _ := os.UserHomeDir()
		path = filepath.Join(home, ".ssh", "known_hosts")
	}
	// knownhosts.New errors if the file doesn't exist. For a first-time
	// connect with no known_hosts file at all, return a callback that
	// rejects every host with a clear message — preferable to silently
	// trusting (which is what InsecureIgnoreHostKey would do).
	if _, err := os.Stat(path); err != nil {
		return func(hostname string, _ net.Addr, _ ssh.PublicKey) error {
			return fmt.Errorf("host %s not in known_hosts (file %s missing); run `ssh-keyscan -H %s >> %s` to add it",
				hostname, path, hostname, path)
		}, nil
	}
	cb, err := knownhosts.New(path)
	if err != nil {
		return nil, fmt.Errorf("load known_hosts %s: %w", path, err)
	}
	// Wrap so the error message names the file the user needs to edit.
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		if err := cb(hostname, remote, key); err != nil {
			return fmt.Errorf("host key verification failed for %s: %w (edit %s to update)", hostname, err, path)
		}
		return nil
	}, nil
}
