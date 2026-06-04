package mcp_test

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/mcp"
)

// writePeersToml writes a minimal peers.toml to a temp dir and returns the path.
func writePeersToml(t *testing.T, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "peers.toml")
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write peers.toml: %v", err)
	}
	return path
}

func TestHandleListPeers_EmptyFile(t *testing.T) {
	path := writePeersToml(t, "")
	args, _ := json.Marshal(map[string]string{"peers_file": path})
	out, err := mcp.HandleListPeers(context.Background(), args)
	if err != nil {
		t.Fatalf("HandleListPeers: %v", err)
	}
	var result struct {
		Total int `json:"total"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Total != 0 {
		t.Errorf("total = %d, want 0", result.Total)
	}
}

func TestHandleListPeers_WithPeers(t *testing.T) {
	const toml = `
[[peers]]
name = "box1"
addr = "192.168.1.10:7777"
transport = "agentd"
token = "tok1"
tags = ["linux", "desktop"]

[[peers]]
name = "box2"
addr = "192.168.1.11:7777"
transport = "agentd"
token = "tok2"
roles = ["storage"]
`
	path := writePeersToml(t, toml)
	args, _ := json.Marshal(map[string]string{"peers_file": path})
	out, err := mcp.HandleListPeers(context.Background(), args)
	if err != nil {
		t.Fatalf("HandleListPeers: %v", err)
	}

	var result struct {
		Total     int    `json:"total"`
		PeersFile string `json:"peers_file"`
		Peers     []struct {
			Name      string   `json:"name"`
			Transport string   `json:"transport"`
			Tags      []string `json:"tags"`
			Roles     []string `json:"roles"`
		} `json:"peers"`
	}
	if err := json.Unmarshal([]byte(out), &result); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if result.Total != 2 {
		t.Errorf("total = %d, want 2", result.Total)
	}
	if result.PeersFile != path {
		t.Errorf("peers_file = %q, want %q", result.PeersFile, path)
	}
	if result.Peers[0].Name != "box1" {
		t.Errorf("peers[0].name = %q, want box1", result.Peers[0].Name)
	}
	if len(result.Peers[0].Tags) != 2 {
		t.Errorf("peers[0].tags len = %d, want 2", len(result.Peers[0].Tags))
	}
	if len(result.Peers[1].Roles) != 1 {
		t.Errorf("peers[1].roles len = %d, want 1", len(result.Peers[1].Roles))
	}
}

func TestHandleListPeers_InvalidArgs(t *testing.T) {
	_, err := mcp.HandleListPeers(context.Background(), []byte("not-json"))
	if err == nil {
		t.Fatal("expected error for invalid JSON args")
	}
}

// TestHandleFleetRunPlan_NoPeers verifies the tool returns an error when
// the peers file is empty — no network required.
func TestHandleFleetRunPlan_NoPeers(t *testing.T) {
	path := writePeersToml(t, "")
	args, _ := json.Marshal(map[string]interface{}{
		"config":     "/nonexistent/plan.yml",
		"peers_file": path,
	})
	_, err := mcp.HandleFleetRunPlan(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when no peers configured")
	}
}

// TestHandleFleetRunPlan_UnknownPeer verifies the tool errors when the
// requested peer name does not exist in peers.toml.
func TestHandleFleetRunPlan_UnknownPeer(t *testing.T) {
	const toml = `
[[peers]]
name = "box1"
addr = "192.168.1.10:7777"
transport = "agentd"
token = "tok1"
`
	path := writePeersToml(t, toml)
	args, _ := json.Marshal(map[string]interface{}{
		"config":     "/nonexistent/plan.yml",
		"peers_file": path,
		"peers":      []string{"no-such-peer"},
	})
	_, err := mcp.HandleFleetRunPlan(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when peer name doesn't match")
	}
}

// TestHandleFleetRunPlan_MissingConfig verifies the tool requires the config param.
func TestHandleFleetRunPlan_MissingConfig(t *testing.T) {
	_, err := mcp.HandleFleetRunPlan(context.Background(), []byte(`{}`))
	if err == nil {
		t.Fatal("expected error when config param is missing")
	}
}
