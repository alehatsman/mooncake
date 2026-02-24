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

const iterationDir = ".mooncake/iterations"

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

func ComputePlanHash(planBytes []byte) string {
	hash := sha256.Sum256(planBytes)
	return hex.EncodeToString(hash[:])
}

func CollectChangedFiles(repoRoot string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
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
	cmd := exec.Command("git", "diff", "--numstat", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
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
