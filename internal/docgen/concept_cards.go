package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// DefaultConceptDirs is the curated list of concept packages whose
// README.md (and, in a follow-up, package godoc) is rendered as a
// concepts/<name>.md page. The list is deliberately short — only
// directories whose purpose is conceptual ("what is X / how does X
// work"), not utility/leaf packages.
//
// New packages are added by amending this list rather than by walking
// internal/* automatically, because most internal/* directories are
// implementation detail (e.g. httputil, pathquery, queryio) whose
// godoc isn't user-facing.
var DefaultConceptDirs = []string{
	"internal/actions",
	"internal/config",
	"internal/effects",
	"internal/events",
	"internal/executor",
	"internal/facts",
	"internal/modules",
	"internal/plan",
	"internal/presets",
	"internal/schemagen",
	"internal/security",
}

// writeConceptCards emits dist/docs/concepts/<slug>.md per concept
// package that has a README.md. Packages without a README are
// silently skipped — the seed step (scripts/docs/seed-concept-readmes.sh)
// is what populates them; until that runs there's no prose to render.
//
// The page content is the README contents verbatim, prefixed with a
// YAML front-matter block so agents can grep predictably.
//
// Returns the list of files written.
func (g *Generator) writeConceptCards(outDir string, dirs []string) ([]string, error) {
	if len(dirs) == 0 {
		dirs = DefaultConceptDirs
	}

	conceptDir := filepath.Join(outDir, "concepts")
	if err := os.MkdirAll(conceptDir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for _, dir := range dirs {
		readme := filepath.Join(dir, "README.md")
		data, err := os.ReadFile(readme) // #nosec G304 -- dir from curated list, fixed suffix
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, fmt.Errorf("read %s: %w", readme, err)
		}

		slug := conceptSlug(dir)
		path := filepath.Join(conceptDir, slug+".md")
		body := renderConceptCard(g, slug, dir, data)
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil { // #nosec G306 -- doc file, readable
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}
	sort.Strings(written)
	return written, nil
}

// conceptSlug derives the page slug from a package directory.
// "internal/executor" → "executor"; "internal/foo/bar" → "foo-bar".
// Paths outside internal/ (e.g. absolute paths used in tests) fall back
// to the leaf name so the dist/docs/concepts/ tree stays flat and
// predictable regardless of where the caller invoked from.
func conceptSlug(dir string) string {
	if strings.HasPrefix(dir, "internal/") {
		return strings.ReplaceAll(strings.TrimPrefix(dir, "internal/"), "/", "-")
	}
	return filepath.Base(dir)
}

// renderConceptCard wraps the README body in stable front matter +
// generation footer so concept pages share the same skeleton as
// action and CLI cards.
func renderConceptCard(g *Generator, slug, sourceDir string, readme []byte) string {
	var b strings.Builder
	b.WriteString("---\n")
	b.WriteString("type: concept\n")
	fmt.Fprintf(&b, "name: %s\n", slug)
	fmt.Fprintf(&b, "source: %s\n", sourceDir)
	b.WriteString("---\n\n")
	b.Write(readme)
	if !strings.HasSuffix(string(readme), "\n") {
		b.WriteString("\n")
	}
	b.WriteString("\n")
	g.stamp(&b)
	return b.String()
}
