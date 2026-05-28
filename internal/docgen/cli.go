package docgen

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
)

// writeCLITree walks a urfave/cli App (or Command tree) and emits one
// markdown page per (sub)command into outDir/cli/. Each page describes:
//
//   - Usage, full command path, aliases, description
//   - Flags (name, type, default, usage)
//   - Direct subcommands (linked to their own pages)
//
// Hidden commands are skipped so the public doc surface matches what
// `mooncake --help` exposes.
//
// Pass the *cli.App used by the binary; the function recurses through
// app.Commands. Returns the list of files written.
func (g *Generator) writeCLITree(outDir string, app *cli.App) ([]string, error) {
	if app == nil {
		return nil, nil
	}

	cliDir := filepath.Join(outDir, "cli")
	if err := os.MkdirAll(cliDir, 0o755); err != nil {
		return nil, err
	}

	var written []string
	for _, cmd := range app.Commands {
		paths, err := g.writeCLICommand(cliDir, []string{app.Name}, cmd)
		if err != nil {
			return nil, err
		}
		written = append(written, paths...)
	}
	sort.Strings(written)
	return written, nil
}

// writeCLICommand renders a single command and recurses into its
// subcommands. parentPath is the full ancestry as a slice (e.g.
// ["mooncake", "docs"]) — used both to build the page title and to
// derive the filename slug.
func (g *Generator) writeCLICommand(cliDir string, parentPath []string, cmd *cli.Command) ([]string, error) {
	if cmd == nil || cmd.Hidden {
		return nil, nil
	}
	// urfave/cli auto-attaches a "help" subcommand to every command.
	// Don't emit a page for it — `mooncake <foo> help` is identical to
	// `mooncake <foo> --help`, which is universal and not worth N pages.
	if cmd.Name == "help" || cmd.Name == "h" {
		return nil, nil
	}

	fullPath := append(append([]string{}, parentPath...), cmd.Name)
	slug := strings.Join(fullPath[1:], "_") // drop the root binary name from filenames
	if slug == "" {
		slug = cmd.Name
	}
	path := filepath.Join(cliDir, slug+".md")

	body := g.renderCLIPage(fullPath, cmd)
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil { // #nosec G306 -- doc file
		return nil, fmt.Errorf("write %s: %w", path, err)
	}
	written := []string{path}

	for _, sub := range cmd.Subcommands {
		paths, err := g.writeCLICommand(cliDir, fullPath, sub)
		if err != nil {
			return nil, err
		}
		written = append(written, paths...)
	}
	return written, nil
}

// renderCLIPage produces the markdown body for one command page.
func (g *Generator) renderCLIPage(fullPath []string, cmd *cli.Command) string {
	var b strings.Builder

	// Front matter — agent-friendly machine read.
	b.WriteString("---\n")
	b.WriteString("type: command\n")
	fmt.Fprintf(&b, "name: %s\n", strings.Join(fullPath, " "))
	if len(cmd.Aliases) > 0 {
		fmt.Fprintf(&b, "aliases: [%s]\n", strings.Join(cmd.Aliases, ", "))
	}
	b.WriteString("---\n\n")

	// Title and one-liner.
	fmt.Fprintf(&b, "# %s\n\n", strings.Join(fullPath, " "))
	if cmd.Usage != "" {
		fmt.Fprintf(&b, "%s\n\n", cmd.Usage)
	}

	// Long description, if any.
	if cmd.Description != "" {
		b.WriteString("## Description\n\n")
		fmt.Fprintf(&b, "%s\n\n", strings.TrimSpace(cmd.Description))
	}

	// Usage text — the literal form shown in --help.
	if cmd.UsageText != "" {
		b.WriteString("## Usage\n\n```\n")
		b.WriteString(strings.TrimSpace(cmd.UsageText))
		b.WriteString("\n```\n\n")
	}

	// Flags table. Each flag is a Flag interface; we extract a few
	// common fields via type assertion onto concrete flag types.
	if len(cmd.Flags) > 0 {
		b.WriteString("## Flags\n\n")
		b.WriteString("| Flag | Type | Default | Description |\n")
		b.WriteString("|------|------|---------|-------------|\n")
		for _, f := range cmd.Flags {
			name, typ, def, usage := describeFlag(f)
			fmt.Fprintf(&b, "| `%s` | %s | %s | %s |\n", name, typ, def, usage)
		}
		b.WriteString("\n")
	}

	// Subcommand index — links into the same cli/ directory.
	if len(cmd.Subcommands) > 0 {
		b.WriteString("## Subcommands\n\n")
		visible := []*cli.Command{}
		for _, sub := range cmd.Subcommands {
			if sub.Hidden || sub.Name == "help" || sub.Name == "h" {
				continue
			}
			visible = append(visible, sub)
		}
		sort.Slice(visible, func(i, j int) bool { return visible[i].Name < visible[j].Name })
		for _, sub := range visible {
			subSlug := strings.Join(append(fullPath[1:], sub.Name), "_")
			fmt.Fprintf(&b, "- [%s](%s.md) — %s\n", sub.Name, subSlug, sub.Usage)
		}
		b.WriteString("\n")
	}

	g.stamp(&b)
	return b.String()
}

// describeFlag extracts (name, type, default, usage) from a cli.Flag.
// urfave/cli's Flag is a loose interface; the concrete types embed the
// metadata so we type-switch over the ones we actually use in this
// codebase. Unknown types fall back to the Flag's Names()/String().
func describeFlag(f cli.Flag) (name, typ, def, usage string) {
	// Default sensible values
	names := f.Names()
	if len(names) > 0 {
		name = "--" + names[0]
		if len(names) > 1 {
			extras := make([]string, 0, len(names)-1)
			for _, n := range names[1:] {
				extras = append(extras, "-"+n)
			}
			name += " / " + strings.Join(extras, " / ")
		}
	}
	typ = "value"
	def = "-"

	switch v := f.(type) {
	case *cli.StringFlag:
		typ = "string"
		if v.Value != "" {
			def = "`" + v.Value + "`"
		}
		usage = v.Usage
	case *cli.BoolFlag:
		typ = "bool"
		if v.Value {
			def = "true"
		} else {
			def = "false"
		}
		usage = v.Usage
	case *cli.IntFlag:
		typ = "int"
		def = fmt.Sprintf("`%d`", v.Value)
		usage = v.Usage
	case *cli.Int64Flag:
		typ = "int64"
		def = fmt.Sprintf("`%d`", v.Value)
		usage = v.Usage
	case *cli.StringSliceFlag:
		typ = "[]string"
		usage = v.Usage
	case *cli.DurationFlag:
		typ = "duration"
		if v.Value != 0 {
			def = "`" + v.Value.String() + "`"
		}
		usage = v.Usage
	default:
		usage = "-"
	}
	if usage == "" {
		usage = "-"
	}
	return
}
