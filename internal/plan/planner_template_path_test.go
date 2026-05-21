package plan

// Regression: walkAndRender pre-joined `plan:"path"` fields with
// currentDir when the rendered string did not start with `/` — but
// RenderPreserving keeps apply-time references like `{{ env.HOME }}`
// as literals at plan time, so a path of `{{ env.HOME }}/foo` rendered
// to `{{ env.HOME }}/foo`, was not-absolute by filepath.IsAbs (the
// literal `{{` masks the eventual leading slash), and got rewritten
// to `<currentDir>/{{ env.HOME }}/foo`. Apply-time Render then
// resolved `env.HOME` → `/home/aleh`, giving
// `<currentDir>//home/aleh/foo` → `<currentDir>/home/aleh/foo`, and
// file.write happily created the file under the playbook directory.
//
// Fix: defer the join when an apply-time template is still present.
// Apply-time ExpandPath already handles absoluteness correctly once
// the template resolves.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPlanner_DefersJoinForApplyTimeTemplateInPath(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	body := `version: "1.0"
steps:
  - name: write under env.HOME
    file.write:
      path: "{{ env.HOME }}/foo.txt"
      content: ok
`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	step := plan.Steps[0]
	if step.FileWrite == nil {
		t.Fatal("FileWrite is nil; planner dropped the action")
	}
	got := step.FileWrite.Path
	want := "{{ env.HOME }}/foo.txt"
	if got != want {
		t.Errorf("FileWrite.Path = %q, want %q\n"+
			"pre-fix bug: planner pre-joined currentDir onto a path whose\n"+
			"leading-slash intent was masked by an unresolved template.",
			got, want)
	}
	// Belt-and-braces: the planner-emitted path must NOT contain the
	// playbook's temp dir. That would mean the join still happened.
	if strings.Contains(got, tmpDir) {
		t.Errorf("FileWrite.Path leaked currentDir %q into rendered value %q", tmpDir, got)
	}
}

// Plain relative paths (no template) must still be joined with
// currentDir — the deferral only kicks in when an apply-time
// template is still present.
func TestPlanner_RelativePathStillJoinedWhenNoTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yml")

	body := `version: "1.0"
steps:
  - name: write relative
    file.write:
      path: "foo.txt"
      content: ok
`
	if err := os.WriteFile(configPath, []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	planner, err := NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	plan, err := planner.BuildPlan(PlannerConfig{ConfigPath: configPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}
	if len(plan.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(plan.Steps))
	}
	step := plan.Steps[0]
	want := filepath.Join(tmpDir, "foo.txt")
	if step.FileWrite.Path != want {
		t.Errorf("FileWrite.Path = %q, want %q (plain relative paths must still be joined)",
			step.FileWrite.Path, want)
	}
}

// Direct unit test of the helper so each branch is documented and
// the call-sites can rely on a single decision predicate.
func TestShouldJoinPlanPath(t *testing.T) {
	cases := []struct {
		in   string
		want bool
		why  string
	}{
		{"/absolute/path", false, "absolute path — already resolved"},
		{"~/foo", false, "tilde — apply-time home expansion"},
		{"~", false, "bare tilde — apply-time home expansion"},
		{"{{ env.HOME }}/foo", false, "apply-time template — defer to ExpandPath"},
		{"prefix-{{ var }}-suffix", false, "template anywhere defers"},
		{"foo/bar", true, "plain relative — needs join"},
		{"", true, "empty — caller must already have skipped"},
	}
	for _, c := range cases {
		if got := shouldJoinPlanPath(c.in); got != c.want {
			t.Errorf("shouldJoinPlanPath(%q) = %v, want %v (%s)", c.in, got, c.want, c.why)
		}
	}
}
