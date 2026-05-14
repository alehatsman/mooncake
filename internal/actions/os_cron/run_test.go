//nolint:revive // package name follows action convention
package os_cron

import (
	"os"
	"path/filepath"
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

// stubFS redirects /etc/cron.d to a tempdir and restores the original
// path on cleanup.
func stubFS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := cronPaths
	cronPaths.dir = dir
	t.Cleanup(func() { cronPaths = original })
	return dir
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
		{"no name", &config.Step{OsCron: &config.OsCron{Command: "/bin/true"}}, true},
		{"bad name", &config.Step{OsCron: &config.OsCron{Name: "bad name", Command: "/bin/true"}}, true},
		{"bad state", &config.Step{OsCron: &config.OsCron{Name: "x", State: "queued", Command: "/bin/true"}}, true},
		{"present no command", &config.Step{OsCron: &config.OsCron{Name: "x"}}, true},
		{"schedule and minute", &config.Step{OsCron: &config.OsCron{
			Name: "x", Schedule: "*/5 * * * *", Minute: "0", Command: "/bin/true",
		}}, true},
		{"ok minimal present", &config.Step{OsCron: &config.OsCron{
			Name: "backup", Minute: "0", Hour: "3", Command: "/opt/backup.sh",
		}}, false},
		{"ok schedule shorthand", &config.Step{OsCron: &config.OsCron{
			Name: "gc", Schedule: "*/15 * * * *", Command: "/opt/gc.sh",
		}}, false},
		{"ok absent without command", &config.Step{OsCron: &config.OsCron{
			Name: "old", State: "absent",
		}}, false},
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

func TestApply_CreatesCronFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "backup-nightly",
		User:    "deploy",
		Minute:  "0",
		Hour:    "3",
		Command: "/opt/backup/run.sh",
		Env:     map[string]string{"MAILTO": "ops@example.com"},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	got, err := os.ReadFile(filepath.Join(dir, "backup-nightly"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := strings.Join([]string{
		"# Managed by mooncake os.cron",
		"MAILTO=ops@example.com",
		"0 3 * * * deploy /opt/backup/run.sh",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("content mismatch\n got %q\nwant %q", got, want)
	}
}

func TestApply_DefaultUserRootAndStarFields(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "tick",
		Hour:    "3",
		Command: "/usr/bin/touch /tmp/tick",
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(dir, "tick"))
	want := "# Managed by mooncake os.cron\n* 3 * * * root /usr/bin/touch /tmp/tick\n"
	if string(got) != want {
		t.Errorf("content mismatch\n got %q\nwant %q", got, want)
	}
}

func TestApply_ScheduleShorthand(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:     "gc",
		Schedule: "*/15 * * * *",
		Command:  "./gc.sh",
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(dir, "gc"))
	want := "# Managed by mooncake os.cron\n*/15 * * * * root ./gc.sh\n"
	if string(got) != want {
		t.Errorf("content mismatch\n got %q\nwant %q", got, want)
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "backup",
		Minute:  "0",
		Hour:    "3",
		Command: "/opt/backup.sh",
	}}
	_ = mustRun(t, false, step)
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
}

func TestApply_DriftTriggersUpdate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "backup",
		Minute:  "0",
		Hour:    "3",
		Command: "/opt/backup.sh",
	}}
	_ = mustRun(t, false, step)
	// Drift: change the schedule.
	step.OsCron.Hour = "4"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected drift to trigger change; reason=%q", r.Reason)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "backup"))
	if !strings.Contains(string(got), "0 4 * * *") {
		t.Errorf("schedule not updated: %q", got)
	}
}

func TestAbsent_RemovesFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "old",
		Minute:  "0",
		Command: "/opt/old.sh",
	}}
	_ = mustRun(t, false, step)
	step.OsCron.State = "absent"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected delete; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, "old")); !os.IsNotExist(err) {
		t.Errorf("file should be removed; stat err=%v", err)
	}
}

func TestAbsent_NoopWhenMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:  "never",
		State: "absent",
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent on missing file should be no-op; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWrite(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "backup",
		Minute:  "0",
		Hour:    "3",
		Command: "/opt/backup.sh",
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, "backup")); err == nil {
		t.Error("plan must not write file")
	}
}

func TestEnvOrderingIsStable(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	step := &config.Step{OsCron: &config.OsCron{
		Name:    "stable",
		Minute:  "0",
		Command: "/bin/true",
		Env: map[string]string{
			"PATH":   "/usr/bin",
			"MAILTO": "ops@example.com",
			"HOME":   "/root",
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(dir, "stable"))
	want := strings.Join([]string{
		"# Managed by mooncake os.cron",
		"HOME=/root",
		"MAILTO=ops@example.com",
		"PATH=/usr/bin",
		"0 * * * * root /bin/true",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("env not sorted/stable\n got %q\nwant %q", got, want)
	}
}
