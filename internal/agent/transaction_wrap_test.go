package agent

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/config"
	_ "github.com/alehatsman/mooncake/internal/register"
)

func TestWrapInTransaction_BareListInput(t *testing.T) {
	in := []byte(`- file.write:
    path: /tmp/a
    content: alpha
- file.write:
    path: /tmp/b
    content: beta
`)
	out, err := WrapInTransaction(in)
	if err != nil {
		t.Fatalf("WrapInTransaction: %v", err)
	}

	var rc config.RunConfig
	if err := yaml.Unmarshal(out, &rc); err != nil {
		t.Fatalf("decode wrapped plan: %v\n%s", err, out)
	}
	if len(rc.Steps) != 1 {
		t.Fatalf("expected exactly one top-level step (the wrapper), got %d", len(rc.Steps))
	}
	wrapper := rc.Steps[0]
	if wrapper.Name != transactionWrapName {
		t.Errorf("wrapper.Name = %q, want %q", wrapper.Name, transactionWrapName)
	}
	if !wrapper.AllowIrreversible {
		t.Error("wrapper.AllowIrreversible = false, want true (otherwise plan-time check rejects irreversible steps in agent plans)")
	}
	if got, want := len(wrapper.Transaction), 2; got != want {
		t.Errorf("len(wrapper.Transaction) = %d, want %d", got, want)
	}
}

func TestWrapInTransaction_StructuredInputPreservesVersionAndVars(t *testing.T) {
	in := []byte(`version: "1.0"
vars:
  greeting: hello
steps:
  - file.write:
      path: /tmp/a
      content: alpha
`)
	out, err := WrapInTransaction(in)
	if err != nil {
		t.Fatalf("WrapInTransaction: %v", err)
	}

	var rc config.RunConfig
	if err := yaml.Unmarshal(out, &rc); err != nil {
		t.Fatalf("decode wrapped plan: %v\n%s", err, out)
	}
	if rc.Version != "1.0" {
		t.Errorf("rc.Version = %q, want %q", rc.Version, "1.0")
	}
	if got, ok := rc.Vars["greeting"]; !ok || got != "hello" {
		t.Errorf("rc.Vars[\"greeting\"] = %v, want %q", got, "hello")
	}
	if len(rc.Steps) != 1 || len(rc.Steps[0].Transaction) != 1 {
		t.Errorf("expected one wrapper step with one child; got %+v", rc.Steps)
	}
}

func TestWrapInTransaction_EmptyPlanPassthrough(t *testing.T) {
	in := []byte("steps: []\n")
	out, err := WrapInTransaction(in)
	if err != nil {
		t.Fatalf("WrapInTransaction: %v", err)
	}
	if string(out) != string(in) {
		t.Errorf("empty plan should pass through unchanged\n  in:  %q\n  out: %q", in, out)
	}
}

func TestCountIrreversibleSteps_AllReversible(t *testing.T) {
	in := []byte(`- file.write:
    path: /tmp/a
    content: alpha
- file.write:
    path: /tmp/b
    content: beta
`)
	n, err := CountIrreversibleSteps(in)
	if err != nil {
		t.Fatalf("CountIrreversibleSteps: %v", err)
	}
	if n != 0 {
		t.Errorf("expected 0 irreversible, got %d", n)
	}
}

func TestCountIrreversibleSteps_ShellIsIrreversible(t *testing.T) {
	// shell.run has no general Reverse() — running an arbitrary shell
	// command can't be undone. Treated as irreversible.
	in := []byte(`- shell:
    cmd: "echo hello"
`)
	n, err := CountIrreversibleSteps(in)
	if err != nil {
		t.Fatalf("CountIrreversibleSteps: %v", err)
	}
	if n != 1 {
		t.Errorf("expected 1 irreversible (shell), got %d", n)
	}
}

// TestTransactionWrap_Step3FailureRollsBackToPreState is the spec-67 §16
// DoD headline: a agent plan whose step 3 fails must leave the system
// byte-identical to its pre-agent state.
func TestTransactionWrap_Step3FailureRollsBackToPreState(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	tmpDir := t.TempDir()
	initGitRepoForTest(t, tmpDir)

	aPath := filepath.Join(tmpDir, "a.txt")
	bPath := filepath.Join(tmpDir, "b.txt")
	// /dev/null is a character device; using it as a parent directory
	// makes file.write fail with ENOTDIR — same failure pattern the
	// in-tree example examples/transactions/rollback-demo.yml uses.
	cPath := "/dev/null/cannot-exist-here"

	for _, p := range []string{aPath, bPath} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Fatalf("pre-state: %s should not exist (got err=%v)", p, err)
		}
	}

	plan := fmt.Sprintf(`- file.write:
    path: %s
    content: "alpha"
- file.write:
    path: %s
    content: "beta"
- file.write:
    path: %s
    content: "gamma"
`, aPath, bPath, cPath)

	planPath := filepath.Join(tmpDir, "plan.yml")
	if err := os.WriteFile(planPath, []byte(plan), 0o644); err != nil {
		t.Fatalf("write plan: %v", err)
	}

	_, err := Run(RunOptions{
		Goal:     "regression: step 3 must trigger LIFO rollback",
		PlanPath: planPath,
		RepoRoot: tmpDir,
	})
	if err == nil {
		t.Fatal("Run returned nil error; expected step 3 (file.write to /dev/null/…) to fail and tear down the transaction")
	}

	for _, p := range []string{aPath, bPath} {
		if _, statErr := os.Stat(p); !os.IsNotExist(statErr) {
			t.Errorf("post-rollback: %s still exists (stat err: %v); transaction did not revert", p, statErr)
		}
	}
	if !strings.Contains(err.Error(), "cannot-exist-here") && !strings.Contains(err.Error(), "not a directory") {
		t.Logf("note: error message did not surface the failing path directly: %v", err)
	}
}

func initGitRepoForTest(t *testing.T, dir string) {
	t.Helper()
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "Test User"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	seed := filepath.Join(dir, ".seed")
	if err := os.WriteFile(seed, []byte("seed"), 0o644); err != nil {
		t.Fatalf("seed write: %v", err)
	}
	for _, args := range [][]string{
		{"add", "."},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}
