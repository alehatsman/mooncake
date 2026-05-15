package fleet

// sshdiag.go runs a one-shot diagnostic command over SSH when the
// agentd transport is unreachable. The goal is to turn "context
// deadline exceeded" into "systemctl says: Active: failed (Result:
// exit-code) since 18:35" — i.e. the actual reason agentd is down.
//
// This is the fallback-channel half of spec-44 — agentd remains the
// primary transport for everything else. SSH is consulted only here
// (and from `fleet bootstrap` / `fleet upgrade`); fleet apply, fleet
// status's main probe, and fleet logs all stay on the HTTP path.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/fleet/transport"
)

// SSHDiagOS is the high-level OS family the diagnostic command was
// dispatched against. "linux" and "darwin" run a unix-style probe
// (systemctl / launchctl); "windows" runs the scheduled-task probe.
type SSHDiagOS string

const (
	SSHDiagOSUnknown SSHDiagOS = ""
	SSHDiagOSLinux   SSHDiagOS = "linux"
	SSHDiagOSDarwin  SSHDiagOS = "darwin"
	SSHDiagOSWindows SSHDiagOS = "windows"
)

// SSHDiagnostic is what the SSH fallback rung produces. Output is
// trimmed and capped so a chatty journal can't dominate the doctor
// table; the caller can still capture the full body via the structured
// JSON path.
type SSHDiagnostic struct {
	// Target is the peer.SSH string as configured. Echoed for clarity.
	Target string `json:"target"`
	// OS is what we detected (or Unknown if probing failed).
	OS SSHDiagOS `json:"os"`
	// Cmd is the exact command we ran. Useful for "reproduce by hand".
	Cmd string `json:"cmd"`
	// Stdout is the trimmed command output (typically systemctl status
	// or Get-ScheduledTaskInfo output).
	Stdout string `json:"stdout,omitempty"`
	// Stderr is the trimmed stderr.
	Stderr string `json:"stderr,omitempty"`
	// ExitCode is the remote command's exit code. Non-zero can be
	// meaningful (systemctl returns 3 for "inactive (dead)") so we
	// surface it rather than convert to a Go error.
	ExitCode int `json:"exit_code"`
	// Took is wall time for the whole probe (connect + run).
	Took time.Duration `json:"took_ns"`
}

// maxDiagOutput caps captured stdout/stderr per stream. 8 KiB is
// generous for a status block (typically <2 KiB) without letting a
// runaway journal dump fill the terminal.
const maxDiagOutput = 8 << 10

// RunSSHDiagnostic dials peer.SSH, detects the remote OS, and runs the
// OS-appropriate "is the agentd daemon healthy" probe. Returns the
// trimmed output as an SSHDiagnostic. A non-nil error means the SSH
// session itself failed (auth, dial, host-key) — a nonzero exit from
// the remote command is part of the diagnostic, not an error.
func RunSSHDiagnostic(ctx context.Context, peer Peer, perStep time.Duration) (SSHDiagnostic, error) {
	if peer.SSH == "" {
		return SSHDiagnostic{}, fmt.Errorf("peer %q: no ssh fallback configured", peer.Name)
	}
	if perStep <= 0 {
		perStep = 5 * time.Second
	}

	target, err := transport.ParseSSHTarget(peer.SSH)
	if err != nil {
		return SSHDiagnostic{Target: peer.SSH}, fmt.Errorf("parse ssh target: %w", err)
	}

	start := time.Now()
	dialCtx, cancelDial := context.WithTimeout(ctx, perStep)
	sess, err := transport.Connect(dialCtx, target, transport.ConnectOptions{Timeout: perStep})
	cancelDial()
	if err != nil {
		return SSHDiagnostic{Target: peer.SSH, Took: time.Since(start)},
			fmt.Errorf("ssh dial %s: %w", target.String(), err)
	}
	defer func() { _ = sess.Close() }()

	diag := SSHDiagnostic{Target: peer.SSH}

	// Detect OS. Try uname first; if it fails (Windows OpenSSH default
	// shell is PowerShell, which doesn't have uname), fall back to a
	// PowerShell-flavoured probe.
	unameCtx, cancelUname := context.WithTimeout(ctx, perStep)
	unameOut, _, unameCode, unameErr := sess.Run(unameCtx, "uname -s")
	cancelUname()

	osFamily := SSHDiagOSUnknown
	if unameErr == nil && unameCode == 0 {
		switch strings.ToLower(strings.TrimSpace(unameOut)) {
		case "linux":
			osFamily = SSHDiagOSLinux
		case "darwin":
			osFamily = SSHDiagOSDarwin
		}
	}
	if osFamily == SSHDiagOSUnknown {
		// Likely Windows. We don't bother re-detecting via `cmd /c ver`
		// — if uname failed, the Windows branch is our best bet and the
		// command itself will surface a clearer error if we're wrong.
		osFamily = SSHDiagOSWindows
	}
	diag.OS = osFamily

	cmd := diagCommandFor(osFamily)
	diag.Cmd = cmd

	runCtx, cancelRun := context.WithTimeout(ctx, perStep)
	stdout, stderr, code, err := sess.Run(runCtx, cmd)
	cancelRun()
	diag.Took = time.Since(start)
	if err != nil {
		// Session error (transport failure mid-command). Stash partials
		// into the diagnostic so the caller can still surface them.
		diag.Stdout = trimDiag(stdout)
		diag.Stderr = trimDiag(stderr)
		diag.ExitCode = code
		return diag, fmt.Errorf("ssh run: %w", err)
	}
	diag.Stdout = trimDiag(stdout)
	diag.Stderr = trimDiag(stderr)
	diag.ExitCode = code
	return diag, nil
}

// diagCommandFor picks the right "is mooncake-agentd healthy" probe
// command per OS family. Kept package-local + table-driven so future
// platforms slot in without touching RunSSHDiagnostic.
func diagCommandFor(os SSHDiagOS) string {
	switch os {
	case SSHDiagOSLinux:
		// systemctl status returns exit 3 when the unit is inactive —
		// that's a useful signal so we don't suppress it. --no-pager
		// keeps the output free of escape codes.
		return "systemctl --no-pager --no-legend status mooncake-agentd 2>&1 | head -40; " +
			"echo '--- recent journal ---'; " +
			"journalctl -u mooncake-agentd --no-pager -n 20 2>/dev/null || true"
	case SSHDiagOSDarwin:
		// launchctl print is the macOS analogue. The label path is the
		// one spec-44 §172 mandates.
		return "launchctl print system/com.mooncake.agentd 2>&1 | head -40"
	case SSHDiagOSWindows:
		// PowerShell Get-ScheduledTaskInfo + a process check covers the
		// dotfiles-side autostart scheme. We avoid setting
		// $ErrorActionPreference because Windows OpenSSH's default
		// shell is itself PowerShell, which expands `$Var` in the
		// outer command string before launching the inner pwsh — that
		// breaks the script in surprising ways. Instead we attach
		// -ErrorAction SilentlyContinue to each cmdlet that can fail.
		return `powershell -NoProfile -OutputFormat Text -Command "` +
			`Get-ScheduledTaskInfo -TaskName 'Mooncake-Agentd-Autostart' -ErrorAction SilentlyContinue | Format-List LastRunTime,LastTaskResult,NextRunTime; ` +
			`Write-Output '--- mooncake processes ---'; ` +
			`Get-Process mooncake -ErrorAction SilentlyContinue | Format-Table Id,ProcessName,StartTime -AutoSize` +
			`"`
	}
	return "echo 'unknown OS family — no diagnostic command'"
}

// trimDiag caps and tidies one captured stream. We don't word-wrap;
// systemctl/journal output is already line-oriented and the doctor
// renderer indents each line directly.
func trimDiag(s string) string {
	s = strings.TrimSpace(s)
	if len(s) > maxDiagOutput {
		s = s[:maxDiagOutput] + "\n…(truncated)"
	}
	return s
}
