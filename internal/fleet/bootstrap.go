package fleet

// bootstrap.go drives `mooncake fleet bootstrap user@host` end-to-end.
// Eight steps per spec-44 §88:
//
//	1. Connect + auth                 (transport.Connect)
//	2. Detect platform                (Session.DetectPlatform)
//	3. Check existing install         (existingInstall)
//	4. Upload binary to /usr/local    (installBinary, sudo)
//	5. Install service unit           (installService, sudo)
//	6. Start service + verify         (startAndVerify, sudo + reachability poll)
//	7. Read bearer token              (readToken, sudo)
//	8. Update peers.toml              (caller's responsibility — cmd/fleet.go)
//
// All sudo-needing commands route through sudoer. Idempotency is enforced
// at step 3: a matching prior install short-circuits steps 4-6.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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

	// ControllerVersion is the version string the controller's binary
	// reports. Compared against the remote's `mooncake --version` to
	// decide whether step 3 short-circuits. Empty disables the
	// short-circuit; the bootstrap always reinstalls in that case.
	ControllerVersion string

	// Upgrade, when true, replaces a different-version install on the
	// remote without prompting. Without it, version mismatch errors.
	Upgrade bool

	// AsUser, when true (Linux only), installs the agentd as a user-scope
	// systemd unit running as the SSH user. Binary → ~/.local/bin/mooncake,
	// unit → ~/.config/systemd/user/mooncake-agentd.service, token →
	// ~/.config/mooncake/agentd.token. No sudo, `loginctl enable-linger`
	// called so the unit survives logout. Ignored on macOS / Windows
	// (those platforms already pick a per-user shape elsewhere).
	AsUser bool

	// Writer receives one-line progress updates. nil = silent.
	Writer io.Writer
}

// BootstrapResult summarizes what bootstrap did.
type BootstrapResult struct {
	Peer      Peer   // entry to write to peers.toml
	OS        string // "linux" or "darwin"
	Arch      string // "amd64" or "arm64"
	AlreadyOK bool   // true if step 3 short-circuited (same version, active)
}

// Bootstrap installs mooncake on a remote box reachable via SSH and
// registers it as an agentd peer. Idempotent: re-running against an
// already-bootstrapped peer with the same version is a no-op (returns
// AlreadyOK=true).
//
// Caller is responsible for writing peers.toml from BootstrapResult.Peer.
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
	w := opts.Writer
	if w == nil {
		w = io.Discard
	}
	report := func(format string, args ...any) {
		fmt.Fprintf(w, "[%s] ", opts.Name)
		fmt.Fprintf(w, format+"\n", args...)
	}

	// === Step 1: Connect + auth ===
	report("connecting via ssh")
	sess, err := transport.Connect(ctx, opts.Target, opts.SSHOpts)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 1 (connect): %w", err)
	}
	defer func() { _ = sess.Close() }()

	// === Step 2: Detect platform ===
	osName, arch, err := sess.DetectPlatform(ctx)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 2 (detect platform): %w", err)
	}
	report("detected %s %s", osName, arch)

	// Windows takes a distinct path: no sudo, paths under %LOCALAPPDATA%,
	// scheduled task instead of systemd unit, firewall handled inline.
	// The shape diverges enough from Linux/macOS that sharing the helper
	// functions would add more branching than clarity. spec-56 has the
	// rationale; the Windows flow is encapsulated in bootstrapWindows
	// below so future spec changes touch one self-contained block.
	if osName == "windows" {
		return bootstrapWindows(ctx, sess, opts, arch, report)
	}

	// Probe root state once; informs the sudo wrapper for the next 4 steps.
	// User-mode installs (Linux only) skip sudo entirely regardless of
	// remote uid — every step writes to paths the SSH user already owns.
	isRoot, err := detectIsRoot(ctx, sess)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("detect root: %w", err)
	}
	noSudo := opts.AsUser && osName == "linux"
	sudoer := newSudoer(sess, isRoot, noSudo)

	inst := Installer{OS: osName, Port: opts.Port, AsUser: opts.AsUser && osName == "linux"}
	peer := Peer{
		Name:      opts.Name,
		Addr:      fmt.Sprintf("%s:%d", opts.Target.Host, opts.Port),
		Transport: TransportAgentd,
		Tags:      opts.Tags,
	}

	// === Step 3: Check existing install ===
	existingVer, serviceActive, err := existingInstall(ctx, sess, sudoer, inst)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 3 (check existing): %w", err)
	}
	if existingVer != "" {
		report("existing install: version %s, service %s", existingVer, activeWord(serviceActive))
		if opts.ControllerVersion != "" && existingVer == opts.ControllerVersion && serviceActive {
			// Idempotent rerun: same version, service running. Read token + return.
			report("already bootstrapped at same version — refreshing peers.toml only")
			token, err := readToken(ctx, sudoer, inst)
			if err != nil {
				return BootstrapResult{}, fmt.Errorf("step 7 (token, idempotent path): %w", err)
			}
			peer.Token = token
			return BootstrapResult{Peer: peer, OS: osName, Arch: arch, AlreadyOK: true}, nil
		}
		if opts.ControllerVersion != "" && existingVer != opts.ControllerVersion && !opts.Upgrade {
			return BootstrapResult{}, fmt.Errorf(
				"different version installed (remote=%s, controller=%s); pass --upgrade to replace",
				existingVer, opts.ControllerVersion)
		}
	}

	// === Step 4: Upload binary ===
	report("uploading binary → %s", inst.BinaryInstallPath())
	if err := installBinary(ctx, sess, sudoer, inst, opts.LocalBinary); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (binary): %w", err)
	}

	// === Step 5: Install service unit ===
	report("installing %s", inst.UnitPath())
	if err := installService(ctx, sess, sudoer, inst); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 5 (service unit): %w", err)
	}

	// === Step 5b (user mode only): enable linger ===
	// Without linger the user-systemd instance dies on SSH logout and
	// takes agentd with it. Polkit on systemd >= 248 lets the user
	// self-linger without sudo; on older boxes the user is expected
	// to have arranged it ahead of time (the error message says so).
	if inst.AsUser {
		if err := enableLinger(ctx, sess); err != nil {
			return BootstrapResult{}, fmt.Errorf("step 5b (enable-linger): %w", err)
		}
	}

	// === Step 6: Start + verify ===
	report("starting service + waiting for /v1/version")
	if err := startAndVerify(ctx, sudoer, inst, opts.Target.Host, opts.Port); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 6 (start+verify): %w", err)
	}

	// === Step 7: Read bearer token ===
	token, err := readToken(ctx, sudoer, inst)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 7 (token): %w", err)
	}
	report("read bearer token (%d chars)", len(token))
	peer.Token = token

	report("✓ bootstrap complete")
	return BootstrapResult{Peer: peer, OS: osName, Arch: arch}, nil
}

// ===== Step helpers =====

// detectIsRoot returns true if the SSH-authenticated user is uid 0.
// `id -u` is POSIX and portable across Linux + macOS.
func detectIsRoot(ctx context.Context, sess *transport.Session) (bool, error) {
	out, _, code, err := sess.Run(ctx, "id -u")
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("id -u exited %d", code)
	}
	return strings.TrimSpace(out) == "0", nil
}

// existingInstall implements step 3 of the spec-44 §88 sequence. Returns
// the remote's `mooncake --version` (empty if not installed) and whether
// the service unit is active.
//
// The version probe is run *without* sudo — `mooncake --version` doesn't
// need it, and avoiding sudo here keeps the error path cleaner if the
// user can't sudo (we'll discover that at step 4 with a clear message).
func existingInstall(ctx context.Context, sess *transport.Session, _ *sudoer, inst Installer) (version string, active bool, err error) {
	// Check binary presence + version via the canonical install path for
	// this mode. /usr/local/bin should be on PATH for an interactive shell
	// in system mode; ~/.local/bin may or may not be (depends on the user's
	// .profile). Probing the canonical path explicitly side-steps PATH
	// issues either way. The tilde in user mode is expanded by the remote
	// shell since `sess.Run` runs through `sh -c`.
	out, _, code, runErr := sess.Run(ctx, inst.BinaryInstallPath()+" --version 2>/dev/null || true")
	if runErr != nil {
		return "", false, runErr
	}
	if code == 0 && strings.TrimSpace(out) != "" {
		version = parseVersion(out)
	}
	if version == "" {
		return "", false, nil
	}
	// Service-state probe. Output captured for caller comparison; exit code
	// is unreliable across systemctl/launchctl, so we look at stdout. Use
	// an exact whole-string match — `strings.Contains` matches "active"
	// inside "inactive", which previously caused a false-positive when the
	// binary was present but the unit had never been installed: bootstrap
	// short-circuited steps 4-6 and then failed at step 7 reading a token
	// path that didn't exist.
	stateOut, _, _, _ := sess.Run(ctx, inst.IsActiveCmd())
	active = strings.TrimSpace(stateOut) == "active"
	return version, active, nil
}

// parseVersion peels the version number out of a `mooncake --version`
// output line. The binary prints something like `mooncake version 0.9.0`
// or `mooncake 0.9.0` depending on the cli framework's mood; either form
// reduces to the last whitespace-separated token.
func parseVersion(out string) string {
	line := strings.TrimSpace(out)
	if line == "" {
		return ""
	}
	parts := strings.Fields(line)
	return parts[len(parts)-1]
}

// installBinary handles step 4: SFTP the local binary to /tmp/mooncake.<rand>,
// then mv it into place. Two-stage is what makes the install race-safe —
// interrupted upload doesn't leave a partial binary at the final path.
// In system mode the mv is sudo'd into /usr/local/bin; in user mode it's
// a plain mv into ~/.local/bin (with mkdir -p for the bin dir, which may
// not exist on a fresh user account).
func installBinary(ctx context.Context, sess *transport.Session, sudoer *sudoer, inst Installer, localPath string) error {
	tmp := fmt.Sprintf("/tmp/mooncake.%s", randomSuffix())
	if err := sess.Upload(ctx, localPath, tmp, 0o755); err != nil {
		return fmt.Errorf("sftp upload: %w", err)
	}
	dest := inst.BinaryInstallPath()
	// mkdir -p the bin dir's parent before mv — necessary in user mode
	// where ~/.local/bin/ might not exist yet. Cheap no-op in system mode.
	// -f on mv overwrites silently if a prior binary exists.
	cmd := fmt.Sprintf("mkdir -p %s && mv -f %s %s", filepathDir(dest), tmp, dest)
	if _, stderr, code, err := sudoer.Run(ctx, cmd); err != nil || code != 0 {
		_, _, _, _ = sess.Run(ctx, fmt.Sprintf("rm -f %s", tmp))
		return fmt.Errorf("install %s → %s (code=%d): %w (stderr: %s)",
			tmp, dest, code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// installService handles step 5: render the platform-appropriate unit
// template, SFTP to /tmp, sudo-mv to the canonical location. Same
// two-stage idiom as installBinary so a half-uploaded file never lands
// where the service manager will read it.
func installService(ctx context.Context, sess *transport.Session, sudoer *sudoer, inst Installer) error {
	body, err := inst.Render()
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("/tmp/%s.%s", filepathBase(inst.UnitPath()), randomSuffix())
	if err := sess.WriteFile(ctx, tmp, body, 0o644); err != nil {
		return fmt.Errorf("sftp write %s: %w", tmp, err)
	}
	// sudo mkdir -p for the unit dir, then mv. /etc/systemd/system exists
	// by default; /Library/LaunchDaemons might not on minimal macOS images.
	dir := filepathDir(inst.UnitPath())
	cmd := fmt.Sprintf("mkdir -p %s && mv -f %s %s", dir, tmp, inst.UnitPath())
	if _, stderr, code, err := sudoer.Run(ctx, cmd); err != nil || code != 0 {
		_, _, _, _ = sess.Run(ctx, fmt.Sprintf("rm -f %s", tmp))
		return fmt.Errorf("install service unit (code=%d): %w (stderr: %s)",
			code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// startAndVerify handles step 6: enable+start via the platform tool, then
// poll /v1/version (with a short connect timeout) until reachable or the
// 10-second budget runs out. On timeout, capture the service status for
// a useful error message.
func startAndVerify(ctx context.Context, sudoer *sudoer, inst Installer, host string, port int) error {
	if _, stderr, code, err := sudoer.Run(ctx, inst.EnableStartCmd()); err != nil || code != 0 {
		return fmt.Errorf("enable+start service (code=%d): %w (stderr: %s)",
			code, err, strings.TrimSpace(stderr))
	}
	addr := fmt.Sprintf("%s:%d", host, port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := dialTCP(ctx, addr, 500*time.Millisecond); err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Last-ditch: pull status output for the user. Don't fail on this —
	// the wrap is purely informational.
	status, _, _, _ := sudoer.Run(ctx, inst.IsActiveCmd()+"; "+inst.statusCmd())
	return fmt.Errorf("agentd never reachable at %s after 10s; status:\n%s",
		addr, strings.TrimSpace(status))
}

// readToken handles step 7. System-mode path /etc/mooncake/agentd.token
// is mode 600 owned by root, so sudo is required even for cat. User-mode
// path ~/.config/mooncake/agentd.token is owned by the SSH user; sudoer
// bypasses sudo in that mode so the cat runs as the user directly.
// The token is the only piece of state the controller needs from the remote.
func readToken(ctx context.Context, sudoer *sudoer, inst Installer) (string, error) {
	out, stderr, code, err := sudoer.Run(ctx, "cat "+inst.TokenFilePath())
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("cat token (code=%d, stderr: %s)", code, strings.TrimSpace(stderr))
	}
	token := strings.TrimSpace(out)
	if token == "" {
		return "", errors.New("token file is empty (agentd may not have finished startup)")
	}
	return token, nil
}

// enableLinger turns on systemd user lingering for the SSH user so the
// user-systemd instance (and the agentd user unit) survives logout. The
// command is idempotent — a probe via `loginctl show-user` short-circuits
// when linger is already on, so re-bootstraps don't trip on it.
//
// On systemd >= 248 polkit allows self-linger without sudo (the SSH user
// owns the action). On older systems the user needs to have run
// `sudo loginctl enable-linger $USER` themselves once; the error message
// surfaces that requirement rather than silently failing.
func enableLinger(ctx context.Context, sess *transport.Session) error {
	probe := `loginctl show-user "$(id -un)" --property=Linger --value 2>/dev/null`
	out, _, _, err := sess.Run(ctx, probe)
	if err == nil && strings.TrimSpace(out) == "yes" {
		return nil
	}
	enable := `loginctl enable-linger "$(id -un)"`
	_, stderr, code, err := sess.Run(ctx, enable)
	if err != nil {
		return fmt.Errorf("loginctl enable-linger: %w", err)
	}
	if code != 0 {
		return fmt.Errorf(
			"loginctl enable-linger exited %d (stderr: %s) — on older systemd "+
				"versions self-linger needs sudo; run `sudo loginctl enable-linger $USER` "+
				"on the target once and retry",
			code, strings.TrimSpace(stderr))
	}
	return nil
}

// ===== Sudo wrapper =====

// sudoer wraps a Session with the right command prefix. When the user is
// root, sudo is skipped entirely; otherwise commands route through
// `sudo -n sh -c '<cmd>'` so a missing passwordless sudo fails fast
// rather than hanging.
type sudoer struct {
	sess   *transport.Session
	isRoot bool
	noSudo bool // unconditional bypass (user-mode install)
}

func newSudoer(sess *transport.Session, isRoot, noSudo bool) *sudoer {
	return &sudoer{sess: sess, isRoot: isRoot, noSudo: noSudo}
}

// Run executes cmd with sudo when needed. Returns the same shape as
// Session.Run.
func (s *sudoer) Run(ctx context.Context, cmd string) (stdout, stderr string, exitCode int, err error) {
	if s.isRoot || s.noSudo {
		return s.sess.Run(ctx, cmd)
	}
	// Single-quote the command for sh -c, escaping any embedded single quotes.
	escaped := strings.ReplaceAll(cmd, "'", `'"'"'`)
	full := "sudo -n sh -c '" + escaped + "'"
	return s.sess.Run(ctx, full)
}

// ===== Small helpers =====

// statusCmd returns a per-platform "status detail" command — used only
// inside error messages for failed start.
func (i Installer) statusCmd() string {
	switch i.OS {
	case "linux":
		if i.AsUser {
			return "systemctl --user status " + i.UnitName() + " --no-pager -n 30 2>&1 || true"
		}
		return "systemctl status " + i.UnitName() + " --no-pager -n 30 2>&1 || true"
	case "darwin":
		return "launchctl print system/" + i.UnitName() + " 2>&1 | head -n 50 || true"
	}
	return ""
}

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

func filepathBase(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i < 0 {
		return p
	}
	return p[i+1:]
}

func filepathDir(p string) string {
	i := strings.LastIndexByte(p, '/')
	if i <= 0 {
		return "."
	}
	return p[:i]
}

func randomSuffix() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func activeWord(b bool) string {
	if b {
		return "active"
	}
	return "inactive"
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
