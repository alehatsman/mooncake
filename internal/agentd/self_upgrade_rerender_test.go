//go:build linux || darwin

package agentd

import (
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestReRenderAutostart_WritesUnitFile(t *testing.T) {
	unitDir := t.TempDir()

	// Intercept daemon-reload so the test doesn't require a running systemd.
	var reloadCalled [][]string
	orig := systemctlReloadFunc
	systemctlReloadFunc = func(args []string) error {
		reloadCalled = append(reloadCalled, args)
		return nil
	}
	t.Cleanup(func() { systemctlReloadFunc = orig })

	// Use a free port so the config is valid.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()

	// Point the unit file at a writable temp dir by overriding the home
	// dir for user mode. We use system mode here (writes to a fixed path
	// we control via the dir setup below) by patching the unit path via
	// the systemMode flag and a writable staging location.
	//
	// Simplest approach: use a system-mode config but write to a temp path
	// by patching the unit file we write to via a tiny wrapper test.
	// Since UnitPath() for system mode returns /etc/systemd/system/...,
	// which is read-only in CI, we use user mode and override HOME so the
	// tilde-expansion lands in our temp dir.
	t.Setenv("HOME", unitDir)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(unitDir, "config"))

	cfg := Config{
		BindAddr:   "0.0.0.0:" + strconv.Itoa(port),
		SystemMode: false, // user mode → writes to $HOME/.config/systemd/user/
	}

	if err := reRenderAutostart(cfg, "/usr/local/bin/mooncake"); err != nil {
		t.Fatalf("reRenderAutostart: %v", err)
	}

	// Verify the unit file was written.
	unitPath := filepath.Join(unitDir, ".config", "systemd", "user", "mooncake-agentd.service")
	content, err := os.ReadFile(unitPath)
	if err != nil {
		t.Fatalf("unit file not written: %v", err)
	}
	body := string(content)
	portStr := strconv.Itoa(port)
	if !strings.Contains(body, portStr) {
		t.Errorf("unit file does not contain port %s:\n%s", portStr, body)
	}
	if strings.Contains(body, "{{PORT}}") {
		t.Errorf("unit file still has unreplaced {{PORT}} placeholder:\n%s", body)
	}

	// Verify daemon-reload was invoked with --user flag.
	if len(reloadCalled) != 1 {
		t.Fatalf("systemctlReloadFunc called %d times, want 1", len(reloadCalled))
	}
	if len(reloadCalled[0]) < 1 || reloadCalled[0][0] != "--user" {
		t.Errorf("reload args = %v, want [--user daemon-reload]", reloadCalled[0])
	}
}

func TestReRenderAutostart_NoopWhenNoBindAddr(t *testing.T) {
	var reloadCalled int
	orig := systemctlReloadFunc
	systemctlReloadFunc = func(_ []string) error { reloadCalled++; return nil }
	t.Cleanup(func() { systemctlReloadFunc = orig })

	cfg := Config{BindAddr: "", SystemMode: false}
	if err := reRenderAutostart(cfg, "/usr/local/bin/mooncake"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if reloadCalled != 0 {
		t.Errorf("daemon-reload called %d times for unix-only daemon, want 0", reloadCalled)
	}
}
