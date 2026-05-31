package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/alehatsman/mooncake/internal/register"
)

// TestHandleRunPlan_PolicyDeniesShell proves the MCP run_plan tool
// honors the #11 permissions-as-contract gate: a `policy` argument that
// denies shell makes the run refuse a shell step before its side
// effect. This is the MCP-driven equivalent of the executor/pilot
// guarantee — the path a Claude-via-MCP agent takes.
func TestHandleRunPlan_PolicyDeniesShell(t *testing.T) {
	dir := t.TempDir()
	sentinel := filepath.Join(dir, "pwned")

	cfg := filepath.Join(dir, "mc.yml")
	content := "version: \"1.0\"\nsteps:\n  - shell: \"touch " + sentinel + "\"\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := HandleRunPlan(nil, mustJSON(t, map[string]interface{}{
		"config": cfg,
		"policy": map[string]interface{}{"denied_actions": []string{"shell", "cmd"}},
	}))
	if err != nil {
		t.Fatalf("HandleRunPlan transport error: %v", err)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal([]byte(out), &resp); err != nil {
		t.Fatalf("unmarshal: %v\nbody: %s", err, out)
	}

	// The run must surface the policy denial in its error field.
	if errStr, _ := resp["error"].(string); !strings.Contains(errStr, "denylist") {
		t.Errorf("expected a policy denylist error, got %q\nbody: %s", resp["error"], out)
	}
	// And the shell step must NOT have run.
	if _, statErr := os.Stat(sentinel); statErr == nil {
		t.Errorf("shell ran despite the policy — sentinel %s exists", sentinel)
	}
}

// TestHandleRunPlan_NoPolicyRunsUngated guards backward-compat: omitting
// the policy argument runs exactly as before (the field is optional).
func TestHandleRunPlan_NoPolicyRunsUngated(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "ok.txt")

	cfg := filepath.Join(dir, "mc.yml")
	content := "version: \"1.0\"\nsteps:\n  - file.write:\n      path: " + target + "\n      content: ok\n      state: file\n"
	if err := os.WriteFile(cfg, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := HandleRunPlan(nil, mustJSON(t, map[string]string{"config": cfg}))
	if err != nil {
		t.Fatalf("HandleRunPlan: %v", err)
	}
	if _, statErr := os.Stat(target); statErr != nil {
		t.Errorf("ungated run did not write the file: %v\nbody: %s", statErr, out)
	}
}
