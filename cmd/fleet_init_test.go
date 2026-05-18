package main

import (
	"bufio"
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
	"github.com/alehatsman/mooncake/internal/fleet/discovery"
)

func TestParseTagInput(t *testing.T) {
	cases := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"   ", nil},
		{"linux", []string{"linux"}},
		{"linux,workstation", []string{"linux", "workstation"}},
		{" linux , workstation ", []string{"linux", "workstation"}},
		{"linux,,workstation", []string{"linux", "workstation"}},
		{"linux,workstation,linux", []string{"linux", "workstation"}}, // dedup
	}
	for _, c := range cases {
		got := parseTagInput(c.in)
		if !equalStrings(got, c.want) {
			t.Errorf("parseTagInput(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestPromptDefault_EmptyReturnsDefault(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("\n"))
	got, err := promptDefault(&bytes.Buffer{}, r, "name", "alpha")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "alpha" {
		t.Errorf("empty input should yield default, got %q", got)
	}
}

func TestPromptDefault_OverridesDefault(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("beta\n"))
	got, err := promptDefault(&bytes.Buffer{}, r, "name", "alpha")
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if got != "beta" {
		t.Errorf("expected override, got %q", got)
	}
}

func TestPromptDefault_TrimsCRLF(t *testing.T) {
	r := bufio.NewReader(strings.NewReader("beta\r\n"))
	got, _ := promptDefault(&bytes.Buffer{}, r, "name", "alpha")
	if got != "beta" {
		t.Errorf("CRLF trim failed: got %q", got)
	}
}

func TestPromptYesNo(t *testing.T) {
	cases := []struct {
		in         string
		defaultYes bool
		want       bool
	}{
		{"y\n", false, true},
		{"yes\n", false, true},
		{"YES\n", false, true},
		{"n\n", true, false},
		{"no\n", true, false},
		{"\n", true, true},
		{"\n", false, false},
		{"garbage\n", true, true},
		{"garbage\n", false, false},
	}
	for i, c := range cases {
		r := bufio.NewReader(strings.NewReader(c.in))
		got, err := promptYesNo(&bytes.Buffer{}, r, "?", c.defaultYes)
		if err != nil {
			t.Fatalf("[%d] err: %v", i, err)
		}
		if got != c.want {
			t.Errorf("[%d] in=%q defaultYes=%v got=%v want=%v", i, c.in, c.defaultYes, got, c.want)
		}
	}
}

func TestBuildInitPlan_Categorises(t *testing.T) {
	cands := []discovery.Candidate{
		{Name: "alive", Addr: "alive.lan:7878", Sources: []string{discovery.SourcePeersTOML}, AgentdOK: true},
		{Name: "broken", Addr: "broken.lan:7878", Sources: []string{discovery.SourcePeersTOML}, ProbeError: "timeout"},
		{Name: "new1", Addr: "new1.lan:7878", Sources: []string{discovery.SourceMDNS}},
		{Name: "sshhost", Addr: "sshhost", Sources: []string{discovery.SourceSSHConfig}, SSHUser: "aleh", SSHPort: 22},
	}
	plan := buildInitPlan(cands)
	if len(plan.ConfiguredAgentd) != 1 || plan.ConfiguredAgentd[0].Name != "alive" {
		t.Errorf("ConfiguredAgentd = %v", plan.ConfiguredAgentd)
	}
	if len(plan.ConfiguredAgentdBroken) != 1 || plan.ConfiguredAgentdBroken[0].Name != "broken" {
		t.Errorf("ConfiguredAgentdBroken = %v", plan.ConfiguredAgentdBroken)
	}
	if len(plan.NewCandidates) != 1 || plan.NewCandidates[0].Name != "new1" {
		t.Errorf("NewCandidates = %v", plan.NewCandidates)
	}
	if len(plan.SSHCandidates) != 1 || plan.SSHCandidates[0].Name != "sshhost" {
		t.Errorf("SSHCandidates = %v", plan.SSHCandidates)
	}
}

func TestRenderCandidateTable_EmptyShowsHelpfulLine(t *testing.T) {
	var buf bytes.Buffer
	renderCandidateTable(&buf, nil)
	if !strings.Contains(buf.String(), "no candidates found") {
		t.Errorf("empty table missing help line:\n%s", buf.String())
	}
}

func TestRenderCandidateTable_RendersAllSources(t *testing.T) {
	var buf bytes.Buffer
	cands := []discovery.Candidate{
		{Name: "alive", Addr: "alive:7878", Sources: []string{"peers.toml"}, AgentdOK: true, AgentdVersion: "0.9.0"},
		{Name: "new1", Addr: "new1:7878", Sources: []string{"mdns"}},
		{Name: "sshhost", Addr: "sshhost:22", Sources: []string{"ssh-config"}, SSHUser: "aleh", SSHPort: 22},
	}
	renderCandidateTable(&buf, cands)
	out := buf.String()
	for _, want := range []string{"SOURCE", "alive", "new1", "sshhost", "mooncake 0.9.0", "mdns responder; needs token", "ssh-only, not bootstrapped"} {
		if !strings.Contains(out, want) {
			t.Errorf("table missing %q:\n%s", want, out)
		}
	}
}

func TestCandidateStatus(t *testing.T) {
	cases := []struct {
		c    discovery.Candidate
		want string
	}{
		{discovery.Candidate{AgentdOK: true, AgentdVersion: "0.9.0"}, "✓ agentd up (mooncake 0.9.0)"},
		{discovery.Candidate{AgentdOK: true}, "✓ agentd up"},
		{discovery.Candidate{Sources: []string{"peers.toml"}, ProbeError: "timeout"}, "✗ timeout"},
		{discovery.Candidate{Sources: []string{"peers.toml"}}, "✗ agentd unreachable"},
		{discovery.Candidate{Sources: []string{"mdns"}}, "mdns responder; needs token"},
		{discovery.Candidate{Sources: []string{"ssh-config"}}, "ssh-only, not bootstrapped"},
	}
	for i, c := range cases {
		got := candidateStatus(c.c)
		if got != c.want {
			t.Errorf("[%d] %v → %q, want %q", i, c.c, got, c.want)
		}
	}
}

func TestSSHTargetFor(t *testing.T) {
	cases := []struct {
		c    discovery.Candidate
		want string
	}{
		{discovery.Candidate{Name: "host", Addr: "host"}, "host"},
		{discovery.Candidate{Name: "host", Addr: "host", SSHUser: "aleh"}, "aleh@host"},
		{discovery.Candidate{Name: "host", Addr: "host", SSHUser: "aleh", SSHPort: 22}, "aleh@host"},
		{discovery.Candidate{Name: "host", Addr: "host", SSHUser: "aleh", SSHPort: 2222}, "aleh@host -p 2222"},
		{discovery.Candidate{Name: "host", Addr: "1.2.3.4:22", SSHUser: "aleh"}, "aleh@1.2.3.4"},
	}
	for i, c := range cases {
		got := sshTargetFor(c.c)
		if got != c.want {
			t.Errorf("[%d] = %q, want %q", i, got, c.want)
		}
	}
}

// --- End-to-end: prompt loop drives Upsert correctly --------------------

func TestRunInitPrompts_AddsNewCandidateWithTokenAndTags(t *testing.T) {
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")

	plan := initPlan{
		NewCandidates: []discovery.Candidate{
			{Name: "desktop1", Addr: "desktop1.lan:7878", Sources: []string{discovery.SourceMDNS}},
		},
	}
	stdin := strings.NewReader(strings.Join([]string{
		"y",                 // Yes, add new peers
		"",                  // accept default name (desktop1)
		"linux,workstation", // tags
		"abc123",            // token
		"",
	}, "\n"))
	var out bytes.Buffer
	added, skipped, err := runInitPrompts(context.Background(), &out, stdin, plan, peersPath, false)
	if err != nil {
		t.Fatalf("runInitPrompts: %v\noutput:\n%s", err, out.String())
	}
	if added != 1 || skipped != 0 {
		t.Errorf("counts: added=%d skipped=%d", added, skipped)
	}

	// Confirm the peer was persisted.
	cfg, err := fleet.LoadPeers(peersPath)
	if err != nil {
		t.Fatalf("LoadPeers: %v", err)
	}
	if len(cfg.Peers) != 1 {
		t.Fatalf("expected 1 peer in peers.toml, got %d", len(cfg.Peers))
	}
	p := cfg.Peers[0]
	if p.Name != "desktop1" || p.Token != "abc123" || p.Transport != fleet.TransportAgentd {
		t.Errorf("peer = %+v", p)
	}
	if !equalStrings(p.Tags, []string{"linux", "workstation"}) {
		t.Errorf("tags = %v", p.Tags)
	}
}

func TestRunInitPrompts_EmptyTokenSkips(t *testing.T) {
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")
	plan := initPlan{
		NewCandidates: []discovery.Candidate{
			{Name: "desktop1", Addr: "desktop1.lan:7878", Sources: []string{discovery.SourceMDNS}},
		},
	}
	stdin := strings.NewReader(strings.Join([]string{"y", "", "", "", ""}, "\n"))
	var out bytes.Buffer
	added, skipped, err := runInitPrompts(context.Background(), &out, stdin, plan, peersPath, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if added != 0 || skipped != 1 {
		t.Errorf("counts: added=%d skipped=%d (empty token should skip)", added, skipped)
	}
	// peers.toml should be empty (no file written, or empty Peers).
	if _, err := os.Stat(peersPath); err == nil {
		cfg, _ := fleet.LoadPeers(peersPath)
		if len(cfg.Peers) != 0 {
			t.Errorf("expected no persisted peer, got %d", len(cfg.Peers))
		}
	}
}

func TestRunInitPrompts_DeclineAddsNothing(t *testing.T) {
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")
	plan := initPlan{
		NewCandidates: []discovery.Candidate{
			{Name: "x", Addr: "x:7878", Sources: []string{discovery.SourceMDNS}},
		},
	}
	stdin := strings.NewReader("n\n") // decline the umbrella "add new peers?" prompt
	var out bytes.Buffer
	added, skipped, err := runInitPrompts(context.Background(), &out, stdin, plan, peersPath, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if added != 0 || skipped != 0 {
		t.Errorf("decline should not increment counters: added=%d skipped=%d", added, skipped)
	}
}

func TestRunInitPrompts_AcceptAllErrorsOnMDNSCandidate(t *testing.T) {
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")
	plan := initPlan{
		NewCandidates: []discovery.Candidate{
			{Name: "x", Addr: "x:7878", Sources: []string{discovery.SourceMDNS}},
		},
	}
	// First prompt is the umbrella add-new-peers? question — we still
	// answer "y" so we reach the per-candidate loop where --accept-all
	// hits its no-token-source error.
	stdin := strings.NewReader("y\n")
	var out bytes.Buffer
	_, _, err := runInitPrompts(context.Background(), &out, stdin, plan, peersPath, true)
	if err == nil {
		t.Fatal("expected error for --accept-all with mDNS-only candidate")
	}
	if !strings.Contains(err.Error(), "no token source") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestRunInitPrompts_SSHCandidateGetsFleetBootstrapHint(t *testing.T) {
	dir := t.TempDir()
	peersPath := filepath.Join(dir, "peers.toml")
	plan := initPlan{
		SSHCandidates: []discovery.Candidate{
			{Name: "vps-1", Addr: "vps-1.example.com", Sources: []string{discovery.SourceSSHConfig}, SSHUser: "root", SSHPort: 22},
		},
	}
	// Decline the bootstrap-now prompt; expect a `mooncake fleet bootstrap` hint.
	stdin := strings.NewReader("n\n")
	var out bytes.Buffer
	added, skipped, err := runInitPrompts(context.Background(), &out, stdin, plan, peersPath, false)
	if err != nil {
		t.Fatalf("err: %v", err)
	}
	if added != 0 || skipped != 1 {
		t.Errorf("counts: added=%d skipped=%d", added, skipped)
	}
	if !strings.Contains(out.String(), "mooncake fleet bootstrap root@vps-1.example.com") {
		t.Errorf("expected bootstrap hint with root@vps-1.example.com, got:\n%s", out.String())
	}
}
