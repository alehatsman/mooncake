//go:build windows

package agentd

import (
	"errors"
	"os"
	"path/filepath"
)

// On Windows we keep everything under %LOCALAPPDATA% (not %APPDATA% /
// roaming) — mooncake state is host-specific and roaming would silently
// duplicate run history across machines on AD-joined boxes.
//
// AF_UNIX is supported on Windows 10 1803+ / 11; Go's net.Listen("unix",
// <path>) works fine with Windows-style paths. The chmod(0o600) call in
// server.go is a documented no-op on Windows, so we skip it there
// explicitly rather than relying on the silent no-op.

// userSocketDir returns the dir under which the agentd.sock file lives.
// Same dir as userStateDir on Windows — there's no XDG-runtime-style
// "tmpfs for tonight" concept on this platform, and %LOCALAPPDATA% is
// already user-private.
func userSocketDir() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return d
	}
	// Last-resort fallback if %LOCALAPPDATA% is unset (very unusual);
	// %USERPROFILE%\AppData\Local matches what Windows itself would have
	// expanded the env var to.
	if up := os.Getenv("USERPROFILE"); up != "" {
		return filepath.Join(up, "AppData", "Local")
	}
	return ""
}

// userStateDir returns the persistent state root. Same parent as
// userSocketDir; the daemon nests "mooncake\agentd" beneath via
// filepath.Join in Default().
func userStateDir() (string, error) {
	d := userSocketDir()
	if d == "" {
		return "", errors.New("locate %LOCALAPPDATA%: not set and %USERPROFILE% missing")
	}
	return d, nil
}

// userConfigDir returns where the agentd.token / controller_id /
// peers.toml files live. Same parent as state — see file-comment above
// for why we don't use %APPDATA% (roaming).
func userConfigDir() (string, error) {
	d := userSocketDir()
	if d == "" {
		return "", errors.New("locate %LOCALAPPDATA%: not set and %USERPROFILE% missing")
	}
	return d, nil
}

// systemModeDefaults isn't shipped on Windows in v1. The unix-side
// system mode targets /run/mooncake and /var/lib/mooncake; the Windows
// equivalent (%ProgramData%\Mooncake\) ships when someone needs it.
func systemModeDefaults() Config {
	// Reasonable per-machine paths under %ProgramData% so callers who
	// invoke --system on Windows at least get a deterministic location.
	// Note that running as SYSTEM with these paths needs admin perms.
	base := os.Getenv("ProgramData")
	if base == "" {
		base = `C:\ProgramData`
	}
	mooncake := filepath.Join(base, "Mooncake")
	return Config{
		SystemMode:   true,
		SocketPath:   filepath.Join(mooncake, "agentd.sock"),
		StateDir:     filepath.Join(mooncake, "agentd"),
		LogLevel:     "info",
		TokenPath:    filepath.Join(mooncake, "agentd.token"),
		MaxSyncBytes: DefaultMaxSyncBytes,
	}
}
