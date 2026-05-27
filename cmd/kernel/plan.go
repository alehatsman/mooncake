package kernel

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/actions"
	"github.com/alehatsman/mooncake/internal/config"
	"github.com/alehatsman/mooncake/internal/diff"
	"github.com/alehatsman/mooncake/internal/executor"
	"github.com/alehatsman/mooncake/internal/logger"
	"github.com/alehatsman/mooncake/internal/plan"
	"github.com/alehatsman/mooncake/internal/security"
)

// PlanCommand returns the `mooncake plan` cli.Command.
func PlanCommand() *cli.Command {
	return &cli.Command{
		Name:  "plan",
		Usage: "Generate and display execution plan (dry-run)",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "config",
				Aliases: []string{"c"},
				Usage:   "Path to configuration file (default: ./mooncake.yml or ./mooncake/main.yml)",
			},
			&cli.StringSliceFlag{
				Name:    "vars",
				Aliases: []string{"v"},
				Usage:   "Path to a variables file. Repeat to layer multiple files; later wins on key collision.",
			},
			&cli.StringFlag{
				Name:    "tags",
				Aliases: []string{"t"},
				Usage:   "Filter steps by tags (comma-separated)",
			},
			&cli.StringFlag{
				Name:  "skip-tags",
				Usage: "Exclude steps whose tags appear in this list (comma-separated)",
			},
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text, json, or yaml",
			},
			&cli.BoolFlag{
				Name:  "show-origins",
				Value: false,
				Usage: "Show origin file:line:col for each step",
			},
			&cli.BoolFlag{
				Name:  "no-inspect",
				Value: false,
				Usage: "Skip the per-step state inspection pass (Spec 16). With this flag, plan output reflects only static YAML expansion — no would-change predictions.",
			},
			&cli.BoolFlag{
				Name:    "diff",
				Aliases: []string{"d"},
				Value:   false,
				Usage:   "Show unified diff for file steps that would change content",
			},
			&cli.StringFlag{
				Name:    "output",
				Aliases: []string{"o"},
				Usage:   "Save plan to file (format determined by extension: .json, .yaml, .yml)",
			},
			// Sudo-credential flags: plan goes through the same
			// dispatchRunner preflight as apply (spec-69 phase 4
			// catches Sudo:true + AsUser + no-password at plan
			// time). Without these flags, a plan against a step
			// with as_user: root errors out before any prediction
			// gets rendered — which is the right behavior for
			// "fail at plan, not at apply" but inconvenient for
			// preview workflows. These flags let operators feed
			// the password to plan too.
			&cli.StringFlag{Name: "sudo-pass-file", Usage: "Read sudo password from file (must have 0600 permissions)"},
			&cli.StringFlag{Name: "sudo-pass", Aliases: []string{"s"}, Usage: "Sudo password (requires --insecure-sudo-pass)"},
			&cli.BoolFlag{Name: "ask-become-pass", Aliases: []string{"K"}, Usage: "Prompt for sudo password interactively"},
			&cli.BoolFlag{Name: "insecure-sudo-pass", Usage: "Allow --sudo-pass flag (WARNING: visible in shell history)"},
		},
		Action: planAction,
	}
}

func planAction(c *cli.Context) error {
	configPath, err := cmdutil.ResolveConfigPath(c)
	if err != nil {
		if cmdutil.PrintNoConfigHintAndExit(err, "plan") {
			return nil
		}
		return err
	}
	// Spec-68 wave 2: every `mooncake plan` invocation produces an
	// ops.jsonl entry with PlanOnly=true and Runs=[]. No runs.jsonl
	// row is written for a plan-only op. The id is discarded here
	// because the plan path doesn't surface it back to the user yet;
	// future iterations may print it alongside the plan summary.
	_ = recordOp("plan", configPath, true)
	// Spec 51: prepend local overlay vars. For standalone `plan`, --host
	// and --overlays aren't registered as flags; the helper falls through
	// to the derived hostname and the default on-state, so behavior matches
	// `apply --dry-run` reached via the apply flag set.
	overlayVars, err := resolveLocalOverlays(c, configPath)
	if err != nil {
		return err
	}
	explicitVars := c.StringSlice("vars")
	varsPaths := make([]string, 0, len(overlayVars)+len(explicitVars))
	varsPaths = append(varsPaths, overlayVars...)
	varsPaths = append(varsPaths, explicitVars...)
	outputPath := c.String("output")
	// When invoked via `apply --dry-run`, the plan-specific --format flag
	// isn't registered on apply; fall back to the same default as `plan`.
	format := c.String("format")
	if format == "" {
		format = outputFormatText
	}
	showOrigins := c.Bool("show-origins")
	showDiff := c.Bool("diff")
	noInspect := c.Bool("no-inspect")

	// Parse tags
	tags := cmdutil.ParseTags(c.String("tags"))
	skipTags := cmdutil.ParseTags(c.String("skip-tags"))

	// Load variables from each file in order; later files override earlier
	// on key collision. Matches `apply -v a.yml -v b.yml` semantics.
	variables := make(map[string]interface{})
	for _, varsPath := range varsPaths {
		if varsPath == "" {
			continue
		}
		vars, err := config.ReadVariables(varsPath)
		if err != nil {
			return fmt.Errorf("failed to read variables from %s: %w", varsPath, err)
		}
		for k, v := range vars {
			variables[k] = v
		}
	}

	// Build plan (planner will inject system facts automatically)
	planner, err := plan.NewPlanner()
	if err != nil {
		return err
	}
	planData, err := planner.BuildPlan(plan.PlannerConfig{
		ConfigPath: configPath,
		Variables:  variables,
		Tags:       tags,
		SkipTags:   skipTags,
	})
	if err != nil {
		return fmt.Errorf("failed to build plan: %w", err)
	}

	// Spec 16: after the static config expansion, inspect each step
	// against current target state. Unless --no-inspect is set.
	// Inspection routes through the executor in check mode; legacy
	// handlers that don't implement Spec-16 Runner report as
	// "not checkable" until they migrate (Phase 5).
	if !noInspect {
		// Spec-69 phase 4: the dispatcher's preflight needs to see
		// the sudo password configuration during plan-mode inspect
		// too, otherwise plan errors on steps that would have
		// applied fine. Resolve via the same security helper apply
		// uses.
		sudoPass, perr := security.ResolvePassword(security.PasswordConfig{
			CLIPassword:    c.String("sudo-pass"),
			AskInteractive: c.Bool("ask-become-pass"),
			PasswordFile:   c.String("sudo-pass-file"),
			InsecureCLI:    c.Bool("insecure-sudo-pass"),
		})
		if perr != nil {
			return fmt.Errorf("sudo password setup failed: %w", perr)
		}
		internalLog := logger.NewLogger(logger.ErrorLevel)
		inspections, err := executor.InspectPlan(planData, sudoPass, internalLog)
		if err != nil {
			return fmt.Errorf("failed to inspect plan: %w", err)
		}
		planData.Inspections = inspections
	}

	// Save to file if output path specified
	if outputPath != "" {
		if err := plan.SavePlanToFile(planData, outputPath); err != nil {
			return fmt.Errorf("failed to save plan: %w", err)
		}
		fmt.Printf("Plan saved to %s\n", outputPath)
		return nil
	}

	// Format and display plan
	switch format {
	case outputFormatJSON:
		return FormatPlanJSON(planData)
	case outputFormatYAML:
		return FormatPlanYAML(planData)
	case outputFormatText:
		return FormatPlanText(planData, showOrigins, showDiff)
	default:
		return fmt.Errorf("unsupported format: %s (use text, json, or yaml)", format)
	}
}

func FormatPlanJSON(p *plan.Plan) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(p)
}

func FormatPlanYAML(p *plan.Plan) error {
	encoder := yaml.NewEncoder(os.Stdout)
	encoder.SetIndent(yamlIndentSpaces)
	defer func() {
		// Intentionally ignore Close() error - encoder writes to stdout which doesn't need explicit close handling
		_ = encoder.Close()
	}()
	return encoder.Encode(p)
}

// planSymbol returns the leading symbol for a per-step plan line.
//
//	↑  WouldChange  — applying this step would change state
//	✓  AlreadyOk    — already in desired state
//	-  Skipped      — when / tag filter removed this step
//	?  not checkable — handler can't predict (e.g. shell)
func planSymbol(ins plan.StepInspection, stepSkipped bool) string {
	switch {
	case stepSkipped || ins.Skipped:
		return "-"
	case !ins.Checkable:
		return "?"
	case ins.WouldChange:
		return "↑"
	default:
		return "✓"
	}
}

func FormatPlanText(p *plan.Plan, showOrigins bool, showDiff bool) error {
	fmt.Printf("Plan: %s\n", p.RootFile)
	hostBits := []string{}
	if p.GeneratedOn.OsFamily != "" {
		hostBits = append(hostBits, p.GeneratedOn.OsFamily)
	}
	if p.GeneratedOn.Arch != "" {
		hostBits = append(hostBits, p.GeneratedOn.Arch)
	}
	if p.GeneratedOn.DistroFamily != "" {
		hostBits = append(hostBits, p.GeneratedOn.DistroFamily)
	}
	if len(hostBits) > 0 {
		fmt.Printf("Generated: %s on %s\n", p.GeneratedAt.Format("2006-01-02 15:04:05"), strings.Join(hostBits, "/"))
	} else {
		fmt.Printf("Generated: %s\n", p.GeneratedAt.Format("2006-01-02 15:04:05"))
	}
	if len(p.Tags) > 0 {
		fmt.Printf("Tags: %s\n", strings.Join(p.Tags, ", "))
	}
	fmt.Println()

	// Build a lookup from stepID to inspection so the order matches steps.
	insByID := make(map[string]plan.StepInspection, len(p.Inspections))
	for _, ins := range p.Inspections {
		insByID[ins.StepID] = ins
	}

	var wouldChange, ok, skipped, notCheckable, maxRisk int
	for _, step := range p.Steps {
		ins := insByID[step.ID]
		sym := planSymbol(ins, step.Skipped)

		// Spec-66 wave 7: a transaction-/try-parent step has no
		// handler (the children carry the actions) so it lands as
		// "?" not-inspected. Synthesize a compound Diff from the
		// parent + siblings so internal/diff renders the compound
		// shape under the parent line.
		compound := diff.SynthesizeCompound(step, p.Steps)

		name := step.Name
		if name == "" {
			name = step.ID
		}
		// One line per step: <sym> <name>   <reason>
		line := fmt.Sprintf("%s %s", sym, name)
		if ins.Reason != "" && compound == nil {
			line = fmt.Sprintf("%-50s  %s", line, ins.Reason)
		} else if step.Skipped {
			line = fmt.Sprintf("%-50s  %s", line, "skipped (tags)")
		}
		fmt.Println(line)

		// Spec-22 phase 6: show a one-line cost summary under any
		// step that would change. Suppressed for ok/skipped to keep
		// "nothing changed" plans uncluttered.
		if sym == "↑" && ins.Cost != nil {
			fmt.Printf("    cost: %s\n", formatCostSummary(ins.Cost))
			if ins.Cost.Risk > maxRisk {
				maxRisk = ins.Cost.Risk
			}
		}

		if showDiff && (sym == "↑" || compound != nil) {
			// Compound diff fires regardless of symbol — the parent
			// step is structural, not action-bearing, so it has no
			// "would change" verdict of its own; child steps each
			// render their own typed diffs under their sibling lines.
			if r := diff.Lookup(ins.Detail, ins.Diff, compound); r != nil {
				var buf strings.Builder
				if err := r.Render(&buf, diff.FormatText); err == nil && buf.Len() > 0 {
					fmt.Print(buf.String())
					fmt.Println()
				}
			}
		}

		switch sym {
		case "↑":
			wouldChange++
		case "✓":
			ok++
		case "-":
			skipped++
		case "?":
			notCheckable++
		}

		if showOrigins && step.Origin != nil {
			fmt.Printf("    %s:%d:%d\n", step.Origin.FilePath, step.Origin.Line, step.Origin.Column)
			if len(step.Origin.IncludeChain) > 0 {
				fmt.Printf("    via: %s\n", strings.Join(step.Origin.IncludeChain, " -> "))
			}
		}
	}

	fmt.Println()
	summary := fmt.Sprintf("PLAN SUMMARY  would-change=%d  ok=%d  skipped=%d  not-checkable=%d",
		wouldChange, ok, skipped, notCheckable)
	if maxRisk > 0 {
		summary += fmt.Sprintf("  max-risk=%d (%s)", maxRisk, riskBand(maxRisk))
	}
	fmt.Println(summary)
	return nil
}

// formatCostSummary renders an actions.CostEstimate as a single
// human-readable line for plan output. Fields that are -1 (unknown
// — handler couldn't predict statically) are omitted rather than
// shown as "-1" to keep the line scannable. Spec-22 phase 6.
func formatCostSummary(c *actions.CostEstimate) string {
	parts := []string{fmt.Sprintf("risk %d (%s)", c.Risk, riskBand(c.Risk))}
	if c.Reversible {
		parts = append(parts, "reversible")
	} else {
		parts = append(parts, "irreversible")
	}
	if c.Resources >= 0 {
		unit := "resource"
		if c.Resources != 1 {
			unit = "resources"
		}
		parts = append(parts, fmt.Sprintf("%d %s", c.Resources, unit))
	}
	if c.Bytes >= 0 {
		parts = append(parts, fmt.Sprintf("%d bytes", c.Bytes))
	}
	return strings.Join(parts, " • ")
}

// riskBand maps a numeric risk score (1..10) to the band label
// the spec-22 phase 6 contract documents:
//
//	1–3   safe (read-only, idempotent writes to scratch)
//	4–6   routine (config writes, package installs)
//	7–9   high impact (service restarts, kernel params)
//	10    destructive (deletes, drops, rm -rf)
func riskBand(r int) string {
	switch {
	case r >= 10:
		return "destructive"
	case r >= 7:
		return "high"
	case r >= 4:
		return "routine"
	case r >= 1:
		return "safe"
	}
	return "unknown"
}
