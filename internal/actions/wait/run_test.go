package wait

import (
	"os"
	"path/filepath"
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
		Variables:  map[string]interface{}{},
		Template:   r,
		PathUtil:   pathutil.NewPathExpander(r),
		Logger:     logger.NewLogger(logger.ErrorLevel),
		CurrentDir: "/tmp",
		CurrentMode: planMode(plan),
		Stats:      executor.NewExecutionStats(),
	}
}

func strPtr(s string) *string { return &s }

// TestRun_FileExists_AlreadyOk: condition is "file exists" and the
// file is currently present → plan reports already-ok, no
// would-change.
func TestRun_FileExists_AlreadyOk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "exists.txt")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		Wait: &config.WaitAction{Condition: "file_exists", Path: strPtr(path)},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("should be already-ok; reason=%q", r.Reason)
	}
}

// TestRun_FileExists_WouldWait: file currently missing → plan reports
// would-wait with the path surfaced.
func TestRun_FileExists_WouldWait(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing.txt")
	step := &config.Step{
		Wait: &config.WaitAction{Condition: "file_exists", Path: strPtr(path)},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("should report would-wait")
	}
	if !strings.Contains(r.Reason, path) {
		t.Errorf("reason should mention path; got %q", r.Reason)
	}
}

// TestRun_FileAbsent_AlreadyOk: condition is "file absent" and the
// file is currently missing → already-ok.
func TestRun_FileAbsent_AlreadyOk(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ghost.txt")
	step := &config.Step{
		Wait: &config.WaitAction{Condition: "file_absent", Path: strPtr(path)},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("should be already-ok; reason=%q", r.Reason)
	}
}

// TestRun_HTTP_SurfacesURL: plan surfaces the wait target.
func TestRun_HTTP_SurfacesURL(t *testing.T) {
	url := "http://localhost:9999/health"
	step := &config.Step{
		Wait: &config.WaitAction{Condition: "http", URL: &url},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !strings.Contains(r.Reason, url) {
		t.Errorf("reason should include URL; got %q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeExecute
}
