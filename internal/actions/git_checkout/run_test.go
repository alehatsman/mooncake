package git_checkout

import (
	"os"
	"os/exec"
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
		{"missing dest", &config.Step{GitCheckout: &config.GitCheckout{Ref: "v1"}}, true},
		{"missing ref", &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x"}}, true},
		{"ok", &config.Step{GitCheckout: &config.GitCheckout{Dest: "/tmp/x", Ref: "main"}}, false},
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

// makeUpstream creates a bare-bones git repo with two tagged commits on
// the same branch. Returns the repo path and both tag names.
func makeUpstream(t *testing.T) (path, firstTag, secondTag string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	path = t.TempDir()
	must := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = path
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}

	must("init", "-q", "-b", "main")
	must("config", "user.email", "test@test")
	must("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("v1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must("add", ".")
	must("commit", "-q", "-m", "initial")
	must("tag", "v1.0.0")

	if err := os.WriteFile(filepath.Join(path, "README.md"), []byte("v2\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	must("add", ".")
	must("commit", "-q", "-m", "second")
	must("tag", "v2.0.0")

	return path, "v1.0.0", "v2.0.0"
}

// cloneInto produces a local clone of upstream at dest.
func cloneInto(t *testing.T, upstream, dest string) {
	t.Helper()
	cmd := exec.Command("git", "clone", "-q", upstream, dest)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("clone: %v\n%s", err, out)
	}
}

func readHead(t *testing.T, dest string) string {
	t.Helper()
	out, err := exec.Command("git", "-C", dest, "rev-parse", "HEAD").Output()
	if err != nil {
		t.Fatalf("rev-parse HEAD: %v", err)
	}
	return strings.TrimSpace(string(out))
}

func TestRun_Plan_NotGitRepoErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dest := t.TempDir()
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: "main"}}
	_, err := (&Handler{}).Run(newCtx(t, true), step)
	if err == nil {
		t.Fatal("expected error when dest is not a git repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' in error; got %v", err)
	}
}

func TestRun_Apply_DestMissingErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dest := filepath.Join(t.TempDir(), "nope")
	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: "main"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error when dest does not exist")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error; got %v", err)
	}
}

func TestRun_Plan_ReportsCurrentAndTargetSHA(t *testing.T) {
	upstream, firstTag, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)

	// Move clone to v1 explicitly so HEAD != v2.
	if out, err := exec.Command("git", "-C", dest, "checkout", "-q", firstTag).CombinedOutput(); err != nil {
		t.Fatalf("seed checkout: %v\n%s", err, out)
	}

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan should report WouldChange; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "->") {
		t.Errorf("reason should show current -> target; got %q", r.Reason)
	}
}

func TestRun_Plan_NoOpWhenAlreadyAtRef(t *testing.T) {
	upstream, _, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("expected no-op; got WouldChange=true reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "already at") {
		t.Errorf("reason should mention 'already at'; got %q", r.Reason)
	}
}

func TestRun_Apply_ChangesRefAndReportsSHA(t *testing.T) {
	upstream, firstTag, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)

	// Move clone to v1 first so the action has work to do.
	if out, err := exec.Command("git", "-C", dest, "checkout", "-q", firstTag).CombinedOutput(); err != nil {
		t.Fatalf("seed checkout: %v\n%s", err, out)
	}
	before := readHead(t, dest)

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run apply: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true; reason=%q", r.Reason)
	}
	after := readHead(t, dest)
	if after == before {
		t.Errorf("HEAD did not move: %s", after)
	}
	if r.Data["sha"].(string) != after {
		t.Errorf("Data.sha=%v, want %s", r.Data["sha"], after)
	}
	if r.Data["ref_resolved"].(string) != secondTag {
		t.Errorf("Data.ref_resolved=%v, want %s", r.Data["ref_resolved"], secondTag)
	}
}

func TestRun_Apply_IdempotentNoOp(t *testing.T) {
	upstream, _, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected no-op when already at ref; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "already at") {
		t.Errorf("reason should mention 'already at'; got %q", r.Reason)
	}
}

func TestRun_Apply_DirtyWithoutForceErrors(t *testing.T) {
	upstream, firstTag, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)
	if out, err := exec.Command("git", "-C", dest, "checkout", "-q", firstTag).CombinedOutput(); err != nil {
		t.Fatalf("seed checkout: %v\n%s", err, out)
	}
	// Introduce dirty working tree.
	if err := os.WriteFile(filepath.Join(dest, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected dirty error without force")
	}
	if !strings.Contains(err.Error(), "local changes") {
		t.Errorf("expected 'local changes' in error; got %v", err)
	}
}

func TestRun_Apply_ForceDiscardsLocalChanges(t *testing.T) {
	upstream, firstTag, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)
	if out, err := exec.Command("git", "-C", dest, "checkout", "-q", firstTag).CombinedOutput(); err != nil {
		t.Fatalf("seed checkout: %v\n%s", err, out)
	}
	if err := os.WriteFile(filepath.Join(dest, "README.md"), []byte("dirty\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: secondTag, Force: true}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run with force: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true; reason=%q", r.Reason)
	}
}

func TestRun_Apply_UnknownRefErrors(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	cloneInto(t, upstream, dest)

	step := &config.Step{GitCheckout: &config.GitCheckout{Dest: dest, Ref: "does-not-exist-tag"}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for unknown ref")
	}
	if !strings.Contains(err.Error(), "cannot resolve ref") {
		t.Errorf("expected 'cannot resolve ref' in error; got %v", err)
	}
}
