package docgen

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/alehatsman/mooncake/internal/actions"
)

// writeActionCards emits one markdown file per registered action into
// outDir/actions/<safe-name>.md. Each card is self-contained and follows
// the same skeleton (YAML front matter + fixed headings) so agents can
// parse predictably.
//
// Returns the list of files written.
func (g *Generator) writeActionCards(outDir string) ([]string, error) {
	actionDir := filepath.Join(outDir, "actions")
	if err := os.MkdirAll(actionDir, 0o755); err != nil {
		return nil, err
	}

	// Load schema once so we can attach a properties table per action.
	schema, err := loadSchema()
	if err != nil {
		return nil, fmt.Errorf("load schema: %w", err)
	}
	propsByName := map[string]ActionProperties{}
	for _, p := range extractActionProperties(schema) {
		propsByName[p.Name] = p
	}

	var written []string
	for _, a := range g.getActions() {
		path := filepath.Join(actionDir, safeActionFilename(a.Name)+".md")
		f, err := os.Create(path) // #nosec G304 -- outDir controlled by caller, name sanitized
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", path, err)
		}
		err = g.writeActionCard(f, a, propsByName[a.Name])
		if cerr := f.Close(); cerr != nil && err == nil {
			err = cerr
		}
		if err != nil {
			return nil, fmt.Errorf("write %s: %w", path, err)
		}
		written = append(written, path)
	}
	sort.Strings(written)
	return written, nil
}

// writeActionCard renders a single action card.
//
// Skeleton (stable, used by both humans and agents):
//
//	---
//	type: action
//	name: <name>
//	category: <category>
//	platforms: [linux, darwin, windows, ...]
//	capabilities: { dry_run, become, check, diff, cost, reverse, permissions }
//	---
//	# <name>
//	<description>
//	## Properties
//	## Examples
//	## Platform Support
//	## Events Emitted
//	<!-- generated metadata footer -->
func (g *Generator) writeActionCard(w io.Writer, meta actions.ActionMetadata, props ActionProperties) error {
	platforms := meta.SupportedPlatforms
	if len(platforms) == 0 {
		// Empty SupportedPlatforms means "all" — list the canonical set
		// so the front matter is honest about what "all" implies today.
		platforms = []string{"linux", "darwin", "windows", "freebsd"}
	}

	// Front matter — keys ordered for stable diffing.
	write(w, "---\n")
	write(w, "type: action\n")
	write(w, "name: %s\n", meta.Name)
	if meta.Category != "" {
		write(w, "category: %s\n", meta.Category)
	}
	if meta.Version != "" {
		write(w, "version: %s\n", meta.Version)
	}
	write(w, "platforms: [%s]\n", strings.Join(platforms, ", "))
	write(w, "capabilities:\n")
	write(w, "  dry_run: %t\n", meta.SupportsDryRun)
	write(w, "  become: %t\n", meta.SupportsBecome)
	write(w, "  check: %t\n", meta.ImplementsCheck)
	write(w, "  diff: %t\n", meta.ImplementsDiff)
	write(w, "  cost: %t\n", meta.ImplementsCost)
	write(w, "  reverse: %t\n", meta.ImplementsReverse)
	write(w, "  permissions: %t\n", meta.ImplementsPermissions)
	if meta.CaptureInPlan {
		write(w, "  capture_in_plan: true\n")
	}
	write(w, "---\n\n")

	// Title and lead description.
	write(w, "# %s\n\n", meta.Name)
	desc := meta.Description
	if desc == "" {
		desc = props.Description
	}
	if desc != "" {
		write(w, "%s\n\n", strings.TrimSpace(desc))
	}

	// Properties table — drawn from the embedded JSON schema, which is
	// itself generated from Go struct tags in internal/config (so this
	// chain ends at one source of truth).
	if len(props.Properties) > 0 {
		write(w, "## Properties\n\n")
		write(w, "| Property | Type | Required | Description |\n")
		write(w, "|----------|------|----------|-------------|\n")
		for _, p := range props.Properties {
			required := "No"
			if p.Required {
				required = "**Yes**"
			}
			d := p.Description
			if d == "" {
				d = "-"
			}
			if p.Default != "" {
				d += fmt.Sprintf(" (default: `%s`)", p.Default)
			}
			if len(p.Enum) > 0 {
				d += fmt.Sprintf(" (allowed: `%s`)", strings.Join(p.Enum, ", "))
			}
			write(w, "| `%s` | %s | %s | %s |\n", p.Name, p.Type, required, d)
		}
		write(w, "\n")
	}

	// Examples come straight from the handler's metadata. Each entry is a
	// complete YAML step block; render verbatim inside a fenced code block.
	if len(meta.Examples) > 0 {
		write(w, "## Examples\n\n")
		for _, ex := range meta.Examples {
			write(w, "```yaml\n%s\n```\n\n", strings.TrimSpace(ex))
		}
	}

	// Platform support — flat list mirroring the front matter.
	write(w, "## Platform Support\n\n")
	write(w, "%s\n\n", strings.Join(platforms, ", "))

	// Events — most actions emit nothing; only render the heading when
	// there's something to say, to avoid empty sections cluttering the page.
	if len(meta.EmitsEvents) > 0 {
		write(w, "## Events Emitted\n\n")
		for _, e := range meta.EmitsEvents {
			write(w, "- `%s`\n", e)
		}
		write(w, "\n")
	}

	// Footer is the same as other generated pages so the drift-check
	// regex in scripts/docs-check.sh strips it consistently.
	write(w, "<!-- Generated by mooncake docs generate -->\n")
	write(w, "<!-- Version: %s | Generated: %s -->\n",
		g.Version,
		g.Timestamp.Format("2006-01-02 15:04:05 MST"))

	return nil
}

// safeActionFilename produces a filesystem-safe filename stem from an
// action name. Action names like "file.copy" or "observe.cpu" become
// "file_copy" / "observe_cpu" so the rendered URL is portable across
// MkDocs themes and filesystems.
func safeActionFilename(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}
