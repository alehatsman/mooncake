package plan

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/config"
)

// M2 (epic #93): a `use:`d component's output paths (plan:"outpath" — the
// file.copy/template dest, file.write path) resolve against the invocation
// dir, not the module-cache dir the component is fetched into. Source paths
// (plan:"path") stay origin-relative so the component still finds its own
// bundled assets. Top-level/include steps are unaffected (covered by
// TestPlanner_RelativePathStillJoinedWhenNoTemplate).
func TestPlanner_ComponentOutputPathResolvesAgainstInvocationDir(t *testing.T) {
	tmp := t.TempDir()
	compDir := filepath.Join(tmp, "comp")
	if err := os.MkdirAll(compDir, 0755); err != nil {
		t.Fatalf("mkdir comp: %v", err)
	}
	writeComponent(t, compDir, "writer.yml", `
steps:
  - name: render bundled asset
    file.template:
      src: ./asset.j2
      dest: ./out/result.txt
`)
	rootPath := writeComponent(t, tmp, "root.yml", `
steps:
  - use: ./comp/writer.yml
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

	var tmpl *struct{ Src, Dest string }
	for _, s := range p.Steps {
		if s.FileTemplate != nil {
			tmpl = &struct{ Src, Dest string }{s.FileTemplate.Src, s.FileTemplate.Dest}
		}
	}
	if tmpl == nil {
		t.Fatalf("no file.template step in plan: %+v", p.Steps)
	}

	// Src: origin-relative → the component dir (so the bundled asset is found).
	if want := filepath.Join(compDir, "asset.j2"); tmpl.Src != want {
		t.Errorf("Src = %q, want %q (component-relative)", tmpl.Src, want)
	}
	// Dest: invocation-relative → the cwd, NOT the module-cache/component dir.
	if want := filepath.Join(wd, "out/result.txt"); tmpl.Dest != want {
		t.Errorf("Dest = %q, want %q (invocation-relative)", tmpl.Dest, want)
	}
	if filepath.Dir(filepath.Dir(tmpl.Dest)) == compDir {
		t.Errorf("Dest leaked into the component/module-cache dir: %q", tmpl.Dest)
	}
}

// Issue #50: ExpandStepsWithContext is the apply-time path for remote/alias use: modules.
// It must mark the expansion as FromComponent=true so plan:"outpath" fields (e.g.
// file.copy dest with a subdir like "web/biome.json") resolve against invocation_dir,
// not the module-cache dir (presetBaseDir / CurrentDir).
func TestExpandStepsWithContext_OutpathResolvesAgainstInvocationDir(t *testing.T) {
	tmp := t.TempDir()

	// Simulate a module-cache dir (where the fetched component lives).
	moduleDir := filepath.Join(tmp, "cache", "modules", "example.com", "ts-quality")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("mkdir moduleDir: %v", err)
	}

	// The component declares file.copy with a relative dest containing a subdir.
	steps := []config.Step{
		{
			Name: "sync config",
			FileCopy: &config.Copy{
				Src:  filepath.Join(moduleDir, "biome.json"),
				Dest: "web/biome.base.json",
			},
		},
	}

	// invocation_dir simulates the consumer's repo root (cwd at apply time).
	invocationDir := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(invocationDir, 0755); err != nil {
		t.Fatalf("mkdir invocationDir: %v", err)
	}
	variables := map[string]interface{}{
		"invocation_dir": invocationDir,
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	expanded, err := planner.ExpandStepsWithContext(steps, variables, moduleDir)
	if err != nil {
		t.Fatalf("ExpandStepsWithContext: %v", err)
	}
	if len(expanded) != 1 || expanded[0].FileCopy == nil {
		t.Fatalf("unexpected expanded steps: %+v", expanded)
	}

	dest := expanded[0].FileCopy.Dest
	want := filepath.Join(invocationDir, "web/biome.base.json")
	if dest != want {
		t.Errorf("Dest = %q, want %q (invocation-relative)", dest, want)
	}
	if filepath.HasPrefix(dest, moduleDir) {
		t.Errorf("Dest leaked into module cache dir: %q", dest)
	}
}

// Issue #50 (secondary): copyContextWithLoopVars must propagate FromComponent so that
// for_each steps inside a component also resolve output paths against invocation_dir.
func TestExpandStepsWithContext_ForEachOutpathResolvesAgainstInvocationDir(t *testing.T) {
	tmp := t.TempDir()

	moduleDir := filepath.Join(tmp, "cache", "modules", "example.com", "ts-quality")
	if err := os.MkdirAll(moduleDir, 0755); err != nil {
		t.Fatalf("mkdir moduleDir: %v", err)
	}

	steps := []config.Step{
		{
			Name: "copy each config",
			FileCopy: &config.Copy{
				Src:  filepath.Join(moduleDir, "biome.json"),
				Dest: "web/{{ item }}.json",
			},
			ForEach: &config.ForEachField{Items: []interface{}{"biome.base", "biome.dev"}},
		},
	}

	invocationDir := filepath.Join(tmp, "myrepo")
	if err := os.MkdirAll(invocationDir, 0755); err != nil {
		t.Fatalf("mkdir invocationDir: %v", err)
	}
	variables := map[string]interface{}{
		"invocation_dir": invocationDir,
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	expanded, err := planner.ExpandStepsWithContext(steps, variables, moduleDir)
	if err != nil {
		t.Fatalf("ExpandStepsWithContext: %v", err)
	}
	if len(expanded) != 2 {
		t.Fatalf("want 2 expanded steps, got %d", len(expanded))
	}

	for _, s := range expanded {
		if s.FileCopy == nil {
			t.Fatalf("expected FileCopy step, got: %+v", s)
		}
		dest := s.FileCopy.Dest
		if !filepath.HasPrefix(dest, invocationDir) {
			t.Errorf("Dest = %q, want prefix %q (invocation-relative)", dest, invocationDir)
		}
		if filepath.HasPrefix(dest, moduleDir) {
			t.Errorf("Dest leaked into module cache dir: %q", dest)
		}
	}
}
