package unarchive

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

// TestRun_CreatesExists_AlreadyOk: with the creates marker pointing
// at an existing path, plan reports already-extracted.
func TestRun_CreatesExists_AlreadyOk(t *testing.T) {
	dir := t.TempDir()
	marker := filepath.Join(dir, "extracted")
	if err := os.MkdirAll(marker, 0o755); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{
		Unarchive: &config.Unarchive{
			Src:     "/tmp/fake.tar.gz",
			Dest:    dir,
			Creates: marker,
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("should be already-ok; reason=%q", r.Reason)
	}
}

// TestRun_CreatesMissing_WouldExtract: creates path doesn't exist →
// would-extract.
func TestRun_CreatesMissing_WouldExtract(t *testing.T) {
	step := &config.Step{
		Unarchive: &config.Unarchive{
			Src:     "/tmp/fake.tar.gz",
			Dest:    "/tmp/dest",
			Creates: filepath.Join(t.TempDir(), "does-not-exist"),
		},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("should report would-extract; reason=%q", r.Reason)
	}
}

// TestRun_NoCreates_WouldExtract: without a creates marker we can't
// inspect → plan always reports would-extract (documented limitation).
func TestRun_NoCreates_WouldExtract(t *testing.T) {
	step := &config.Step{
		Unarchive: &config.Unarchive{Src: "/tmp/x.tgz", Dest: "/tmp/d"},
	}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Error("should report would-extract without creates")
	}
	if !strings.Contains(r.Reason, "no creates") {
		t.Errorf("reason should hint at the limitation; got %q", r.Reason)
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
