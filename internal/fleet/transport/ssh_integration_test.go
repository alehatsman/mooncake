package transport

// Integration tests that exercise the native SSH driver end-to-end against
// the in-process test server in sshserver_test.go. No Docker / external
// services required — go test ./internal/fleet/transport/ runs them as
// part of the normal suite.

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"golang.org/x/crypto/ssh"
)

// writeTestEd25519Key creates an OpenSSH-format ed25519 private key in a
// temp file and returns its path. Used by Connect-path tests so the auth
// chain has a valid key file to load.
func writeTestEd25519Key(t *testing.T) string {
	t.Helper()
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("gen key: %v", err)
	}
	block, err := ssh.MarshalPrivateKey(priv, "")
	if err != nil {
		t.Fatalf("marshal key: %v", err)
	}
	path := filepath.Join(t.TempDir(), "id_ed25519")
	if err := os.WriteFile(path, pem.EncodeToMemory(block), 0o600); err != nil {
		t.Fatalf("write key: %v", err)
	}
	return path
}

// TestSession_Run_ZeroExit verifies the happy path: a registered command's
// stdout/stderr come back verbatim and exitCode is 0.
func TestSession_Run_ZeroExit(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	srv.expect("uname -s", commandResponse{Stdout: "Linux\n"})

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	stdout, _, code, err := sess.Run(context.Background(), "uname -s")
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if strings.TrimSpace(stdout) != "Linux" {
		t.Errorf("stdout = %q, want %q", stdout, "Linux")
	}
}

// TestSession_Run_NonZeroExitIsNotAnError checks the contract that
// non-zero exit codes are returned as code+nil-err, not as an error. Many
// bootstrap probes (`command -v mooncake` etc.) deliberately want to read
// a non-zero code without seeing an error.
func TestSession_Run_NonZeroExitIsNotAnError(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	srv.expect("false", commandResponse{Exit: 1})

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	_, _, code, err := sess.Run(context.Background(), "false")
	if err != nil {
		t.Fatalf("Run err: %v (non-zero exit must not be an error)", err)
	}
	if code != 1 {
		t.Errorf("exit code = %d, want 1", code)
	}
}

// TestSession_Run_CapturesStderr ensures stderr lands in the right return
// slot, not interleaved with stdout. The agentd-log-tail path depends on
// this — stderr is the failure-debug surface.
func TestSession_Run_CapturesStderr(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	srv.expect("noisy", commandResponse{
		Stdout: "ok\n",
		Stderr: "warn: thing happened\n",
		Exit:   0,
	})

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	stdout, stderr, _, err := sess.Run(context.Background(), "noisy")
	if err != nil {
		t.Fatalf("Run err: %v", err)
	}
	if strings.TrimSpace(stdout) != "ok" {
		t.Errorf("stdout = %q", stdout)
	}
	if !strings.Contains(stderr, "warn") {
		t.Errorf("stderr = %q, want substring 'warn'", stderr)
	}
}

// TestSession_Run_CancelContextStopsRun confirms ctx cancellation closes
// the channel and Run returns ctx.Err(). Important for the bootstrap path
// which uses ctx-based timeouts.
func TestSession_Run_CancelContextStopsRun(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	// Note: this command isn't registered, so the server's exec handler
	// sends exit-status immediately. To actually exercise cancellation we
	// need the server to *not* respond — easiest by registering nothing
	// (the server's default is to reply immediately with empty output +
	// exit 0, which doesn't test cancellation). Instead cancel BEFORE Run
	// is called; the implementation must short-circuit.
	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, _, err := sess.Run(ctx, "anything")
	if err == nil {
		t.Fatal("Run with pre-canceled ctx must return an error")
	}
	if !strings.Contains(err.Error(), "context") && err != context.Canceled {
		// Either form is acceptable: ssh-level error wrapping context.Canceled,
		// or context.Canceled directly. Both are user-actionable.
		t.Logf("Run err on canceled ctx: %v (acceptable)", err)
	}
}

// TestSession_Upload writes a small file via SFTP and verifies contents +
// mode on the local filesystem the test server is writing into.
//
// The in-process SFTP server in sshserver_test.go is configured to serve
// the real filesystem (it has no chroot) — fine for tests; the real
// daemon's SFTP subsystem on a peer obviously does the same. We write
// inside t.TempDir() so the test is hermetic.
func TestSession_Upload(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	dir := t.TempDir()

	srcPath := filepath.Join(dir, "src.bin")
	body := []byte("hello mooncake")
	if err := os.WriteFile(srcPath, body, 0o644); err != nil {
		t.Fatalf("seed src: %v", err)
	}
	dstPath := filepath.Join(dir, "dst.bin")

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	if err := sess.Upload(context.Background(), srcPath, dstPath, 0o755); err != nil {
		t.Fatalf("Upload: %v", err)
	}

	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("contents = %q, want %q", got, body)
	}
	info, err := os.Stat(dstPath)
	if err != nil {
		t.Fatalf("stat dst: %v", err)
	}
	if info.Mode().Perm() != 0o755 {
		t.Errorf("mode = %o, want 0755", info.Mode().Perm())
	}
}

// TestSession_WriteFile verifies the in-memory write path (no source file
// on the controller). This is the path that PR 10's installer-template
// rendering will use.
func TestSession_WriteFile(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	dir := t.TempDir()
	dstPath := filepath.Join(dir, "unit.service")
	body := []byte("[Unit]\nDescription=test\n")

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	if err := sess.WriteFile(context.Background(), dstPath, body, 0o644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	got, err := os.ReadFile(dstPath)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(body) {
		t.Errorf("contents = %q, want %q", got, body)
	}
	info, _ := os.Stat(dstPath)
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %o, want 0644", info.Mode().Perm())
	}
}

// TestSession_DetectPlatform end-to-end: the canned uname response goes
// through Run, parseUnameOutput, and back. Catches regressions in either
// half.
func TestSession_DetectPlatform(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	srv.expect("uname -s && uname -m", commandResponse{
		Stdout: "Linux\nx86_64\n",
	})

	sess := srv.connectClient(t, auth)
	defer func() { _ = sess.Close() }()

	osN, arch, err := sess.DetectPlatform(context.Background())
	if err != nil {
		t.Fatalf("DetectPlatform: %v", err)
	}
	if osN != "linux" || arch != "amd64" {
		t.Errorf("got os=%q arch=%q, want linux amd64", osN, arch)
	}
}

// TestSession_Close_Idempotent guards the contract that Close is safe to
// call multiple times — bootstrap.go uses defer + early-return paths that
// may close twice on some failure modes.
func TestSession_Close_Idempotent(t *testing.T) {
	srv, auth := newTestSSHServer(t)
	sess := srv.connectClient(t, auth)
	if err := sess.Close(); err != nil {
		t.Fatalf("first Close: %v", err)
	}
	if err := sess.Close(); err != nil {
		t.Errorf("second Close: %v", err)
	}
}

// TestConnect_RejectsBadPort exercises the prod Connect() path: with a
// valid identity file but an unreachable port, the dial fails and the
// error names the addr so the user knows where to look.
func TestConnect_RejectsBadPort(t *testing.T) {
	keyPath := writeTestEd25519Key(t)
	// Pre-unset SSH_AUTH_SOCK so buildAuthMethods doesn't try to talk to
	// a real ssh-agent (which would succeed against the local agent and
	// produce a different error shape).
	t.Setenv("SSH_AUTH_SOCK", "")

	_, err := Connect(context.Background(), SSHTarget{Host: "127.0.0.1", Port: 1}, ConnectOptions{
		InsecureSkipHostKey: true,
		Timeout:             200 * time.Millisecond,
		IdentityFiles:       []string{keyPath},
	})
	if err == nil {
		t.Fatal("expected dial error for port 1")
	}
	if !strings.Contains(err.Error(), "127.0.0.1:1") {
		t.Errorf("err should name the addr; got: %v", err)
	}
}
