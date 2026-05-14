package main

import (
	"os"

	"github.com/alehatsman/mooncake/internal/scaffold"
	"github.com/urfave/cli/v2"
)

func initCommand() *cli.Command {
	return &cli.Command{
		Name:  "init",
		Usage: "Scaffold a new mooncake project in the current directory",
		Description: "Creates mooncake.yml, mooncake.vars.yml, .gitignore, and .mooncake/ " +
			"in the working directory (or --dir). Picks a template interactively unless " +
			"--non-interactive --template <name> is given.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "template",
				Aliases: []string{"t"},
				Usage:   "Template to use (dotfiles, server, empty, agent-sandbox)",
			},
			&cli.BoolFlag{
				Name:    "non-interactive",
				Aliases: []string{"n"},
				Usage:   "Skip prompts; --template is required",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Overwrite existing mooncake.yml / mooncake.vars.yml",
			},
			&cli.StringFlag{
				Name:  "dir",
				Value: ".",
				Usage: "Scaffold into this directory (created if missing)",
			},
			&cli.BoolFlag{
				Name:  "no-vars",
				Usage: "Skip the mooncake.vars.yml file",
			},
			&cli.BoolFlag{
				Name:  "list-templates",
				Usage: "Print available templates and exit",
			},
		},
		Action: initAction,
	}
}

func initAction(c *cli.Context) error {
	if c.Bool("list-templates") {
		return scaffold.ListTemplates(os.Stdout)
	}
	return scaffold.Scaffold(scaffold.Options{
		Template:       c.String("template"),
		NonInteractive: c.Bool("non-interactive"),
		Force:          c.Bool("force"),
		Dir:            c.String("dir"),
		NoVars:         c.Bool("no-vars"),
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
	})
}
