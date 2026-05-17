//nolint:revive // package name follows action convention
package os_sysctl

import (
	"fmt"
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

// stubFS captures all package-level side-effect hooks and restores
// them on cleanup. Tests interact with the returned stub to seed the
// kernel-side state and inspect the recorded apply calls.
type stubFS struct {
	persistFile string
	current     map[string]string
	getErr      map[string]error
	applies     []string // "name=value" entries
	applyErr    error
}

func newStub(t *testing.T) *stubFS {
	t.Helper()
	dir := t.TempDir()
	persist := filepath.Join(dir, "99-mooncake.conf")
	s := &stubFS{
		persistFile: persist,
		current:     map[string]string{},
		getErr:      map[string]error{},
	}
	origPaths := sysctlPaths
	origGet := sysctlGet
	origApply := sysctlApply
	sysctlPaths.persistFile = persist
	sysctlGet = func(name string) (string, error) {
		if err, ok := s.getErr[name]; ok {
			return "", err
		}
		v, ok := s.current[name]
		if !ok {
			return "", fmt.Errorf("unknown key %s", name)
		}
		return v, nil
	}
	sysctlApply = func(_ actions.PrivilegedRunner, name, value string) error {
		if s.applyErr != nil {
			return s.applyErr
		}
		s.applies = append(s.applies, name+"="+value)
		s.current[name] = value
		return nil
	}
	t.Cleanup(func() {
		sysctlPaths = origPaths
		sysctlGet = origGet
		sysctlApply = origApply
	})
	return s
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
		{"no name", &config.Step{OsSysctl: &config.OsSysctl{Value: 1}}, true},
		{"bad state", &config.Step{OsSysctl: &config.OsSysctl{Name: "k", State: "queued", Value: 1}}, true},
		{"present no value", &config.Step{OsSysctl: &config.OsSysctl{Name: "k"}}, true},
		{"ok int value", &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}}, false},
		{"ok string value", &config.Step{OsSysctl: &config.OsSysctl{Name: "k", Value: "abc"}}, false},
		{"ok absent without value", &config.Step{OsSysctl: &config.OsSysctl{Name: "k", State: "absent"}}, false},
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

func TestApply_WritesPersistAndApplies(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "0"
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "net.ipv4.ip_forward",
		Value: 1,
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected change; reason=%q", r.Reason)
	}
	got, err := os.ReadFile(s.persistFile)
	if err != nil {
		t.Fatalf("read persist: %v", err)
	}
	want := "# Managed by mooncake os.sysctl\nnet.ipv4.ip_forward = 1\n"
	if string(got) != want {
		t.Errorf("persist mismatch\n got %q\nwant %q", got, want)
	}
	if len(s.applies) != 1 || s.applies[0] != "net.ipv4.ip_forward=1" {
		t.Errorf("expected single runtime apply; got %v", s.applies)
	}
}

func TestApply_Idempotent(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "0"
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "net.ipv4.ip_forward",
		Value: 1,
	}}
	_ = mustRun(t, false, step)
	before := len(s.applies)
	r := mustRun(t, false, step)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
	if len(s.applies) != before {
		t.Errorf("apply should not be called when no drift (before=%d now=%d)", before, len(s.applies))
	}
}

func TestApply_DriftTriggersWriteAndApply(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["vm.swappiness"] = "60"
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "vm.swappiness",
		Value: 10,
	}}
	_ = mustRun(t, false, step)
	// Simulate external drift on the kernel side.
	s.current["vm.swappiness"] = "60"
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected drift detection; reason=%q", r.Reason)
	}
	if len(s.applies) != 2 {
		t.Errorf("expected two applies (initial + drift); got %v", s.applies)
	}
}

func TestApply_ReloadFalse_OnlyPersistsNoApply(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "0"
	boolFalse := false
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:   "net.ipv4.ip_forward",
		Value:  1,
		Reload: &boolFalse,
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected persist write; reason=%q", r.Reason)
	}
	if len(s.applies) != 0 {
		t.Errorf("apply must not run when reload=false; got %v", s.applies)
	}
	if _, err := os.Stat(s.persistFile); err != nil {
		t.Errorf("persist file should exist: %v", err)
	}
}

func TestApply_PersistFalse_AppliesOnly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "0"
	boolFalse := false
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:    "net.ipv4.ip_forward",
		Value:   1,
		Persist: &boolFalse,
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected runtime apply; reason=%q", r.Reason)
	}
	if _, err := os.Stat(s.persistFile); !os.IsNotExist(err) {
		t.Errorf("persist file must not be written when persist=false; stat err=%v", err)
	}
}

func TestAbsent_RemovesLine(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	// Pre-seed: two managed lines.
	s.current["net.ipv4.ip_forward"] = "1"
	s.current["vm.swappiness"] = "10"
	_ = mustRun(t, false, &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})
	_ = mustRun(t, false, &config.Step{OsSysctl: &config.OsSysctl{Name: "vm.swappiness", Value: 10}})

	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "net.ipv4.ip_forward",
		State: "absent",
	}}
	r := mustRun(t, false, step)
	if !r.Changed {
		t.Fatalf("expected delete; reason=%q", r.Reason)
	}
	got, _ := os.ReadFile(s.persistFile)
	if strings.Contains(string(got), "net.ipv4.ip_forward") {
		t.Errorf("line not removed: %q", got)
	}
	if !strings.Contains(string(got), "vm.swappiness = 10") {
		t.Errorf("other lines should remain: %q", got)
	}
}

func TestAbsent_DropsFileWhenLastLineRemoved(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "1"
	_ = mustRun(t, false, &config.Step{OsSysctl: &config.OsSysctl{Name: "net.ipv4.ip_forward", Value: 1}})

	r := mustRun(t, false, &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "net.ipv4.ip_forward",
		State: "absent",
	}})
	if !r.Changed {
		t.Fatalf("expected delete; reason=%q", r.Reason)
	}
	if _, err := os.Stat(s.persistFile); !os.IsNotExist(err) {
		t.Errorf("persist file should be removed when empty; stat err=%v", err)
	}
}

func TestAbsent_NoopWhenLineMissing(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	_ = newStub(t)
	r := mustRun(t, false, &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "vm.swappiness",
		State: "absent",
	}})
	if r.Changed {
		t.Errorf("absent on missing line should be no-op; reason=%q", r.Reason)
	}
}

func TestPlan_DoesNotWriteOrApply(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["net.ipv4.ip_forward"] = "0"
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "net.ipv4.ip_forward",
		Value: 1,
	}}
	r := mustRun(t, true, step)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if _, err := os.Stat(s.persistFile); err == nil {
		t.Error("plan must not write persist file")
	}
	if len(s.applies) != 0 {
		t.Errorf("plan must not run runtime apply; got %v", s.applies)
	}
}

func TestValueCoercion(t *testing.T) {
	cases := []struct {
		name string
		in   interface{}
		want string
	}{
		{"int", 1, "1"},
		{"int64", int64(123), "123"},
		{"float-whole", float64(7), "7"},
		{"float-frac", 1.5, "1.5"},
		{"string", "abc", "abc"},
		{"bool-true", true, "1"},
		{"bool-false", false, "0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := coerceValue(c.in)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("coerceValue(%v) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestApply_StringValueRendersTemplate(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux only")
	}
	s := newStub(t)
	s.current["kernel.hostname"] = "old"
	step := &config.Step{OsSysctl: &config.OsSysctl{
		Name:  "kernel.hostname",
		Value: "new-host",
	}}
	_ = mustRun(t, false, step)
	got, _ := os.ReadFile(s.persistFile)
	if !strings.Contains(string(got), "kernel.hostname = new-host") {
		t.Errorf("persist line not written: %q", got)
	}
}
