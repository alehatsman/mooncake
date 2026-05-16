package tool

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/lockfile"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/pathutil"
	"github.com/alehatsman/mooncake/internal/template"
)

// chdirTo changes process CWD for the duration of the test. The lockfile
// path resolution uses os.Getwd(), so tests that expect the lockfile to
// land in a temp dir must chdir into it.
func chdirTo(t *testing.T, dir string) {
	t.Helper()
	prev, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(prev) })
}

func newToolCtx(t *testing.T, cwd string) *executor.ExecutionContext {
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
			Mode:     actions.ModeApply,
			Stats:    executor.NewExecutionStats(),
		},
		Scope: &executor.VariableScope{
			User:    map[string]interface{}{"os": "linux", "arch": "amd64"},
			Results: make(map[string]executor.RegisteredResult),
		},
		CurrentDir: cwd,
	}
}

func TestHandler_MiseExecute_HappyPath(t *testing.T) {
	withTempStore(t) // unused for mise but isolates the lockfile path lookup
	cwd := t.TempDir()
	chdirTo(t, cwd)
	withFakeMise(t, &fakeMiseRunner{})

	step := &config.Step{
		Tool: &config.Tool{
			Name:    "node",
			Version: "24.0.0",
			Backend: BackendMise,
		},
	}

	h := &Handler{}
	if err := h.Validate(step); err != nil {
		t.Fatalf("Validate: %v", err)
	}

	res, err := h.Run(newToolCtx(t, cwd), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Error("expected Changed=true on first install")
	}

	// Lockfile recorded with abbreviated mise entry.
	lock, err := lockfile.Load(filepath.Join(cwd, lockfile.Filename))
	if err != nil {
		t.Fatalf("load lockfile: %v", err)
	}
	e, ok := lock.LookupByName("node", "24.0.0")
	if !ok {
		t.Fatal("mise entry missing from lockfile")
	}
	if e.Backend != BackendMise {
		t.Errorf("backend = %q, want %q", e.Backend, BackendMise)
	}
	if e.ResolvedURL != "" || e.SHA256 != "" {
		t.Errorf("mise entry should omit url/sha256, got %+v", e)
	}
}

func TestHandler_MiseExecute_Idempotent(t *testing.T) {
	withTempStore(t)
	cwd := t.TempDir()
	chdirTo(t, cwd)
	f := &fakeMiseRunner{
		// Pre-populated: tool already installed before this run.
		whichResponses: map[string]string{"node@24.0.0": "/opt/mise/node/24.0.0/bin/node"},
	}
	withFakeMise(t, f)

	step := &config.Step{
		Tool: &config.Tool{Name: "node", Version: "24.0.0", Backend: BackendMise},
	}

	res, err := (&Handler{}).Run(newToolCtx(t, cwd), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Error("expected Changed=false when mise already has the tool")
	}
	if f.installCalls != 0 {
		t.Errorf("expected 0 install invocations, got %d", f.installCalls)
	}
}

func TestHandler_CrossBackendLockRejection(t *testing.T) {
	withTempStore(t)
	cwd := t.TempDir()
	chdirTo(t, cwd)
	withFakeMise(t, &fakeMiseRunner{})

	// Seed lockfile with an archive-url entry for (node, 24.0.0).
	lock := &lockfile.Lock{}
	lock.Set(lockfile.Entry{
		Backend:      BackendArchiveURL,
		Name:         "node",
		Version:      "24.0.0",
		ResolvedURL:  "https://nodejs.org/dist/v24.0.0/node.tar.gz",
		SHA256:       "sha256:abc",
		LockedByArch: "linux-amd64",
	})
	if err := lock.Save(filepath.Join(cwd, lockfile.Filename)); err != nil {
		t.Fatalf("seed lockfile: %v", err)
	}

	// Now try to install the same (name, version) via mise. Should fail.
	step := &config.Step{
		Tool: &config.Tool{Name: "node", Version: "24.0.0", Backend: BackendMise},
	}
	_, err := (&Handler{}).Run(newToolCtx(t, cwd), step)
	if err == nil {
		t.Fatal("expected cross-backend lockfile rejection")
	}
}

func TestHandler_MiseRun_PlanMode_AlreadyInstalled(t *testing.T) {
	withTempStore(t)
	cwd := t.TempDir()
	chdirTo(t, cwd)
	withFakeMise(t, &fakeMiseRunner{
		whichResponses: map[string]string{"node@24.0.0": "/opt/mise/node/24.0.0/bin/node"},
	})

	ctx := newToolCtx(t, cwd)
	ctx.Svc.Mode = actions.ModePlan

	step := &config.Step{
		Tool: &config.Tool{Name: "node", Version: "24.0.0", Backend: BackendMise},
	}
	res, err := (&Handler{}).Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("WouldChange should be false when tool is installed; reason=%q", r.Reason)
	}
}

func TestHandler_MiseRun_PlanMode_NotInstalled(t *testing.T) {
	withTempStore(t)
	cwd := t.TempDir()
	chdirTo(t, cwd)
	withFakeMise(t, &fakeMiseRunner{}) // empty whichResponses

	ctx := newToolCtx(t, cwd)
	ctx.Svc.Mode = actions.ModePlan

	step := &config.Step{
		Tool: &config.Tool{Name: "node", Version: "24.0.0", Backend: BackendMise},
	}
	res, err := (&Handler{}).Run(ctx, step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("WouldChange should be true when tool is not installed; reason=%q", r.Reason)
	}
}
