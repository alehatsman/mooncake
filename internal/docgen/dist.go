package docgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli/v2"
)

// DistOptions configures multi-file generation into a target directory.
//
// Unlike GenerateSection (single section → io.Writer), GenerateDist writes
// many files into a directory tree shaped for both MkDocs rendering and
// machine consumption (llms.txt, predictable per-topic paths).
type DistOptions struct {
	// OutDir is the root directory under which the docs tree is written
	// (e.g. "dist/docs"). The caller is responsible for clearing it first
	// when fresh state is desired — GenerateDist does not prune.
	OutDir string

	// PresetsDir is the directory containing preset.yml files for the
	// preset-examples section. Defaults to "presets" when empty.
	PresetsDir string

	// CLIRoot, when non-nil, drives generation of dist/docs/cli/<command>.md
	// pages by walking the urfave/cli command tree.
	CLIRoot *cli.App

	// ConceptDirs lists internal/<pkg> directories whose README.md is
	// rendered as dist/docs/concepts/<slug>.md. Empty falls back to
	// DefaultConceptDirs (defined in concept_cards.go).
	ConceptDirs []string

	// EnableAPI controls whether the gomarkdoc-driven API reference pages
	// under dist/docs/api/ are generated. When true, gomarkdoc must be on
	// PATH (api.go skips with a warning otherwise).
	EnableAPI bool
}

// GenerateDist regenerates the docs tree under opts.OutDir.
//
// Returns the list of absolute paths written (sorted), so callers can
// emit summary lines or feed a drift-check pass. The directory is not
// cleared before writing — caller responsibility, to keep this function
// pure-additive and testable.
func (g *Generator) GenerateDist(opts DistOptions) ([]string, error) {
	if opts.OutDir == "" {
		return nil, fmt.Errorf("DistOptions.OutDir is required")
	}
	if opts.PresetsDir == "" {
		opts.PresetsDir = "presets"
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir %s: %w", opts.OutDir, err)
	}

	var written []string

	// 1. Per-action cards under actions/<name>.md.
	paths, err := g.writeActionCards(opts.OutDir)
	if err != nil {
		return nil, fmt.Errorf("action cards: %w", err)
	}
	written = append(written, paths...)

	// 2. Schema reference (single page).
	p, err := g.writeSingleFile(opts.OutDir, "schema.md", g.generateSchemaDoc)
	if err != nil {
		return nil, fmt.Errorf("schema: %w", err)
	}
	written = append(written, p)

	// 3. Properties reference (single page; per-action schema tables).
	p, err = g.writeSingleFile(opts.OutDir, "properties.md", g.generateActionProperties)
	if err != nil {
		return nil, fmt.Errorf("properties: %w", err)
	}
	written = append(written, p)

	// 4. Platform-support matrix (single page).
	p, err = g.writeSingleFile(opts.OutDir, "platforms.md", g.generatePlatformMatrix)
	if err != nil {
		return nil, fmt.Errorf("platforms: %w", err)
	}
	written = append(written, p)

	// 5. Capabilities matrix (single page).
	p, err = g.writeSingleFile(opts.OutDir, "capabilities.md", g.generateCapabilities)
	if err != nil {
		return nil, fmt.Errorf("capabilities: %w", err)
	}
	written = append(written, p)

	// 6. Preset examples — only when PresetsDir exists. The bare-repo
	// `mooncake presets` CLI was retired (commit eee2c15f) and the
	// presets/ directory is gone from this checkout; the single-section
	// generator stays available for downstream consumers that still
	// curate their own presets tree, but the dist tree skips this page
	// when there's nothing to render.
	if _, statErr := os.Stat(opts.PresetsDir); statErr == nil {
		p, err = g.writeSingleFile(opts.OutDir, "presets.md", func(w io.Writer) error {
			return g.generatePresetExamples(w, opts.PresetsDir)
		})
		if err != nil {
			return nil, fmt.Errorf("presets: %w", err)
		}
		written = append(written, p)
	}

	// 7. Concept cards — one page per curated internal/<pkg>/README.md.
	paths, err = g.writeConceptCards(opts.OutDir, opts.ConceptDirs)
	if err != nil {
		return nil, fmt.Errorf("concept cards: %w", err)
	}
	written = append(written, paths...)

	// 8. CLI reference — walks the urfave/cli command tree when supplied.
	if opts.CLIRoot != nil {
		paths, err = g.writeCLITree(opts.OutDir, opts.CLIRoot)
		if err != nil {
			return nil, fmt.Errorf("cli tree: %w", err)
		}
		written = append(written, paths...)
	}

	// 9. API reference (gomarkdoc) — opt-in via EnableAPI; gracefully
	// skips with a warning when gomarkdoc isn't on PATH.
	if opts.EnableAPI {
		paths, err = g.writeAPIReference(opts.OutDir)
		if err != nil {
			return nil, fmt.Errorf("api reference: %w", err)
		}
		written = append(written, paths...)
	}

	// 10. Index pages — index.md + llms.txt are derived from the
	// already-written file list, so adding emitters above is reflected
	// automatically.
	paths, err = g.writeIndex(opts.OutDir, written)
	if err != nil {
		return nil, fmt.Errorf("index: %w", err)
	}
	written = append(written, paths...)

	// 11. SUMMARY.md for mkdocs-literate-nav — same derivation.
	navPath, err := g.writeNav(opts.OutDir, written)
	if err != nil {
		return nil, fmt.Errorf("nav: %w", err)
	}
	written = append(written, navPath)

	sort.Strings(written)
	return written, nil
}

// writeSingleFile invokes an existing io.Writer-style section generator
// and writes its output to outDir/relPath. Returns the path written.
func (g *Generator) writeSingleFile(outDir, relPath string, fn func(io.Writer) error) (string, error) {
	abs := filepath.Join(outDir, relPath)
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return "", err
	}
	f, err := os.Create(abs) // #nosec G304 -- path derived from caller-controlled outDir
	if err != nil {
		return "", err
	}
	if err := fn(f); err != nil {
		_ = f.Close()
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("close %s: %w", abs, err)
	}
	return abs, nil
}
