package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// writeIndex emits the top-level dist/docs/index.md page and the
// llms.txt machine-readable index. Both are derived from the list of
// files already written, so adding a new emitter is automatically
// reflected without touching this function.
//
// llms.txt follows the convention at https://llmstxt.org/ — one section
// per content kind, each entry: `- [Title](path) — one-line description`.
//
// Returns the paths of both files written.
func (g *Generator) writeIndex(outDir string, written []string) ([]string, error) {
	groups := groupBySection(outDir, written)

	indexPath := filepath.Join(outDir, "index.md")
	if err := os.WriteFile(indexPath, []byte(renderIndex(g, groups)), 0o644); err != nil { // #nosec G306
		return nil, fmt.Errorf("write index.md: %w", err)
	}

	llmsPath := filepath.Join(outDir, "llms.txt")
	if err := os.WriteFile(llmsPath, []byte(renderLLMs(g, groups)), 0o644); err != nil { // #nosec G306
		return nil, fmt.Errorf("write llms.txt: %w", err)
	}
	return []string{indexPath, llmsPath}, nil
}

// section is one named group of files (e.g. "Actions", "Concepts").
type section struct {
	Name  string
	Slug  string
	Entry string // one-line group description for index.md
	Files []indexEntry
}

type indexEntry struct {
	Title   string // page title
	RelPath string // path relative to outDir
}

// groupBySection partitions `written` paths into named sections based on
// their top-level subdirectory. Files at the dist root (schema.md,
// platforms.md, etc.) become the "Reference" section.
func groupBySection(outDir string, written []string) []section {
	bucket := map[string][]indexEntry{}
	for _, p := range written {
		rel, err := filepath.Rel(outDir, p)
		if err != nil {
			rel = p
		}
		// Skip the index files themselves to avoid self-reference loops
		// when this function runs again on a re-listing of the same dir.
		if rel == "index.md" || rel == "llms.txt" || rel == "SUMMARY.md" {
			continue
		}

		parts := strings.SplitN(rel, string(filepath.Separator), 2)
		sectionKey := "reference"
		if len(parts) == 2 {
			sectionKey = parts[0]
		}
		bucket[sectionKey] = append(bucket[sectionKey], indexEntry{
			Title:   titleFromPath(rel),
			RelPath: filepath.ToSlash(rel),
		})
	}

	for k := range bucket {
		sort.Slice(bucket[k], func(i, j int) bool {
			return bucket[k][i].Title < bucket[k][j].Title
		})
	}

	// Stable section ordering for index.md / llms.txt / SUMMARY.md.
	order := []section{
		{Name: "Actions", Slug: "actions", Entry: "One page per registered action: properties, examples, capabilities."},
		{Name: "Concepts", Slug: "concepts", Entry: "How the kernel, planner, executor, and other core systems work."},
		{Name: "CLI", Slug: "cli", Entry: "Reference for every `mooncake` subcommand and its flags."},
		{Name: "API", Slug: "api", Entry: "Go package reference generated from godoc."},
		{Name: "Reference", Slug: "reference", Entry: "Schema, properties, platform support, capability matrices."},
	}
	out := make([]section, 0, len(order))
	for _, s := range order {
		if entries, ok := bucket[s.Slug]; ok {
			s.Files = entries
			out = append(out, s)
		}
	}
	return out
}

// titleFromPath converts a path like "actions/file_copy.md" into a
// human-readable title. For action and CLI pages, dots return — e.g.
// "file_copy" → "file.copy". Concept and API slugs keep their hyphens
// since those map to filesystem paths, not action names.
func titleFromPath(rel string) string {
	base := strings.TrimSuffix(filepath.Base(rel), ".md")
	parent := filepath.Dir(rel)
	switch parent {
	case "actions", "cli":
		return strings.ReplaceAll(base, "_", ".")
	default:
		return base
	}
}

// renderIndex produces dist/docs/index.md — the MkDocs landing page.
func renderIndex(g *Generator, groups []section) string {
	var b strings.Builder
	b.WriteString("# Mooncake Documentation\n\n")
	b.WriteString("Declarative config-management tool. Docker-style safe execution runtime ")
	b.WriteString("with idempotency guarantees.\n\n")
	b.WriteString("This site is fully generated from code — every page is derived from a Go ")
	b.WriteString("source or registered handler metadata. If a page is wrong, the fix lives ")
	b.WriteString("next to the code, not here.\n\n")

	for _, s := range groups {
		fmt.Fprintf(&b, "## %s\n\n", s.Name)
		fmt.Fprintf(&b, "%s\n\n", s.Entry)
		for _, e := range s.Files {
			fmt.Fprintf(&b, "- [%s](%s)\n", e.Title, e.RelPath)
		}
		b.WriteString("\n")
	}

	g.stamp(&b)
	return b.String()
}

// renderLLMs produces llms.txt — the llmstxt.org-style enumeration of
// every page, intended for agent consumption. One entry per line keeps
// it grep-friendly.
func renderLLMs(g *Generator, groups []section) string {
	var b strings.Builder
	b.WriteString("# Mooncake\n\n")
	b.WriteString("> Declarative config-management tool. Generated documentation.\n\n")

	for _, s := range groups {
		fmt.Fprintf(&b, "## %s\n\n", s.Name)
		fmt.Fprintf(&b, "%s\n\n", s.Entry)
		for _, e := range s.Files {
			fmt.Fprintf(&b, "- [%s](%s)\n", e.Title, e.RelPath)
		}
		b.WriteString("\n")
	}

	g.stamp(&b)
	return b.String()
}
