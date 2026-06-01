package apply_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/apply"
	"github.com/alehatsman/mooncake/internal/plan"
	_ "github.com/alehatsman/mooncake/internal/register" // register handlers
)

// TestComponentProps_CwdResolvesAtExecuteTime is the end-to-end regression
// for #49: a `use:`d component's execute-time fields (here `cwd:`) referenced
// `{{ props.* }}`, but the planner dropped the prop namespace after plan-time
// expansion, so cwd rendered empty and the command ran in the invocation dir
// instead of the prop dir. The executor now restores Step.ComponentProps into
// scope per step, so cwd resolves to the prop value.
func TestComponentProps_CwdResolvesAtExecuteTime(t *testing.T) {
	tmp := t.TempDir()
	// Distinct dirs so a fallback to the invocation dir is unambiguous.
	propDir := filepath.Join(tmp, "sub")
	stamp := filepath.Join(propDir, "where.txt")
	if err := os.MkdirAll(propDir, 0o755); err != nil {
		t.Fatalf("mkdir propDir: %v", err)
	}

	compPath := filepath.Join(tmp, "comp.yml")
	comp := `props:
  dir:
    type: string
    default: "."
steps:
  - name: write cwd marker
    shell: pwd > where.txt
    cwd: "{{ props.dir }}"
`
	if err := os.WriteFile(compPath, []byte(comp), 0o644); err != nil {
		t.Fatalf("write comp: %v", err)
	}

	cfgPath := filepath.Join(tmp, "consumer.yml")
	consumer := `steps:
  - name: use comp
    use: ./comp.yml
    props:
      dir: ` + propDir + `
`
	if err := os.WriteFile(cfgPath, []byte(consumer), 0o644); err != nil {
		t.Fatalf("write consumer: %v", err)
	}

	planner, err := plan.NewPlanner()
	if err != nil {
		t.Fatalf("NewPlanner: %v", err)
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{ConfigPath: cfgPath})
	if err != nil {
		t.Fatalf("BuildPlan: %v", err)
	}

	// The component step must carry its props into the plan so the executor
	// can restore them — this is the mechanism under test.
	var found bool
	for _, s := range planData.Steps {
		if s.Cwd == "{{ props.dir }}" {
			found = true
			if s.ComponentProps == nil || s.ComponentProps["dir"] != propDir {
				t.Fatalf("planned step ComponentProps = %v, want dir=%q", s.ComponentProps, propDir)
			}
		}
	}
	if !found {
		t.Fatal("did not find the component's cwd step in the plan")
	}

	opts := apply.InMemoryPlanOptions{LogLevel: "error", RootFile: cfgPath}
	if _, err := apply.NewRunnerFromInMemoryPlan(planData, opts).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}

	// The shell wrote `pwd` into where.txt under propDir. If cwd had fallen
	// back to the invocation dir (the bug), the file would not be in propDir
	// and its contents would not be propDir.
	if _, err := os.ReadFile(stamp); err != nil {
		t.Fatalf("read cwd marker (cwd did not resolve to prop dir): %v", err)
	}
	// pwd may resolve symlinks (e.g. /tmp → /private/tmp on macOS); compare by
	// resolved path so the assertion is portable.
	wantResolved, _ := filepath.EvalSymlinks(propDir)
	gotResolved, _ := filepath.EvalSymlinks(filepath.Dir(stamp))
	if gotResolved != wantResolved {
		t.Fatalf("marker dir = %q, want %q", gotResolved, wantResolved)
	}
}
