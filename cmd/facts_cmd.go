package main

import "github.com/urfave/cli/v2"

// factsCommand returns the `mooncake facts` cli.Command. The action
// function (factsAction) lives in mooncake.go.
func factsCommand() *cli.Command {
	return &cli.Command{
		Name:  "facts",
		Usage: "Display system facts",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
			&cli.StringSliceFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Query a specific fact by dot-path key (e.g. go.version). Repeatable.",
			},
		},
		Action: factsAction,
	}
}
