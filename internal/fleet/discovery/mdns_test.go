package discovery

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestAdvertiseOptions_TXTRecord(t *testing.T) {
	tests := []struct {
		name string
		opts AdvertiseOptions
		want []string
	}{
		{
			name: "user mode with all fields",
			opts: AdvertiseOptions{
				InstanceName: "main_pc",
				Port:         7878,
				Version:      "0.9.0",
				Hostname:     "main_pc",
				SystemMode:   false,
			},
			want: []string{"v=1", "hn=main_pc", "ver=0.9.0", "sm=user"},
		},
		{
			name: "system mode",
			opts: AdvertiseOptions{SystemMode: true},
			want: []string{"v=1", "sm=system"},
		},
		{
			name: "hostname omitted when blank",
			opts: AdvertiseOptions{Version: "1.0"},
			want: []string{"v=1", "ver=1.0", "sm=user"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.opts.txtRecord()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAdvertiseOptions_TXTNeverContainsToken(t *testing.T) {
	// Defense in depth: even if someone refactors AdvertiseOptions to
	// carry a token field, the TXT record must not surface it. The
	// `auth` keyword is the obvious one to grep for; expand if more
	// auth-shaped names appear.
	opts := AdvertiseOptions{InstanceName: "x", Port: 1, Version: "y", Hostname: "z"}
	rec := opts.txtRecord()
	for _, entry := range rec {
		for _, banned := range []string{"token", "auth", "secret", "bearer", "password"} {
			if strings.Contains(strings.ToLower(entry), banned) {
				t.Errorf("TXT record %q contains forbidden keyword %q", entry, banned)
			}
		}
	}
}

func TestAdvertise_RejectsZeroPortAndEmptyName(t *testing.T) {
	// Catch operator misconfiguration before zeroconf gets a malformed
	// registration. Both checks happen before any network IO.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	if err := Advertise(ctx, AdvertiseOptions{Port: 7878}); err == nil {
		t.Errorf("expected error for empty InstanceName")
	}
	if err := Advertise(ctx, AdvertiseOptions{InstanceName: "x", Port: 0}); err == nil {
		t.Errorf("expected error for zero Port")
	}
}

func TestSplitKV(t *testing.T) {
	tests := []struct {
		in     string
		wantK  string
		wantV  string
		wantOK bool
	}{
		{"hn=mainpc", "hn", "mainpc", true},
		{"ver=0.9.0", "ver", "0.9.0", true},
		{"v=1", "v", "1", true},
		{"value-with=multiple=equals", "value-with", "multiple=equals", true},
		{"no-equals", "no-equals", "", false},
		{"=leading-equal", "", "leading-equal", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			k, v, ok := splitKV(tt.in)
			if k != tt.wantK || v != tt.wantV || ok != tt.wantOK {
				t.Errorf("splitKV(%q) = (%q, %q, %v), want (%q, %q, %v)",
					tt.in, k, v, ok, tt.wantK, tt.wantV, tt.wantOK)
			}
		})
	}
}

func TestMergeMDNS_AppendsUnmatchedAndTagsExisting(t *testing.T) {
	// Two existing candidates from peers.toml; mDNS reports two responders
	// — one matches an existing peer by name (sources should merge), the
	// other is fresh (gets appended). Names+addrs deliberately distinct so
	// dedup-by-name is the only matching axis.
	existing := []Candidate{
		{Name: "laptop", Addr: "laptop.lan:7878", Sources: []string{SourcePeersTOML}},
		{Name: "macbook", Addr: "macbook.lan:7878", Sources: []string{SourcePeersTOML}, AgentdVersion: "1.0"},
	}
	mdns := []Candidate{
		// Matches macbook by name → sources extended; existing version
		// (1.0) takes precedence over the mDNS-reported one.
		{Name: "macbook", Addr: "10.0.0.5:7878", Sources: []string{SourceMDNS}, AgentdVersion: "0.9"},
		// Fresh hit → appended.
		{Name: "ironcage", Addr: "10.0.0.6:7878", Sources: []string{SourceMDNS}, AgentdVersion: "0.9.5"},
	}

	merged := mergeMDNS(existing, mdns)
	if len(merged) != 3 {
		t.Fatalf("expected 3 merged candidates, got %d: %+v", len(merged), merged)
	}

	// Find macbook entry — sources should be ["peers.toml", "mdns"].
	var macbook *Candidate
	for i := range merged {
		if merged[i].Name == "macbook" {
			macbook = &merged[i]
		}
	}
	if macbook == nil {
		t.Fatalf("macbook missing")
	}
	if !macbook.HasSource(SourcePeersTOML) || !macbook.HasSource(SourceMDNS) {
		t.Errorf("macbook sources = %v, want both peers.toml and mdns", macbook.Sources)
	}
	if macbook.AgentdVersion != "1.0" {
		t.Errorf("existing version 1.0 should win over mDNS 0.9, got %q", macbook.AgentdVersion)
	}

	// New mDNS-only entry preserves its source and reported version.
	var iron *Candidate
	for i := range merged {
		if merged[i].Name == "ironcage" {
			iron = &merged[i]
		}
	}
	if iron == nil {
		t.Fatalf("ironcage missing")
	}
	if len(iron.Sources) != 1 || iron.Sources[0] != SourceMDNS {
		t.Errorf("ironcage sources = %v, want [mdns]", iron.Sources)
	}
	if iron.AgentdVersion != "0.9.5" {
		t.Errorf("ironcage version = %q, want 0.9.5", iron.AgentdVersion)
	}
}

func TestMergeMDNS_MDNSVersionFillsWhenExistingEmpty(t *testing.T) {
	// SSH-config-only candidates have no version (no agentd probe was
	// attempted). When mDNS reports a version for the same name, it
	// should fill the gap.
	existing := []Candidate{
		{Name: "ssh-box", Sources: []string{SourceSSHConfig}},
	}
	mdns := []Candidate{
		{Name: "ssh-box", AgentdVersion: "0.9", Sources: []string{SourceMDNS}},
	}
	merged := mergeMDNS(existing, mdns)
	if len(merged) != 1 {
		t.Fatalf("expected 1, got %d", len(merged))
	}
	if merged[0].AgentdVersion != "0.9" {
		t.Errorf("mDNS version should fill empty, got %q", merged[0].AgentdVersion)
	}
}

func TestMergeMDNS_EmptyInputs(t *testing.T) {
	// Empty in, empty out — must not panic and must return a usable
	// slice (the Aggregate caller relies on len()).
	got := mergeMDNS(nil, nil)
	if got != nil {
		t.Errorf("expected nil, got %v", got)
	}
}

// TestQueryMDNS_LoopbackRoundtripBestEffort tries a real Advertise+Query
// round-trip on a free port. mDNS over loopback / WSL multicast is
// flaky; this test is intentionally tolerant — it passes both when the
// browse finds the responder AND when no multicast interface is usable.
// The point is to catch egregious regressions (wiring failures, panics)
// without flapping on environments where loopback mDNS just doesn't
// work.
func TestQueryMDNS_LoopbackRoundtripBestEffort(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping mDNS network test in -short mode")
	}

	advertiseCtx, cancelAdvertise := context.WithCancel(context.Background())
	t.Cleanup(cancelAdvertise)

	go func() {
		_ = Advertise(advertiseCtx, AdvertiseOptions{
			InstanceName: "mdns-loopback-test",
			Port:         18787,
			Version:      "0.0.0-test",
			Hostname:     "mdns-loopback-test",
			SystemMode:   false,
		})
	}()

	// Give the responder a moment to register.
	time.Sleep(150 * time.Millisecond)

	queryCtx, cancelQuery := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancelQuery()
	cands, err := QueryMDNS(queryCtx, MDNSQueryOptions{Timeout: 1500 * time.Millisecond})
	if err != nil {
		// A multicast failure is non-fatal — see test comment.
		t.Logf("QueryMDNS errored (acceptable in restricted networks): %v", err)
		return
	}
	for _, c := range cands {
		if c.Name == "mdns-loopback-test" {
			if c.AgentdVersion != "0.0.0-test" {
				t.Errorf("round-trip version mismatch: %q", c.AgentdVersion)
			}
			return // success
		}
	}
	// Not finding our advertised instance is acceptable — see test
	// comment about WSL/loopback mDNS flakiness.
	t.Logf("loopback round-trip did not surface our instance (acceptable on this platform)")
}
