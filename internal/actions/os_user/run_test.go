//nolint:revive // package name follows action convention
package os_user

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

// stubLookup swaps the package-level lookupUser hook to return a
// pre-built userState, then restores the original at test cleanup.
func stubLookup(t *testing.T, state *userState) {
	t.Helper()
	original := lookupUser
	lookupUser = func(string) (*userState, error) { return state, nil }
	t.Cleanup(func() { lookupUser = original })
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func TestValidate(t *testing.T) {
	intp := func(i int) *int { return &i }
	cases := []struct {
		name    string
		step    *config.Step
		wantErr bool
	}{
		{"nil", &config.Step{}, true},
		{"no name", &config.Step{OsUser: &config.OsUser{}}, true},
		{"bad state", &config.Step{OsUser: &config.OsUser{Name: "x", State: "queued"}}, true},
		{"gid + group", &config.Step{OsUser: &config.OsUser{Name: "x", GID: intp(1500), Group: "g"}}, true},
		{"ok present", &config.Step{OsUser: &config.OsUser{Name: "x"}}, false},
		{"ok absent", &config.Step{OsUser: &config.OsUser{Name: "x", State: "absent"}}, false},
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

// The following plan tests use stubLookup so they work on all supported platforms.

func TestPlan_CreateWhenAbsent(t *testing.T) {
	stubLookup(t, &userState{exists: false})
	intp := func(i int) *int { return &i }
	step := &config.Step{OsUser: &config.OsUser{
		Name:   "deploy",
		UID:    intp(1500),
		Shell:  "/bin/bash",
		Home:   "/home/deploy",
		Groups: []string{"sudo", "docker"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatal(err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange || r.Data["operation"] != "create" {
		t.Errorf("expected create plan; reason=%q op=%v", r.Reason, r.Data["operation"])
	}
}

func TestPlan_NoopWhenAlreadyDesired(t *testing.T) {
	stubLookup(t, &userState{
		exists: true, uid: 1500, gid: 1500,
		shell:   "/bin/bash",
		home:    "/home/deploy",
		comment: "Deploy",
		groups:  []string{"docker", "sudo"},
	})
	intp := func(i int) *int { return &i }
	step := &config.Step{OsUser: &config.OsUser{
		Name:    "deploy",
		UID:     intp(1500),
		Shell:   "/bin/bash",
		Home:    "/home/deploy",
		Comment: "Deploy",
		Groups:  []string{"sudo", "docker"},
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("expected noop; reason=%q", r.Reason)
	}
}

func TestPlan_ModifyFieldsThatDrifted(t *testing.T) {
	stubLookup(t, &userState{
		exists: true, uid: 1500, gid: 1500,
		shell:  "/bin/sh",
		home:   "/home/deploy",
		groups: []string{"sudo"},
	})
	step := &config.Step{OsUser: &config.OsUser{
		Name:   "deploy",
		Shell:  "/bin/bash",
		Groups: []string{"sudo", "docker"},
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Fatalf("expected modify plan; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "shell") {
		t.Errorf("reason should mention shell drift; got %q", r.Reason)
	}
	if !strings.Contains(r.Reason, "docker") {
		t.Errorf("reason should mention docker group; got %q", r.Reason)
	}
}

func TestPlan_GroupReplaceWhenAppendFalse(t *testing.T) {
	stubLookup(t, &userState{
		exists: true, uid: 1500, gid: 1500,
		shell:  "/bin/bash",
		home:   "/home/deploy",
		groups: []string{"sudo", "docker", "extra"},
	})
	noAppend := false
	step := &config.Step{OsUser: &config.OsUser{
		Name:         "deploy",
		Groups:       []string{"sudo"},
		AppendGroups: &noAppend,
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("expected change when replacing group set; reason=%q", r.Reason)
	}
}

func TestPlan_AbsentNoopWhenMissing(t *testing.T) {
	stubLookup(t, &userState{exists: false})
	step := &config.Step{OsUser: &config.OsUser{Name: "deploy", State: "absent"}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("absent on missing user should be noop; reason=%q", r.Reason)
	}
}

func TestPlan_AbsentRemovesWhenPresent(t *testing.T) {
	stubLookup(t, &userState{exists: true, uid: 1500, gid: 1500, home: "/home/deploy", shell: "/bin/bash"})
	step := &config.Step{OsUser: &config.OsUser{Name: "deploy", State: "absent", RemoveHome: true}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange || r.Data["operation"] != "remove" {
		t.Errorf("expected remove plan; reason=%q op=%v", r.Reason, r.Data["operation"])
	}
}

func TestComputePlan_AppendOnlyAddsMissingGroups(t *testing.T) {
	current := &userState{
		exists: true, uid: 1500, gid: 1500,
		groups: []string{"sudo"},
	}
	plan := computePlan(current, desired{
		state:        statePresent,
		name:         "deploy",
		groups:       []string{"sudo", "docker"},
		appendGroups: true,
	})
	if !plan.changed {
		t.Fatal("expected change")
	}
	joined := strings.Join(plan.modifyArgs, " ")
	if !strings.Contains(joined, "--append") {
		t.Errorf("expected --append in args: %s", joined)
	}
	if !strings.Contains(joined, "sudo,docker") {
		t.Errorf("expected groups arg in modify args: %s", joined)
	}
}

func TestUnsupportedOS_ReturnsClearError(t *testing.T) {
	if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
		t.Skip("os.user is supported on this platform; skip the unsupported-OS test")
	}
	step := &config.Step{OsUser: &config.OsUser{Name: "x"}}
	_, err := (&Handler{}).Run(newCtx(t, true), step)
	if err == nil {
		t.Fatal("expected unsupported-os error")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) {
		t.Errorf("error should mention GOOS; got %v", err)
	}
}
