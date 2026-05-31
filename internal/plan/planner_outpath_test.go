package plan

import (
	"os"
	"path/filepath"
	"testing"
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
