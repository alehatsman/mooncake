package fleet

// bootstrap_windows_target.go implements the Windows-target branch of
// `mooncake fleet bootstrap`. The file is named *_windows_target.go
// rather than *_windows.go on purpose — the latter would trigger Go's
// build-tag-by-filename convention and exclude the file from
// linux/darwin builds. We want this code to compile and run on the
// controller (linux/darwin), since it talks to a Windows *remote* over
// SSH; the file shouldn't be filtered out by build tags.
//
// Diverges from the linux/darwin path because Windows brings its own
// idioms:
//   - No sudo. The SSH-authenticated user (an administrator per
//     spec-56 assumptions) runs every command directly.
//   - No /usr/local/bin or /etc/mooncake. Paths live under the
//     user's %LOCALAPPDATA%\Mooncake.
//   - No systemctl. The autostart wrapper is a Task Scheduler entry
//     registered via Register-ScheduledTask -Xml.
//   - Firewall handled inline. spec-36 §191 left a high-level Windows
//     firewall action deferred (spec-57 reopens it); for now the
//     bootstrap step calls winutil.RenderEnsure directly.
//
// Tests for this file are limited to the pure-helper paths (path
// rendering, env-probe parsing). The end-to-end flow is exercised by
// manual run against a real Windows box — same testing posture as
// spec-36 §28 ("CI for Windows is out of scope").

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"os"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/install"
	"github.com/alehatsman/mooncake/internal/fleet/transport"
	"github.com/alehatsman/mooncake/internal/winutil"
)

// winContext captures the per-user / per-host facts the bootstrap
// flow needs to render paths and task XML. Probed once early in the
// flow so subsequent steps don't re-roundtrip the same env vars.
type winContext struct {
	LocalAppData string // %LOCALAPPDATA% — e.g. C:\Users\aleh\AppData\Local
	UserName     string // %USERNAME%      — e.g. aleh
	ComputerName string // %COMPUTERNAME%  — e.g. DESKTOP-X
	TempDir      string // %TEMP%          — staging for SFTP uploads
}

// FullUserID returns the principal string the scheduled task uses:
// COMPUTERNAME\USERNAME, the canonical local-account form.
func (w winContext) FullUserID() string {
	return w.ComputerName + `\` + w.UserName
}

// BinaryPath returns the canonical mooncake.exe location on this host.
func (w winContext) BinaryPath() string {
	return w.LocalAppData + `\Mooncake\bin\mooncake.exe`
}

// TokenPath returns the canonical agentd.token location.
func (w winContext) TokenPath() string {
	return w.LocalAppData + `\Mooncake\agentd.token`
}

// TaskXMLPath returns the staging path for the Register-ScheduledTask
// -Xml input. We tuck it inside the Mooncake dir (not %TEMP%) so
// review tools that walk the install dir find every artefact in one
// place.
func (w winContext) TaskXMLPath() string {
	return w.LocalAppData + `\Mooncake\agentd-task.xml`
}

// bootstrapWindows is the Windows-target entry point — analogue of the
// Linux/macOS step sequence in Bootstrap() above, with Windows-shaped
// helpers.
func bootstrapWindows(ctx context.Context, sess *transport.Session, opts BootstrapOptions, arch string, report func(string, ...any)) (BootstrapResult, error) {
	// === Step 2b: probe the remote env once ===
	wc, err := probeWinContext(ctx, sess)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 2b (probe windows env): %w", err)
	}
	report("user %s, profile %s", wc.FullUserID(), wc.LocalAppData)

	inst := install.Installer{
		OS:          "windows",
		Port:        opts.Port,
		BinaryPath:  wc.BinaryPath(),
		TokenPath:   wc.TokenPath(),
		UserID:      wc.FullUserID(),
		StagingPath: wc.TaskXMLPath(),
	}
	peer := Peer{
		Name:      opts.Name,
		Addr:      fmt.Sprintf("%s:%d", opts.Target.Host, opts.Port),
		Transport: TransportAgentd,
		Tags:      opts.Tags,
	}

	// === Step 3: check existing install ===
	existingVer, listening, err := existingWindowsInstall(ctx, sess, wc.BinaryPath(), opts.Port)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 3 (check existing): %w", err)
	}
	if existingVer != "" {
		report("existing install: version %s, listening %s", existingVer, activeWord(listening))
		if opts.ControllerVersion != "" && existingVer == opts.ControllerVersion && listening {
			report("already bootstrapped at same version — refreshing peers.toml only")
			token, err := readWindowsToken(ctx, sess, wc.TokenPath())
			if err != nil {
				return BootstrapResult{}, fmt.Errorf("step 7 (token, idempotent path): %w", err)
			}
			peer.Token = token
			return BootstrapResult{Peer: peer, OS: "windows", Arch: arch, AlreadyOK: true}, nil
		}
		if opts.ControllerVersion != "" && existingVer != opts.ControllerVersion && !opts.Upgrade {
			return BootstrapResult{}, fmt.Errorf(
				"different version installed (remote=%s, controller=%s); pass --upgrade to replace",
				existingVer, opts.ControllerVersion)
		}
	}

	// === Step 4: upload binary ===
	// Resolve which binary to ship (explicit --binary, then the
	// ~/.mooncake/bin store, then a matching controller binary) and verify
	// it's a windows/<arch> PE before uploading. Without this a linux
	// controller would ship an ELF as mooncake.exe and only learn at
	// task-run time (ERROR_EXE_MACHINE_TYPE_MISMATCH). Runs after the
	// step-3 idempotent return so a refresh-only run isn't blocked.
	binPath, err := install.ResolveBinary("windows", arch, opts.LocalBinary)
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (resolve binary): %w", err)
	}
	if err := install.VerifyBinaryPlatform(binPath, "windows", arch); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (binary platform check): %w", err)
	}
	report("uploading binary → %s", wc.BinaryPath())
	if err := installWindowsBinary(ctx, sess, wc, binPath); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 4 (binary): %w", err)
	}

	// === Step 5: install scheduled task ===
	report("registering scheduled task %s", inst.UnitName())
	xml, err := inst.Render()
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 5 (render xml): %w", err)
	}
	if err := uploadWindowsXML(ctx, sess, wc.TaskXMLPath(), xml); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 5 (upload xml): %w", err)
	}
	if _, stderr, code, err := sess.Run(ctx, psWrap(inst.EnableStartCmd())); err != nil || code != 0 {
		return BootstrapResult{}, fmt.Errorf("step 5 (register+start, code=%d): %w (stderr: %s)",
			code, err, strings.TrimSpace(stderr))
	}

	// === Step 5b: open firewall ===
	report("ensuring firewall rule for :%d", opts.Port)
	if err := ensureWindowsFirewall(ctx, sess, opts.Port); err != nil {
		return BootstrapResult{}, fmt.Errorf("step 5b (firewall): %w", err)
	}

	// The agentd rule covers Domain,Private only — opening a control-plane
	// daemon on Public networks would be an unsafe default. But if the
	// host's active connection profile IS Public, the rule won't apply and
	// the port stays filtered. Warn now (before the reachability wait) so
	// the operator can reclassify the network instead of staring at a bare
	// timeout. Best-effort: a probe failure here must not fail bootstrap.
	cats, _ := windowsNetworkCategories(ctx, sess)
	if publicNetworkUncovered(cats) {
		report("WARNING: an active network profile is Public; the 'Mooncake Agentd' "+
			"rule covers Domain,Private only. If this host is reached over the Public "+
			"interface, :%d will be filtered. Set it Private with: "+
			"Set-NetConnectionProfile -InterfaceIndex <n> -NetworkCategory Private", opts.Port)
	}

	// === Step 6: poll /v1/version ===
	report("waiting for /v1/version reachable")
	addr := fmt.Sprintf("%s:%d", opts.Target.Host, opts.Port)
	deadline := time.Now().Add(15 * time.Second) // S4U task startup is slower than systemd
	for time.Now().Before(deadline) {
		if err := dialTCP(ctx, addr, 500*time.Millisecond); err == nil {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if err := dialTCP(ctx, addr, 500*time.Millisecond); err != nil {
		// Final probe failed; try to grab the task's LastTaskResult
		// for an actionable error message.
		taskInfo, _, _, _ := sess.Run(ctx, psWrap(
			"Get-ScheduledTaskInfo -TaskName 'Mooncake-Agentd-Autostart' | Format-List LastTaskResult,LastRunTime"))
		hint := ""
		if publicNetworkUncovered(cats) {
			hint = fmt.Sprintf("\nNOTE: active network profile is Public but the firewall rule "+
				"covers Domain,Private only — if reached over that interface, set it Private "+
				"(Set-NetConnectionProfile -InterfaceIndex <n> -NetworkCategory Private). Active profiles: %v", cats)
		}
		return BootstrapResult{}, fmt.Errorf("agentd not reachable at %s after 15s; task info:\n%s%s",
			addr, strings.TrimSpace(taskInfo), hint)
	}

	// === Step 7: read bearer token ===
	token, err := readWindowsToken(ctx, sess, wc.TokenPath())
	if err != nil {
		return BootstrapResult{}, fmt.Errorf("step 7 (token): %w", err)
	}
	report("read bearer token (%d chars)", len(token))
	peer.Token = token

	report("✓ bootstrap complete")
	return BootstrapResult{Peer: peer, OS: "windows", Arch: arch}, nil
}

// probeWinContext queries the remote for the four env-var values
// bootstrap needs to render paths + principals. Single SSH run; outputs
// one value per line so a simple split keeps parsing trivial.
func probeWinContext(ctx context.Context, sess *transport.Session) (winContext, error) {
	cmd := `powershell -NoProfile -Command "` +
		`Write-Output $env:LOCALAPPDATA; ` +
		`Write-Output $env:USERNAME; ` +
		`Write-Output $env:COMPUTERNAME; ` +
		`Write-Output $env:TEMP"`
	out, stderr, code, err := sess.Run(ctx, cmd)
	if err != nil {
		return winContext{}, fmt.Errorf("env probe: %w", err)
	}
	if code != 0 {
		return winContext{}, fmt.Errorf("env probe exited %d: %s", code, strings.TrimSpace(stderr))
	}
	return parseWinContext(out)
}

// parseWinContext is the pure-function half of probeWinContext — kept
// separate so tests can drive the parser without SSH.
func parseWinContext(out string) (winContext, error) {
	lines := strings.Split(strings.ReplaceAll(strings.TrimSpace(out), "\r", ""), "\n")
	if len(lines) < 4 {
		return winContext{}, fmt.Errorf("env probe: expected 4 lines, got %d: %q", len(lines), out)
	}
	wc := winContext{
		LocalAppData: strings.TrimSpace(lines[0]),
		UserName:     strings.TrimSpace(lines[1]),
		ComputerName: strings.TrimSpace(lines[2]),
		TempDir:      strings.TrimSpace(lines[3]),
	}
	for name, val := range map[string]string{
		"LOCALAPPDATA": wc.LocalAppData,
		"USERNAME":     wc.UserName,
		"COMPUTERNAME": wc.ComputerName,
		"TEMP":         wc.TempDir,
	} {
		if val == "" {
			return winContext{}, fmt.Errorf("env probe: $env:%s is empty", name)
		}
	}
	return wc, nil
}

// existingWindowsInstall returns (version, listening, err). version is
// the value reported by `mooncake.exe --version` at the canonical
// path, empty if the binary doesn't exist. listening is true when
// something is bound to opts.Port (we use a port probe instead of
// scheduled-task state for the same reason IsActiveCmd does — a task
// can report Running with no process).
func existingWindowsInstall(ctx context.Context, sess *transport.Session, binaryPath string, port int) (version string, listening bool, err error) {
	// Step 1: version probe. Test-Path first to short-circuit cleanly
	// on a never-installed box.
	verCmd := `powershell -NoProfile -Command "` +
		`if (Test-Path ` + psQuote(binaryPath) + `) { & ` + psQuote(binaryPath) + ` --version } else { Write-Output '' }"`
	out, _, code, runErr := sess.Run(ctx, verCmd)
	if runErr != nil {
		return "", false, runErr
	}
	if code != 0 {
		// Test-Path itself shouldn't fail; mooncake --version on a
		// corrupted binary might. Treat as "not installed" rather
		// than an error so step 4 reinstalls cleanly.
		return "", false, nil
	}
	if v := parseVersion(out); v != "" {
		version = v
	}
	if version == "" {
		return "", false, nil
	}

	// Step 2: listener probe.
	listenCmd := fmt.Sprintf(
		`powershell -NoProfile -Command "if (Get-NetTCPConnection -State Listen -LocalPort %d -ErrorAction SilentlyContinue) { 'active' } else { 'inactive' }"`,
		port)
	lOut, _, _, _ := sess.Run(ctx, listenCmd)
	listening = strings.TrimSpace(lOut) == "active"
	return version, listening, nil
}

// installWindowsBinary uploads localPath to %LOCALAPPDATA%\Mooncake\bin\.
// Two-stage write (temp → Move-Item) mirrors the Linux installBinary
// pattern: an interrupted SFTP can never leave a half-mooncake.exe at
// the canonical location.
func installWindowsBinary(ctx context.Context, sess *transport.Session, wc winContext, localPath string) error {
	if localPath == "" {
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("locate current binary: %w", err)
		}
		localPath = exe
	}
	// Ensure target dir exists.
	mkdirCmd := psWrap(`New-Item -ItemType Directory -Force -Path ` + psQuote(wc.LocalAppData+`\Mooncake\bin`) + ` | Out-Null`)
	if _, stderr, code, err := sess.Run(ctx, mkdirCmd); err != nil || code != 0 {
		return fmt.Errorf("mkdir bin (code=%d): %w (stderr: %s)", code, err, strings.TrimSpace(stderr))
	}

	tmp := wc.TempDir + `\mooncake-` + randomSuffix() + `.exe`
	if err := sess.Upload(ctx, localPath, tmp, 0o755); err != nil {
		return fmt.Errorf("sftp upload: %w", err)
	}
	// Move-Item -Force replaces the existing file atomically on NTFS
	// (it's MoveFileExW under the hood with MOVEFILE_REPLACE_EXISTING).
	moveCmd := psWrap(`Move-Item -Force -Path ` + psQuote(tmp) + ` -Destination ` + psQuote(wc.BinaryPath()))
	if _, stderr, code, err := sess.Run(ctx, moveCmd); err != nil || code != 0 {
		// Best-effort cleanup of the temp file.
		_, _, _, _ = sess.Run(ctx, psWrap(`Remove-Item -Force -ErrorAction SilentlyContinue `+psQuote(tmp)))
		return fmt.Errorf("move into place (code=%d): %w (stderr: %s)", code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// uploadWindowsXML writes body to dest on the remote via SFTP. Used for
// the task XML staging file.
func uploadWindowsXML(ctx context.Context, sess *transport.Session, dest string, body []byte) error {
	// WriteFile takes a content reader and target path with mode bits.
	// We use 0o644 since the file is read by SYSTEM (Task Scheduler
	// service) and a per-user mode would over-restrict.
	return sess.WriteFile(ctx, dest, body, 0o644)
}

// ensureWindowsFirewall opens the agentd bind port in the host's
// Windows Firewall. Uses winutil.RenderEnsure so the rule is created
// idempotently (no-op if already present).
//
// Note: this is the host firewall, not Hyper-V firewall — Hyper-V
// rules are for WSL VM boundary, which doesn't apply to the
// Windows-host agentd. WSL-host agentd bootstrap goes through the
// Linux branch above.
func ensureWindowsFirewall(ctx context.Context, sess *transport.Session, port int) error {
	rule := winutil.Rule{
		DisplayName: "Mooncake Agentd",
		Description: "mooncake fleet peer (Windows host)",
		Direction:   winutil.DirInbound,
		Protocol:    winutil.ProtoTCP,
		LocalPorts:  []string{fmt.Sprintf("%d", port)},
		Action:      winutil.ActionAllow,
		Profiles:    []winutil.Profile{winutil.ProfDomain, winutil.ProfPrivate},
	}
	ps, err := winutil.RenderEnsure(rule)
	if err != nil {
		return fmt.Errorf("render firewall ensure: %w", err)
	}
	if _, stderr, code, err := sess.Run(ctx, psWrap(ps)); err != nil || code != 0 {
		return fmt.Errorf("firewall ensure (code=%d): %w (stderr: %s)", code, err, strings.TrimSpace(stderr))
	}
	return nil
}

// windowsNetworkCategories returns the NetworkCategory of every active
// connection profile (one per line, e.g. "Public", "Private", "DomainAuthenticated").
// Best-effort: callers treat a probe error as "unknown" and skip the
// warning rather than failing bootstrap.
func windowsNetworkCategories(ctx context.Context, sess *transport.Session) ([]string, error) {
	cmd := psWrap(`Get-NetConnectionProfile | ForEach-Object { $_.NetworkCategory }`)
	out, _, code, err := sess.Run(ctx, cmd)
	if err != nil {
		return nil, err
	}
	if code != 0 {
		return nil, fmt.Errorf("Get-NetConnectionProfile exited %d", code)
	}
	var cats []string
	for _, ln := range strings.Split(strings.ReplaceAll(out, "\r", ""), "\n") {
		if s := strings.TrimSpace(ln); s != "" {
			cats = append(cats, s)
		}
	}
	return cats, nil
}

// publicNetworkUncovered reports whether any active connection profile is
// Public — the one category the Domain,Private agentd firewall rule does
// not cover. Pure so it's unit-testable without a remote.
func publicNetworkUncovered(categories []string) bool {
	for _, c := range categories {
		if strings.EqualFold(strings.TrimSpace(c), "Public") {
			return true
		}
	}
	return false
}

// readWindowsToken returns the contents of the agentd token file. No
// sudo needed — the SSH-authed user owns the file (S4U principal
// runs agentd as the same user).
func readWindowsToken(ctx context.Context, sess *transport.Session, tokenPath string) (string, error) {
	cmd := psWrap(`Get-Content -Raw -Path ` + psQuote(tokenPath))
	out, stderr, code, err := sess.Run(ctx, cmd)
	if err != nil {
		return "", err
	}
	if code != 0 {
		return "", fmt.Errorf("cat token (code=%d, stderr: %s)", code, strings.TrimSpace(stderr))
	}
	tok := strings.TrimSpace(out)
	if tok == "" {
		return "", fmt.Errorf("token file at %s is empty", tokenPath)
	}
	return tok, nil
}

// psWrap returns a `powershell -NoProfile -Command "<inner>"` wrapper
// around inner. inner is assumed to contain only single-quoted strings
// (psQuote-style) so we can safely sit inside double quotes without
// escape gymnastics. We deliberately don't use -EncodedCommand here
// because the outer SSH-default-shell is itself PowerShell on
// Windows; the inner -Command argument is already in a PowerShell
// parser, no encoding needed.
func psWrap(inner string) string {
	// Use ` -Command "` and let the inner string close it. Since
	// inner uses single-quoted strings (psQuote escapes single
	// quotes to ''), embedded double-quotes can't appear so the
	// outer double-quote is unambiguous.
	return `powershell -NoProfile -Command "` + inner + `"`
}

// psQuote is the same single-quote-escape as winutil's psStringLiteral
// — re-exposed at the fleet package level so call sites here don't
// need to import the inner helper.
func psQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

// Ensure io is used (avoid unused-import lint if the function set
// shrinks during follow-ups).
var _ = io.Discard

// ===== Helpers local to the Windows path =====
//
// Mirror copies of the small helpers that the Linux/macOS bootstrap
// path moved into internal/fleet/install during spec-70's refactor.
// Kept private here so the Windows path stays self-contained (spec-70
// §Non-goals: "Windows in v1" — orthogonal to the user/system Linux
// split being untangled).

func parseVersion(out string) string {
	line := strings.TrimSpace(out)
	if line == "" {
		return ""
	}
	parts := strings.Fields(line)
	return parts[len(parts)-1]
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
