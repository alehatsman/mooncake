// Package snapshot provides repository snapshot functionality for agent context.
package snapshot

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

type Snapshot struct {
	Branch       string   `json:"branch"`
	Head         string   `json:"head"`
	Clean        bool     `json:"clean"`
	TopLevelDirs []string `json:"top_level_dirs"`
	Actions      []string `json:"actions"`
}

func Collect(repoRoot string) (*Snapshot, error) {
	snap := &Snapshot{}

	branch, err := gitBranch(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get git branch: %w", err)
	}
	snap.Branch = branch

	head, err := gitHead(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get git HEAD: %w", err)
	}
	snap.Head = head

	clean, err := gitClean(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to check git status: %w", err)
	}
	snap.Clean = clean

	dirs, err := topLevelDirs(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to get top-level directories: %w", err)
	}
	snap.TopLevelDirs = dirs

	snap.Actions = registeredActions()

	return snap, nil
}

func gitBranch(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitHead(repoRoot string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func gitClean(repoRoot string) (bool, error) {
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	out, err := cmd.Output()
	if err != nil {
		return false, err
	}
	return len(strings.TrimSpace(string(out))) == 0, nil
}

func topLevelDirs(repoRoot string) ([]string, error) {
	entries, err := os.ReadDir(repoRoot)
	if err != nil {
		return nil, err
	}

	var dirs []string
	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			dirs = append(dirs, entry.Name())
		}
	}

	sort.Strings(dirs)
	return dirs, nil
}

func registeredActions() []string {
	handlers := actions.List()
	names := make([]string, 0, len(handlers))
	for _, meta := range handlers {
		names = append(names, meta.Name)
	}
	sort.Strings(names)
	return names
}
