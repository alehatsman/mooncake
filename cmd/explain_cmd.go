package main

import "github.com/urfave/cli/v2"

// explainCommand returns the `mooncake explain` cli.Command. The
// action function (explainAction) and its renderers live in
// mooncake.go.
func explainCommand() *cli.Command {
	return &cli.Command{
		Name:      "explain",
		Usage:     "Look up typed information about a mooncake noun (action verb, run, resource, op)",
		ArgsUsage: "<noun>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   outputFormatText,
				Usage:   "Output format: text, json, or yaml",
			},
			&cli.IntFlag{
				Name:  "examples-limit",
				Value: 3,
				Usage: "Max example excerpts to include for kind:action results",
			},
		},
		Action: explainAction,
	}
}
