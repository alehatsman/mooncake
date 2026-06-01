package docgen

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
)

// gomarkdoc embeds godoc source links (`## func [Name](<url>/blob/<branch>/
// <file>#L<n>)`) derived from the local git checkout. Left to auto-detect,
// that base depends on the environment: a feature-branch worktree, or a
// checkout whose `origin` isn't the github remote (e.g. a `moongit` mirror),
// makes gomarkdoc emit link-less output — a ~990-line spurious diff that
// breaks docs-check and, via the pre-commit auto-regen, silently regresses
// the committed docs. Pin the repository identity explicitly so the api/*.md
// output is byte-identical regardless of branch/remote. These match the go
// module path and the repo's default branch.
const (
	apiRepoURL           = "https://github.com/alehatsman/mooncake"
	apiRepoDefaultBranch = "main"
	// apiRepoPath is the path from the repo root to the dir gomarkdoc runs
	// from. writeAPIReference runs from the module root, so this is "/".
	apiRepoPath = "/"
)

// DefaultAPIPackages is the curated list of Go packages whose godoc is
// rendered as dist/docs/api/<slug>.md via gomarkdoc. The list is chosen
// for *consumer-facing* surface: SDK-grade packages, the action/handler
// interface, and the kernel boundary. Implementation-detail packages
// are intentionally excluded — gomarkdoc on them produces noise.
//
// Each entry is the module-relative package path (no leading slash).
var DefaultAPIPackages = []string{
	"internal/actions",
	"internal/config",
	"internal/effects",
	"internal/events",
	"internal/executor",
	"internal/facts",
	"internal/logger",
	"internal/modules",
	"internal/plan",
	"internal/presets",
	"internal/schemagen",
}

// writeAPIReference shells out to `gomarkdoc` to produce one markdown
// page per package in DefaultAPIPackages, written under outDir/api/.
//
// If `gomarkdoc` is not on PATH, the function returns nil + a non-fatal
// hint on stderr — callers can choose to surface or swallow. We don't
// want the whole dist tree to fail when one optional tool is missing.
func (g *Generator) writeAPIReference(outDir string) ([]string, error) {
	if _, err := exec.LookPath("gomarkdoc"); err != nil {
		fmt.Fprintln(os.Stderr,
			"warning: gomarkdoc not found on PATH — skipping dist/docs/api/. "+
				"Install via `task docs-tools-install` (or "+
				"`go install github.com/princjef/gomarkdoc/cmd/gomarkdoc@v0.4.1`).")
		return nil, nil
	}

	apiDir := filepath.Join(outDir, "api")
	if err := os.MkdirAll(apiDir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for _, pkg := range DefaultAPIPackages {
		slug := apiSlug(pkg)
		out := filepath.Join(apiDir, slug+".md")

		// gomarkdoc writes directly to -o; we trust its output (it
		// inserts its own DO-NOT-EDIT marker we keep, which is also
		// stripped by scripts/docs-check.sh just like our own footers).
		// #nosec G204 -- argv is built from our curated package list, no user input
		cmd := exec.Command("gomarkdoc", gomarkdocArgs(pkg, out)...)
		if combined, err := cmd.CombinedOutput(); err != nil {
			return nil, fmt.Errorf("gomarkdoc ./%s: %w\n%s", pkg, err, combined)
		}
		written = append(written, out)
	}
	sort.Strings(written)
	return written, nil
}

// gomarkdocArgs builds the gomarkdoc argv for one package. The
// --repository.* overrides pin the source-link base (see the const block)
// so api/*.md is byte-identical regardless of the local branch/remote.
func gomarkdocArgs(pkg, out string) []string {
	return []string{
		"--repository.url", apiRepoURL,
		"--repository.default-branch", apiRepoDefaultBranch,
		"--repository.path", apiRepoPath,
		"-o", out, "./" + pkg,
	}
}

// apiSlug converts a package path into a flat filename slug.
// "internal/executor" → "executor"; "internal/foo/bar" → "foo-bar".
func apiSlug(pkg string) string {
	s := pkg
	if len(s) > len("internal/") && s[:len("internal/")] == "internal/" {
		s = s[len("internal/"):]
	}
	out := []byte(s)
	for i, c := range out {
		if c == '/' {
			out[i] = '-'
		}
	}
	return string(out)
}
