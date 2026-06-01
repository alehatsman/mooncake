package docgen

import (
	"strings"
	"testing"
)

// TestGomarkdocArgs_PinsRepositoryForDeterminism guards the fix for the
// environment-dependent api/*.md churn: gomarkdoc embeds godoc source links
// from the local git checkout, so without explicit --repository.* overrides
// the link base changes with branch/remote (a feature-branch worktree or a
// non-github origin strips the links, producing a ~990-line spurious diff and
// regressing the committed docs). The overrides must always be present.
func TestGomarkdocArgs_PinsRepositoryForDeterminism(t *testing.T) {
	args := gomarkdocArgs("internal/logger", "/tmp/out.md")
	joined := strings.Join(args, " ")

	pairs := map[string]string{
		"--repository.url":            apiRepoURL,
		"--repository.default-branch": apiRepoDefaultBranch,
		"--repository.path":           apiRepoPath,
	}
	for flag, want := range pairs {
		idx := indexOf(args, flag)
		if idx < 0 {
			t.Fatalf("gomarkdocArgs missing %s; got %q", flag, joined)
		}
		if idx+1 >= len(args) || args[idx+1] != want {
			t.Errorf("%s value = %q, want %q", flag, valueAfter(args, idx), want)
		}
	}

	// The package and output target must still be passed through.
	if idx := indexOf(args, "-o"); idx < 0 || args[idx+1] != "/tmp/out.md" {
		t.Errorf("missing/incorrect -o target in %q", joined)
	}
	if args[len(args)-1] != "./internal/logger" {
		t.Errorf("last arg = %q, want ./internal/logger", args[len(args)-1])
	}

	// The pinned URL must be the canonical github base, not empty — an empty
	// override would re-enable auto-detection and reintroduce the drift.
	if !strings.HasPrefix(apiRepoURL, "https://github.com/") {
		t.Errorf("apiRepoURL = %q, want a github URL", apiRepoURL)
	}
}

func indexOf(ss []string, target string) int {
	for i, s := range ss {
		if s == target {
			return i
		}
	}
	return -1
}

func valueAfter(ss []string, idx int) string {
	if idx+1 < len(ss) {
		return ss[idx+1]
	}
	return ""
}
