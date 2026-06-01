package agent

import (
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"testing"
)

func TestNextIterationNumber(t *testing.T) {
	tmpDir := t.TempDir()

	num, err := NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("NextIterationNumber failed: %v", err)
	}
	if num != 1 {
		t.Errorf("Expected first iteration to be 1, got %d", num)
	}

	log := &IterationLog{
		Iteration: num,
		Goal:      "test",
		PlanHash:  "abc123",
		Status:    "success",
	}
	if _, err := WriteIterationLog(tmpDir, log); err != nil {
		t.Fatalf("WriteIterationLog failed: %v", err)
	}

	num, err = NextIterationNumber(tmpDir)
	if err != nil {
		t.Fatalf("NextIterationNumber failed: %v", err)
	}
	if num != 2 {
		t.Errorf("Expected second iteration to be 2, got %d", num)
	}
}

// initRepoWithBaseCommit makes a temp git repo with one committed file
// ("f.txt" = "base\n") and returns its root. HEAD is a real commit so
// diffAgainstWorktree's `read-tree HEAD` has a tree to seed from.
func initRepoWithBaseCommit(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		// Pin identity + skip hooks so the test is hermetic regardless of
		// the caller's git config / inherited GIT_* env.
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\n%s", args, err, out)
		}
	}
	runGit("init")
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGit("add", "f.txt")
	runGit("commit", "-m", "base", "--no-gpg-sign")
	return dir
}

// TestCollectUntrackedFile is the #72 regression: a brand-new (untracked)
// file must be reported by both CollectChangedFiles and CollectDiffStat. A
// plain `git diff HEAD` (tracked-only) misses it.
func TestCollectUntrackedFile(t *testing.T) {
	dir := initRepoWithBaseCommit(t)

	if err := os.WriteFile(filepath.Join(dir, "agent.txt"), []byte("agent was here\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := CollectChangedFiles(dir)
	if err != nil {
		t.Fatalf("CollectChangedFiles: %v", err)
	}
	if want := []string{"agent.txt"}; !reflect.DeepEqual(files, want) {
		t.Errorf("changed files = %v, want %v", files, want)
	}

	stat, err := CollectDiffStat(dir)
	if err != nil {
		t.Fatalf("CollectDiffStat: %v", err)
	}
	if want := (DiffStat{Files: 1, Insertions: 1, Deletions: 0}); stat != want {
		t.Errorf("diff stat = %+v, want %+v", stat, want)
	}
}

// TestCollectTrackedEditAndUntracked confirms tracked edits and untracked
// additions are both counted in one pass.
func TestCollectTrackedEditAndUntracked(t *testing.T) {
	dir := initRepoWithBaseCommit(t)

	// Edit the tracked file (+1 line) and add a new untracked file (+2 lines).
	if err := os.WriteFile(filepath.Join(dir, "f.txt"), []byte("base\nedited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "new.txt"), []byte("a\nb\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := CollectChangedFiles(dir)
	if err != nil {
		t.Fatalf("CollectChangedFiles: %v", err)
	}
	if want := []string{"f.txt", "new.txt"}; !reflect.DeepEqual(files, want) {
		t.Errorf("changed files = %v, want %v", files, want)
	}

	stat, err := CollectDiffStat(dir)
	if err != nil {
		t.Fatalf("CollectDiffStat: %v", err)
	}
	if want := (DiffStat{Files: 2, Insertions: 3, Deletions: 0}); stat != want {
		t.Errorf("diff stat = %+v, want %+v", stat, want)
	}
}

// TestCollectIgnoredFileExcluded confirms .gitignore'd files are NOT counted
// (git add -A respects the ignore list).
func TestCollectIgnoredFileExcluded(t *testing.T) {
	dir := initRepoWithBaseCommit(t)

	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("ignored.txt\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "ignored.txt"), []byte("nope\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := CollectChangedFiles(dir)
	if err != nil {
		t.Fatalf("CollectChangedFiles: %v", err)
	}
	// .gitignore itself is a new tracked-able file and shows up; ignored.txt must not.
	if want := []string{".gitignore"}; !reflect.DeepEqual(files, want) {
		t.Errorf("changed files = %v, want %v", files, want)
	}
}

// TestCollectExcludesAgentInternals confirms agent's own transient state —
// the .mooncake-plan-*.yml temp file (on disk during collection, deleted
// right after) and .mooncake/ iteration logs — is not reported as work the
// plan did.
func TestCollectExcludesAgentInternals(t *testing.T) {
	dir := initRepoWithBaseCommit(t)

	if err := os.MkdirAll(filepath.Join(dir, ".mooncake/agent/iterations"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mooncake/agent/iterations/00001.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".mooncake-plan-123.yml"), []byte("- name: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "artifact.txt"), []byte("real work\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	files, err := CollectChangedFiles(dir)
	if err != nil {
		t.Fatalf("CollectChangedFiles: %v", err)
	}
	if want := []string{"artifact.txt"}; !reflect.DeepEqual(files, want) {
		t.Errorf("changed files = %v, want %v (agent internals must be excluded)", files, want)
	}

	stat, err := CollectDiffStat(dir)
	if err != nil {
		t.Fatalf("CollectDiffStat: %v", err)
	}
	if want := (DiffStat{Files: 1, Insertions: 1, Deletions: 0}); stat != want {
		t.Errorf("diff stat = %+v, want %+v", stat, want)
	}
}

func TestComputePlanHash(t *testing.T) {
	plan1 := []byte("name: test\nsteps:\n  - print:\n      msg: hello")
	plan2 := []byte("name: test\nsteps:\n  - print:\n      msg: hello")
	plan3 := []byte("name: test\nsteps:\n  - print:\n      msg: world")

	hash1 := ComputePlanHash(plan1)
	hash2 := ComputePlanHash(plan2)
	hash3 := ComputePlanHash(plan3)

	if hash1 != hash2 {
		t.Errorf("Identical plans produced different hashes: %s != %s", hash1, hash2)
	}

	if hash1 == hash3 {
		t.Errorf("Different plans produced same hash: %s", hash1)
	}

	if len(hash1) != 64 {
		t.Errorf("Expected 64-character hex string, got %d characters", len(hash1))
	}
}

func TestWriteIterationLog(t *testing.T) {
	tmpDir := t.TempDir()

	log := &IterationLog{
		Iteration:    1,
		Goal:         "test goal",
		PlanHash:     "abc123",
		Status:       "success",
		ChangedFiles: []string{"file1.txt", "file2.txt"},
		DiffStat: DiffStat{
			Files:      2,
			Insertions: 10,
			Deletions:  5,
		},
	}

	path, err := WriteIterationLog(tmpDir, log)
	if err != nil {
		t.Fatalf("WriteIterationLog failed: %v", err)
	}

	expectedPath := filepath.Join(tmpDir, ".mooncake/agent/iterations/00001.json")
	if path != expectedPath {
		t.Errorf("Expected path %s, got %s", expectedPath, path)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("Iteration log file was not created")
	}
}
