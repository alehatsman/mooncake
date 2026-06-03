package sandbox

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// resetActive clears the cached Active() result so each test re-probes.
func resetActive() { activeOnce = sync.Once{} }

func writeMountinfo(t *testing.T, body string) {
	t.Helper()
	p := filepath.Join(t.TempDir(), "mountinfo")
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	mountinfoPath = p
}

func TestUsrReadOnly(t *testing.T) {
	roLine := "200 100 8:1 / /usr ro,relatime shared:1 - ext4 /dev/sdd ro\n"
	rwLine := "200 100 8:1 / /usr rw,relatime shared:1 - ext4 /dev/sdd rw\n"

	writeMountinfo(t, "100 1 8:1 / / rw,relatime - ext4 /dev/sdd rw\n"+roLine)
	if !usrReadOnly() {
		t.Error("expected /usr ro to be detected")
	}
	writeMountinfo(t, "100 1 8:1 / / rw,relatime - ext4 /dev/sdd rw\n"+rwLine)
	if usrReadOnly() {
		t.Error("expected /usr rw to NOT be detected as read-only")
	}
	writeMountinfo(t, "100 1 8:1 / / rw,relatime - ext4 /dev/sdd rw\n")
	if usrReadOnly() {
		t.Error("no /usr mount → not read-only")
	}
}

func TestActive_EnvOverride(t *testing.T) {
	t.Setenv("MOONCAKE_SANDBOX_ESCAPE", "off")
	resetActive()
	if Active() {
		t.Error("MOONCAKE_SANDBOX_ESCAPE=off must force inactive")
	}
	t.Setenv("MOONCAKE_SANDBOX_ESCAPE", "force")
	resetActive()
	if !Active() {
		t.Error("MOONCAKE_SANDBOX_ESCAPE=force must force active")
	}
}

func TestWrap_NoOpWhenInactive(t *testing.T) {
	t.Setenv("MOONCAKE_SANDBOX_ESCAPE", "off")
	resetActive()
	in := []string{"apt-get", "install", "-y", "git"}
	got := Wrap(in)
	if strings.Join(got, " ") != strings.Join(in, " ") {
		t.Errorf("inactive Wrap should be a no-op, got %v", got)
	}
}

func TestWrap_RewritesWhenActive(t *testing.T) {
	t.Setenv("MOONCAKE_SANDBOX_ESCAPE", "force")
	resetActive()
	origGetwd, origEnviron := getwd, environ
	getwd = func() (string, error) { return "/work", nil }
	environ = func() []string {
		return []string{"PATH=/usr/bin", "JOURNAL_STREAM=8:123", "DEBIAN_FRONTEND=noninteractive"}
	}
	defer func() { getwd, environ = origGetwd, origEnviron }()

	got := Wrap([]string{"apt-get", "install", "-y", "git"})
	joined := strings.Join(got, " ")

	if got[0] != "systemd-run" {
		t.Fatalf("expected systemd-run prefix, got %v", got)
	}
	for _, want := range []string{"--wait", "--pipe", "--working-directory=/work", "--setenv=PATH=/usr/bin", "--setenv=DEBIAN_FRONTEND=noninteractive"} {
		if !strings.Contains(joined, want) {
			t.Errorf("missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "JOURNAL_STREAM") {
		t.Errorf("denylisted env leaked: %q", joined)
	}
	sep := -1
	for i, a := range got {
		if a == "--" {
			sep = i
			break
		}
	}
	if sep < 0 || strings.Join(got[sep+1:], " ") != "apt-get install -y git" {
		t.Errorf("argv not preserved after --: %v", got)
	}
}
