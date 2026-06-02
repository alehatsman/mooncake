// Package agent provides autonomous agent functionality for iterative plan generation and execution.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const iterationDir = ".mooncake/agent/iterations"

func NextIterationNumber(repoRoot string) (int, error) {
	dir := filepath.Join(repoRoot, iterationDir)
	if err := os.MkdirAll(dir, 0755); err != nil { // #nosec G301 -- standard directory permissions
		return 0, fmt.Errorf("failed to create iterations directory: %w", err)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0, fmt.Errorf("failed to read iterations directory: %w", err)
	}

	maxNum := 0
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		numStr := strings.TrimSuffix(name, ".json")
		num, err := strconv.Atoi(numStr)
		if err != nil {
			continue
		}
		if num > maxNum {
			maxNum = num
		}
	}

	return maxNum + 1, nil
}

func WriteIterationLog(repoRoot string, log *IterationLog) (string, error) {
	dir := filepath.Join(repoRoot, iterationDir)
	if err := os.MkdirAll(dir, 0755); err != nil { // #nosec G301 -- standard directory permissions
		return "", fmt.Errorf("failed to create iterations directory: %w", err)
	}

	filename := fmt.Sprintf("%05d.json", log.Iteration)
	path := filepath.Join(dir, filename)

	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal iteration log: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil { // #nosec G306 -- standard file permissions
		return "", fmt.Errorf("failed to write iteration log: %w", err)
	}

	return path, nil
}

// scrubGitEnv returns env with all GIT_* vars removed. Needed when shelling
// out to git from a process that may itself have been launched by a git hook —
// in which case GIT_DIR / GIT_WORK_TREE would otherwise redirect the
// subprocess at the parent repo regardless of cmd.Dir.
func scrubGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, e := range env {
		if strings.HasPrefix(e, "GIT_") {
			continue
		}
		out = append(out, e)
	}
	return out
}

func ComputePlanHash(planBytes []byte) string {
	hash := sha256.Sum256(planBytes)
	return hex.EncodeToString(hash[:])
}

// diffAgainstWorktree runs `git diff --cached HEAD <args...>` in repoRoot
// against a throwaway index that has every working-tree change staged
// (read-tree HEAD + add -A). Unlike a plain `git diff HEAD`, this counts
// untracked (new) files too — `git add -A` stages them, so they show up in
// the diff against HEAD (#72). The real index is never touched: GIT_INDEX_FILE
// points at a temp file we delete on return.
func diffAgainstWorktree(repoRoot string, diffArgs ...string) ([]byte, error) {
	idx, err := os.CreateTemp("", "mooncake-agent-index-*")
	if err != nil {
		return nil, fmt.Errorf("create scratch index: %w", err)
	}
	idxPath := idx.Name()
	_ = idx.Close()
	defer func() { _ = os.Remove(idxPath) }()

	run := func(args ...string) ([]byte, error) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoRoot
		// Scrub inherited GIT_* vars before pointing git at our scratch
		// index. git honors GIT_DIR/GIT_WORK_TREE over cmd.Dir, so when
		// the agent runs from inside a git hook (which exports them) the
		// read-tree/add -A/diff would operate on the parent repo and
		// `git add -A` would stage the wrong tree. Same hazard the rest
		// of the codebase guards (snapshot.gitCleanEnv, modules.scrubGitEnv).
		cmd.Env = append(scrubGitEnv(os.Environ()), "GIT_INDEX_FILE="+idxPath)
		return cmd.Output()
	}

	// Seed the scratch index with HEAD's tree, then stage every working-tree
	// change (tracked edits, deletions, AND untracked additions). The
	// subsequent `diff --cached HEAD` then reports the full set.
	if _, err := run("read-tree", "HEAD"); err != nil {
		return nil, fmt.Errorf("read-tree HEAD into scratch index: %w", err)
	}
	if _, err := run("add", "-A"); err != nil {
		return nil, fmt.Errorf("stage worktree into scratch index: %w", err)
	}
	// Exclude agent's own bookkeeping from the diff. Staging untracked files
	// (above) would otherwise surface the run's transient temp plan
	// (.mooncake-plan-*.yml, deleted right after collection) and the
	// .mooncake/ iteration logs — agent's internal state, not the work the
	// plan did. The leading "." is the required positive pathspec that the
	// :(exclude) magic subtracts from.
	args := append([]string{"diff", "--cached", "HEAD"}, diffArgs...)
	args = append(args, "--", ".", ":(glob,exclude).mooncake/**", ":(glob,exclude).mooncake-plan-*.yml")
	return run(args...)
}

func CollectChangedFiles(repoRoot string) ([]string, error) {
	out, err := diffAgainstWorktree(repoRoot, "--name-only")
	if err != nil {
		return nil, fmt.Errorf("failed to get changed files: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}

	sort.Strings(files)
	return files, nil
}

func CollectDiffStat(repoRoot string) (DiffStat, error) {
	out, err := diffAgainstWorktree(repoRoot, "--numstat")
	if err != nil {
		return DiffStat{}, fmt.Errorf("failed to get diff stat: %w", err)
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	stat := DiffStat{}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		stat.Files++

		if parts[0] != "-" {
			ins, err := strconv.Atoi(parts[0])
			if err == nil {
				stat.Insertions += ins
			}
		}

		if parts[1] != "-" {
			del, err := strconv.Atoi(parts[1])
			if err == nil {
				stat.Deletions += del
			}
		}
	}

	return stat, nil
}

// collectChangedOrEmpty is CollectChangedFiles with the error flattened to an
// empty slice: a failure to read the diff degrades to "no known dirt" rather
// than aborting the loop (#87). Used to capture the workspace's inherited
// baseline at loop start.
func collectChangedOrEmpty(repoRoot string) []string {
	changed, err := CollectChangedFiles(repoRoot)
	if err != nil {
		return nil
	}
	return changed
}

// fileSet builds a membership set from a changed-files slice. Returns nil for
// an empty input — a nil set is a valid empty lookup (changedBeyondBaseline
// then counts every change).
func fileSet(files []string) map[string]bool {
	if len(files) == 0 {
		return nil
	}
	s := make(map[string]bool, len(files))
	for _, f := range files {
		s[f] = true
	}
	return s
}

// changedBeyondBaseline reports whether `changed` includes any file not already
// in baseline — i.e. whether the iteration touched something the workspace
// didn't arrive with (#87). With a nil/empty baseline every changed file
// counts, preserving the original "any change vs HEAD" semantics on a clean
// workspace. Filename-level: an agent edit to a file that was *already* dirty
// at loop start isn't distinguished (treated as inherited), an acceptable edge
// for the convergence heuristic.
func changedBeyondBaseline(changed []string, baseline map[string]bool) bool {
	for _, f := range changed {
		if !baseline[f] {
			return true
		}
	}
	return false
}
