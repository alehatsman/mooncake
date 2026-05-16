package package_handler

import (
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

// TestRun_UpgradeAlwaysWouldChange: upgrade isn't idempotent at the
// package-list level — we can't know without running whether anything
// would update. Plan always reports would-change.
func TestRun_UpgradeAlwaysWouldChange(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Manager: "pacman", // explicit so plan doesn't have to auto-detect
			Upgrade: true,
		},
	}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("upgrade plan should report WouldChange; got %+v", r)
	}
	if !strings.Contains(r.Reason, "upgrade") {
		t.Errorf("reason should mention upgrade; got %q", r.Reason)
	}
}

func TestRun_ImplementsRunner(t *testing.T) {
	var _ actions.Runner = &Handler{}
}

func planMode(b bool) actions.Mode {
	if b {
		return actions.ModePlan
	}
	return actions.ModeApply
}

// TestRun_RenderErrorOnPackageName_Apply: a pkg step whose name contains
// a template syntax error must surface that error from Run() before any
// package-manager invocation (F023). Previously the loop silently
// dropped the render error and let apt/yum complain about the literal,
// which masked the real cause. (Note: pongo2 in this codebase does NOT
// error on undefined variables — they render to empty string — so the
// realistic trigger for the silently-swallowed path is a syntax / filter
// error, not a missing var. See internal/template/renderer_test.go.)
func TestRun_RenderErrorOnPackageName_Apply(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Manager: "pacman",
			Name:    "{{ unclosed-tools", // syntax error: missing }}
		},
	}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected render error for malformed template; got nil")
	}
	if !strings.Contains(err.Error(), "render package name") {
		t.Errorf("error should name the source of failure; got %q", err.Error())
	}
}

// TestRun_RenderErrorOnPackageName_Plan: same contract applies on the
// plan-mode path through Run — if the name can't render, plan must
// surface the error rather than predict a successful install of the
// literal placeholder.
func TestRun_RenderErrorOnPackageName_Plan(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Manager: "pacman",
			Name:    "{{ unclosed-tools",
		},
	}
	_, err := (&Handler{}).Run(newCtx(t, true), step)
	if err == nil {
		t.Fatal("expected render error in plan mode; got nil")
	}
	if !strings.Contains(err.Error(), "render package name") {
		t.Errorf("error should name the source of failure; got %q", err.Error())
	}
}

// TestExecute_RenderErrorOnPackageName: same contract on the legacy
// Execute() path. Kept as a separate regression because Execute is
// still wired in for backward compatibility (F011).
func TestExecute_RenderErrorOnPackageName(t *testing.T) {
	step := &config.Step{
		Pkg: &config.Package{
			Manager: "pacman",
			Name:    "{{ unclosed-tools",
		},
	}
	_, err := (&Handler{}).Execute(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected render error from Execute; got nil")
	}
	if !strings.Contains(err.Error(), "render package name") {
		t.Errorf("error should name the source of failure; got %q", err.Error())
	}
}
