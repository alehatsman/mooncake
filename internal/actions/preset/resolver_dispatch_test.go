package preset

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/modules"
	"github.com/alehatsman/mooncake/internal/presets"
)

// makeFixtureModule builds a bare git repo containing a tiny module with one
// default export. Returned path is the bare repo (used as a file:// URL).
func makeFixtureModule(t *testing.T, tag string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	work := filepath.Join(dir, "work")
	if err := os.MkdirAll(filepath.Join(work, "components"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeFixtureFile(t, filepath.Join(work, "index.yml"), `name: testmod
exports:
  default: components/install.yml
`)
	writeFixtureFile(t, filepath.Join(work, "components", "install.yml"), `name: install
props:
  message: { type: string, required: true }
steps:
  - name: greet
    log: "{{ props.message }}"
`)
	runGitCmd(t, work, "init", "-q")
	runGitCmd(t, work, "config", "user.email", "test@example.com")
	runGitCmd(t, work, "config", "user.name", "Test")
	runGitCmd(t, work, "add", ".")
	runGitCmd(t, work, "commit", "-q", "-m", "init")
	runGitCmd(t, work, "tag", tag)
	bare := filepath.Join(dir, "bare.git")
	runGitCmd(t, "", "clone", "--bare", "-q", work, bare)
	return bare
}

func runGitCmd(t *testing.T, dir string, args ...string) {
	t.Helper()
	// Disable hooks so the parent project's pre-commit (which assumes a
	// Taskfile-rooted checkout) does not fire inside fixture repos.
	full := append([]string{"-c", "core.hooksPath=/dev/null"}, args...)
	cmd := exec.Command("git", full...)
	if dir != "" {
		cmd.Dir = dir
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func writeFixtureFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestResolverDispatch_InlineRemote verifies the resolve + expand chain the
// handler uses for `use: github.com/owner/repo@vX`. The bare git repo + cache
// root keep the test hermetic.
func TestResolverDispatch_InlineRemote(t *testing.T) {
	bare := makeFixtureModule(t, "v1.0.0")
	resolver := &modules.Resolver{
		Fetcher: &modules.Fetcher{
			Root:     t.TempDir(),
			CloneURL: func(_ modules.Reference) string { return "file://" + bare },
		},
	}
	invocation := &config.PresetInvocation{
		Name: "github.com/owner/testmod@v1.0.0",
		With: map[string]interface{}{"message": "hello"},
	}
	if invocation.Kind() != config.ComponentRefRemote {
		t.Fatalf("Kind = %v, want Remote", invocation.Kind())
	}
	resolved, err := resolver.Resolve(context.Background(), invocation.Name)
	if err != nil {
		t.Fatalf("resolver.Resolve: %v", err)
	}
	steps, ns, _, err := presets.ExpandPresetFromPath(invocation, resolved.ComponentPath)
	if err != nil {
		t.Fatalf("ExpandPresetFromPath: %v", err)
	}
	if len(steps) != 1 {
		t.Errorf("expected 1 step, got %d", len(steps))
	}
	if ns["props"].(map[string]interface{})["message"] != "hello" {
		t.Errorf("props.message = %v", ns["props"])
	}
}

// TestResolverDispatch_Alias verifies that an alias resolves via the playbook
// modules: map.
func TestResolverDispatch_Alias(t *testing.T) {
	bare := makeFixtureModule(t, "v1.0.0")
	resolver := &modules.Resolver{
		Fetcher: &modules.Fetcher{
			Root:     t.TempDir(),
			CloneURL: func(_ modules.Reference) string { return "file://" + bare },
		},
		Modules: map[string]string{"testmod": "github.com/owner/testmod@v1.0.0"},
	}
	invocation := &config.PresetInvocation{
		Name: "testmod",
		With: map[string]interface{}{"message": "via-alias"},
	}
	resolved, err := resolver.Resolve(context.Background(), invocation.Name)
	if err != nil {
		t.Fatalf("resolver.Resolve: %v", err)
	}
	if !strings.HasSuffix(resolved.ComponentPath, "/components/install.yml") {
		t.Errorf("ComponentPath = %q", resolved.ComponentPath)
	}
	_, ns, _, err := presets.ExpandPresetFromPath(invocation, resolved.ComponentPath)
	if err != nil {
		t.Fatalf("ExpandPresetFromPath: %v", err)
	}
	if ns["props"].(map[string]interface{})["message"] != "via-alias" {
		t.Errorf("props.message wrong: %v", ns["props"])
	}
}
