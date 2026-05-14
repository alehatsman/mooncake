package discovery

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseSSHConfig_SingleHostBlock(t *testing.T) {
	in := `Host laptop
    HostName laptop.lan
    User aleh
    Port 2222
`
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := []Candidate{{
		Name:    "laptop",
		Addr:    "laptop.lan",
		SSHUser: "aleh",
		SSHPort: 2222,
		Sources: []string{SourceSSHConfig},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v\nwant %+v", got, want)
	}
}

func TestParseSSHConfig_MultiHostLineSharesDirectives(t *testing.T) {
	// `Host a b c` is a single block whose directives apply to all three
	// names. Each name becomes its own Candidate.
	in := `Host a b c
    User shared
`
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 candidates, got %d (%+v)", len(got), got)
	}
	for _, c := range got {
		if c.SSHUser != "shared" {
			t.Errorf("%s: SSHUser %q, want \"shared\"", c.Name, c.SSHUser)
		}
		// Addr defaults to the alias name when HostName is omitted.
		if c.Addr != c.Name {
			t.Errorf("%s: Addr %q, want default-to-name %q", c.Name, c.Addr, c.Name)
		}
	}
}

func TestParseSSHConfig_WildcardHostsSkipped(t *testing.T) {
	// Wildcard blocks define patterns, not specific machines, so they
	// shouldn't surface as candidates. The concrete block after the
	// wildcard should still be picked up.
	in := `Host *
    User globaldefault

Host *.example.com
    User customer

Host real-host
    HostName 10.0.0.1
`
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "real-host" {
		t.Fatalf("expected only real-host, got %+v", got)
	}
}

func TestParseSSHConfig_CaseInsensitiveKeywords(t *testing.T) {
	in := "HOST upper\n    HOSTNAME upper.lan\n    user mixed\n    PORT 22\n"
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].Addr != "upper.lan" || got[0].SSHUser != "mixed" || got[0].SSHPort != 22 {
		t.Fatalf("case-folding broken: %+v", got[0])
	}
}

func TestParseSSHConfig_CommentsAndBlanks(t *testing.T) {
	in := `# top-level comment

Host commented  # trailing comment
    HostName c.example.com   # trailing on directive
`
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Name != "commented" || got[0].Addr != "c.example.com" {
		t.Fatalf("comment handling broken: %+v", got)
	}
}

func TestParseSSHConfig_EqualsSeparator(t *testing.T) {
	// ssh_config accepts `Key=Value` as well as `Key Value`. Both must
	// work or operators get surprised.
	in := "Host eq\n    HostName=eq.lan\n    Port=2200\n"
	got, err := parseSSHConfigReader(strings.NewReader(in))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Addr != "eq.lan" || got[0].SSHPort != 2200 {
		t.Fatalf("equals syntax not honored: %+v", got)
	}
}

func TestParseSSHConfig_AbsentFileIsNotAnError(t *testing.T) {
	// A missing ~/.ssh/config is the normal state for many users. The
	// discover flow must not bail on it.
	got, err := ParseSSHConfig(filepath.Join(t.TempDir(), "does-not-exist"))
	if err != nil {
		t.Fatalf("expected nil error for missing file, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected empty result for missing file, got %+v", got)
	}
}

func TestParseSSHConfig_OnDiskRoundTrip(t *testing.T) {
	// Confirm ParseSSHConfig (the os.Open path) agrees with
	// parseSSHConfigReader on a real file.
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	content := "Host x\n    HostName x.lan\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := ParseSSHConfig(path)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Addr != "x.lan" {
		t.Fatalf("on-disk parse broken: %+v", got)
	}
}

func TestParseSSHConfig_InvalidPortErrors(t *testing.T) {
	_, err := parseSSHConfigReader(strings.NewReader("Host bad\n    Port not-a-number\n"))
	if err == nil {
		t.Fatalf("expected error on invalid Port")
	}
}
