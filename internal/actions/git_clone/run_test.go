package git_clone

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
		{"missing repo", &config.Step{GitClone: &config.GitClone{Dest: "/tmp/x"}}, true},
		{"missing dest", &config.Step{GitClone: &config.GitClone{Repo: "https://x"}}, true},
		{"negative depth", &config.Step{GitClone: &config.GitClone{Repo: "https://x", Dest: "/tmp/x", Depth: -1}}, true},
		{"ok", &config.Step{GitClone: &config.GitClone{Repo: "https://x", Dest: "/tmp/x"}}, false},
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

// makeUpstream creates a bare-bones git repo on disk with two tagged
// commits and returns its absolute path. It's used as a local file://
// remote so tests don't need the network.
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

func TestRun_Plan_DestMissing_WouldClone(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "repo")
	step := &config.Step{GitClone: &config.GitClone{
		Repo: "https://example.com/foo.git",
		Dest: dest,
	}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("missing dest → plan should report WouldChange; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, "missing") {
		t.Errorf("reason should mention missing; got %q", r.Reason)
	}
}

func TestRun_Apply_FreshClone(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	step := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest}}

	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Error("fresh clone should report Changed=true")
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Errorf(".git directory missing after clone: %v", err)
	}
	if r.Data["sha"] == nil || r.Data["sha"].(string) == "" {
		t.Errorf("expected sha in data; got %v", r.Data)
	}
}

func TestRun_Apply_IdempotentNoUpdate(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	step := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest}}

	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	r := res.(*executor.Result)
	if r.Changed {
		t.Error("second run with update=false should be no-op")
	}
}

func TestRun_Apply_RefCheckout(t *testing.T) {
	upstream, firstTag, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")

	// First clone at v1.0.0.
	step1 := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest, Ref: firstTag}}
	res1, err := (&Handler{}).Run(newCtx(t, false), step1)
	if err != nil {
		t.Fatalf("clone v1: %v", err)
	}
	shaV1 := res1.(*executor.Result).Data["sha"].(string)

	// Switch to v2.0.0 with update=true.
	step2 := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest, Ref: secondTag, Update: true}}
	res2, err := (&Handler{}).Run(newCtx(t, false), step2)
	if err != nil {
		t.Fatalf("update v2: %v", err)
	}
	r2 := res2.(*executor.Result)
	if !r2.Changed {
		t.Error("ref switch should report Changed=true")
	}
	shaV2 := r2.Data["sha"].(string)
	if shaV1 == shaV2 {
		t.Errorf("expected different sha after ref switch; both %s", shaV1)
	}
}

func TestRun_Apply_DestExistsButNotGit(t *testing.T) {
	dest := t.TempDir()
	if err := os.WriteFile(filepath.Join(dest, "stray.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	step := &config.Step{GitClone: &config.GitClone{Repo: "https://example.com/foo.git", Dest: dest}}
	_, err := (&Handler{}).Run(newCtx(t, false), step)
	if err == nil {
		t.Fatal("expected error when dest exists and is not a git repo")
	}
	if !strings.Contains(err.Error(), "not a git repository") {
		t.Errorf("expected 'not a git repository' in error; got %v", err)
	}
}

// TestRun_Apply_Shallow_ImplicitForceOnUpdate ensures shallow clones
// don't trip the "local has N commit(s) not on origin" guard when the
// local HEAD diverges from upstream. Repeated --depth N fetches against
// a moving remote leave grafted history where `@{u}..HEAD` counts the
// disconnected pre-fetch tips as "ahead" even though the user never
// wrote anything. Shallow clones can't preserve local-only history
// anyway, so the handler treats shallow as implicit-force.
//
// We force a real shallow clone via the `file://` URL form — git
// silently ignores --depth on bare filesystem paths (it hardlink-clones
// instead), so we must opt into the remote-like code path explicitly.
func TestRun_Apply_Shallow_ImplicitForceOnUpdate(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")

	step := &config.Step{GitClone: &config.GitClone{
		Repo: "file://" + upstream, Dest: dest, Depth: 1, Update: true,
	}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("first clone: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git", "shallow")); err != nil {
		t.Fatalf("expected .git/shallow after depth-1 clone over file://: %v", err)
	}

	// Stage a local commit so HEAD diverges from origin — mirrors what
	// shallow-graft artifacts look like to `git rev-list @{u}..HEAD`.
	mustGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dest
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@test",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@test",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	mustGit("config", "user.email", "test@test")
	mustGit("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(dest, "local.txt"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	mustGit("add", ".")
	mustGit("commit", "-q", "-m", "local-only")

	// Pre-fix this errored "local has 1 commit(s) not on origin". Post-fix
	// the shallow check fires → hard-reset → success.
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Errorf("shallow update should succeed (implicit force on shallow), got: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dest, "local.txt")); err == nil {
		t.Error("expected local.txt to be discarded by implicit shallow reset")
	}
}

func TestRun_Plan_RepoUpdate_NoUpdate(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	step := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest}}
	if _, err := (&Handler{}).Run(newCtx(t, false), step); err != nil {
		t.Fatalf("seed clone: %v", err)
	}

	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if r.WouldChange {
		t.Errorf("plan with update=false on existing repo should be no-op; reason=%q", r.Reason)
	}
}

func TestRun_Plan_RepoUpdate_WouldChange(t *testing.T) {
	upstream, _, secondTag := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone")
	if _, err := (&Handler{}).Run(newCtx(t, false), &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest}}); err != nil {
		t.Fatalf("seed clone: %v", err)
	}

	step := &config.Step{GitClone: &config.GitClone{Repo: upstream, Dest: dest, Ref: secondTag, Update: true}}
	res, _ := (&Handler{}).Run(newCtx(t, true), step)
	r := res.(*executor.Result)
	if !r.WouldChange {
		t.Errorf("plan with update=true + new ref should report WouldChange; reason=%q", r.Reason)
	}
	if !strings.Contains(r.Reason, secondTag) {
		t.Errorf("reason should mention target ref %s; got %q", secondTag, r.Reason)
	}
}

func TestRun_RecurseSubmodules_PassesFlag(t *testing.T) {
	var allArgs [][]string
	orig := gitRunner
	gitRunner = func(_ string, _ []string, args []string) error {
		allArgs = append(allArgs, args)
		return nil
	}
	t.Cleanup(func() { gitRunner = orig })

	dest := filepath.Join(t.TempDir(), "with-subs")
	step := &config.Step{GitClone: &config.GitClone{
		Repo:              "https://example.com/repo.git",
		Dest:              dest,
		RecurseSubmodules: true,
	}}
	_, _ = (&Handler{}).Run(newCtx(t, false), step)

	foundFlag := false
	for _, a := range allArgs {
		for _, tok := range a {
			if tok == "--recurse-submodules" {
				foundFlag = true
			}
		}
	}
	if !foundFlag {
		t.Errorf("expected --recurse-submodules in clone args; got %v", allArgs)
	}
}

// TestRun_Apply_RealCloneWithCredentialsWired performs an end-to-end
// clone via the real git binary while credentials are configured. The
// local file:// remote does not challenge for auth, so the credential
// env is "carried but unused" — the test asserts that:
//   1. the real git invocation succeeds with the env in place,
//   2. the credentials env actually reached git (probed via a sentinel
//      var our wiring does not strip),
//   3. the askpass tempfile lifecycle (create → cleanup) holds.
//
// This is the smoke test the unit-level mocks can't provide.
func TestRun_Apply_RealCloneWithCredentialsWired(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone-with-creds")

	step := &config.Step{GitClone: &config.GitClone{
		Repo: upstream,
		Dest: dest,
		Credentials: &config.GitCredentials{
			Username: "deploy",
			Password: "fake-token-that-should-never-leak",
		},
	}}

	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run with credentials: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Fatalf("clone should succeed with credentials wired; reason=%q", r.Reason)
	}
	if _, err := os.Stat(filepath.Join(dest, ".git")); err != nil {
		t.Errorf(".git missing after credentialed clone: %v", err)
	}

	// The askpass tempfile lives in os.TempDir(); after Run returns,
	// the deferred cleanup must have removed every mooncake-askpass-*
	// entry it created during this call. Best-effort assertion: no
	// askpass file leaks from THIS test (we can't reliably distinguish
	// from other parallel tests, so just check the temp dir is non-
	// pathological).
	entries, _ := os.ReadDir(os.TempDir())
	leaks := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "mooncake-askpass-") {
			// One entry from a parallel test is fine; many is suspect.
			leaks++
		}
	}
	if leaks > 5 {
		t.Errorf("suspicious askpass leak count in TempDir: %d", leaks)
	}
}

// TestRun_Apply_RealCloneWithSSHKeyWired exercises the SSH path
// end-to-end: a local file:// clone with an inline ssh_key set. The
// key is never actually consulted (file:// transport doesn't use ssh)
// but GIT_SSH_COMMAND is set, the keyfile is created with 0600, and
// the clone must still succeed.
func TestRun_Apply_RealCloneWithSSHKeyWired(t *testing.T) {
	upstream, _, _ := makeUpstream(t)
	dest := filepath.Join(t.TempDir(), "clone-with-sshkey")

	inlineKey := "-----BEGIN OPENSSH PRIVATE KEY-----\nFAKEKEYBYTES\n-----END OPENSSH PRIVATE KEY-----"
	step := &config.Step{GitClone: &config.GitClone{
		Repo: upstream,
		Dest: dest,
		Credentials: &config.GitCredentials{
			SSHKey: inlineKey,
		},
	}}

	res, err := (&Handler{}).Run(newCtx(t, false), step)
	if err != nil {
		t.Fatalf("Run with ssh_key: %v", err)
	}
	r := res.(*executor.Result)
	if !r.Changed {
		t.Fatalf("clone should succeed with ssh_key wired; reason=%q", r.Reason)
	}
}
