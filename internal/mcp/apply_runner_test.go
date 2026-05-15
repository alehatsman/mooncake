package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
	_ "github.com/alehatsman/mooncake/internal/register" // register handlers
)

// TestKernelResult_UsableFromMCP is the R1.1b "no circular deps,
// real shape" smoke test for the kernel-surface contract. The MCP
// server constructs apply.Runner directly (see internal/mcp/tools.go);
// the test proves a) the import graph compiles from this package,
// and b) the *KernelResult returned by Run carries the four
// documented fields with sensible content for a one-step run.
//
// Lives in internal/mcp so the test fails loudly if a future change
// reintroduces an import cycle between mcp ↔ apply ↔ executor.
func TestKernelResult_UsableFromMCP(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "mcp-smoke.txt")
	cfgPath := filepath.Join(tmp, "mooncake.yml")
	if err := os.WriteFile(cfgPath, []byte(`
- name: mcp smoke create
  file.write:
    path: `+path+`
    state: file
    content: "mcp\n"
    mode: "0644"
`), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg := &apply.Config{
		ConfigPath:   cfgPath,
		OutputFormat: "quiet",
		LogLevel:     "error",
	}

	result, err := apply.NewRunner(cfg).Run(context.Background())
	if err != nil {
		t.Fatalf("apply.NewRunner.Run returned error: %v", err)
	}
	if result == nil {
		t.Fatalf("Run returned nil *KernelResult")
	}

	// All four documented fields populated.
	if result.Plan == nil {
		t.Error("KernelResult.Plan = nil; want compiled plan")
	}
	if len(result.Steps) == 0 {
		t.Error("KernelResult.Steps empty; want >= 1 step record")
	}
	if len(result.Events) == 0 {
		t.Error("KernelResult.Events empty; want >= 1 lifecycle event")
	}
	if !result.Summary.Success || result.Summary.TotalSteps == 0 {
		t.Errorf("KernelResult.Summary = %+v; want Success=true, TotalSteps>0",
			result.Summary)
	}

	// Reverse() is the second kernel-surface method MCP callers want.
	// Smoke-test it returns an inverse plan without erroring.
	inverse, rerr := result.Reverse()
	if rerr != nil {
		t.Fatalf("Reverse returned error: %v", rerr)
	}
	if inverse == nil {
		t.Fatalf("Reverse returned nil plan")
	}
}
