package main

import (
	"reflect"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/fleet"
)

// peersForMachineTest mirrors the canonical Windows+WSL layout the
// machine convention is designed for — two peers, same physical box,
// distinct names. Used across the resolveMachinePhase tests.
func peersForMachineTest() []fleet.Peer {
	return []fleet.Peer{
		{Name: "main_pc-win", Addr: "192.168.1.10:7879", Token: "t", Transport: fleet.TransportAgentd, Tags: []string{"windows"}},
		{Name: "main_pc", Addr: "192.168.1.10:7878", Token: "t", Transport: fleet.TransportAgentd, Tags: []string{"linux", "wsl"}},
		{Name: "mac", Addr: "192.168.1.20:7878", Token: "t", Transport: fleet.TransportSSH}, // non-agentd, exercises the skip path
	}
}

func TestResolveMachinePhase_FindsPeerAndLayersVarsAndTags(t *testing.T) {
	peers := peersForMachineTest()
	phase := fleet.MachinePhase{
		Name: "windows-host",
		Peer: "main_pc-win",
		Plan: "/abs/plan.yml",
		Vars: []string{"/abs/win.yml"},
		Tags: []string{"windows"},
	}
	in, err := resolveMachinePhase(
		machinePhaseInput{Phase: phase, PhaseNum: 1, TotalPhases: 2},
		peers, "/abs/plan-dir",
		[]string{"/abs/cli.yml"},  // CLI-level --vars-file
		[]string{"deploy"},        // CLI-level --tags
		[]string{"install nvim"},  // step-filter names
		1024, 4, "controller-id",
		nil, nil, // no peer-filter, no os resolver
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}

	// Peer selection: phase pins to main_pc-win exactly.
	if len(in.Peers) != 1 || in.Peers[0].Name != "main_pc-win" {
		t.Errorf("peers = %v, want [main_pc-win]", in.Peers)
	}
	// Vars stack: CLI first, then per-phase (later wins on key collision,
	// matching mooncake's later-wins var-file semantics).
	wantVars := []string{"/abs/cli.yml", "/abs/win.yml"}
	if !reflect.DeepEqual(in.VarsAbs, wantVars) {
		t.Errorf("vars = %v, want %v", in.VarsAbs, wantVars)
	}
	// Tags: CLI ∪ per-phase. Union semantics match utils.MatchesTags.
	wantTags := []string{"deploy", "windows"}
	if !reflect.DeepEqual(in.Tags, wantTags) {
		t.Errorf("tags = %v, want %v", in.Tags, wantTags)
	}
	// Step names pass through untouched (step-filter is CLI-only).
	if !reflect.DeepEqual(in.StepNames, []string{"install nvim"}) {
		t.Errorf("step names dropped: %v", in.StepNames)
	}
	// Banner heading is meaningful for the user.
	for _, want := range []string{"phase 1/2", "windows-host", "main_pc-win", "/abs/plan.yml"} {
		if !strings.Contains(in.BannerHeading, want) {
			t.Errorf("banner %q missing %q", in.BannerHeading, want)
		}
	}
}

func TestResolveMachinePhase_UnknownPeerErrors(t *testing.T) {
	peers := peersForMachineTest()
	phase := fleet.MachinePhase{Name: "p", Peer: "not-in-peers-toml", Plan: "/p"}
	_, err := resolveMachinePhase(
		machinePhaseInput{Phase: phase, PhaseNum: 1, TotalPhases: 1},
		peers, "/d", nil, nil, nil, 0, 0, "id", nil, nil,
	)
	if err == nil {
		t.Fatalf("expected error for unknown peer")
	}
	if !strings.Contains(err.Error(), "not-in-peers-toml") {
		t.Errorf("error %q should name the missing peer", err)
	}
}

func TestResolveMachinePhase_NonAgentdPeerGoesToSkippedList(t *testing.T) {
	// A phase pinned to an ssh-transport peer should not fail loudly — the
	// existing single-phase code surfaces it as a "skipped" banner. Same
	// behaviour in machine mode: agentd peers populate in.Peers; the ssh
	// peer goes into in.SkippedPeers. The caller (runMachineApply) sees a
	// phase with zero agentd peers and bails out cleanly.
	peers := peersForMachineTest()
	phase := fleet.MachinePhase{Name: "p", Peer: "mac", Plan: "/p"}
	in, err := resolveMachinePhase(
		machinePhaseInput{Phase: phase, PhaseNum: 1, TotalPhases: 1},
		peers, "/d", nil, nil, nil, 0, 0, "id", nil, nil,
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(in.Peers) != 0 {
		t.Errorf("agentd peer list should be empty for ssh peer, got %v", in.Peers)
	}
	if len(in.SkippedPeers) != 1 || in.SkippedPeers[0].Name != "mac" {
		t.Errorf("expected ssh peer in skipped list, got %v", in.SkippedPeers)
	}
}

func TestResolveMachinePhase_PeerFilterCanZeroOutAPhase(t *testing.T) {
	// --peer-filter on the command line is AND'd with the manifest's
	// pinned peer. When the phase's peer doesn't match the filter, we
	// return an empty in.Peers so the caller can print a "skipped by
	// --peer-filter" banner and move on rather than failing the run.
	peers := peersForMachineTest()
	phase := fleet.MachinePhase{Name: "p", Peer: "main_pc-win", Plan: "/p"}
	// Filter for linux peers: main_pc-win (tagged windows) shouldn't match.
	groups, parseErr := parseFilterFlags([]string{"tag=linux"})
	if parseErr != nil {
		t.Fatalf("parse filter: %v", parseErr)
	}
	in, err := resolveMachinePhase(
		machinePhaseInput{Phase: phase, PhaseNum: 1, TotalPhases: 1},
		peers, "/d", nil, nil, nil, 0, 0, "id", groups, nil,
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(in.Peers) != 0 {
		t.Errorf("expected --peer-filter to zero out the phase, got %v", in.Peers)
	}
}

func TestResolveMachinePhase_PeerFilterMatchKeepsPhase(t *testing.T) {
	// Sibling assertion: when the filter does match the phase's peer, the
	// phase runs normally (in.Peers has one entry).
	peers := peersForMachineTest()
	phase := fleet.MachinePhase{Name: "p", Peer: "main_pc-win", Plan: "/p"}
	groups, _ := parseFilterFlags([]string{"tag=windows"})
	in, err := resolveMachinePhase(
		machinePhaseInput{Phase: phase, PhaseNum: 1, TotalPhases: 1},
		peers, "/d", nil, nil, nil, 0, 0, "id", groups, nil,
	)
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(in.Peers) != 1 || in.Peers[0].Name != "main_pc-win" {
		t.Errorf("phase should still match windows tag, got %v", in.Peers)
	}
}
