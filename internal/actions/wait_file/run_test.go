package wait_file

import (
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

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
		{"missing path", &config.Step{WaitFile: &config.WaitFile{}}, true},
		{"ok", &config.Step{WaitFile: &config.WaitFile{Path: "/tmp/x"}}, false},
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

// TestRun_Plan_AlreadyOk: file exists → no would-change.
func TestRun_Plan_AlreadyOk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{WaitFile: &config.WaitFile{Path: path}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("file exists → should be already-ok; reason=%q", r.Reason)
	}
}

// TestRun_Plan_WouldWait: file missing → would-change with path.
func TestRun_Plan_WouldWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	step := &config.Step{WaitFile: &config.WaitFile{Path: path}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("missing file → plan should report WouldChange")
	}
	if !strings.Contains(r.Reason, path) {
		t.Errorf("reason should mention path; got %q", r.Reason)
	}
}

// TestRun_Apply_FileAppears: file is created shortly after starting wait.
func TestRun_Apply_FileAppears(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ready.txt")

	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(path, []byte("ok"), 0o644)
	}()

	step := &config.Step{WaitFile: &config.WaitFile{
		Path:         path,
		Timeout:      "2s",
		PollInterval: "100ms",
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Data["success"] != true {
		t.Errorf("expected success=true; data=%v", r.Data)
	}
}

// TestRun_Apply_Contains: file appears, then contents grow to include needle.
func TestRun_Apply_Contains(t *testing.T) {
	path := filepath.Join(t.TempDir(), "log.txt")
	if err := os.WriteFile(path, []byte("starting"), 0o644); err != nil {
		t.Fatal(err)
	}

	var done int32
	go func() {
		time.Sleep(150 * time.Millisecond)
		_ = os.WriteFile(path, []byte("starting\nserver ready\n"), 0o644)
		atomic.StoreInt32(&done, 1)
	}()

	step := &config.Step{WaitFile: &config.WaitFile{
		Path:         path,
		Contains:     "ready",
		Timeout:      "2s",
		PollInterval: "100ms",
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if atomic.LoadInt32(&done) != 1 {
		t.Error("expected wait to complete after content was updated")
	}
}

// TestRun_Apply_Timeout: file never appears.
func TestRun_Apply_Timeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "never.txt")
	step := &config.Step{WaitFile: &config.WaitFile{
		Path:         path,
		Timeout:      "200ms",
		PollInterval: "100ms",
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected timeout")
	}
	if !strings.Contains(err.Error(), "timeout") {
		t.Errorf("expected timeout error; got %v", err)
	}
}
