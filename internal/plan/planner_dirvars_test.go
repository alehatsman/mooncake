package plan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// component_dir / invocation_dir are the two built-in directory vars (M1).
//   - component_dir: the dir of the file/component that DECLARES a step
//     (the module-cache dir for a `use:`d component). Resolved per step.
//   - invocation_dir: the process cwd where mooncake was launched. Global.
//
// These let a shared component reference its OWN bundled assets — e.g.
// `bash {{ component_dir }}/scripts/ai-lint.sh` — while shell steps still
// run in the consumer's cwd.

// TestComponentDir_UseComponentSubdir verifies a `use:`d local component's
// steps resolve {{ component_dir }} to the COMPONENT's dir, not the consumer
// dir that referenced it.
func TestComponentDir_UseComponentSubdir(t *testing.T) {
	tmp := t.TempDir()
	compDir := filepath.Join(tmp, "comp")
	if err := os.MkdirAll(compDir, 0755); err != nil {
		t.Fatalf("mkdir comp: %v", err)
	}
	writeComponent(t, compDir, "gate.yml", `
steps:
  - name: run gate
    shell:
      cmd: "bash {{ component_dir }}/scripts/x.sh"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}

	step := config.Step{
		Name: "use gate",
		Use:  "./comp/gate.yml",
	}
	// CurrentDir is the consumer dir (tmp), deliberately != the component dir.
	ctx := &ExpansionContext{
		Variables:  map[string]interface{}{},
		CurrentDir: tmp,
	}
	plan := &Plan{}

	expanded, err := planner.tryExpandUse(step, ctx, plan, 0)
	if err != nil {
		t.Fatalf("tryExpandUse: %v", err)
	}
	if !expanded {
		t.Fatal("expected expanded=true for concrete local component")
	}
	if len(plan.Steps) != 1 || plan.Steps[0].Shell == nil {
		t.Fatalf("expected 1 shell step, got %+v", plan.Steps)
	}

	wantSubstr := filepath.Join(compDir, "scripts/x.sh")
	got := plan.Steps[0].Shell.Cmd
	if !strings.Contains(got, wantSubstr) {
		t.Errorf("component_dir resolved wrong:\n got cmd: %q\n want substring: %q", got, wantSubstr)
	}
	if strings.Contains(got, "{{") {
		t.Errorf("component_dir left unresolved in: %q", got)
	}
}

// TestComponentDir_LocalInclude verifies an imported file's steps resolve
// {{ component_dir }} to the INCLUDED file's dir (each include flips the
// per-step component_dir), while the root file's steps resolve it to the
// root config dir.
func TestComponentDir_LocalInclude(t *testing.T) {
	tmp := t.TempDir()
	subDir := filepath.Join(tmp, "sub")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("mkdir sub: %v", err)
	}
	writeComponent(t, subDir, "included.yml", `
steps:
  - name: included step
    shell:
      cmd: "cat {{ component_dir }}/data.txt"
`)
	rootPath := writeComponent(t, tmp, "root.yml", `
steps:
  - name: root step
    shell:
      cmd: "cat {{ component_dir }}/root.txt"
  - import: ./sub/included.yml
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	p, err := planner.BuildPlan(PlannerConfig{ConfigPath: rootPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	var rootCmd, includedCmd string
	for _, s := range p.Steps {
		if s.Shell == nil {
			continue
		}
		switch s.Name {
		case "root step":
			rootCmd = s.Shell.Cmd
		case "included step":
			includedCmd = s.Shell.Cmd
		}
	}

	if want := filepath.Join(tmp, "root.txt"); !strings.Contains(rootCmd, want) {
		t.Errorf("root step component_dir = %q, want substring %q", rootCmd, want)
	}
	if want := filepath.Join(subDir, "data.txt"); !strings.Contains(includedCmd, want) {
		t.Errorf("included step component_dir = %q, want substring %q", includedCmd, want)
	}
}

// TestInvocationDir_IsCwd verifies invocation_dir is the process cwd (where
// shell steps run), constant across the run regardless of the config dir.
func TestInvocationDir_IsCwd(t *testing.T) {
	tmp := t.TempDir()
	rootPath := writeComponent(t, tmp, "root.yml", `
steps:
  - name: echo invocation
    shell:
      cmd: "echo {{ invocation_dir }}"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	p, err := planner.BuildPlan(PlannerConfig{ConfigPath: rootPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Getwd: %v", err)
	}
	if got, ok := p.InitialVars["invocation_dir"].(string); !ok || got != wd {
		t.Errorf("InitialVars[invocation_dir] = %v, want %q", p.InitialVars["invocation_dir"], wd)
	}

	var cmd string
	for _, s := range p.Steps {
		if s.Shell != nil && s.Name == "echo invocation" {
			cmd = s.Shell.Cmd
		}
	}
	if !strings.Contains(cmd, wd) {
		t.Errorf("invocation_dir resolved to %q, want substring %q", cmd, wd)
	}
}

// TestDirVars_StrictTemplatesAllow verifies the strict template pass treats
// component_dir / invocation_dir as defined even when they survive into the
// plan via an execute-time-only field like `when:`.
func TestDirVars_StrictTemplatesAllow(t *testing.T) {
	tmp := t.TempDir()
	rootPath := writeComponent(t, tmp, "root.yml", `
steps:
  - name: guarded
    when: "'{{ component_dir }}' != '' and '{{ invocation_dir }}' != ''"
    shell:
      cmd: "echo hi"
`)

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	p, err := planner.BuildPlan(PlannerConfig{ConfigPath: rootPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	for _, ref := range p.UnresolvedTemplates {
		if ref.Root == "component_dir" || ref.Root == "invocation_dir" {
			t.Errorf("strict pass flagged built-in dir var %q as unresolved", ref.Root)
		}
	}
}
