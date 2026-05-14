//nolint:revive // package name follows action convention
package pkg_list

import (
	"runtime"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

func newCtx(t *testing.T, plan bool) *executor.ExecutionContext {
	t.Helper()
	r, err := template.NewPongo2Renderer()
	if err != nil {
		t.Fatal(err)
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     planMode(plan),
			Stats:    executor.NewExecutionStats(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: "/tmp",
	}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

func stubDpkgQuery(t *testing.T, stdout string) {
	t.Helper()
	orig := dpkgQuery
	origLook := lookPath
	dpkgQuery = func() (string, error) { return stdout, nil }
	lookPath = func(name string) (string, error) { return "/usr/bin/" + name, nil }
	t.Cleanup(func() {
		dpkgQuery = orig
		lookPath = origLook
	})
}

func mustRun(t *testing.T, plan bool, step *config.Step) *executor.Result {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return res.(*executor.Result)
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"empty ok", &config.Step{PkgList: &config.PkgList{}}, false},
		{"explicit manager ok", &config.Step{PkgList: &config.PkgList{Manager: "apt"}}, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := (&Handler{}).Validate(c.step)
			if (err != nil) != c.wantErr {
				t.Errorf("err=%v wantErr=%v", err, c.wantErr)
			}
		})
	}
}

const sampleDpkg = "zlib1g\t1:1.2.13-1\n" +
	"curl\t7.88.1-10\n" +
	"nginx\t1.24.0-2\n" +
	"\n" + // blank line ignored
	"malformed-no-tab\n" + // skipped
	"libc6\t2.36-9\n"

func TestApply_ReturnsSortedPackages(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, sampleDpkg)
	step := &config.Step{PkgList: &config.PkgList{}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("pkg.list must never report Changed")
	}
	pkgs, ok := r.Data["packages"].([]map[string]interface{})
	if !ok {
		t.Fatalf("expected packages []map[string]interface{}; got %T", r.Data["packages"])
	}
	wantNames := []string{"curl", "libc6", "nginx", "zlib1g"}
	if len(pkgs) != len(wantNames) {
		t.Fatalf("expected %d packages; got %d (%v)", len(wantNames), len(pkgs), pkgs)
	}
	for i, want := range wantNames {
		got := pkgs[i]["name"].(string)
		if got != want {
			t.Errorf("packages[%d].name = %q; want %q", i, got, want)
		}
		if v, ok := pkgs[i]["version"].(string); !ok || v == "" {
			t.Errorf("packages[%d].version missing", i)
		}
		if pkgs[i]["manager"] != "apt" {
			t.Errorf("packages[%d].manager = %v; want apt", i, pkgs[i]["manager"])
		}
	}
	if c, ok := r.Data["count"].(int); !ok || c != len(wantNames) {
		t.Errorf("count mismatch: %v", r.Data["count"])
	}
}

func TestPlanAndApply_AreIdentical(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, sampleDpkg)
	step := &config.Step{PkgList: &config.PkgList{}}
	applied := mustRun(t, false, step)
	planned := mustRun(t, true, step)
	if applied.Changed || planned.Changed {
		t.Errorf("Changed must be false in both modes")
	}
	if planned.WouldChange {
		t.Errorf("pkg.list must not flip WouldChange (query is read-only)")
	}
	ap := applied.Data["packages"].([]map[string]interface{})
	pp := planned.Data["packages"].([]map[string]interface{})
	if len(ap) != len(pp) {
		t.Fatalf("plan/apply differ in length: %d vs %d", len(ap), len(pp))
	}
	for i := range ap {
		if ap[i]["name"] != pp[i]["name"] || ap[i]["version"] != pp[i]["version"] {
			t.Errorf("plan/apply differ at %d: %v vs %v", i, ap[i], pp[i])
		}
	}
}

func TestApply_ExplicitManagerApt(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, "foo\t1.0\n")
	step := &config.Step{PkgList: &config.PkgList{Manager: "apt"}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("must not report Changed")
	}
	pkgs := r.Data["packages"].([]map[string]interface{})
	if len(pkgs) != 1 || pkgs[0]["name"] != "foo" {
		t.Errorf("expected [foo]; got %v", pkgs)
	}
}

func TestRun_OnlyAptSupported(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubDpkgQuery(t, "")
	step := &config.Step{PkgList: &config.PkgList{Manager: "dnf"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-apt manager")
	}
}
