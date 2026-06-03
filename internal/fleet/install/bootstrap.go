package install

// bootstrap.go drives the spec-44 §88 install sequence (steps 2-7)
// against an Executor. Spec-70 §Design 2 — separating "what to put
// where" (Installer + templates) from "how to put it there" (the
// Executor abstraction) so the SSH path and the local path share one
// orchestrator.
//
// fleet.Bootstrap (the controller-side SSH entry point) wraps a
// transport.Session in SSHExecutor and calls install.Bootstrap.
// cmd/agentd.go's `agentd bootstrap` (spec-70 §Design 3) wraps a
// LocalExecutor and calls the same install.Bootstrap.
//
// Steps owned here:
//
//	2.  Caller-detected OS/arch are inputs.
//	3.  ExistingInstall — version probe + service-active probe.
//	4.  PlaceBinary — stage local binary into BinaryInstallPath.
//	5.  PlaceUnit — render + stage the systemd unit / launchd plist.
//	5b. EnableLinger — user-mode only, so the unit survives logout.
//	6.  EnableAndStart — enable+start + /v1/version reachability poll.
//	7.  ReadToken — cat the bearer-token file.
//
// Step 1 (transport.Connect) and step 8 (peers.toml upsert) stay on
// the caller's side: install knows nothing about SSH or peers.toml.

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"time"
)

// BootstrapOptions configures one install.Bootstrap invocation.
type BootstrapOptions struct {
	// OS is the target's runtime.GOOS ("linux" / "darwin"). Windows
	// is out of scope (spec-70 §Non-goals); callers route Windows to
	// the legacy bootstrapWindows path.
	OS string

	// Arch is informational only — passed through to BootstrapResult
	// so the caller can record it on the peers.toml entry.
	Arch string

	// Port is the agentd TCP bind port (default 7878). Substituted
	// into the unit template's ExecStart.
	Port int

	// AsUser flips Installer into user-scope mode on Linux:
	// ~/.local/bin, ~/.config/systemd/user/, ~/.config/mooncake/,
	// systemctl --user, no sudo. Ignored on darwin.
	AsUser bool

	// LocalBinary is the absolute path on the controller's filesystem
	// of the mooncake binary to install. For SSH it's the controller's
	// own binary; for local it's os.Executable().
	LocalBinary string

	// ControllerVersion is the controller's `mooncake --version`.
	// Compared against the target's version to decide whether step 3
	// short-circuits. Empty disables the short-circuit.
	ControllerVersion string

	// Upgrade=true replaces a different-version install without
	// prompting. Without it, version mismatch errors out.
	Upgrade bool

	// ReachableHost is the host portion of the address used for the
	// post-start /v1/version reachability probe. SSH callers pass the
	// SSH target's hostname; local callers pass "127.0.0.1".
	ReachableHost string

	// Writer receives one-line progress updates. nil = silent.
	Writer io.Writer

	// LogPrefix is prepended in `[prefix] ` form to every progress
	// line. Empty = no prefix.
	LogPrefix string
}

// BootstrapResult is what install.Bootstrap returns to the caller.
// Notably absent: peers.toml shape — that's controller-side concern.
type BootstrapResult struct {
	Token     string // bearer token read from TokenFilePath
	OS        string // echoed from BootstrapOptions.OS
	Arch      string // echoed from BootstrapOptions.Arch
	AlreadyOK bool   // true if the same-version short-circuit fired
}

// Bootstrap installs mooncake on the target reachable via exec.
// Idempotent on the same axes the SSH bootstrap was: same version +
// active service → no-op (just reads token).
func Bootstrap(ctx context.Context, exec Executor, opts BootstrapOptions) (BootstrapResult, error) {
	if opts.OS != "linux" && opts.OS != "darwin" {
		return BootstrapResult{}, fmt.Errorf("install: unsupported os %q (linux/darwin only)", opts.OS)
	}
	if opts.LocalBinary == "" {
		return BootstrapResult{}, errors.New("install: LocalBinary is empty")
	}
	if opts.Port == 0 {
		opts.Port = 7878
	}
	if opts.ReachableHost == "" {
		return BootstrapResult{}, errors.New("install: ReachableHost is empty")
	}
	w := opts.Writer
	if w == nil {
		w = io.Discard
	}
	report := func(format string, args ...any) {
		if opts.LogPrefix != "" {
			fmt.Fprintf(w, "[%s] ", opts.LogPrefix)
		}
		fmt.Fprintf(w, format+"\n", args...)
	}

	// Probe root state once; informs the sudo wrapper for the next 4
	// steps. User-mode installs (Linux only) skip sudo entirely
	// regardless of uid — every step writes to paths the install user
	// already owns.
	isRoot, err := DetectIsRoot(ctx, exec)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("detect root: %w", err)
	}
	noSudo := opts.AsUser && opts.OS == "linux"
	sudoer := NewSudoer(exec, isRoot, noSudo)

	inst := Installer{
		OS:     opts.OS,
		Port:   opts.Port,
		AsUser: opts.AsUser && opts.OS == "linux",
	}

	// === Step 3: Check existing install ===
	existingVer, serviceActive, err := ExistingInstall(ctx, exec, inst)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 3 (check existing): %w", err)
	}
	if existingVer != "" {
		report("existing install: version %s, service %s", existingVer, activeWord(serviceActive))
		if opts.ControllerVersion != "" && existingVer == opts.ControllerVersion && serviceActive {
			report("already bootstrapped at same version — reading token only")
			token, err := inst.ReadToken(ctx, sudoer)
			if err != nil {
				return BootstrapResult{}, fmt.Errorf("step 7 (token, idempotent path): %w", err)
			}
			return BootstrapResult{
				Token: token, OS: opts.OS, Arch: opts.Arch, AlreadyOK: true,
			}, nil
		}
		if opts.ControllerVersion != "" && existingVer != opts.ControllerVersion && !opts.Upgrade {
			return BootstrapResult{}, fmt.Errorf(
				"different version installed (target=%s, controller=%s); pass --upgrade to replace",
				existingVer, opts.ControllerVersion)
		}
	}

	// === Step 4: Place binary ===
	// Verify the artefact can run on the target before placing it — a
	// cross-arch/OS mismatch (e.g. amd64 controller → arm64 target) would
	// otherwise only surface as a failed service start. Runs after the
	// idempotent short-circuit so a refresh that skips placement isn't
	// blocked by a mismatched local binary.
	if err := VerifyBinaryPlatform(opts.LocalBinary, opts.OS, opts.Arch); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (binary platform check): %w", err)
	}
	report("placing binary → %s", inst.BinaryInstallPath())
	if err := inst.PlaceBinary(ctx, exec, sudoer, opts.LocalBinary); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (binary): %w", err)
	}

	// === Step 5: Place service unit ===
	report("installing %s", inst.UnitPath())
	if err := inst.PlaceUnit(ctx, exec, sudoer); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 5 (service unit): %w", err)
	}

	// === Step 5b: Enable linger (user mode only) ===
	if inst.AsUser {
		if err := EnableLinger(ctx, exec); err != nil {
			return BootstrapResult{}, fmt.Errorf("step 5b (enable-linger): %w", err)
		}
	}

	// === Step 6: Start + verify ===
	report("starting service + waiting for /v1/version")
	if err := inst.EnableAndStart(ctx, sudoer, opts.ReachableHost); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 6 (start+verify): %w", err)
	}

	// === Step 7: Read bearer token ===
	token, err := inst.ReadToken(ctx, sudoer)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 7 (token): %w", err)
	}
	report("read bearer token (%d chars)", len(token))

	report("✓ install complete")
	return BootstrapResult{Token: token, OS: opts.OS, Arch: opts.Arch}, nil
}

// ===== Step primitives =====

// DetectIsRoot returns true if the target's shell runs as uid 0.
// `id -u` is POSIX and portable across Linux + macOS. Used to decide
// whether install commands need to be wrapped in sudo.
func DetectIsRoot(ctx context.Context, exec Executor) (bool, error) {
	out, _, code, err := exec.Run(ctx, "id -u")
	if err != nil {
		return false, err
	}
	if code != 0 {
		return false, fmt.Errorf("id -u exited %d", code)
	}
	return strings.TrimSpace(out) == "0", nil
}

// ExistingInstall implements step 3. Returns the target's `mooncake
// --version` (empty if not installed) and whether the service unit is
// active.
//
// The version probe runs *without* sudo — `mooncake --version`
// doesn't need it, and avoiding sudo here keeps the error path
// cleaner if the user can't sudo (we'll discover that at step 4 with
// a clear message).
func ExistingInstall(ctx context.Context, exec Executor, inst Installer) (version string, active bool, err error) {
	// Check binary presence + version via the canonical install path
	// for this mode. /usr/local/bin should be on PATH for an
	// interactive shell in system mode; ~/.local/bin may or may not
	// be (depends on the user's .profile). Probing the canonical
	// path explicitly side-steps PATH issues either way. The tilde
	// in user mode is expanded by the target shell since
	// Executor.Run runs through `sh -c`.
	out, _, code, runErr := exec.Run(ctx, inst.BinaryInstallPath()+" --version 2>/dev/null || true")
	if runErr != nil {
		return "", false, runErr
	}
	if code == 0 && strings.TrimSpace(out) != "" {
		version = parseVersion(out)
	}
	if version == "" {
		return "", false, nil
	}
	// Service-state probe. Output captured for caller comparison;
	// exit code is unreliable across systemctl/launchctl, so we look
	// at stdout. Use an exact whole-string match — `strings.Contains`
	// matches "active" inside "inactive", which previously caused a
	// false-positive when the binary was present but the unit had
	// never been installed: bootstrap short-circuited steps 4-6 and
	// then failed at step 7 reading a token path that didn't exist.
	stateOut, _, _, _ := exec.Run(ctx, inst.IsActiveCmd())
	active = strings.TrimSpace(stateOut) == "active"
	return version, active, nil
}

// parseVersion peels the version number out of a `mooncake --version`
// output line. The binary prints something like `mooncake version
// 0.9.0` or `mooncake 0.9.0` depending on the cli framework's mood;
// either form reduces to the last whitespace-separated token.
func parseVersion(out string) string {
	line := strings.TrimSpace(out)
	if line == "" {
		return ""
	}
	parts := strings.Fields(line)
	return parts[len(parts)-1]
}

// PlaceBinary handles step 4: stage the local binary at
// /tmp/mooncake.<rand>, then mv it into BinaryInstallPath. Two-stage
// is what makes the install race-safe — an interrupted copy doesn't
// leave a partial binary at the final path.
func (i Installer) PlaceBinary(ctx context.Context, exec Executor, sudoer *Sudoer, localPath string) error {
	tmp := fmt.Sprintf("/tmp/mooncake.%s", randomSuffix())
	if err := exec.CopyLocalFile(ctx, localPath, tmp, 0o755); err != nil {
		return fmt.Errorf("stage binary: %w", err)
	}
	dest := i.BinaryInstallPath()
	// mkdir -p the bin dir's parent before mv — necessary in user
	// mode where ~/.local/bin/ might not exist yet. Cheap no-op in
	// system mode. -f on mv overwrites silently if a prior binary
	// exists.
	cmd := fmt.Sprintf("mkdir -p %s && mv -f %s %s", filepathDir(dest), tmp, dest)
	if _, stderr, code, err := sudoer.Run(ctx, cmd); err != nil || code != 0 {
		_, _, _, _ = exec.Run(ctx, fmt.Sprintf("rm -f %s", tmp))
		return fmt.Errorf("install %s → %s (code=%d): %w (stderr: %s)",
			tmp, dest, code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// PlaceUnit handles step 5: render the platform-appropriate unit
// template, stage to /tmp, sudo-mv to the canonical location. Same
// two-stage idiom as PlaceBinary so a half-written file never lands
// where the service manager will read it.
func (i Installer) PlaceUnit(ctx context.Context, exec Executor, sudoer *Sudoer) error {
	body, err := i.Render()
	if err != nil {
		return err
	}
	tmp := fmt.Sprintf("/tmp/%s.%s", filepathBase(i.UnitPath()), randomSuffix())
	if err := exec.WriteFile(ctx, tmp, body, 0o644); err != nil {
		return fmt.Errorf("stage unit %s: %w", tmp, err)
	}
	// sudo mkdir -p for the unit dir, then mv. /etc/systemd/system
	// exists by default; /Library/LaunchDaemons might not on minimal
	// macOS images. User-mode ~/.config/systemd/user/ also needs the
	// mkdir on a fresh account where systemd has never been touched.
	dir := filepathDir(i.UnitPath())
	cmd := fmt.Sprintf("mkdir -p %s && mv -f %s %s", dir, tmp, i.UnitPath())
	if _, stderr, code, err := sudoer.Run(ctx, cmd); err != nil || code != 0 {
		_, _, _, _ = exec.Run(ctx, fmt.Sprintf("rm -f %s", tmp))
		return fmt.Errorf("install service unit (code=%d): %w (stderr: %s)",
			code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// EnableAndStart handles step 6: enable+start via the platform tool,
// then poll /v1/version (with a short connect timeout) until
// reachable or the 10-second budget runs out. On timeout, capture
// the service status for a useful error message.
func (i Installer) EnableAndStart(ctx context.Context, sudoer *Sudoer, host string) error {
	if _, stderr, code, err := sudoer.Run(ctx, i.EnableStartCmd()); err != nil || code != 0 {
		return fmt.Errorf("enable+start service (code=%d): %w (stderr: %s)",
			code, err, strings.TrimSpace(stderr))
	}
	addr := fmt.Sprintf("%s:%d", host, i.Port)
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		if err := dialTCP(ctx, addr, 500*time.Millisecond); err == nil {
			return nil
		}
		time.Sleep(300 * time.Millisecond)
	}
	// Last-ditch: pull status output for the user. Don't fail on
	// this — the wrap is purely informational.
	status, _, _, _ := sudoer.Run(ctx, i.IsActiveCmd()+"; "+i.statusCmd())
	return fmt.Errorf("agentd never reachable at %s after 10s; status:\n%s",
		addr, strings.TrimSpace(status))
}

// ReadToken handles step 7. System-mode path /etc/mooncake/agentd.token
// is mode 600 owned by root, so sudo is required even for cat. User-mode
// path ~/.config/mooncake/agentd.token is owned by the install user;
// sudoer bypasses sudo in that mode so the cat runs as the user directly.
// The token is the only piece of state the controller needs from the
// target.
func (i Installer) ReadToken(ctx context.Context, sudoer *Sudoer) (string, error) {
	out, stderr, code, err := sudoer.Run(ctx, "cat "+i.TokenFilePath())
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

// EnableLinger turns on systemd user lingering for the install user
// so the user-systemd instance (and the agentd user unit) survives
// logout. The command is idempotent — a probe via `loginctl
// show-user` short-circuits when linger is already on, so
// re-bootstraps don't trip on it.
//
// On systemd >= 248 polkit allows self-linger without sudo (the user
// owns the action). On older systems the user needs to have run
// `sudo loginctl enable-linger $USER` themselves once; the error
// message surfaces that requirement rather than silently failing.
func EnableLinger(ctx context.Context, exec Executor) error {
	probe := `loginctl show-user "$(id -un)" --property=Linger --value 2>/dev/null`
	out, _, _, err := exec.Run(ctx, probe)
	if err == nil && strings.TrimSpace(out) == "yes" {
		return nil
	}
	enable := `loginctl enable-linger "$(id -un)"`
	_, stderr, code, err := exec.Run(ctx, enable)
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

// ===== Small helpers =====

// statusCmd returns a per-platform "status detail" command — used
// only inside error messages for failed start.
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
