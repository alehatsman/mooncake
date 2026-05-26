package main

import "github.com/urfave/cli/v2"

// pilotCommand returns the `mooncake pilot` parent with its single
// `run` subcommand. The action function (pilotRunAction) and the
// summary renderer live in mooncake.go.
func pilotCommand() *cli.Command {
	return &cli.Command{
		Name:  "pilot",
		Usage: "Pilot operations",
		Subcommands: []*cli.Command{
			{
				Name:  "run",
				Usage: "Execute pilot iteration",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "goal",
						Aliases:  []string{"g"},
						Required: true,
						Usage:    "Goal description",
					},
					&cli.StringFlag{
						Name:    "plan",
						Aliases: []string{"p"},
						Usage:   "Path to plan YAML file",
					},
					&cli.BoolFlag{
						Name:  "stdin",
						Usage: "Read plan from stdin",
					},
					&cli.StringFlag{
						Name:  "provider",
						Usage: "LLM provider (claude for loop mode)",
					},
					&cli.StringFlag{
						Name:  "model",
						Value: "sonnet",
						Usage: "Model name (when using --provider)",
					},
					&cli.IntFlag{
						Name:  "max-iterations",
						Value: 5,
						Usage: "Maximum iterations for loop mode",
					},
					&cli.BoolFlag{
						Name:  "auto-apply",
						Usage: "Skip the plan-confirm gate (required for unattended/CI runs; spec-67 §10)",
					},
				},
				Action: pilotRunAction,
			},
		},
	}
}
