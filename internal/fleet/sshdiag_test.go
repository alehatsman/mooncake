package fleet

import (
	"strings"
	"testing"

	"github.com/pelletier/go-toml/v2"
)

func TestDiagCommandFor_Linux(t *testing.T) {
	got := diagCommandFor(SSHDiagOSLinux)
	for _, want := range []string{"systemctl", "mooncake-agentd", "--no-pager", "journalctl"} {
		if !strings.Contains(got, want) {
			t.Errorf("linux diag missing %q: %s", want, got)
		}
	}
}

func TestDiagCommandFor_Darwin(t *testing.T) {
	got := diagCommandFor(SSHDiagOSDarwin)
	for _, want := range []string{"launchctl", "com.mooncake.agentd"} {
		if !strings.Contains(got, want) {
			t.Errorf("darwin diag missing %q: %s", want, got)
		}
	}
}

func TestDiagCommandFor_Windows(t *testing.T) {
	got := diagCommandFor(SSHDiagOSWindows)
	for _, want := range []string{"powershell", "Get-ScheduledTaskInfo", "Mooncake-Agentd-Autostart", "Get-Process"} {
		if !strings.Contains(got, want) {
			t.Errorf("windows diag missing %q: %s", want, got)
		}
	}
}

func TestDiagCommandFor_UnknownFallback(t *testing.T) {
	got := diagCommandFor(SSHDiagOSUnknown)
	if !strings.Contains(got, "unknown OS family") {
		t.Errorf("unknown OS should produce a soft-fail command, got %q", got)
	}
}

func TestTrimDiag_CapsLongOutput(t *testing.T) {
	in := strings.Repeat("a", maxDiagOutput+1000)
	got := trimDiag(in)
	if !strings.HasSuffix(got, "…(truncated)") {
		t.Errorf("expected truncation marker, got tail %q", got[len(got)-30:])
	}
	if len(got) > maxDiagOutput+len("\n…(truncated)") {
		t.Errorf("trim exceeded cap+marker: %d", len(got))
	}
}

func TestTrimDiag_TrimsWhitespaceShortInput(t *testing.T) {
	got := trimDiag("\n  systemctl status...\n  \n\n")
	if got != "systemctl status..." {
		t.Errorf("trim should strip surrounding whitespace, got %q", got)
	}
}

// TestPeer_SSHField_RoundTrips confirms the new toml field survives
// LoadPeers → Upsert → re-LoadPeers via the in-package round trip. No
// external SSH machinery needed.
func TestPeer_SSHField_RoundTrips(t *testing.T) {
	p := Peer{
		Name:      "main_pc",
		Addr:      "192.168.1.68:7878",
		Transport: TransportAgentd,
		Token:     "tok",
		SSH:       "aleh@192.168.1.68:2222",
	}
	if err := p.Validate(); err != nil {
		t.Fatalf("validate: %v", err)
	}
	// Marshal + unmarshal via the toml.Marshal/Unmarshal path that
	// peers.go uses to round-trip the file. Confirms the field tag is
	// wired.
	cfg := Config{Peers: []Peer{p}}
	buf, err := toml.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(buf), `ssh = 'aleh@192.168.1.68:2222'`) &&
		!strings.Contains(string(buf), `ssh = "aleh@192.168.1.68:2222"`) {
		t.Fatalf("marshalled output missing ssh field:\n%s", buf)
	}
	var got Config
	if err := toml.Unmarshal(buf, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(got.Peers) != 1 || got.Peers[0].SSH != p.SSH {
		t.Fatalf("ssh round-trip lost: %+v", got.Peers)
	}
}
