package git_config

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
		{"missing scope", &config.Step{GitConfig: &config.GitConfig{Set: map[string]string{"a.b": "c"}}}, true},
		{"invalid scope", &config.Step{GitConfig: &config.GitConfig{Scope: "world", Set: map[string]string{"a.b": "c"}}}, true},
		{"local without repo", &config.Step{GitConfig: &config.GitConfig{Scope: "local", Set: map[string]string{"a.b": "c"}}}, true},
		{"nothing to do", &config.Step{GitConfig: &config.GitConfig{Scope: "global"}}, true},
		{"empty set key", &config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"": "v"}}}, true},
		{"empty unset key", &config.Step{GitConfig: &config.GitConfig{Scope: "global", Unset: []string{""}}}, true},
		{"ok local", &config.Step{GitConfig: &config.GitConfig{Scope: "local", Repo: "/tmp/x", Set: map[string]string{"a.b": "c"}}}, false},
		{"ok global", &config.Step{GitConfig: &config.GitConfig{Scope: "global", Set: map[string]string{"a.b": "c"}}}, false},
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

// makeRepo initialises a local git repo for `scope: local` tests.
func makeRepo(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}

	path := t.TempDir()
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
	return path
}

func readLocalKey(t *testing.T, repo, key string) (string, bool) {
	t.Helper()
	cmd := exec.Command("git", "-C", repo, "config", "--local", "--get", key)
	out, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			return "", false
		}
		t.Fatalf("read %s: %v", key, err)
	}
	return strings.TrimRight(string(out), "\n"), true
}

func TestRun_Plan_NoDriftAllSet(t *testing.T) {
	repo := makeRepo(t)
	// Seed desired state.
	if err := exec.Command("git", "-C", repo, "config", "--local", "user.email", "dev@example.com").Run(); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Set:   map[string]string{"user.email": "dev@example.com"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run plan: %v", err)
	}
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("expected no drift; got WouldChange=true reason=%q", r.Reason)
	}
}

func TestRun_Plan_ReportsDriftCount(t *testing.T) {
	repo := makeRepo(t)
	if err := exec.Command("git", "-C", repo, "config", "--local", "credential.helper", "cache").Run(); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Set: map[string]string{
			"user.email":     "dev@example.com",
			"core.autocrlf":  "false",
		},
		Unset: []string{"credential.helper"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, true), step)
	if err != nil {
		t.Fatalf("Run plan: %v", err)
	}
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("expected drift; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "3 key(s) drift") {
		t.Errorf("expected 3 key drift summary; got %q", r.Reason)
	}
}

func TestRun_Apply_SetsAndUnsets(t *testing.T) {
	repo := makeRepo(t)
	if err := exec.Command("git", "-C", repo, "config", "--local", "credential.helper", "cache").Run(); err != nil {
		t.Fatal(err)
	}

	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Set: map[string]string{
			"user.email":    "dev@example.com",
			"core.autocrlf": "false",
		},
		Unset: []string{"credential.helper"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run apply: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true; reason=%q", r.Reason)
	}
	if got, ok := readLocalKey(t, repo, "user.email"); !ok || got != "dev@example.com" {
		t.Errorf("user.email: got=%q ok=%v", got, ok)
	}
	if got, ok := readLocalKey(t, repo, "core.autocrlf"); !ok || got != "false" {
		t.Errorf("core.autocrlf: got=%q ok=%v", got, ok)
	}
	if _, ok := readLocalKey(t, repo, "credential.helper"); ok {
		t.Errorf("credential.helper should be unset")
	}
}

func TestRun_Apply_Idempotent(t *testing.T) {
	repo := makeRepo(t)
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Set:   map[string]string{"user.email": "dev@example.com"},
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("second run should be no-op; reason=%q", r.Reason)
	}
}

func TestRun_Apply_OverwritesDriftValue(t *testing.T) {
	repo := makeRepo(t)
	if err := exec.Command("git", "-C", repo, "config", "--local", "user.email", "stale@example.com").Run(); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Set:   map[string]string{"user.email": "fresh@example.com"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Errorf("expected Changed=true; reason=%q", r.Reason)
	}
	if got, _ := readLocalKey(t, repo, "user.email"); got != "fresh@example.com" {
		t.Errorf("user.email: got=%q want fresh@example.com", got)
	}
}

func TestRun_Apply_UnsetMissingIsNoOp(t *testing.T) {
	repo := makeRepo(t)
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  repo,
		Unset: []string{"never.was.set"},
	}}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Errorf("expected no-op; reason=%q", r.Reason)
	}
}

func TestRun_LocalScope_NotGitRepoErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	dir := t.TempDir()
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  dir,
		Set:   map[string]string{"user.email": "x@y"},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for non-git repo dest")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' in error; got %v", err)
	}
}

func TestRun_LocalScope_MissingRepoErrors(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not available")
	}
	step := &config.Step{GitConfig: &config.GitConfig{
		Scope: "local",
		Repo:  filepath.Join(t.TempDir(), "nope"),
		Set:   map[string]string{"user.email": "x@y"},
	}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error for missing repo")
	}
	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("expected 'does not exist' in error; got %v", err)
	}
}
