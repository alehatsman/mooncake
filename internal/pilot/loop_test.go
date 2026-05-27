package pilot

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/pilot/llm"
)

// stubLLMClient returns a canned plan per call. Used by step-style
// loop tests to drive RunLoop without touching a real provider.
type stubLLMClient struct {
	plans []string
	calls int
}

func (s *stubLLMClient) GeneratePlan(_ context.Context, _, _, _ string) (string, error) {
	if s.calls >= len(s.plans) {
		return "", errors.New("stub exhausted")
	}
	out := s.plans[s.calls]
	s.calls++
	return out, nil
}

// withStubClient swaps the package-level newClient factory for the
// duration of one test. Returns a cleanup func.
func withStubClient(t *testing.T, stub *stubLLMClient) func() {
	t.Helper()
	orig := newClient
	newClient = func(_ llm.ClientOptions) (llm.Client, error) {
		return stub, nil
	}
	return func() { newClient = orig }
}

// initGitRepo turns a TempDir into a minimal git repo so
// snapshot.Collect (which shells out to `git rev-parse`) doesn't
// fail with exit 128 inside loop tests. Inherited GIT_* env vars
// are stripped — pilot's TestMain already does this, but defending
// here keeps the helper safe under direct invocation too.
func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cleanEnv := make([]string, 0, len(os.Environ()))
	for _, e := range os.Environ() {
		if !strings.HasPrefix(e, "GIT_") {
			cleanEnv = append(cleanEnv, e)
		}
	}
	for _, args := range [][]string{
		{"init", "-q"},
		{"config", "user.email", "test@example.com"},
		{"config", "user.name", "test"},
		{"commit", "--allow-empty", "-q", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		cmd.Env = cleanEnv
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestNoProgressDetection(t *testing.T) {
	plan1 := []byte("- shell:\n    cmd: echo hello")
	plan2 := []byte("- shell:\n    cmd: echo hello")
	plan3 := []byte("- shell:\n    cmd: echo world")

	hash1 := ComputePlanHash(plan1)
	hash2 := ComputePlanHash(plan2)
	hash3 := ComputePlanHash(plan3)

	if hash1 != hash2 {
		t.Errorf("Identical plans should have same hash")
	}

	if hash1 == hash3 {
		t.Errorf("Different plans should have different hash")
	}
}

func TestIterationNumbering(t *testing.T) {
	tmpDir := t.TempDir()

	num1, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get iteration 1: %v", err)
	}
	if num1 != 1 {
		t.Errorf("Expected iteration 1, got %d", num1)
	}

	log1 := &IterationLog{
		Iteration: num1,
		Goal:      "test",
		Status:    "success",
	}
	WriteIterationLog(tmpDir, log1)

	num2, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("Failed to get iteration 2: %v", err)
	}
	if num2 != 2 {
		t.Errorf("Expected iteration 2, got %d", num2)
	}
}

// TestSavePlan_FilePerms — F039(c). Plan files contain resolved
// !secret values (post-F037 the planner expands secret markers into
// concrete values before serialization) plus the operator's goal /
// LLM prompt. World-readable permissions on a shared host would leak
// them. The fix pins the directory to 0700 and the file to 0600 —
// matching the rest of mooncake's state-dir convention.
func TestSavePlan_FilePerms(t *testing.T) {
	repoRoot := t.TempDir()
	path, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("plan file perms = %04o, want 0600", got)
	}
	parent := filepath.Dir(path)
	dirInfo, err := os.Stat(parent)
	if err != nil {
		t.Fatalf("stat parent: %v", err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Errorf("iterations dir perms = %04o, want 0700", got)
	}
}

// TestSavePlan_CreatesIterationsDir — F039 also exposed a latent bug
// where the iterations directory was never created; SavePlan would
// `os.WriteFile` into a non-existent path and silently return "" on
// the resulting ENOENT. With MkdirAll the first iteration creates
// the dir as a side-effect.
func TestSavePlan_CreatesIterationsDir(t *testing.T) {
	repoRoot := t.TempDir()
	path, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err != nil {
		t.Fatalf("SavePlan: %v", err)
	}
	if path == "" {
		t.Fatal("SavePlan returned empty path on first iteration (missing MkdirAll regression)")
	}
}

// TestSavePlan_ReturnsErrorOnFailure — F039(d). Pre-fix SavePlan
// silently returned "" on any WriteFile error, leaving the caller to
// guess whether the empty path meant "didn't save" or "shouldn't
// save." Now the error surfaces so the caller can log it.
func TestSavePlan_ReturnsErrorOnFailure(t *testing.T) {
	// Force a write failure by pointing the iterations dir at a path
	// whose parent is a regular file (MkdirAll will refuse).
	repoRoot := t.TempDir()
	// Pre-create a file at the path MkdirAll wants to use as a
	// directory: <repoRoot>/.mooncake/iterations.
	mooncakeDir := filepath.Join(repoRoot, ".mooncake")
	if err := os.MkdirAll(mooncakeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(mooncakeDir, "iterations"), []byte("not a dir"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := SavePlan(repoRoot, 1, []byte("steps: []\n"))
	if err == nil {
		t.Fatal("expected SavePlan error when iterations path is occupied by a file; got nil")
	}
	if !strings.Contains(err.Error(), "create iterations dir") {
		t.Errorf("error should name the failing stage; got %q", err.Error())
	}
}

// TestRunLoop_StyleStep_EmptyPlanReturnsStepDone covers plan §4: an
// empty YAML plan under --style step is the documented "goal reached"
// signal, terminating the loop with StopStepDone after exactly one
// iteration.
func TestRunLoop_StyleStep_EmptyPlanReturnsStepDone(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	stub := &stubLLMClient{plans: []string{"[]\n"}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "no-op",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true, // skip TTY gate
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result.StopReason != StopStepDone {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopStepDone)
	}
	if len(result.Iterations) != 1 {
		t.Errorf("Iterations = %d, want 1", len(result.Iterations))
	}
	if stub.calls != 1 {
		t.Errorf("LLM calls = %d, want 1", stub.calls)
	}
	if result.FinalLog == nil || result.FinalLog.Status != "step_done" {
		t.Errorf("FinalLog status = %v, want step_done", result.FinalLog)
	}
}

// TestRunLoop_StyleStep_MultiStepRejected covers plan §8 decision 2:
// when the model emits >1 step under --style step, the iteration
// fails with a clear error and the loop continues so the model can
// self-correct. We give the stub a multi-step plan then an empty
// plan; iter 1 must be a contract-violation log, iter 2 terminates
// cleanly. The error message must carry the actual step count.
func TestRunLoop_StyleStep_MultiStepRejected(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	multiStep := "- shell: echo one\n- shell: echo two\n"
	stub := &stubLLMClient{plans: []string{multiStep, "[]\n"}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "reject multi-step",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if len(result.Iterations) != 2 {
		t.Fatalf("Iterations = %d, want 2", len(result.Iterations))
	}
	if got := result.Iterations[0].Status; got != "step_contract_violation" {
		t.Errorf("iter 1 status = %q, want step_contract_violation", got)
	}
	wantSubstr := "--style step requires exactly one step, got 2"
	if !strings.Contains(result.Iterations[0].ValidationError, wantSubstr) {
		t.Errorf("iter 1 error = %q, want substring %q", result.Iterations[0].ValidationError, wantSubstr)
	}
	if result.StopReason != StopStepDone {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopStepDone)
	}
}

// TestRunLoop_StyleStep_FeedsResultBack: a single-step plan followed
// by an empty plan should terminate cleanly with StopStepDone and
// the second LLM call must have happened (proving the iteration loop
// fed the first result back). The single step doesn't have to
// execute successfully — we use --auto-apply so the TTY gate is
// skipped, and the executor may fail under a no-op step; either path
// still proceeds to iter 2 (execution_failed continues the loop).
func TestRunLoop_StyleStep_FeedsResultBack(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	// A trivial single-step plan + an empty terminator. The exact
	// action doesn't matter; we only care that iter 1 is processed
	// and iter 2 sees the model.
	singleStep := "- shell: echo step-one\n"
	stub := &stubLLMClient{plans: []string{singleStep, "[]\n"}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "feed-back",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if stub.calls != 2 {
		t.Errorf("LLM calls = %d, want 2 (iter 1 fed back into iter 2)", stub.calls)
	}
	if result.StopReason != StopStepDone {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopStepDone)
	}
}

// TestCreatePlanTempFile_AnchoredOnRepoRoot — pilot-tmpfile-cwd. The
// executor resolves plan-relative paths against the config file's
// directory; before the fix, pilot wrote that file to os.CreateTemp("",
// ...) which is $TMPDIR. A plan saying `file.write: { path: hello.txt }`
// would land in /tmp instead of the operator's repo. Lock the temp
// file's parent to repoRoot itself (not a subdirectory) so the
// resolution honors operator intent.
func TestCreatePlanTempFile_AnchoredOnRepoRoot(t *testing.T) {
	repoRoot := t.TempDir()
	f, err := createPlanTempFile(repoRoot)
	if err != nil {
		t.Fatalf("createPlanTempFile: %v", err)
	}
	defer func() {
		_ = f.Close()
		_ = os.Remove(f.Name())
	}()

	gotDir := filepath.Dir(f.Name())
	if gotDir != repoRoot {
		t.Errorf("plan tempfile parent = %q, want %q (executor resolves plan-relative paths against this dir)", gotDir, repoRoot)
	}
	if !strings.HasSuffix(f.Name(), ".yml") {
		t.Errorf("plan tempfile = %q, want .yml suffix so config.ReadConfigWithValidation accepts it", f.Name())
	}
	// The leading dot keeps the artifact unobtrusive when the operator
	// lists their working directory (pilot tempfiles are short-lived
	// but they exist during executor.Start).
	if !strings.HasPrefix(filepath.Base(f.Name()), ".mooncake-plan-") {
		t.Errorf("plan tempfile = %q, want a dot-prefixed name", filepath.Base(f.Name()))
	}
}

// TestRunLoop_RelativePathResolvesAgainstRepoRoot — end-to-end proof
// of the pilot-tmpfile-cwd fix. A plan with a relative `file.write`
// path must land inside repoRoot, not in $TMPDIR. Pre-fix: pilot's
// tempfile lived in /tmp, so the executor (which resolves plan-
// relative paths against the config file's dir) wrote to /tmp/hello.txt.
// Post-fix: the tempfile is under <repoRoot>/.mooncake/ and the file
// resolves to <repoRoot>/hello.txt.
func TestRunLoop_RelativePathResolvesAgainstRepoRoot(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	// A single-step plan with a RELATIVE path. If the executor resolved
	// from $TMPDIR (the bug), hello.txt lands at <$TMPDIR>/hello.txt.
	// Post-fix, it lands at <repo>/hello.txt because the tempfile lives
	// in <repo>/.mooncake/.
	plan := "- file.write:\n    path: hello.txt\n    content: hi\n"
	stub := &stubLLMClient{plans: []string{plan, "[]\n"}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	_, err := RunLoop(RunOptions{
		Goal:          "create hello.txt with relative path",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}

	wantPath := filepath.Join(repo, "hello.txt")
	got, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("expected hello.txt at %q (relative path should resolve against repoRoot): %v", wantPath, err)
	}
	if string(got) != "hi" {
		t.Errorf("hello.txt content = %q, want %q", string(got), "hi")
	}
}
