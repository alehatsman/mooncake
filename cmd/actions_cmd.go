package main

import "github.com/urfave/cli/v2"

// actionsCommand returns the `mooncake actions` parent with its
// list / show subcommands. The action functions (actionsListAction,
// actionsShowAction) and their helpers live in mooncake.go.
func actionsCommand() *cli.Command {
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
