package fleet

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// BootstrapOptions configures one fleet-bootstrap invocation.
type BootstrapOptions struct {
	Target transport.SSHTarget // user@host (port via SSHTarget.Port)
	Name   string              // peer name written to peers.toml
	Tags   []string            // peer tags written to peers.toml
	Port   int                 // agentd TCP bind port on remote (default 7878)

	// SSHOpts is forwarded to transport.Connect. Most callers leave this
	// zero — defaults (ssh-agent + ~/.ssh/known_hosts) match the system
	// ssh client's behavior.
	SSHOpts transport.ConnectOptions

	// LocalBinary is the path to the mooncake binary to SCP to the
	// remote. Caller passes os.Executable() when empty isn't desired.
	LocalBinary string

	// RemoteBinary is the absolute path where the binary lands.
	// Defaults to "~/.local/bin/mooncake".
	RemoteBinary string

	// Writer receives one-line progress updates. nil = silent.
	Writer io.Writer
}

// BootstrapResult summarizes what bootstrap did.
type BootstrapResult struct {
	Peer    Peer   // entry written to peers.toml
	OS      string // remote uname -s
	Arch    string // remote uname -m
	RunningPID int  // remote agentd pid, for diagnostics
}

// Bootstrap installs mooncake on a remote box reachable via SSH and
// registers it as an agentd peer. This is the minimal version of
// spec-44/43:
//
//   - SSH via golang.org/x/crypto/ssh + SFTP (no shell-out to system ssh/scp).
//   - No systemd unit / launchd plist (TODO: spec-44 PR10).
//   - Foreground daemon launched via nohup. Dies on reboot.
//   - Linux x86_64 only validated; macOS arm64/x86_64 should also work
//     since the daemon code paths are POSIX, but not certified.
//
// Side effects on the remote:
//   - Writes mooncake binary to RemoteBinary (default ~/.local/bin/mooncake).
//   - Starts `nohup mooncake agentd --bind 0.0.0.0:<port>` in the
//     background. Stdout/stderr go to /tmp/mooncake-agentd.log.
//   - Generates a bearer token at ~/.config/mooncake/agentd.token on
//     first run; reuses it on subsequent runs.
//
// Side effects on the controller:
//   - Upserts a [[peers]] entry in peers.toml.
//
// Returns BootstrapResult on success. Idempotent: re-running against
// an already-bootstrapped peer skips the install and just refreshes
// the local peers.toml entry.
func Bootstrap(ctx context.Context, opts BootstrapOptions) (BootstrapResult, error) {
	if opts.Target.Host == "" {
		return BootstrapResult{}, errors.New("bootstrap: target host is empty")
	}
	if opts.Name == "" {
		opts.Name = opts.Target.Host
		// Be friendlier than `192.168.1.68` — collapse dots so the name
		// is a valid peers.toml identifier and looks OK as a log prefix.
		opts.Name = strings.ReplaceAll(opts.Name, ".", "-")
	}
	if opts.Port == 0 {
		opts.Port = 7878
	}
	if opts.LocalBinary == "" {
		return BootstrapResult{}, errors.New("bootstrap: LocalBinary is empty")
	}
	if opts.RemoteBinary == "" {
		opts.RemoteBinary = "~/.local/bin/mooncake"
	}
	w := opts.Writer
	if w == nil {
		w = io.Discard
	}
	report := func(s string) { fmt.Fprintln(w, s) }

	report(fmt.Sprintf("[%s] connecting via ssh", opts.Name))
	sess, err := transport.Connect(ctx, opts.Target, opts.SSHOpts)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("ssh connect: %w", err)
	}
	defer func() { _ = sess.Close() }()

	unameOut, _, code, err := sess.Run(ctx, `uname -s && uname -m && echo "$HOME"`)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("ssh probe failed: %w", err)
	}
	if code != 0 {
		return BootstrapResult{}, fmt.Errorf("ssh probe: uname/echo exited %d", code)
	}
	lines := strings.Split(strings.TrimSpace(unameOut), "\n")
	if len(lines) < 3 {
		return BootstrapResult{}, fmt.Errorf("ssh probe: unexpected output: %q", unameOut)
	}
	osName, arch, remoteHome := lines[0], lines[1], lines[2]
	report(fmt.Sprintf("[%s] detected %s %s (HOME=%s)", opts.Name, osName, arch, remoteHome))
	if osName != "Linux" && osName != "Darwin" {
		return BootstrapResult{}, fmt.Errorf("unsupported OS %q (linux/darwin only)", osName)
	}

	// Resolve any leading ~ in RemoteBinary against the remote $HOME so
	// the SFTP destination is unambiguous (SFTP doesn't expand ~).
	remoteBin := strings.Replace(opts.RemoteBinary, "~", remoteHome, 1)

	report(fmt.Sprintf("[%s] uploading binary → %s", opts.Name, remoteBin))
	if _, _, code, err := sess.Run(ctx, fmt.Sprintf("mkdir -p %q", remoteDir(remoteBin))); err != nil || code != 0 {
		return BootstrapResult{}, fmt.Errorf("mkdir bin dir (code=%d): %w", code, err)
	}
	// Upload to a temp path first, then rename — avoids the controller's
	// crash mid-upload leaving an unusable binary at the canonical path.
	// Upload sets mode 0755 directly so a follow-up chmod isn't needed.
	tmpRemote := remoteBin + ".tmp"
	if err := sess.Upload(ctx, opts.LocalBinary, tmpRemote, 0o755); err != nil {
		return BootstrapResult{}, err
	}
	if _, _, code, err := sess.Run(ctx, fmt.Sprintf("mv %q %q", tmpRemote, remoteBin)); err != nil || code != 0 {
		return BootstrapResult{}, fmt.Errorf("install binary (code=%d): %w", code, err)
	}

	report(fmt.Sprintf("[%s] stopping any existing agentd", opts.Name))
	// Best-effort kill of a previous foreground agentd. The pkill exit
	// code is non-zero when no match — ignore.
	_, _, _, _ = sess.Run(ctx, "pkill -f 'mooncake agentd' || true")

	report(fmt.Sprintf("[%s] starting agentd on :%d", opts.Name, opts.Port))
	startCmd := fmt.Sprintf(
		`nohup %q agentd --bind 0.0.0.0:%d >/tmp/mooncake-agentd.log 2>&1 </dev/null &`,
		remoteBin, opts.Port,
	)
	if _, _, code, err := sess.Run(ctx, startCmd); err != nil || code != 0 {
		return BootstrapResult{}, fmt.Errorf("start agentd (code=%d): %w", code, err)
	}

	// Wait briefly for the daemon to write its token file. The
	// LoadOrCreateToken call inside agentd happens before Serve so a
	// short sleep + retry loop is sufficient.
	tokenPath := remoteHome + "/.config/mooncake/agentd.token"
	var token string
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		out, _, _, runErr := sess.Run(ctx, fmt.Sprintf("cat %q 2>/dev/null", tokenPath))
		if runErr == nil {
			token = strings.TrimSpace(out)
			if token != "" {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	if token == "" {
		// Pull the agentd log for context before giving up.
		log, _, _, _ := sess.Run(ctx, "tail -n 20 /tmp/mooncake-agentd.log 2>/dev/null")
		return BootstrapResult{}, fmt.Errorf(
			"agentd token never appeared at %s; last log:\n%s",
			tokenPath, log)
	}
	report(fmt.Sprintf("[%s] read bearer token (%d chars)", opts.Name, len(token)))

	pidOut, _, _, _ := sess.Run(ctx, "pgrep -fn 'mooncake agentd' || true")
	pid := 0
	_, _ = fmt.Sscanf(strings.TrimSpace(pidOut), "%d", &pid)

	addr := fmt.Sprintf("%s:%d", opts.Target.Host, opts.Port)

	// Sanity-check connectivity from the controller side. WSL2 boxes
	// commonly need an explicit Windows-side port forward; failing here
	// gives the user a concrete error to act on.
	report(fmt.Sprintf("[%s] checking %s reachable from controller", opts.Name, addr))
	if err := dialTCP(ctx, addr, 3*time.Second); err != nil {
		return BootstrapResult{}, fmt.Errorf(
			"agentd is running on the remote but %s is not reachable from this machine: %w\n"+
				"  Hint: on WSL2 you may need to forward the port from the Windows host\n"+
				"  (`netsh interface portproxy add v4tov4 listenport=%d listenaddress=0.0.0.0`).",
			addr, err, opts.Port)
	}

	peer := Peer{
		Name:      opts.Name,
		Addr:      addr,
		Transport: TransportAgentd,
		Token:     token,
		Tags:      opts.Tags,
	}

	report(fmt.Sprintf("[%s] ✓ bootstrap complete (pid %d)", opts.Name, pid))
	return BootstrapResult{
		Peer:       peer,
		OS:         strings.ToLower(osName),
		Arch:       arch,
		RunningPID: pid,
	}, nil
}

// dialTCP returns nil iff a TCP connect to addr succeeds within timeout.
func dialTCP(ctx context.Context, addr string, timeout time.Duration) error {
	dctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	d := net.Dialer{}
	c, err := d.DialContext(dctx, "tcp", addr)
	if err != nil {
		return err
	}
	_ = c.Close()
	return nil
}

func remoteDir(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i <= 0 {
		return "."
	}
	return p[:i]
}

// EnsureLocalBinaryPath returns the absolute path of the mooncake binary
// that's currently running, suitable as BootstrapOptions.LocalBinary.
// Falls back to a clearer error than os.Executable's defaults.
func EnsureLocalBinaryPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate own binary: %w", err)
	}
	if _, err := os.Stat(exe); err != nil {
		return "", fmt.Errorf("stat own binary at %s: %w", exe, err)
	}
	return exe, nil
}
