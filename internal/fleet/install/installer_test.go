package install

import (
	"strings"
	"testing"
)

func TestInstaller_UnitPath(t *testing.T) {
	cases := []struct {
		os   string
		want string
	}{
		{"linux", "/etc/systemd/system/mooncake-agentd.service"},
		{"darwin", "/Library/LaunchDaemons/com.mooncake.agentd.plist"},
		{"freebsd", ""},
		{"", ""},
	}
	for _, c := range cases {
		t.Run(c.os, func(t *testing.T) {
			got := Installer{OS: c.os}.UnitPath()
			if got != c.want {
				t.Errorf("got %q, want %q", got, c.want)
			}
		})
	}

	t.Run("windows-with-staging", func(t *testing.T) {
		got := Installer{OS: "windows", StagingPath: `C:\Tmp\foo.xml`}.UnitPath()
		if got != `C:\Tmp\foo.xml` {
			t.Errorf("got %q", got)
		}
	})
}

func TestInstaller_UnitName(t *testing.T) {
	if got := (Installer{OS: "linux"}.UnitName()); got != "mooncake-agentd" {
		t.Errorf("linux UnitName = %q", got)
	}
	if got := (Installer{OS: "darwin"}.UnitName()); got != "com.mooncake.agentd" {
		t.Errorf("darwin UnitName = %q", got)
	}
	if got := (Installer{OS: "windows"}.UnitName()); got != "Mooncake-Agentd-Autostart" {
		t.Errorf("windows UnitName = %q", got)
	}
}

func TestInstaller_Render_Windows(t *testing.T) {
	body, err := Installer{
		OS:         "windows",
		Port:       7879,
		BinaryPath: `C:\Users\aleh\AppData\Local\Mooncake\bin\mooncake.exe`,
		TokenPath:  `C:\Users\aleh\AppData\Local\Mooncake\agentd.token`,
		UserID:     `DESKTOP-X\aleh`,
	}.Render()
	if err != nil {
		t.Fatalf("Render windows: %v", err)
	}
	for _, want := range []string{
		`<Task version="1.4"`,
		`<URI>\Mooncake-Agentd-Autostart</URI>`,
		`<BootTrigger>`,
		`<UserId>DESKTOP-X\aleh</UserId>`,
		`<LogonType>S4U</LogonType>`,
		`<Command>C:\Users\aleh\AppData\Local\Mooncake\bin\mooncake.exe</Command>`,
		`agentd run --bind 0.0.0.0:7879`,
		`agentd.token`,
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("windows render missing %q:\n%s", want, body)
		}
	}
}

func TestInstaller_Render_WindowsRequiresExtraFields(t *testing.T) {
	cases := []struct {
		name string
		i    Installer
	}{
		{"no binary", Installer{OS: "windows", Port: 7879, TokenPath: "t", UserID: "u"}},
		{"no token", Installer{OS: "windows", Port: 7879, BinaryPath: "b", UserID: "u"}},
		{"no user", Installer{OS: "windows", Port: 7879, BinaryPath: "b", TokenPath: "t"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := c.i.Render(); err == nil {
				t.Fatalf("expected error")
			}
		})
	}
}

func TestInstaller_EnableStartCmd_Windows(t *testing.T) {
	got := Installer{
		OS:          "windows",
		StagingPath: `C:\Tmp\agentd-task.xml`,
	}.EnableStartCmd()
	for _, want := range []string{
		`Register-ScheduledTask`,
		`-TaskName 'Mooncake-Agentd-Autostart'`,
		`-Xml (Get-Content -Raw 'C:\Tmp\agentd-task.xml')`,
		`Start-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("windows enable cmd missing %q:\n%s", want, got)
		}
	}
}

func TestInstaller_StopDisableCmd_Windows(t *testing.T) {
	got := Installer{OS: "windows"}.StopDisableCmd()
	for _, want := range []string{
		`Stop-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'`,
		`Unregister-ScheduledTask -TaskName 'Mooncake-Agentd-Autostart'`,
		`-ErrorAction SilentlyContinue`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("windows stop cmd missing %q:\n%s", want, got)
		}
	}
}

func TestInstaller_IsActiveCmd_Windows(t *testing.T) {
	got := Installer{OS: "windows", Port: 7879}.IsActiveCmd()
	if !strings.Contains(got, "Get-NetTCPConnection") {
		t.Errorf("windows IsActiveCmd missing Get-NetTCPConnection: %q", got)
	}
	if !strings.Contains(got, "7879") {
		t.Errorf("windows IsActiveCmd missing port: %q", got)
	}
	if !strings.Contains(got, "'active'") || !strings.Contains(got, "'inactive'") {
		t.Errorf("windows IsActiveCmd missing active/inactive branches: %q", got)
	}
}

// TestInstaller_Render_LinuxHasReadWritePaths is the issue #19
// regression. ProtectSystem=true makes /usr read-only inside the
// daemon's mount namespace, so `fleet upgrade`'s self-replace cannot
// write the new binary into /usr/local/bin/ without ReadWritePaths
// punching a narrow hole back open. Without this assertion a future
// hardening tweak could quietly drop the line and re-introduce the
// EROFS failure that issue-19 (and the cross-fs path from issue-12)
// surfaced.
func TestInstaller_Render_LinuxHasReadWritePaths(t *testing.T) {
	body, err := Installer{OS: "linux", Port: 7878}.Render()
	if err != nil {
		t.Fatalf("Render linux: %v", err)
	}
	if !strings.Contains(string(body), "ReadWritePaths=/usr/local/bin") {
		t.Errorf("linux unit missing ReadWritePaths=/usr/local/bin (issue #19):\n%s", body)
	}
	// ProtectSystem stays in place — the test asserts the combination,
	// not just the new line on its own. The fix is meaningless without
	// the underlying restriction.
	if !strings.Contains(string(body), "ProtectSystem=true") {
		t.Errorf("ProtectSystem=true dropped — ReadWritePaths is a no-op without it:\n%s", body)
	}
}

func TestInstaller_Render_PortSubstitution(t *testing.T) {
	body, err := Installer{OS: "linux", Port: 7878}.Render()
	if err != nil {
		t.Fatalf("Render linux: %v", err)
	}
	if !strings.Contains(string(body), "0.0.0.0:7878") {
		t.Errorf("expected port substitution in body:\n%s", body)
	}
	// Make sure the literal {{PORT}} marker is gone.
	if strings.Contains(string(body), "{{PORT}}") {
		t.Errorf("template marker not substituted:\n%s", body)
	}

	dbody, err := Installer{OS: "darwin", Port: 9999}.Render()
	if err != nil {
		t.Fatalf("Render darwin: %v", err)
	}
	if !strings.Contains(string(dbody), "0.0.0.0:9999") {
		t.Errorf("expected port substitution in plist:\n%s", dbody)
	}
}

func TestInstaller_Render_RejectsBadInput(t *testing.T) {
	if _, err := (Installer{OS: "freebsd", Port: 7878}).Render(); err == nil {
		t.Error("freebsd should error")
	}
	if _, err := (Installer{OS: "linux", Port: 0}).Render(); err == nil {
		t.Error("port 0 should error")
	}
	if _, err := (Installer{OS: "linux", Port: 70000}).Render(); err == nil {
		t.Error("port 70000 should error")
	}
}

func TestInstaller_EnableStartCmd(t *testing.T) {
	linux := Installer{OS: "linux"}.EnableStartCmd()
	if !strings.Contains(linux, "systemctl") || !strings.Contains(linux, "daemon-reload") {
		t.Errorf("linux enable cmd missing systemctl daemon-reload: %q", linux)
	}
	if !strings.Contains(linux, "enable --now") {
		t.Errorf("linux enable cmd missing 'enable --now': %q", linux)
	}

	darwin := Installer{OS: "darwin"}.EnableStartCmd()
	if !strings.Contains(darwin, "launchctl bootstrap") {
		t.Errorf("darwin enable cmd missing 'launchctl bootstrap': %q", darwin)
	}
}

func TestInstaller_IsActiveCmd(t *testing.T) {
	linux := Installer{OS: "linux"}.IsActiveCmd()
	if !strings.Contains(linux, "is-active") {
		t.Errorf("linux IsActiveCmd missing 'is-active': %q", linux)
	}

	darwin := Installer{OS: "darwin"}.IsActiveCmd()
	if !strings.Contains(darwin, "launchctl print") {
		t.Errorf("darwin IsActiveCmd missing 'launchctl print': %q", darwin)
	}
}

func TestInstaller_StopDisableCmd(t *testing.T) {
	if got := (Installer{OS: "linux"}.StopDisableCmd()); !strings.Contains(got, "disable") {
		t.Errorf("linux stop cmd: %q", got)
	}
	if got := (Installer{OS: "darwin"}.StopDisableCmd()); !strings.Contains(got, "bootout") {
		t.Errorf("darwin stop cmd: %q", got)
	}
}

// TestInstaller_LinuxUserMode pins the user-scope branches added when
// `mooncake fleet bootstrap --user` lands: unit lives under
// ~/.config/systemd/user/, every systemctl call gets --user, the binary
// goes to ~/.local/bin, and the token file is the per-user XDG path.
// Each is a single line that's easy to "tidy" in a refactor and silently
// regress the asymmetric-install bug this mode was added to fix.
func TestInstaller_LinuxUserMode(t *testing.T) {
	i := Installer{OS: "linux", Port: 7878, AsUser: true}

	if got, want := i.UnitPath(), "~/.config/systemd/user/mooncake-agentd.service"; got != want {
		t.Errorf("UnitPath = %q, want %q", got, want)
	}
	if got, want := i.BinaryInstallPath(), "~/.local/bin/mooncake"; got != want {
		t.Errorf("BinaryInstallPath = %q, want %q", got, want)
	}
	if got, want := i.TokenFilePath(), "~/.config/mooncake/agentd.token"; got != want {
		t.Errorf("TokenFilePath = %q, want %q", got, want)
	}
	for name, got := range map[string]string{
		"EnableStartCmd": i.EnableStartCmd(),
		"StopDisableCmd": i.StopDisableCmd(),
		"IsActiveCmd":    i.IsActiveCmd(),
	} {
		if !strings.Contains(got, "systemctl --user") {
			t.Errorf("%s missing 'systemctl --user': %q", name, got)
		}
	}
}

// TestInstaller_Render_LinuxUser pins the user-mode unit body: no
// User=root (user units already run as the owning user; a stray User=
// would either be ignored or rejected depending on systemd version),
// no `--system` in ExecStart (would force /etc + /var paths the user
// can't write), and %h-relative binary path so the unit renders
// identically across machines.
func TestInstaller_Render_LinuxUser(t *testing.T) {
	body, err := Installer{OS: "linux", Port: 7878, AsUser: true}.Render()
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(body)
	for _, want := range []string{
		"ExecStart=%h/.local/bin/mooncake agentd run --bind 0.0.0.0:7878",
		"WantedBy=default.target",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("user unit missing %q:\n%s", want, s)
		}
	}
	for _, unwanted := range []string{
		"User=root",
		"Group=root",
		"--system",
		"ReadWritePaths=",
		"WantedBy=multi-user.target",
	} {
		if strings.Contains(s, unwanted) {
			t.Errorf("user unit contains unexpected %q:\n%s", unwanted, s)
		}
	}
}

// TestParseVersion guards the version-string extraction at the heart of
// the existing-install check. uname-style and cli-style outputs both have
// the version as the last whitespace-separated token.
func TestParseVersion(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"mooncake version 0.9.0\n", "0.9.0"},
		{"mooncake 0.9.0\n", "0.9.0"},
		{"  v1.2.3-rc1  \n", "v1.2.3-rc1"},
		{"", ""},
		{"\n", ""},
	}
	for _, c := range cases {
		t.Run(strings.TrimSpace(c.in), func(t *testing.T) {
			if got := parseVersion(c.in); got != c.want {
				t.Errorf("parseVersion(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}
