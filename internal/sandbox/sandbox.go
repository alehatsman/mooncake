// Package sandbox detects when mooncake runs inside a hardened systemd
// service sandbox (the agentd unit sets ProtectSystem=yes, which mounts
// /usr read-only for the service and every child it spawns) and escapes it
// for commands that legitimately mutate the system (apt/dpkg writing /usr).
//
// The escape is a transient systemd-run *service*: it is started by PID 1
// in a fresh mount namespace, so it does NOT inherit the agentd's
// read-only /usr. `systemd-run --scope` does NOT work here — a scope forks
// from the caller and stays in the caller's namespace (empirically EROFS).
//
// Wrap is a no-op outside the sandbox (direct `mooncake apply` by a user,
// a non-hardened agentd, non-linux, or no systemd-run), so the normal
// execution path is unchanged.
package sandbox

import (
	"bufio"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
)

// Test seams.
var (
	mountinfoPath = "/proc/self/mountinfo"
	lookPath      = exec.LookPath
	getwd         = os.Getwd
	environ       = os.Environ
)

var (
	activeOnce sync.Once
	activeVal  bool
)

// envDenylistPrefixes are variables systemd sets for the agentd service
// that must not leak into the transient unit — forwarding them would
// confuse the unit's own lifecycle (sd_notify, journal stream, etc.).
var envDenylistPrefixes = []string{
	"INVOCATION_ID", "JOURNAL_STREAM", "NOTIFY_SOCKET", "MANAGERPID",
	"LISTEN_FDS", "LISTEN_PID", "LISTEN_FDNAMES", "WATCHDOG_PID", "WATCHDOG_USEC",
}

// Active reports whether the process runs in a sandbox that makes /usr
// read-only and systemd-run is available to escape it. Cached after first
// call. MOONCAKE_SANDBOX_ESCAPE=force|off overrides the probe (testing /
// operator override).
func Active() bool {
	activeOnce.Do(func() { activeVal = probe() })
	return activeVal
}

func probe() bool {
	switch os.Getenv("MOONCAKE_SANDBOX_ESCAPE") {
	case "force":
		return true
	case "off":
		return false
	}
	if runtime.GOOS != "linux" {
		return false
	}
	if !usrReadOnly() {
		return false
	}
	_, err := lookPath("systemd-run")
	return err == nil
}

// usrReadOnly reports whether /usr is mounted read-only per
// /proc/self/mountinfo (the shape ProtectSystem=yes produces).
func usrReadOnly() bool {
	f, err := os.Open(mountinfoPath)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		// mountinfo: ... <field4=mountpoint> <field5=mount opts> ...
		fields := strings.Fields(sc.Text())
		if len(fields) < 6 || fields[4] != "/usr" {
			continue
		}
		for _, opt := range strings.Split(fields[5], ",") {
			if opt == "ro" {
				return true
			}
		}
	}
	return false
}

// Wrap rewrites argv to run via a transient systemd-run service when
// Active(); otherwise returns argv unchanged. Working directory and a
// filtered copy of the environment are forwarded so the wrapped command
// behaves like an unwrapped one.
func Wrap(argv []string) []string {
	if len(argv) == 0 || !Active() {
		return argv
	}
	out := []string{"systemd-run", "--quiet", "--wait", "--pipe", "--collect"}
	if wd, err := getwd(); err == nil && wd != "" {
		out = append(out, "--working-directory="+wd)
	}
	for _, kv := range environ() {
		if denied(kv) {
			continue
		}
		out = append(out, "--setenv="+kv)
	}
	out = append(out, "--")
	return append(out, argv...)
}

func denied(kv string) bool {
	key := kv
	if i := strings.IndexByte(kv, '='); i >= 0 {
		key = kv[:i]
	}
	for _, p := range envDenylistPrefixes {
		if key == p {
			return true
		}
	}
	return false
}
