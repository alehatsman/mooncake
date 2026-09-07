package observe_file

import (
	"context"
	"os"
	"path/filepath"
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
	mode := actions.ModeApply
	if plan {
		mode = actions.ModePlan
	}
	return &executor.ExecutionContext{
		Svc: &executor.RunServices{
			Template: r,
			PathUtil: pathutil.NewPathExpander(r),
			Logger:   logger.NewLogger(logger.ErrorLevel),
			Mode:     mode,
			Stats:    executor.NewExecutionStats(),
			Ctx:      context.Background(),
		},
		Scope:      executor.NewVariableScope(),
		CurrentDir: t.TempDir(),
	}
}

func run(t *testing.T, ctx *executor.ExecutionContext, o *config.ObserveFile) (actions.Result, error) {
	t.Helper()
	h := &Handler{}
	step := &config.Step{ObserveFile: o}
	if err := h.Validate(step); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return h.Run(ctx, step)
}

// found reads the universal Found flag off a published observation.
// PublishObservation flattens the envelope into Data, so the fields live at
// the top level — that is what makes {{ name.found }} resolve after `as:`.
func found(t *testing.T, res actions.Result) bool {
	t.Helper()
	r, ok := res.(*executor.Result)
	if !ok {
		t.Fatalf("unexpected result type %T", res)
	}
	f, ok := r.Data["found"].(bool)
	if !ok {
		t.Fatalf("no `found` flag in result data: %+v", r.Data)
	}
	return f
}

// observeError reads the envelope's user-facing error string, if any.
func observeError(t *testing.T, res actions.Result) string {
	t.Helper()
	r := res.(*executor.Result)
	s, _ := r.Data["error"].(string)
	return s
}

func TestRun_ExistingFileIsFound(t *testing.T) {
	ctx := newCtx(t, false)
	path := filepath.Join(t.TempDir(), "ready")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := run(t, ctx, &config.ObserveFile{Path: path})
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !found(t, res) {
		t.Error("existing file should be Found")
	}
}

// A missing file is the answer, not a probe failure: Found=false, no error.
func TestRun_MissingFileIsNotAnError(t *testing.T) {
	ctx := newCtx(t, false)
	path := filepath.Join(t.TempDir(), "nope")

	res, err := run(t, ctx, &config.ObserveFile{Path: path})
	if err != nil {
		t.Fatalf("a missing file should not fail the step: %v", err)
	}
	if found(t, res) {
		t.Error("missing file should not be Found")
	}
	if e := observeError(t, res); e != "" {
		t.Errorf("missing file should leave Error empty (it is the answer, not a failure); got %q", e)
	}
}

func TestRun_Contains(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "log")
	if err := os.WriteFile(path, []byte("server started ok\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Run("matching substring", func(t *testing.T) {
		res, err := run(t, newCtx(t, false), &config.ObserveFile{Path: path, Contains: "started"})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if !found(t, res) {
			t.Error("file containing the substring should be Found")
		}
	})

	t.Run("absent substring", func(t *testing.T) {
		res, err := run(t, newCtx(t, false), &config.ObserveFile{Path: path, Contains: "crashed"})
		if err != nil {
			t.Fatalf("Run: %v", err)
		}
		if found(t, res) {
			t.Error("file without the substring should not be Found")
		}
	})

	// A directory can't contain a substring. That is a false match, not an
	// error, so a `wait: { until: found }` keeps polling.
	t.Run("directory with contains", func(t *testing.T) {
		res, err := run(t, newCtx(t, false), &config.ObserveFile{Path: dir, Contains: "anything"})
		if err != nil {
			t.Fatalf("a directory should not fail the step: %v", err)
		}
		if found(t, res) {
			t.Error("a directory cannot match `contains:`")
		}
	})
}

func TestRun_WaitSucceedsWhenFileAppears(t *testing.T) {
	ctx := newCtx(t, false)
	path := filepath.Join(t.TempDir(), "delayed")

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.WriteFile(path, []byte("here"), 0o644)
	}()

	res, err := run(t, ctx, &config.ObserveFile{
		Path: path,
		Wait: &config.WaitSpec{For: "5s", Interval: "5ms"},
	})
	if err != nil {
		t.Fatalf("wait should have succeeded once the file appeared: %v", err)
	}
	if !found(t, res) {
		t.Error("file appeared but was not Found")
	}
}

// A wait whose budget elapses fails the step — that is what makes it an
// orchestration primitive rather than a slow read.
func TestRun_WaitTimeoutFailsTheStep(t *testing.T) {
	ctx := newCtx(t, false)
	path := filepath.Join(t.TempDir(), "never")

	res, err := run(t, ctx, &config.ObserveFile{
		Path: path,
		Wait: &config.WaitSpec{For: "20ms", Interval: "5ms"},
	})
	if err == nil {
		t.Fatal("a wait that never satisfies must fail the step")
	}
	// The last observation is still published, so `as:` capture downstream of
	// a failed wait sees real data.
	if res == nil {
		t.Fatal("a timed-out wait should still return its last observation")
	}
	if found(t, res) {
		t.Error("timed-out wait should report Found=false")
	}
}

func TestRun_WaitUntilGone(t *testing.T) {
	ctx := newCtx(t, false)
	path := filepath.Join(t.TempDir(), "vanishing")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(30 * time.Millisecond)
		_ = os.Remove(path)
	}()

	if _, err := run(t, ctx, &config.ObserveFile{
		Path: path,
		Wait: &config.WaitSpec{For: "5s", Interval: "5ms", Until: "gone"},
	}); err != nil {
		t.Fatalf("until:gone should have succeeded once the file was removed: %v", err)
	}
}

func TestRun_PlanModeDefersAndDoesNotPoll(t *testing.T) {
	ctx := newCtx(t, true)
	path := filepath.Join(t.TempDir(), "never")

	start := time.Now()
	res, err := run(t, ctx, &config.ObserveFile{
		Path: path,
		Wait: &config.WaitSpec{For: "1h", Interval: "1s"},
	})
	if err != nil {
		t.Fatalf("plan mode should not fail: %v", err)
	}
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Errorf("plan mode polled for %v; it must defer, not wait", elapsed)
	}
	r := res.(*executor.Result)
	if r.Reason == "" {
		t.Error("plan mode should explain what it would observe")
	}
}

func TestValidate(t *testing.T) {
	h := &Handler{}
	t.Run("missing config", func(t *testing.T) {
		if err := h.Validate(&config.Step{}); err == nil {
			t.Error("expected an error with no config")
		}
	})
	t.Run("missing path", func(t *testing.T) {
		if err := h.Validate(&config.Step{ObserveFile: &config.ObserveFile{}}); err == nil {
			t.Error("expected an error with no path")
		}
	})
	// A bad wait must fail at plan time, not sixty seconds into an apply.
	t.Run("bad wait condition", func(t *testing.T) {
		step := &config.Step{ObserveFile: &config.ObserveFile{
			Path: "/tmp/x", Wait: &config.WaitSpec{Until: "soonish"},
		}}
		if err := h.Validate(step); err == nil {
			t.Error("expected an error for an unknown wait condition")
		}
	})
}

func TestHandlerIsNotAReverser(t *testing.T) {
	var h any = &Handler{}
	if _, ok := h.(actions.Reverser); ok {
		t.Error("observe.file implements actions.Reverser; a read has nothing to undo")
	}
}
