package fleet

// installer.go embeds the per-platform service-unit templates that
// `mooncake fleet bootstrap` writes to the remote during step 5 of the
// spec-44 §88 sequence. The templates live under `init/` at the repo
// root so they're discoverable for review without crawling the Go code.
//
// Substitution is intentionally minimal — just `{{PORT}}` for the bind
// port on the embedded templates. text/template would be overkill for
// a one-token replacement and would tempt future contributors to add
// more knobs that don't belong in the unit file (e.g. log levels,
// which agentd flags handle).
//
// Windows is handled differently — the scheduled-task XML is built
// dynamically via internal/winutil because it carries per-user paths
// (%LOCALAPPDATA% expansion happens once at install time, not at task
// fire time, so each install captures the registering user's profile).

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/alehatsman/mooncake/internal/winutil"
)

//go:embed init/mooncake-agentd.service
var systemdUnitTemplate []byte

//go:embed init/com.mooncake.agentd.plist
var launchdPlistTemplate []byte

// Installer holds the per-platform layout choices for writing a service
// unit on the remote. One Installer per (osName, port, ...) tuple.
//
// The Linux/macOS constants here are the spec-44 §150 / §172 paths.
// They're not configurable because moving them would break peer
// discovery — `systemctl status mooncake-agentd` and `launchctl list
// com.mooncake.agentd` are the canonical "is this thing running"
// commands and they're path-implicit.
//
// The Windows fields (BinaryPath / TokenPath / UserID) carry the
// per-user values that don't fit a fixed system-wide constant. They
// are populated by bootstrap.go's Windows branch after detecting the
// remote user's profile.
type Installer struct {
	OS   string // "linux" / "darwin" / "windows" (from Session.DetectPlatform)
	Port int    // agentd bind port; substituted into the unit body / task XML

	// BinaryPath is the absolute path of mooncake.exe on the remote.
	// Windows-only — Linux/macOS bake the binary path into the unit
	// template at /usr/local/bin/mooncake. Empty on non-Windows.
	BinaryPath string

	// TokenPath is the absolute path of the agentd token file on the
	// remote. Windows-only; Linux/macOS use /etc/mooncake/agentd.token
	// which the daemon manages itself. Empty on non-Windows.
	TokenPath string

	// UserID is the principal the Windows scheduled task runs as
	// (e.g. "DESKTOP-X\\aleh"). Linux/macOS run as root via the unit
	// file's User=root directive; this field is ignored there.
	UserID string

	// StagingPath is the temp location on the remote where the
	// rendered task XML is written before Register-ScheduledTask -Xml
	// consumes it. Windows-only.
	StagingPath string
}

// UnitPath returns the absolute path of the service-unit file on the
// remote for this platform.
//
//	linux:  /etc/systemd/system/mooncake-agentd.service
//	darwin: /Library/LaunchDaemons/com.mooncake.agentd.plist
func (i Installer) UnitPath() string {
	switch i.OS {
	case "linux":
		return "/etc/systemd/system/mooncake-agentd.service"
	case "darwin":
		return "/Library/LaunchDaemons/com.mooncake.agentd.plist"
	case "windows":
		// The XML is consumed by Register-ScheduledTask -Xml and then
		// can be discarded; we stage it in the same Mooncake dir as
		// the binary + token so cleanup is one rmdir.
		if i.StagingPath != "" {
			return i.StagingPath
		}
		return `agentd-task.xml` // relative — set explicitly in production
	}
	return ""
}

// UnitName returns the management-tool identifier for the unit (the
// argument to `systemctl <verb>` or `launchctl <verb>`). Different shape
// per platform: systemd takes the filename without the path; launchctl
// takes the bundle ID (the Label string).
func (i Installer) UnitName() string {
	switch i.OS {
	case "linux":
		return "mooncake-agentd"
	case "darwin":
		return "com.mooncake.agentd"
	case "windows":
		// Matches the dotfiles-side scheduled-task name pre-spec-56;
		// keeping it stable means a dotfiles-bootstrapped machine and
		// a `mooncake fleet bootstrap`-ed machine produce
		// indistinguishable Task Scheduler state. spec-57's ssh-diag
		// rung also greps for this exact name.
		return "Mooncake-Agentd-Autostart"
	}
	return ""
}

// Render returns the unit-file body with `{{PORT}}` substituted. Returns
// an error for unsupported OS so the caller fails before SFTP'ing an
// empty file.
func (i Installer) Render() ([]byte, error) {
	if i.Port <= 0 || i.Port > 65535 {
		return nil, fmt.Errorf("installer: invalid port %d", i.Port)
	}
	switch i.OS {
	case "linux":
		return []byte(strings.ReplaceAll(string(systemdUnitTemplate), "{{PORT}}", strconv.Itoa(i.Port))), nil
	case "darwin":
		return []byte(strings.ReplaceAll(string(launchdPlistTemplate), "{{PORT}}", strconv.Itoa(i.Port))), nil
	case "windows":
		// Build the Task XML dynamically from the Installer fields.
		// The per-user paths (BinaryPath, TokenPath, UserID) are
		// captured here at install time — Task Scheduler doesn't
		// reliably expand %LOCALAPPDATA% for the user the task runs
		// as, so we resolve it once via the SSH-side `$env:LOCALAPPDATA`
		// in the bootstrap caller and bake the absolute path in.
		if i.BinaryPath == "" {
			return nil, fmt.Errorf("installer: BinaryPath required for windows")
		}
		if i.TokenPath == "" {
			return nil, fmt.Errorf("installer: TokenPath required for windows")
		}
		if i.UserID == "" {
			return nil, fmt.Errorf("installer: UserID required for windows")
		}
		task := winutil.Task{
			Name:        i.UnitName(),
			Description: "Start mooncake agentd at boot (registered by `mooncake fleet bootstrap`)",
			Triggers:    []winutil.Trigger{{Type: winutil.TrigBoot}},
			Actions: []winutil.ExecAction{{
				Command:   i.BinaryPath,
				Arguments: fmt.Sprintf(`agentd --bind 0.0.0.0:%d --token-file "%s"`, i.Port, i.TokenPath),
			}},
			Principal: winutil.Principal{
				UserID:    i.UserID,
				LogonType: winutil.LogonS4U,
				RunLevel:  winutil.RunHighest,
			},
			Settings: winutil.Settings{
				RestartCount:    3,
				RestartInterval: time.Minute,
			},
		}
		xml, err := winutil.RenderXML(task)
		if err != nil {
			return nil, fmt.Errorf("installer: render task xml: %w", err)
		}
		return []byte(xml), nil
	default:
		return nil, fmt.Errorf("installer: unsupported os %q", i.OS)
	}
}

// EnableStartCmd is the sudo'd shell command that loads + enables + starts
// the unit. Returned as a single string so the orchestrator's `sess.Run`
// can pipe it through one SSH exec request rather than three round-trips.
// Each platform's tool already supports "do all three" in one invocation.
func (i Installer) EnableStartCmd() string {
	switch i.OS {
	case "linux":
		// daemon-reload picks up the new unit; enable --now both enables on
		// boot AND starts immediately.
		return "systemctl daemon-reload && systemctl enable --now " + i.UnitName()
	case "darwin":
		// bootstrap loads + starts; the plist's RunAtLoad=true makes the
		// run happen as part of bootstrap.
		return "launchctl bootstrap system " + i.UnitPath()
	case "windows":
		// Two PowerShell statements joined by `;` so a single SSH
		// round-trip handles both: register-from-XML, then start.
		// -Force overrides any existing task with the same name (no
		// "skip if exists" guard — this is bootstrap; we want the
		// canonical definition installed).
		return winutil.RenderRegisterCommand(i.UnitName(), i.UnitPath()) +
			"; Start-ScheduledTask -TaskName " + psStringLit(i.UnitName())
	}
	return ""
}

// StopDisableCmd is the inverse: stop + disable on Linux, bootout on
// macOS. Used in failure rollback (spec-44 §253) and `fleet decommission`
// (future).
func (i Installer) StopDisableCmd() string {
	switch i.OS {
	case "linux":
		return "systemctl disable --now " + i.UnitName() + " 2>/dev/null || true"
	case "darwin":
		return "launchctl bootout system " + i.UnitPath() + " 2>/dev/null || true"
	case "windows":
		// Stop then unregister. -ErrorAction SilentlyContinue on each
		// so a missing task isn't an error during rollback / decommission.
		return "Stop-ScheduledTask -TaskName " + psStringLit(i.UnitName()) +
			" -ErrorAction SilentlyContinue; " +
			winutil.RenderUnregisterCommand(i.UnitName())
	}
	return ""
}

// IsActiveCmd returns the per-platform "is this unit running" check.
// Stdout is the truth (cf. exit code semantics differ between systemctl
// and launchctl); callers should match on the string.
func (i Installer) IsActiveCmd() string {
	switch i.OS {
	case "linux":
		// `is-active` exits 0 for active, 3 otherwise; we use the stdout
		// string so a single match suffices.
		return "systemctl is-active " + i.UnitName() + " 2>/dev/null || true"
	case "darwin":
		// `launchctl print` is verbose but reliable; the absence of the
		// label in the output (exit 113) means not loaded.
		return "launchctl print system/" + i.UnitName() + " >/dev/null 2>&1 && echo active || echo inactive"
	case "windows":
		// We probe the bind port for a listener rather than asking
		// Task Scheduler — a task with State=Running but no process
		// (which we've seen in this codebase's history) would mislead.
		// The HTTP poll in bootstrap.startAndVerify is the real
		// liveness check; this string is used for the existingInstall
		// detector + status output in error paths.
		return "powershell -NoProfile -Command \"if (Get-NetTCPConnection -State Listen -LocalPort " +
			strconv.Itoa(i.Port) +
			" -ErrorAction SilentlyContinue) { 'active' } else { 'inactive' }\""
	}
	return ""
}

// psStringLit is a thin wrapper around winutil.psStringLiteral. We
// can't import unexported symbols, so we re-implement the single-quote
// + double-quote escape inline. The wrapper exists so the call sites
// above don't shout `"'" + strings.ReplaceAll(...) + "'"` at the reader.
func psStringLit(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
