//nolint:revive // package name follows action convention
package os_systemd

import (
	"errors"
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

// stub captures the systemctl calls a test triggers and lets the test
// inject pre-canned enabled/active states. All package-level systemctl
// hooks are redirected to its methods for the duration of the test.
type stub struct {
	t              *testing.T
	enabled        map[string]bool
	active         map[string]bool
	enableErr      map[string]error
	activeErr      map[string]error
	calls          []string
	reloadErr      error
	enableActionErr map[string]error
	startActionErr  map[string]error
}

func newStub(t *testing.T) *stub {
	t.Helper()
	s := &stub{
		t:               t,
		enabled:         map[string]bool{},
		active:          map[string]bool{},
		enableErr:       map[string]error{},
		activeErr:       map[string]error{},
		enableActionErr: map[string]error{},
		startActionErr:  map[string]error{},
	}
	origDR, origIE, origE, origD, origIA, origST, origSP := systemctlDaemonReload, systemctlIsEnabled, systemctlEnable, systemctlDisable, systemctlIsActive, systemctlStart, systemctlStop
	systemctlDaemonReload = func() error {
		s.calls = append(s.calls, "daemon-reload")
		return s.reloadErr
	}
	systemctlIsEnabled = func(name string) (bool, error) {
		s.calls = append(s.calls, "is-enabled "+name)
		if err, ok := s.enableErr[name]; ok {
			return false, err
		}
		return s.enabled[name], nil
	}
	systemctlEnable = func(name string) error {
		s.calls = append(s.calls, "enable "+name)
		if err, ok := s.enableActionErr[name]; ok {
			return err
		}
		s.enabled[name] = true
		return nil
	}
	systemctlDisable = func(name string) error {
		s.calls = append(s.calls, "disable "+name)
		s.enabled[name] = false
		return nil
	}
	systemctlIsActive = func(name string) (bool, error) {
		s.calls = append(s.calls, "is-active "+name)
		if err, ok := s.activeErr[name]; ok {
			return false, err
		}
		return s.active[name], nil
	}
	systemctlStart = func(name string) error {
		s.calls = append(s.calls, "start "+name)
		if err, ok := s.startActionErr[name]; ok {
			return err
		}
		s.active[name] = true
		return nil
	}
	systemctlStop = func(name string) error {
		s.calls = append(s.calls, "stop "+name)
		s.active[name] = false
		return nil
	}
	t.Cleanup(func() {
		systemctlDaemonReload = origDR
		systemctlIsEnabled = origIE
		systemctlEnable = origE
		systemctlDisable = origD
		systemctlIsActive = origIA
		systemctlStart = origST
		systemctlStop = origSP
	})
	return s
}

func stubFS(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	original := systemdPaths
	systemdPaths.dir = dir
	t.Cleanup(func() { systemdPaths = original })
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

func boolPtr(b bool) *bool { return &b }

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
		{"no name", &config.Step{OsSystemd: &config.OsSystemd{Service: map[string]interface{}{"ExecStart": "/bin/true"}}}, true},
		{"missing suffix", &config.Step{OsSystemd: &config.OsSystemd{Name: "foo", Service: map[string]interface{}{"ExecStart": "/bin/true"}}}, true},
		{"unsupported suffix", &config.Step{OsSystemd: &config.OsSystemd{Name: "foo.path", Service: map[string]interface{}{"ExecStart": "/bin/true"}}}, true},
		{"bad state", &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service", State: "queued", Service: map[string]interface{}{"ExecStart": "/bin/true"}}}, true},
		{"present no sections", &config.Step{OsSystemd: &config.OsSystemd{Name: "x.service"}}, true},
		{"service with timer section rejected", &config.Step{OsSystemd: &config.OsSystemd{
			Name:    "x.service",
			Service: map[string]interface{}{"ExecStart": "/bin/true"},
			Timer:   map[string]interface{}{"OnCalendar": "daily"},
		}}, true},
		{"timer with service section rejected", &config.Step{OsSystemd: &config.OsSystemd{
			Name:    "x.timer",
			Service: map[string]interface{}{"ExecStart": "/bin/true"},
			Timer:   map[string]interface{}{"OnCalendar": "daily"},
		}}, true},
		{"ok minimal service", &config.Step{OsSystemd: &config.OsSystemd{
			Name:    "myapp.service",
			Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		}}, false},
		{"ok timer", &config.Step{OsSystemd: &config.OsSystemd{
			Name:  "backup.timer",
			Unit:  map[string]interface{}{"Description": "nightly"},
			Timer: map[string]interface{}{"OnCalendar": "daily"},
		}}, false},
		{"ok absent", &config.Step{OsSystemd: &config.OsSystemd{Name: "old.service", State: "absent"}}, false},
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

func TestApply_CreatesUnitFile(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name: "myapp.service",
		Unit: map[string]interface{}{
			"Description": "My app",
			"After":       "network.target",
		},
		Service: map[string]interface{}{
			"ExecStart": "/opt/myapp/bin/run",
			"Restart":   "on-failure",
			"User":      "deploy",
		},
		Install: map[string]interface{}{
			"WantedBy": "multi-user.target",
		},
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	got, err := os.ReadFile(filepath.Join(dir, "myapp.service"))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	want := strings.Join([]string{
		"# Managed by mooncake os.systemd",
		"[Unit]",
		"After=network.target",
		"Description=My app",
		"",
		"[Service]",
		"ExecStart=/opt/myapp/bin/run",
		"Restart=on-failure",
		"User=deploy",
		"",
		"[Install]",
		"WantedBy=multi-user.target",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("content mismatch\n got %q\nwant %q", got, want)
	}
	// Default ReloadOnChange=true triggers daemon-reload on create.
	if !contains(st.calls, "daemon-reload") {
		t.Errorf("expected daemon-reload; calls=%v", st.calls)
	}
}

func TestApply_ListValuesEmitOneLineEach(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	_ = newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name: "multi.service",
		Service: map[string]interface{}{
			"ExecStartPre": []interface{}{"/bin/mkdir -p /var/lib/myapp", "/bin/chown deploy /var/lib/myapp"},
			"ExecStart":    "/opt/myapp/run",
		},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(dir, "multi.service"))
	want := strings.Join([]string{
		"# Managed by mooncake os.systemd",
		"[Service]",
		"ExecStart=/opt/myapp/run",
		"ExecStartPre=/bin/mkdir -p /var/lib/myapp",
		"ExecStartPre=/bin/chown deploy /var/lib/myapp",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("multi-line emit\n got %q\nwant %q", got, want)
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
	}}
	_ = mustRun(t, false, step)
	stCalls := append([]string(nil), st.calls...)
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
	// Second run must not daemon-reload (nothing changed).
	for _, c := range st.calls[len(stCalls):] {
		if strings.HasPrefix(c, "daemon-reload") {
			t.Errorf("unexpected daemon-reload on idempotent run")
		}
	}
}

func TestApply_DriftTriggersUpdateAndReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
	}}
	_ = mustRun(t, false, step)
	st.calls = nil
	step.OsSystemd.Service["ExecStart"] = "/opt/myapp/run --new"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected drift; reason=%q", r.Reason)
	}
	got, _ := os.ReadFile(filepath.Join(dir, "myapp.service"))
	if !strings.Contains(string(got), "ExecStart=/opt/myapp/run --new") {
		t.Errorf("file not updated: %s", got)
	}
	if !contains(st.calls, "daemon-reload") {
		t.Errorf("expected daemon-reload on drift; calls=%v", st.calls)
	}
}

func TestApply_EnableAndStart(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		Enabled: boolPtr(true),
		Started: boolPtr(true),
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change")
	}
	if !contains(st.calls, "enable myapp.service") {
		t.Errorf("expected enable; calls=%v", st.calls)
	}
	if !contains(st.calls, "start myapp.service") {
		t.Errorf("expected start; calls=%v", st.calls)
	}
}

func TestApply_RestartOnContentChangeWhenStarted(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		Started: boolPtr(true),
	}}
	_ = mustRun(t, false, step) // create + start
	st.calls = nil

	// Now drift: content changes; service already active per the stub.
	step.OsSystemd.Service["ExecStart"] = "/opt/myapp/run --v2"
	_ = mustRun(t, false, step)
	if !contains(st.calls, "start myapp.service") {
		t.Errorf("content drift must restart service when started=true; calls=%v", st.calls)
	}
}

func TestApply_AbsentRemovesAndDisables(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		Enabled: boolPtr(true),
		Started: boolPtr(true),
	}}
	_ = mustRun(t, false, step)
	st.calls = nil
	step.OsSystemd.State = "absent"
	step.OsSystemd.Enabled = nil
	step.OsSystemd.Started = nil
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change on absent; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, "myapp.service")); !os.IsNotExist(err) {
		t.Errorf("file should be removed; stat err=%v", err)
	}
	if !contains(st.calls, "stop myapp.service") {
		t.Errorf("expected stop; calls=%v", st.calls)
	}
	if !contains(st.calls, "disable myapp.service") {
		t.Errorf("expected disable; calls=%v", st.calls)
	}
	if !contains(st.calls, "daemon-reload") {
		t.Errorf("expected daemon-reload; calls=%v", st.calls)
	}
}

func TestAbsent_NoopWhenAlreadyGone(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	// Pretend systemctl reports the unit unknown (errors) — handler should
	// still cleanly report noop when the unit is absent and not active.
	st.activeErr["never.service"] = errors.New("unit not found")
	st.enableErr["never.service"] = errors.New("unit not found")
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:  "never.service",
		State: "absent",
	}}
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("absent on never-existed should be no-op; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWriteOrCallMutators(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	st := newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		Enabled: boolPtr(true),
		Started: boolPtr(true),
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should set WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(dir, "myapp.service")); err == nil {
		t.Error("plan must not write file")
	}
	for _, c := range st.calls {
		if c == "daemon-reload" || strings.HasPrefix(c, "enable ") || strings.HasPrefix(c, "start ") || strings.HasPrefix(c, "stop ") || strings.HasPrefix(c, "disable ") {
			t.Errorf("plan mode must not mutate; saw call %q", c)
		}
	}
}

func TestApply_ReloadOnChangeFalseSkipsReload(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	reload := false
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:           "myapp.service",
		Service:        map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		ReloadOnChange: &reload,
	}}
	_ = mustRun(t, false, step)
	if contains(st.calls, "daemon-reload") {
		t.Errorf("daemon-reload should be skipped when reload_on_change=false; calls=%v", st.calls)
	}
}

func TestApply_AlreadyEnabledNoOp(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = stubFS(t)
	st := newStub(t)
	st.enabled["myapp.service"] = true
	st.active["myapp.service"] = true
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "myapp.service",
		Service: map[string]interface{}{"ExecStart": "/opt/myapp/run"},
		Enabled: boolPtr(true),
		Started: boolPtr(true),
	}}
	_ = mustRun(t, false, step)
	// First run creates the file (which triggers a restart since the unit
	// is already active and content is new); track that this is the only
	// start/enable call.
	starts := 0
	enables := 0
	for _, c := range st.calls {
		if strings.HasPrefix(c, "start ") {
			starts++
		}
		if strings.HasPrefix(c, "enable ") {
			enables++
		}
	}
	if enables != 0 {
		t.Errorf("already-enabled unit should not be re-enabled; enables=%d calls=%v", enables, st.calls)
	}
	if starts > 1 {
		t.Errorf("at most one start (the post-write restart); got starts=%d calls=%v", starts, st.calls)
	}
}

func TestRender_Timer(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	dir := stubFS(t)
	_ = newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "backup.timer",
		Unit:    map[string]interface{}{"Description": "Nightly backup"},
		Timer:   map[string]interface{}{"OnCalendar": "daily", "Persistent": true},
		Install: map[string]interface{}{"WantedBy": "timers.target"},
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(filepath.Join(dir, "backup.timer"))
	want := strings.Join([]string{
		"# Managed by mooncake os.systemd",
		"[Unit]",
		"Description=Nightly backup",
		"",
		"[Timer]",
		"OnCalendar=daily",
		"Persistent=true",
		"",
		"[Install]",
		"WantedBy=timers.target",
		"",
	}, "\n")
	if string(got) != want {
		t.Errorf("timer mismatch\n got %q\nwant %q", got, want)
	}
}

func TestRender_PathOverride(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	tmp := t.TempDir()
	_ = newStub(t)
	step := &config.Step{OsSystemd: &config.OsSystemd{
		Name:    "user.service",
		Path:    tmp,
		Service: map[string]interface{}{"ExecStart": "/bin/true"},
	}}
	_ = mustRun(t, false, step)
	if _, err := os.Stat(filepath.Join(tmp, "user.service")); err != nil {
		t.Errorf("expected unit at custom path: %v", err)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
