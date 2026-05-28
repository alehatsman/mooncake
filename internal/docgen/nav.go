package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// writeNav emits dist/docs/SUMMARY.md in the format consumed by the
// mkdocs-literate-nav plugin. The plugin reads SUMMARY.md and uses it
// as the site navigation, freeing mkdocs.yml from hand-curated nav
// entries.
//
// Format (one level of nesting per section):
//
//   - [Home](index.md)
//   - Actions:
//   - [file.copy](actions/file_copy.md)
//   - [shell](actions/shell.md)
//   - Concepts:
//   - [planner](concepts/plan.md)
//   - ...
//
// Returns the SUMMARY.md path.
func (g *Generator) writeNav(outDir string, written []string) (string, error) {
	groups := groupBySection(outDir, written)

	var b strings.Builder
	b.WriteString("- [Home](index.md)\n")

	for _, s := range groups {
		fmt.Fprintf(&b, "- %s:\n", s.Name)
		for _, e := range s.Files {
			fmt.Fprintf(&b, "    - [%s](%s)\n", e.Title, e.RelPath)
		}
	}

	path := filepath.Join(outDir, "SUMMARY.md")
	if err := os.WriteFile(path, []byte(b.String()), 0o644); err != nil { // #nosec G306
		return "", fmt.Errorf("write SUMMARY.md: %w", err)
	}
	return path, nil
}
