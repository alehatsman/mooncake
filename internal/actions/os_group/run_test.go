//nolint:revive // package name follows action convention
package os_group

import (
	"runtime"
	"strings"
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

// stubLookup replaces the package-level group reader with a fake table
// keyed by name, and restores it on test teardown.
func stubLookup(t *testing.T, table map[string]*groupState) {
	t.Helper()
	orig := lookupGroup
	lookupGroup = func(name string) (*groupState, error) {
		if s, ok := table[name]; ok {
			return s, nil
		}
		return &groupState{exists: false}, nil
	}
	t.Cleanup(func() { lookupGroup = orig })
}

func runStep(t *testing.T, plan bool, step *config.Step) (*executor.Result, error) {
	t.Helper()
	res, err := (&Handler{}).Run(newCtx(t, plan), step)
	if err != nil {
		return nil, err
	}
	return res.(*executor.Result), nil
}

func intPtr(i int) *int { return &i }

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
		{"no name", &config.Step{OsGroup: &config.OsGroup{}}, true},
		{"bad state", &config.Step{OsGroup: &config.OsGroup{Name: "x", State: "queued"}}, true},
		{"ok present", &config.Step{OsGroup: &config.OsGroup{Name: "deploy"}}, false},
		{"ok absent", &config.Step{OsGroup: &config.OsGroup{Name: "old", State: "absent"}}, false},
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

func TestPlan_NewGroup(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "deploy", GID: intPtr(1500)}}
	r, err := runStep(t, true, step)
	if err != nil {
		t.Fatal(err)
	}
	if !r.WouldChange {
		t.Errorf("expected WouldChange on missing group; reason=%q", r.Reason)
	}
}

func TestPlan_ExistingNoop(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{
		"deploy": {exists: true, gid: 1500},
	})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "deploy", GID: intPtr(1500)}}
	r, err := runStep(t, true, step)
	if err != nil {
		t.Fatal(err)
	}
	if r.WouldChange || r.Changed {
		t.Errorf("expected no change; reason=%q", r.Reason)
	}
}

func TestPlan_GIDDriftRejected(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{
		"deploy": {exists: true, gid: 1500},
	})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "deploy", GID: intPtr(1600)}}
	_, err := runStep(t, true, step)
	if err == nil {
		t.Fatal("expected error on GID renumbering")
	}
	if !strings.Contains(err.Error(), "renumber") {
		t.Errorf("expected error to mention renumber; got %v", err)
	}
}

func TestPlan_AbsentNoopWhenMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "old", State: "absent"}}
	r, err := runStep(t, true, step)
	if err != nil {
		t.Fatal(err)
	}
	if r.WouldChange || r.Changed {
		t.Errorf("expected noop on absent+missing; reason=%q", r.Reason)
	}
}

func TestPlan_AbsentRemovesExisting(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{
		"old": {exists: true, gid: 1700},
	})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "old", State: "absent"}}
	r, err := runStep(t, true, step)
	if err != nil {
		t.Fatal(err)
	}
	if !r.WouldChange {
		t.Errorf("expected WouldChange for removal; reason=%q", r.Reason)
	}
}

func TestPlan_AbsentRejectedWhenMembers(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{
		"deploy": {exists: true, gid: 1500, members: []string{"alice", "bob"}},
	})
	step := &config.Step{OsGroup: &config.OsGroup{Name: "deploy", State: "absent"}}
	_, err := runStep(t, true, step)
	if err == nil {
		t.Fatal("expected error on removing group with members")
	}
	if !strings.Contains(err.Error(), "members") {
		t.Errorf("expected error to mention members; got %v", err)
	}
}

func TestPlan_TemplatedName(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	stubLookup(t, map[string]*groupState{
		"deploy": {exists: true, gid: 1500},
	})
	ctx := newCtx(t, true)
	ctx.Scope.User["group_name"] = "deploy"
	step := &config.Step{OsGroup: &config.OsGroup{Name: "{{ group_name }}", GID: intPtr(1500)}}
	res, err := (&Handler{}).Run(ctx, step)
	if err != nil {
		t.Fatal(err)
	}
	r := res.(*executor.Result)
	if r.WouldChange || r.Changed {
		t.Errorf("templated name should resolve to existing group; reason=%q", r.Reason)
	}
}

func TestCreate_Args(t *testing.T) {
	plan, err := planPresent(&groupState{exists: false}, desired{
		name:   "build",
		gid:    intPtr(2000),
		system: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"--system", "--gid", "2000", "build"}
	if !equalSlices(plan.createArgs, want) {
		t.Errorf("createArgs mismatch\n got %v\nwant %v", plan.createArgs, want)
	}
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
