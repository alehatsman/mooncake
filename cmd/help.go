package main

import (
	"io"

	"github.com/urfave/cli/v2"

	kernelcmd "github.com/alehatsman/mooncake/cmd/kernel"
)

// tierOrder is the order categories appear in `mooncake --help`.
//
// urfave/cli sorts VisibleCategories alphabetically, which would put
// `Daemon` above `Run` — what an operator actually opened the help for.
// The tiers are the whole point of the regroup, so the order is stated
// here rather than left to the alphabet.
//
// A category not listed here still renders, after the listed ones, so a
// new tier can never silently vanish from help.
var tierOrder = []string{
	kernelcmd.CategoryRun,
	kernelcmd.CategoryInspect,
	kernelcmd.CategoryFleet,
	kernelcmd.CategoryManage,
	kernelcmd.CategoryDaemon,
}

// orderedCategories returns the app's visible categories in tierOrder,
// with any unlisted category appended in urfave's own (alphabetical)
// order.
func orderedCategories(app *cli.App) []cli.CommandCategory {
	// VisibleCategories dereferences a field urfave only populates in
	// Setup, so it panics on an App that hasn't run yet. Setup is
	// idempotent (it guards on didSetup), so calling it here costs
	// nothing on the live path and makes the helper safe to call from a
	// test that builds an App without running it.
	app.Setup()

	visible := app.VisibleCategories()
	byName := make(map[string]cli.CommandCategory, len(visible))
	for _, c := range visible {
		byName[c.Name()] = c
	}

	out := make([]cli.CommandCategory, 0, len(visible))
	placed := make(map[string]bool, len(tierOrder))
	for _, name := range tierOrder {
		if c, ok := byName[name]; ok {
			out = append(out, c)
			placed[name] = true
		}
	}
	for _, c := range visible {
		if !placed[c.Name()] {
			out = append(out, c)
		}
	}
	return out
}

// tieredCommandsTemplate replaces urfave's visibleCommandCategoryTemplate
// with one that walks orderedCategories. Uncategorised commands (help)
// fall through to the flat listing, as before.
const tieredCommandsTemplate = `{{range orderedCategories .}}{{if .Name}}
   {{.Name}}:{{range .VisibleCommands}}
     {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{else}}{{range .VisibleCommands}}
   {{join .Names ", "}}{{"\t"}}{{.Usage}}{{end}}{{end}}{{end}}`

// installTieredHelp swaps the category block in urfave's app help
// template for the ordered one and registers the `orderedCategories`
// template function the replacement needs.
func installTieredHelp() {
	cli.AppHelpTemplate = replaceOnce(
		cli.AppHelpTemplate,
		`{{template "visibleCommandCategoryTemplate" .}}`,
		tieredCommandsTemplate,
	)

	base := cli.HelpPrinter
	cli.HelpPrinter = func(w io.Writer, templ string, data interface{}) {
		if app, ok := data.(*cli.App); ok {
			cli.HelpPrinterCustom(w, templ, data, map[string]interface{}{
				"orderedCategories": func(_ interface{}) []cli.CommandCategory {
					return orderedCategories(app)
				},
			})
			return
		}
		// Command- and subcommand-level help doesn't use the tiered
		// block; hand it back to urfave untouched.
		base(w, templ, data)
	}
}

// replaceOnce substitutes the first occurrence of old in s. Returns s
// unchanged when old is absent — an urfave upgrade that renames the
// template block degrades to alphabetical tiers rather than a panic or
// a help page with a literal template fragment in it.
func replaceOnce(s, old, new string) string {
	for i := 0; i+len(old) <= len(s); i++ {
		if s[i:i+len(old)] == old {
			return s[:i] + new + s[i+len(old):]
		}
	}
	return s
}
