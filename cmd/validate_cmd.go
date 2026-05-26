package main

import "github.com/urfave/cli/v2"

// validateCommand returns the `mooncake validate` cli.Command. The
// action function (validateAction) lives in mooncake.go.
func validateCommand() *cli.Command {
	return &cli.Command{
		Name:  "validate",
		Usage: "Validate configuration file without executing",
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
				// MT-68: accept --output-format too so users
				// who learned the verb on `apply` don't get a
				// "flag not defined" error on `validate`.
				Name:    "format",
				Aliases: []string{"f", "output-format"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
		},
		Action: validateAction,
	}
}
