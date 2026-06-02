package agent

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alehatsman/mooncake/internal/agent/llm"
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
// are stripped — agent's TestMain already does this, but defending
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

// TestTerminalStatus covers #64: the run's terminal status must be the
// worst outcome across all iterations, so a later no-op/success iteration
// can't mask an earlier failure for a consumer keying on the status field.
func TestTerminalStatus(t *testing.T) {
	mk := func(statuses ...string) *LoopResult {
		var iters []IterationLog
		for _, s := range statuses {
			iters = append(iters, IterationLog{Status: s})
		}
		r := &LoopResult{Iterations: iters}
		if len(iters) > 0 {
			r.FinalLog = &iters[len(iters)-1]
		}
		return r
	}

	tests := []struct {
		name string
		res  *LoopResult
		want string
	}{
		{
			name: "failure then success no-op (the reported bug)",
			res:  mk("execution_failed", "success"),
			want: "execution_failed",
		},
		{
			name: "all success",
			res:  mk("success", "success"),
			want: "success",
		},
		{
			name: "validation failure masked by later success",
			res:  mk("validation_failed", "success"),
			want: "validation_failed",
		},
		{
			name: "execution outranks validation",
			res:  mk("validation_failed", "execution_failed", "success"),
			want: "execution_failed",
		},
		{
			name: "user rejected then success",
			res:  mk("user_rejected", "success"),
			want: "user_rejected",
		},
		{
			name: "step_done is terminal-good",
			res:  mk("step_done"),
			want: "step_done",
		},
		{
			name: "tie keeps earliest occurrence",
			res:  mk("execution_failed", "execution_failed"),
			want: "execution_failed",
		},
		{
			name: "no iterations falls back to FinalLog",
			res:  &LoopResult{FinalLog: &IterationLog{Status: "success"}},
			want: "success",
		},
		{
			// #79: an earlier success then a no_progress convergence stop is
			// benign "already done", not a failure.
			name: "success then no_progress reads success",
			res:  mk("success", "no_progress"),
			want: "success",
		},
		{
			name: "step_done then no_progress reads success",
			res:  mk("step_done", "no_progress"),
			want: "success",
		},
		{
			// A real failure still outranks no_progress — #64 protection holds.
			name: "failure then no_progress keeps failure",
			res:  mk("validation_failed", "no_progress"),
			want: "validation_failed",
		},
		{
			// No success ever recorded: a lone no_progress stays no_progress.
			name: "no_progress without prior success stays no_progress",
			res:  mk("no_progress"),
			want: "no_progress",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.res.TerminalStatus(); got != tt.want {
				t.Errorf("TerminalStatus() = %q, want %q", got, tt.want)
			}
		})
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

// TestRunLoop_RepeatedFailure_ShortCircuits is the #71 regression: when the
// planner re-proposes a step that fails the same way, the loop must stop
// early instead of burning every iteration. The two plans differ textually
// (different leading no-op step name) so they hash differently — defeating
// the existing plan-identical no_progress check — yet both fail on the same
// "boom" step (same action + exit code), which my failure fingerprint
// catches. MaxIterations is 5; we expect to stop after exactly 2.
func TestRunLoop_RepeatedFailure_ShortCircuits(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	plan1 := `[{"name":"prep-a","shell":"true"},{"name":"boom","shell":"exit 7"}]`
	plan2 := `[{"name":"prep-b","shell":"true"},{"name":"boom","shell":"exit 7"}]`
	// A third differing plan proves we'd keep going if the short-circuit
	// failed to fire — the stub would hand it out on call 3.
	plan3 := `[{"name":"prep-c","shell":"true"},{"name":"boom","shell":"exit 7"}]`
	stub := &stubLLMClient{plans: []string{plan1, plan2, plan3, plan3, plan3}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "do the thing",
		RepoRoot:      repo,
		MaxIterations: 5,
		AutoApply:     true,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if stub.calls != 2 {
		t.Errorf("LLM calls = %d, want 2 (stop after the 2nd identical failure, not all 5)", stub.calls)
	}
	if result.StopReason != StopNoProgress {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopNoProgress)
	}
	if got := result.FinalLog.Status; got != "execution_failed" {
		t.Errorf("FinalLog.Status = %q, want execution_failed (the run still failed)", got)
	}
}

// TestRunLoop_DistinctFailures_DoNotShortCircuit guards the inverse: failing
// on *different* steps each iteration is not a repeat, so the loop keeps
// going (the fingerprint must distinguish distinct failures).
func TestRunLoop_DistinctFailures_DoNotShortCircuit(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// Each plan fails on a differently-named step → distinct fingerprints →
	// no short-circuit; the loop runs until max_iterations.
	stub := &stubLLMClient{plans: []string{
		`[{"name":"boom-1","shell":"exit 1"}]`,
		`[{"name":"boom-2","shell":"exit 1"}]`,
		`[{"name":"boom-3","shell":"exit 1"}]`,
	}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "do the thing",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if stub.calls != 3 {
		t.Errorf("LLM calls = %d, want 3 (distinct failures must not short-circuit)", stub.calls)
	}
	if result.StopReason != StopMaxReached {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopMaxReached)
	}
}

// TestRunLoop_StylePlan_NoFileChange_Repro reproduces #87: a plan-style one-shot
// task that changes no files (print hello) should stop after ONE iteration with
// StopSuccess, not re-plan into a redundant 2nd iteration that hits no_progress.
func TestRunLoop_StylePlan_NoFileChange_Repro(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	plan := `[{"name":"print hello 10 times","cmd":{"argv":["sh","-c","for i in 1 2 3 4 5 6 7 8 9 10; do echo hello; done"]}}]`
	stub := &stubLLMClient{plans: []string{plan, plan, plan}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "print hello 10 times",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	t.Logf("calls=%d stop_reason=%q terminal_status=%q iterations=%d", stub.calls, result.StopReason, result.TerminalStatus(), len(result.Iterations))
	if stub.calls != 1 {
		t.Errorf("LLM calls = %d, want 1 (a no-file-change plan-style task is done after one iteration)", stub.calls)
	}
	if result.StopReason != StopSuccess {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
}

// TestRunLoop_StylePlan_InheritedDirtDoesNotBlockStopSuccess is the #87
// regression: an untracked file already present in the workspace (NOT the
// agent's own .mooncake scratch — e.g. moongit's harness-dropped MCP config in
// /work) used to make diff-vs-HEAD non-empty every iteration, so plan-style's
// "no files changed = done" never fired and a one-shot task re-planned into a
// redundant no_progress iteration. The agent didn't touch that file, so it must
// not count: the task still completes in ONE iteration with StopSuccess.
func TestRunLoop_StylePlan_InheritedDirtDoesNotBlockStopSuccess(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)
	// Simulate moongit's /work: a non-.mooncake untracked file present before
	// the agent runs (the plan never touches it).
	if err := os.WriteFile(filepath.Join(repo, "dex-mcp.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	plan := `[{"name":"print hello 10 times","cmd":{"argv":["sh","-c","for i in 1 2 3 4 5 6 7 8 9 10; do echo hello; done"]}}]`
	stub := &stubLLMClient{plans: []string{plan, plan, plan}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "print hello 10 times",
		RepoRoot:      repo,
		MaxIterations: 3,
		AutoApply:     true,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	t.Logf("calls=%d stop_reason=%q terminal_status=%q iterations=%d", stub.calls, result.StopReason, result.TerminalStatus(), len(result.Iterations))
	if stub.calls != 1 {
		t.Errorf("LLM calls = %d, want 1 (inherited workspace dirt must not force a redundant iteration)", stub.calls)
	}
	if result.StopReason != StopSuccess {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopSuccess)
	}
}

// TestRunLoop_StyleStep_NoChangeStall is the #77 regression: a step-style
// planner that keeps proposing no-op (zero-mutation) steps never sends the
// empty-plan done signal, so without a guard it burns every iteration. The
// plans differ textually (different log message each time) so they hash
// differently — defeating the byte-identical planHash no_progress check — yet
// none changes anything. One no-op is tolerated; we stop on the 2nd in a row.
// MaxIterations is 5; we expect to stop after exactly 2.
func TestRunLoop_StyleStep_NoChangeStall(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// Single-step plans (step-style contract), each a `log` directive that
	// runs successfully but mutates nothing. Differing messages → differing
	// hashes. A 3rd+ plan proves we'd keep going if the guard didn't fire.
	stub := &stubLLMClient{plans: []string{
		`[{"name":"note-a","log":"no-op a"}]`,
		`[{"name":"note-b","log":"no-op b"}]`,
		`[{"name":"note-c","log":"no-op c"}]`,
		`[{"name":"note-d","log":"no-op d"}]`,
		`[{"name":"note-e","log":"no-op e"}]`,
	}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "spin forever",
		RepoRoot:      repo,
		MaxIterations: 5,
		AutoApply:     true,
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if stub.calls != maxNoChangeStreak {
		t.Errorf("LLM calls = %d, want %d (stop on the 2nd consecutive no-op, not all 5)", stub.calls, maxNoChangeStreak)
	}
	if result.StopReason != StopNoProgress {
		t.Errorf("StopReason = %q, want %q", result.StopReason, StopNoProgress)
	}
	// The iterations themselves succeeded; only the stop reason flags the
	// stall (mirrors the #71 pattern — status reflects the iteration).
	if got := result.TerminalStatus(); got != "success" {
		t.Errorf("TerminalStatus = %q, want success (no-op steps ran fine)", got)
	}
}

// TestRunLoop_StyleStep_ProgressResetsStall guards the inverse: an iteration
// that actually changes something resets the no-change streak, so an
// occasional no-op between real work does not trip the #77 guard.
func TestRunLoop_StyleStep_ProgressResetsStall(t *testing.T) {
	repo := t.TempDir()
	initGitRepo(t, repo)

	// no-op, then a real mutation (file.write), then a no-op, then done.
	// The streak never reaches 2 in a row, so the guard stays quiet and the
	// empty plan terminates the run normally.
	stub := &stubLLMClient{plans: []string{
		`[{"name":"note","log":"thinking"}]`,
		`[{"name":"write","file.write":{"path":"out.txt","content":"hi"}}]`,
		`[{"name":"note2","log":"thinking again"}]`,
		"[]\n",
	}}
	cleanup := withStubClient(t, stub)
	defer cleanup()

	result, err := RunLoop(RunOptions{
		Goal:          "do real work with pauses",
		RepoRoot:      repo,
		MaxIterations: 6,
		AutoApply:     true,
		Style:         StyleStep,
	})
	if err != nil {
		t.Fatalf("RunLoop: %v", err)
	}
	if result.StopReason != StopStepDone {
		t.Errorf("StopReason = %q, want %q (a real change between no-ops resets the streak)", result.StopReason, StopStepDone)
	}
	if stub.calls != 4 {
		t.Errorf("LLM calls = %d, want 4 (ran to the empty-plan terminator)", stub.calls)
	}
}

// TestCreatePlanTempFile_AnchoredOnRepoRoot — agent-tmpfile-cwd. The
// executor resolves plan-relative paths against the config file's
// directory; before the fix, agent wrote that file to os.CreateTemp("",
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
	// lists their working directory (agent tempfiles are short-lived
	// but they exist during executor.Start).
	if !strings.HasPrefix(filepath.Base(f.Name()), ".mooncake-plan-") {
		t.Errorf("plan tempfile = %q, want a dot-prefixed name", filepath.Base(f.Name()))
	}
}

// TestRunLoop_RelativePathResolvesAgainstRepoRoot — end-to-end proof
// of the agent-tmpfile-cwd fix. A plan with a relative `file.write`
// path must land inside repoRoot, not in $TMPDIR. Pre-fix: agent's
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
