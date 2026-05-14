package fleet

// installer.go embeds the per-platform service-unit templates that
// `mooncake fleet bootstrap` writes to the remote during step 5 of the
// spec-44 §88 sequence. The templates live under `init/` at the repo
// root so they're discoverable for review without crawling the Go code.
//
// Substitution is intentionally minimal — just `{{PORT}}` for the bind
// port. text/template would be overkill for a one-token replacement and
// would tempt future contributors to add more knobs that don't belong in
// the unit file (e.g. log levels, which agentd flags handle).

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"
)

//go:embed init/mooncake-agentd.service
var systemdUnitTemplate []byte

//go:embed init/com.mooncake.agentd.plist
var launchdPlistTemplate []byte

// Installer holds the per-platform layout choices for writing a service
// unit on the remote. One Installer per (osName, port) pair.
//
// The constants here are the spec-44 §150 / §172 paths. They're not
// configurable because moving them would break peer discovery from the
// system journal — `systemctl status mooncake-agentd` and `launchctl
// list com.mooncake.agentd` are the canonical "is this thing running"
// commands and they're path-implicit.
type Installer struct {
	OS   string // "linux" or "darwin" (from Session.DetectPlatform)
	Port int    // agentd bind port; substituted into the unit body
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
	}
	return ""
}

// Render returns the unit-file body with `{{PORT}}` substituted. Returns
// an error for unsupported OS so the caller fails before SFTP'ing an
// empty file.
func (i Installer) Render() ([]byte, error) {
	var template []byte
	switch i.OS {
	case "linux":
		template = systemdUnitTemplate
	case "darwin":
		template = launchdPlistTemplate
	default:
		return nil, fmt.Errorf("installer: unsupported os %q", i.OS)
	}
	if i.Port <= 0 || i.Port > 65535 {
		return nil, fmt.Errorf("installer: invalid port %d", i.Port)
	}
	body := strings.ReplaceAll(string(template), "{{PORT}}", strconv.Itoa(i.Port))
	return []byte(body), nil
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
	}
	return ""
}
