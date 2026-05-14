package fleet

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
}

func TestInstaller_UnitName(t *testing.T) {
	if got := (Installer{OS: "linux"}.UnitName()); got != "mooncake-agentd" {
		t.Errorf("linux UnitName = %q", got)
	}
	if got := (Installer{OS: "darwin"}.UnitName()); got != "com.mooncake.agentd" {
		t.Errorf("darwin UnitName = %q", got)
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
