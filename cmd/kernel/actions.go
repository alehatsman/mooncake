package kernel

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/schemagen"
)

// ActionsCommand returns the `mooncake actions` parent with its
// list / show subcommands.
func ActionsCommand() *cli.Command {
	return &cli.Command{
		Name:  "actions",
		Usage: "Manage and inspect actions",
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List all available actions with platform support",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text or json",
					},
				},
				Action: actionsListAction,
			},
			{
				Name:      "show",
				Usage:     "Show per-action documentation (params, platforms, capabilities, minimum example)",
				ArgsUsage: "<action-name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:    "format",
						Aliases: []string{"f"},
						Value:   "text",
						Usage:   "Output format: text, json, or yaml",
					},
				},
				Action: actionsShowAction,
			},
		},
	}
}

// actionsListAction lists all registered actions with their platform support.
func actionsListAction(c *cli.Context) error {
	format := c.String("format")

	// Validate format
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	// Get all registered actions
	actionsList := actions.List()

	// Sort by name for consistent output
	sort.Slice(actionsList, func(i, j int) bool {
		return actionsList[i].Name < actionsList[j].Name
	})

	// Output based on format
	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(actionsList)
	case outputFormatText:
		displayActionsTable(actionsList)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// displayActionsTable displays actions in a formatted table.
//
// proposal-05: DIFF/COST/REVERSE/PERM columns surface the spec-22
// four-method ABI per handler. Values come from ActionMetadata's
// Implements* bools, which Registry.List() populates centrally from
// the live interface satisfaction (so the table can't drift from
// what each handler actually implements).
func displayActionsTable(actionsList []actions.ActionMetadata) {
	const rowFmt = "%-15s %-10s %-25s %-5s %-5s %-5s %-5s %-7s %-5s\n"
	// Print header
	fmt.Printf(rowFmt,
		"ACTION", "CATEGORY", "PLATFORMS",
		"SUDO", "CHECK", "DIFF", "COST", "REVERSE", "PERM")
	fmt.Println(strings.Repeat("-", 95))

	// Print each action
	for _, meta := range actionsList {
		// Format platforms
		platforms := "all"
		if len(meta.SupportedPlatforms) > 0 {
			platforms = strings.Join(meta.SupportedPlatforms, ",")
			if len(platforms) > 23 {
				platforms = platforms[:20] + "..."
			}
		}

		fmt.Printf(rowFmt,
			meta.Name,
			meta.Category,
			platforms,
			yesNo(meta.RequiresSudo),
			yesNo(meta.ImplementsCheck),
			yesNo(meta.ImplementsDiff),
			yesNo(meta.ImplementsCost),
			yesNo(meta.ImplementsReverse),
			yesNo(meta.ImplementsPermissions))
	}
}

// yesNo renders a bool as the two-state token the actions-list table
// has used since the SUDO/CHECK columns shipped. Kept tiny + local
// because it's a display detail, not a vocabulary worth promoting.
func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}

// loadActionShowDefinition resolves <name> against the live registry
// and returns the matching ActionMetadata + a schemagen Definition
// generated with the same options actionsShowAction renders from.
// Extracted from actionsShowAction so the F047 regression test can
// drive the real lookup + generator path without forking a process
// or refactoring stdout away from the command entry point.
//
// StrictValidation is load-bearing for documentation here, not just
// validation: the generator gates description/enum/pattern enrichment
// behind this same flag (see internal/schemagen/generator.go
// applyKnownValidation + applyEnhancedDescription). Without it
// `actions show` rendered every parameter as a bare type with no
// description and no enum list (F047).
func loadActionShowDefinition(name string) (*actions.ActionMetadata, *schemagen.Definition, error) {
	// Pull the metadata via the same Registry.List() pipeline
	// `actions list` uses — the four spec-22 ABI capability bools
	// (proposal-05) are populated there from live interface
	// satisfaction, so the card stays in lockstep with the table
	// without per-call probing.
	var meta *actions.ActionMetadata
	all := actions.List()
	for i := range all {
		if all[i].Name == name {
			meta = &all[i]
			break
		}
	}
	if meta == nil {
		known := make([]string, 0, len(all))
		for _, m := range all {
			known = append(known, m.Name)
		}
		if suggestion := nearestActionName(name, known); suggestion != "" {
			return nil, nil, fmt.Errorf("unknown action %q (did you mean %q? try `mooncake actions list`)", name, suggestion)
		}
		return nil, nil, fmt.Errorf("unknown action %q (try `mooncake actions list`)", name)
	}

	gen := schemagen.NewGenerator(schemagen.GeneratorOptions{
		IncludeExtensions: true,
		StrictValidation:  true,
		OutputFormat:      "json",
	})
	schema, err := gen.Generate()
	if err != nil {
		return nil, nil, fmt.Errorf("generate schema: %w", err)
	}
	def, ok := schema.Definitions[name]
	if !ok {
		// Registry knew the name but schemagen didn't — shouldn't
		// happen for v1, but surface a clear message rather than
		// printing a card with no fields.
		return nil, nil, fmt.Errorf("action %q has no schema definition (registry/schemagen drift)", name)
	}
	return meta, def, nil
}

// actionsShowAction prints a per-action card (dx proposal-04). The
// per-action surface is "what parameters does this action take, what's
// required, what's the minimum example" — the question a user hits the
// moment after `actions list`. All data is already in the registry +
// schemagen; this is the rendering shell over both.
//
// --format text (default) prints a human-shaped card; json/yaml dump
// the schema Definition (already x-implements-*-decorated by
// proposal-05) so editors and agents can consume it directly.
func actionsShowAction(c *cli.Context) error {
	if c.NArg() == 0 {
		return fmt.Errorf("specify an action name; try `mooncake actions list`")
	}
	name := c.Args().First()
	format := c.String("format")

	// Validate format before doing any work — clearer error than a
	// switch-default panic at the bottom of the function.
	switch format {
	case outputFormatText, outputFormatJSON, "yaml":
	default:
		return fmt.Errorf("invalid format: %s (use 'text', 'json', or 'yaml')", format)
	}

	meta, def, err := loadActionShowDefinition(name)
	if err != nil {
		return err
	}

	switch format {
	case outputFormatText:
		renderActionShowText(meta, def, os.Stdout)
		return nil
	case outputFormatJSON:
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(def)
	case "yaml":
		return yaml.NewEncoder(os.Stdout).Encode(def)
	}
	return nil
}

// renderActionShowText writes the human-shaped per-action card to w.
// Exported-ish through tests; the rest of cmd doesn't need it.
func renderActionShowText(meta *actions.ActionMetadata, def *schemagen.Definition, w io.Writer) {
	title := meta.Name
	fmt.Fprintln(w, title)
	fmt.Fprintln(w, strings.Repeat("─", len(title)))
	if def.Description != "" {
		fmt.Fprintln(w, def.Description)
	} else if meta.Description != "" {
		fmt.Fprintln(w, meta.Description)
	}
	fmt.Fprintln(w)

	platforms := "all"
	if len(meta.SupportedPlatforms) > 0 {
		platforms = strings.Join(meta.SupportedPlatforms, ", ")
	}
	fmt.Fprintf(w, "Category:         %s\n", meta.Category)
	fmt.Fprintf(w, "Platforms:        %s\n", platforms)
	fmt.Fprintf(w, "Requires sudo:    %s\n", yesNo(meta.RequiresSudo))
	fmt.Fprintf(w, "Implements check: %s\n", yesNo(meta.ImplementsCheck))
	fmt.Fprintf(w, "Implements diff:  %s\n", yesNo(meta.ImplementsDiff))
	fmt.Fprintf(w, "Implements cost:  %s\n", yesNo(meta.ImplementsCost))
	fmt.Fprintf(w, "Implements reverse: %s\n", yesNo(meta.ImplementsReverse))
	fmt.Fprintf(w, "Implements permissions: %s\n", yesNo(meta.ImplementsPermissions))
	fmt.Fprintf(w, "Supports dry-run: %s\n", yesNo(meta.SupportsDryRun))
	if meta.Version != "" {
		fmt.Fprintf(w, "Version:          %s\n", meta.Version)
	}
	if len(meta.EmitsEvents) > 0 {
		fmt.Fprintf(w, "Emits events:     %s\n", strings.Join(meta.EmitsEvents, ", "))
	}

	required := map[string]bool{}
	for _, name := range def.Required {
		required[name] = true
	}
	var reqNames, optNames []string
	for n := range def.Properties {
		if required[n] {
			reqNames = append(reqNames, n)
		} else {
			optNames = append(optNames, n)
		}
	}
	sort.Strings(reqNames)
	sort.Strings(optNames)

	if len(reqNames) > 0 {
		fmt.Fprintln(w, "\nRequired parameters:")
		for _, n := range reqNames {
			fmt.Fprintln(w, "  "+formatPropertyLine(n, def.Properties[n]))
		}
	}
	if len(optNames) > 0 {
		fmt.Fprintln(w, "\nOptional parameters:")
		for _, n := range optNames {
			fmt.Fprintln(w, "  "+formatPropertyLine(n, def.Properties[n]))
		}
	}

	// Minimum example: a single-step playbook with the required fields
	// only, default-valued. Skipped when no required fields exist (the
	// action is either parameterless or takes a scalar — neither warrants
	// a synthetic example with no real semantics).
	if len(reqNames) > 0 {
		fmt.Fprintln(w, "\nMinimum example:")
		fmt.Fprintf(w, "  - %s:\n", meta.Name)
		for _, n := range reqNames {
			fmt.Fprintf(w, "      %s: %s\n", n, exampleValue(def.Properties[n]))
		}
	}
}

// formatPropertyLine renders one row in the required/optional table:
// "name  type      description". Width is fixed so columns align
// across short and long names; long descriptions wrap on whitespace
// to keep card width sensible.
func formatPropertyLine(name string, p *schemagen.Property) string {
	t := p.Type
	if t == "" && p.Ref != "" {
		t = "ref"
	}
	if t == "" {
		t = "object"
	}
	desc := strings.TrimSpace(p.Description)
	if desc == "" {
		desc = "—"
	}
	return fmt.Sprintf("%-18s %-9s %s", name, t, desc)
}

// exampleValue picks a stand-in literal for a property when building
// the minimum example. Strings → "string"; integers → 0; bools →
// false; arrays → []; everything else → null. Cheap, but enough to
// make the rendered example syntactically valid YAML.
func exampleValue(p *schemagen.Property) string {
	if p == nil {
		return "null"
	}
	switch p.Type {
	case "string":
		return `"…"`
	case "integer":
		return "0"
	case "number":
		return "0.0"
	case "boolean":
		return "false"
	case "array":
		return "[]"
	case "object":
		return "{}"
	}
	return "null"
}

// nearestActionName returns the closest match from candidates using
// a cheap edit-distance metric, or "" when no candidate is close
// enough for a confident suggestion. Mirrors closestTag /
// levenshtein in internal/plan/filter but uses tighter thresholds:
// action names are longer than tags (e.g. "pkg.install" vs "linux"),
// so the filter package's 67%-of-maxLen window admits matches a user
// wouldn't recognise. Add an absolute edit-distance cap of 4 — human
// typos cluster at 1-2 characters; 4 covers transpose + substitute
// + adjacent insertion without admitting "completely-unrelated"-
// suggests-"file.template".
func nearestActionName(needle string, candidates []string) string {
	if needle == "" || len(candidates) == 0 {
		return ""
	}
	const maxAbsoluteDist = 4
	best := ""
	bestDist := 1 << 30
	for _, c := range candidates {
		d := actionsShowDistance(needle, c)
		if d < bestDist {
			bestDist = d
			best = c
		}
	}
	if bestDist > maxAbsoluteDist {
		return best[:0] // empty — distance too large for confident suggestion
	}
	maxLen := len(needle)
	if len(best) > maxLen {
		maxLen = len(best)
	}
	if maxLen > 0 && bestDist*2 <= maxLen {
		return best
	}
	return ""
}

// actionsShowDistance is a small Levenshtein wrapper local to cmd —
// the filter package's helper is unexported and we don't want to
// promote it to public API for one caller.
func actionsShowDistance(a, b string) int {
	la, lb := len(a), len(b)
	if la == 0 {
		return lb
	}
	if lb == 0 {
		return la
	}
	prev := make([]int, lb+1)
	cur := make([]int, lb+1)
	for j := 0; j <= lb; j++ {
		prev[j] = j
	}
	for i := 1; i <= la; i++ {
		cur[0] = i
		for j := 1; j <= lb; j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = prev[j] + 1
			if cur[j-1]+1 < cur[j] {
				cur[j] = cur[j-1] + 1
			}
			if prev[j-1]+cost < cur[j] {
				cur[j] = prev[j-1] + cost
			}
		}
		prev, cur = cur, prev
	}
	return prev[lb]
}
