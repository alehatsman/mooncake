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

	// Load per-action READMEs once. Missing dir is non-fatal — the
	// rendered cards still ship without the Notes section.
	notes, err := loadActionReadmes("internal/actions")
	if err != nil {
		return nil, fmt.Errorf("load action READMEs: %w", err)
	}

	var written []string
	for _, a := range g.getActions() {
		path := filepath.Join(actionDir, safeActionFilename(a.Name)+".md")
		f, err := os.Create(path) // #nosec G304 -- outDir controlled by caller, name sanitized
		if err != nil {
			return nil, fmt.Errorf("create %s: %w", path, err)
		}
		err = g.writeActionCard(f, a, propsByName[a.Name], notes[a.Name])
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

// stubMarker tells the action_cards emitter that a README is still
// boilerplate and should not be inlined into the rendered card. When
// real prose is added, the author deletes this marker; the README's
// body then shows up as a "Notes" section on the action page.
const stubMarker = "<!-- mooncake:stub -->"

// loadActionReadmes walks internal/actions/<dir>/README.md, parses the
// `action: <name>` front matter, and returns a map[action.Name]body —
// where body is the README contents with YAML front matter and the
// stub marker stripped. Files containing the stub marker are skipped
// entirely (treated as "no notes available").
//
// Returns an empty map (no error) if the actions root is missing —
// callers can still render cards without notes.
func loadActionReadmes(actionsRoot string) (map[string]string, error) {
	out := map[string]string{}

	entries, err := os.ReadDir(actionsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			return out, nil
		}
		return nil, err
	}

	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		readme := filepath.Join(actionsRoot, e.Name(), "README.md")
		data, err := os.ReadFile(readme) // #nosec G304 -- enumerated from actionsRoot
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		body := string(data)
		if strings.Contains(body, stubMarker) {
			continue
		}
		name, payload := splitReadme(body)
		if name == "" {
			continue
		}
		out[name] = payload
	}
	return out, nil
}

// splitReadme parses a README body: pulls the `action: <name>` value
// out of the YAML front-matter block, strips both the front matter
// and the README's own H1 title (which would duplicate the rendered
// card's title), and returns (name, remaining-body).
//
// Returns ("", "") when the file has no front matter or no action key.
func splitReadme(body string) (name, payload string) {
	if !strings.HasPrefix(body, "---\n") {
		return "", ""
	}
	rest := body[len("---\n"):]
	end := strings.Index(rest, "\n---\n")
	if end < 0 {
		return "", ""
	}
	header := rest[:end]
	payload = strings.TrimLeft(rest[end+len("\n---\n"):], "\n")

	for _, line := range strings.Split(header, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "action:") {
			continue
		}
		name = strings.TrimSpace(strings.TrimPrefix(line, "action:"))
		break
	}
	if name == "" {
		return "", ""
	}

	// Drop the first H1 — it duplicates the action card's own title.
	if strings.HasPrefix(payload, "# ") {
		if nl := strings.Index(payload, "\n"); nl >= 0 {
			payload = strings.TrimLeft(payload[nl+1:], "\n")
		}
	}
	return name, payload
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
func (g *Generator) writeActionCard(w io.Writer, meta actions.ActionMetadata, props ActionProperties, notes string) error {
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

	// Author-written notes from internal/actions/<dir>/README.md. Empty
	// for stub READMEs (skipped at load time). The body has its front
	// matter and H1 already stripped — render verbatim.
	if trimmed := strings.TrimSpace(notes); trimmed != "" {
		write(w, "## Notes\n\n%s\n\n", trimmed)
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

	g.stamp(w)
	return nil
}

// safeActionFilename produces a filesystem-safe filename stem from an
// action name. Action names like "file.copy" or "observe.cpu" become
// "file_copy" / "observe_cpu" so the rendered URL is portable across
// MkDocs themes and filesystems.
func safeActionFilename(name string) string {
	return strings.ReplaceAll(name, ".", "_")
}
