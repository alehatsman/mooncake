package main

import "github.com/urfave/cli/v2"

// planCommand returns the `mooncake plan` cli.Command. The action +
// formatters (planAction, formatPlanJSON/YAML/Text, planSymbol,
// formatCostSummary, riskBand) live in mooncake.go.
func planCommand() *cli.Command {
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
