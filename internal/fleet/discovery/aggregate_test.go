package discovery

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeFile is a thin helper to keep test setup readable.
func writeFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

func TestAggregate_PeersOnly(t *testing.T) {
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "laptop"
addr = "laptop.lan:7878"
token = "tok"
tags = ["linux"]
`)

	noProbe := false
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: "-",
		Probe:         &noProbe,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 1 || got[0].Name != "laptop" {
		t.Fatalf("got %+v", got)
	}
	if !got[0].HasSource(SourcePeersTOML) || got[0].HasSource(SourceSSHConfig) {
		t.Fatalf("sources wrong: %+v", got[0].Sources)
	}
	if len(got[0].Tags) != 1 || got[0].Tags[0] != "linux" {
		t.Fatalf("tags lost: %+v", got[0].Tags)
	}
}

func TestAggregate_DeduplicatesAcrossSources(t *testing.T) {
	// Same name in both peers.toml and ~/.ssh/config must collapse into
	// one Candidate with merged Sources. peers.toml wins the slot (its
	// addr is kept), but the ssh User/Port are layered in so the entry
	// remains bootstrappable.
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "macbook"
addr = "macbook.lan:7878"
token = "tok"
`)
	sshPath := writeFile(t, dir, "ssh_config", `Host macbook
    HostName macbook.lan
    User aleh
    Port 2222

Host vps-1
    HostName vps-1.example.com
    Port 22
`)

	noProbe := false
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: sshPath,
		Probe:         &noProbe,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 candidates (macbook merged + vps-1 alone), got %d: %+v", len(got), got)
	}
	// Find macbook
	var mb *Candidate
	for i := range got {
		if got[i].Name == "macbook" {
			mb = &got[i]
		}
	}
	if mb == nil {
		t.Fatalf("macbook missing: %+v", got)
	}
	if !mb.HasSource(SourcePeersTOML) || !mb.HasSource(SourceSSHConfig) {
		t.Fatalf("macbook should have both sources, got %v", mb.Sources)
	}
	// peers.toml addr wins.
	if mb.Addr != "macbook.lan:7878" {
		t.Fatalf("peers.toml addr should win on dedup, got %q", mb.Addr)
	}
	// ssh User/Port layered in.
	if mb.SSHUser != "aleh" || mb.SSHPort != 2222 {
		t.Fatalf("ssh user/port not merged: %+v", mb)
	}
}

func TestAggregate_SortedByName(t *testing.T) {
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "zebra"
addr = "z.lan:7878"
token = "t"

[[peers]]
name = "alpha"
addr = "a.lan:7878"
token = "t"
`)
	noProbe := false
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: "-",
		Probe:         &noProbe,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 2 || got[0].Name != "alpha" || got[1].Name != "zebra" {
		t.Fatalf("not sorted by name: %+v", got)
	}
}

func TestAggregate_MissingPeersFileIsOk(t *testing.T) {
	// LoadPeers returns an empty Config when peers.toml doesn't exist.
	// Aggregate must propagate that as zero candidates, not an error.
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     filepath.Join(t.TempDir(), "missing.toml"),
		SSHConfigPath: "-",
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(got) != 0 {
		t.Fatalf("expected zero candidates, got %+v", got)
	}
}

func TestAggregate_ProbesAgentdVersion(t *testing.T) {
	// Spin up a fake /v1/version handler; the probe should hit it and
	// fill AgentdOK + AgentdVersion. Asserts the bearer-token path also
	// works (the handler rejects wrong tokens with 401).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer probe-token" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if r.URL.Path != "/v1/version" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"version":     "1.2.3",
			"hostname":    "probehost",
			"synced_root": "/var/lib/mooncake/synced",
			"system_mode": false,
		})
	}))
	defer srv.Close()

	addr := strings.TrimPrefix(srv.URL, "http://")
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "fake"
addr = "`+addr+`"
token = "probe-token"
`)

	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: "-",
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if !got[0].AgentdOK {
		t.Fatalf("probe should have succeeded, got %+v", got[0])
	}
	if got[0].AgentdVersion != "1.2.3" {
		t.Fatalf("version not parsed: %+v", got[0])
	}
	if got[0].ProbeError != "" {
		t.Fatalf("ProbeError set on success: %q", got[0].ProbeError)
	}
}

func TestAggregate_ProbeFailureSurfacesShortReason(t *testing.T) {
	// peers.toml points at an address that nothing listens on; the
	// probe must downgrade gracefully — populate ProbeError, leave
	// AgentdOK false. The candidate is NOT dropped from the result.
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "dead"
addr = "127.0.0.1:1"
token = "tok"
`)
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: "-",
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].AgentdOK {
		t.Fatalf("expected probe failure, got AgentdOK")
	}
	if got[0].ProbeError == "" {
		t.Fatalf("expected ProbeError to be set on failure")
	}
}

func TestAggregate_ProbeDisabledLeavesAgentdStatusEmpty(t *testing.T) {
	// --no-probe path: skip the round-trip entirely. Even unreachable
	// peers should come back with AgentdOK=false and an empty
	// ProbeError (so the CLI can render "(probe skipped)").
	dir := t.TempDir()
	peersPath := writeFile(t, dir, "peers.toml", `
[[peers]]
name = "dead"
addr = "127.0.0.1:1"
token = "tok"
`)
	probe := false
	got, err := Aggregate(context.Background(), Options{
		PeersPath:     peersPath,
		SSHConfigPath: "-",
		Probe:         &probe,
	})
	if err != nil {
		t.Fatalf("aggregate: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("got %+v", got)
	}
	if got[0].AgentdOK || got[0].ProbeError != "" {
		t.Fatalf("expected probe to be skipped, got %+v", got[0])
	}
}
