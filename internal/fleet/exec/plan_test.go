package exec

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

func TestSynthesize_RequiresCmd(t *testing.T) {
	if _, err := Synthesize(SynthOptions{}); err == nil {
		t.Fatal("expected error when Cmd is empty")
	}
}

// loadAsPlan writes data to a temp file and runs it through the kernel's
// plan reader, returning the parsed steps. Round-tripping the synthesized
// bytes through the real loader is the contract that protects callers
// from drift between this helper and the runtime — if the kernel adds a
// required field on Step, this test fails loudly rather than producing a
// silently-invalid scratch plan at runtime.
func loadAsPlan(t *testing.T, data []byte) []config.Step {
	t.Helper()
	dir := t.TempDir()
	p := filepath.Join(dir, "exec.yml")
	if err := os.WriteFile(p, data, 0600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}
	cfg, err := config.ReadConfig(p)
	if err != nil {
		t.Fatalf("ReadConfig(%q):\n%s\n---\nerror: %v", p, string(data), err)
	}
	return cfg.Steps
}

func TestSynthesize_MinimalRoundTrip(t *testing.T) {
	data, err := Synthesize(SynthOptions{Cmd: "echo hi"})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	steps := loadAsPlan(t, data)
	if len(steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(steps))
	}
	if steps[0].Shell == nil || steps[0].Shell.Cmd != "echo hi" {
		t.Errorf("unexpected step shape: %+v", steps[0])
	}
	if steps[0].Name != "fleet-exec" {
		t.Errorf("Name = %q, want fleet-exec", steps[0].Name)
	}
}

func TestSynthesize_AllFlagsRoundTrip(t *testing.T) {
	opts := SynthOptions{
		Cmd:         "uname -a",
		Interpreter: "zsh",
		Env:         map[string]string{"FOO": "bar", "BAZ": "qux"},
		Cwd:         "/tmp",
		Timeout:     "30s",
		Become:      true,
	}
	data, err := Synthesize(opts)
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	steps := loadAsPlan(t, data)
	if len(steps) != 1 {
		t.Fatalf("len(steps)=%d", len(steps))
	}
	s := steps[0]
	if s.Shell == nil || s.Shell.Cmd != opts.Cmd {
		t.Errorf("Shell.Cmd = %v", s.Shell)
	}
	if s.Shell.Interpreter != "zsh" {
		t.Errorf("Interpreter = %q", s.Shell.Interpreter)
	}
	if s.Env["FOO"] != "bar" || s.Env["BAZ"] != "qux" {
		t.Errorf("Env = %v", s.Env)
	}
	if s.Cwd != "/tmp" {
		t.Errorf("Cwd = %q", s.Cwd)
	}
	if s.Timeout != "30s" {
		t.Errorf("Timeout = %q", s.Timeout)
	}
	if s.AsUser != "root" {
		t.Errorf("AsUser = %q (Become should map to AsUser=root)", s.AsUser)
	}
}

func TestSynthesize_BecomeOffLeavesAsUserEmpty(t *testing.T) {
	data, err := Synthesize(SynthOptions{Cmd: "ls"})
	if err != nil {
		t.Fatalf("Synthesize: %v", err)
	}
	steps := loadAsPlan(t, data)
	if steps[0].AsUser != "" {
		t.Errorf("AsUser should be empty when Become=false, got %q", steps[0].AsUser)
	}
}

func TestSynthesize_PreservesCmdQuoting(t *testing.T) {
	// Operator quoting must pass through to the peer's shell unchanged.
	// We do NOT split or normalize — the spec is explicit that exec is
	// the "I want shell" entry point.
	cases := []string{
		`echo 'hello world'`,
		`echo "double quotes"`,
		`printf '%s\n' a b c`,
		`grep -r '$FOO' /etc`,
		`ls -la | wc -l`,
	}
	for _, cmd := range cases {
		data, err := Synthesize(SynthOptions{Cmd: cmd})
		if err != nil {
			t.Fatalf("Synthesize(%q): %v", cmd, err)
		}
		steps := loadAsPlan(t, data)
		if steps[0].Shell.Cmd != cmd {
			t.Errorf("cmd %q round-tripped to %q", cmd, steps[0].Shell.Cmd)
		}
	}
}

// Synthesized YAML is internal scratch — Step has dozens of action
// fields without omitempty, so the marshalled output is verbose. The
// shape doesn't matter as long as it round-trips through ReadConfig;
// the other tests in this file cover that.
