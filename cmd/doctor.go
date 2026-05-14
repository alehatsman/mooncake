package main

import (
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/doctor"
	"github.com/urfave/cli/v2"
)

func doctorCommand() *cli.Command {
	return &cli.Command{
		Name:  "doctor",
		Usage: "Check mooncake's installation, state, tools, and project for issues",
		Description: "Runs a fixed battery of checks across install, system, state, presets, " +
			"tools, project, and services. Reports OK/info/warning/error with concrete fix " +
			"hints. Designed to finish in under a second. Use --format json for CI consumers.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
			&cli.BoolFlag{
				Name:  "strict",
				Usage: "Exit 1 on warnings, not just errors",
			},
			&cli.StringSliceFlag{
				Name:  "section",
				Usage: "Run only this section (install, system, state, presets, tools, project, services). Repeatable.",
			},
			&cli.BoolFlag{
				Name:  "skip-project",
				Usage: "Skip the cwd project-config checks",
			},
			&cli.BoolFlag{
				Name:  "no-color",
				Usage: "Disable colour output (NO_COLOR env var also honoured)",
			},
		},
		Action: doctorAction,
	}
}

func doctorAction(c *cli.Context) error {
	// Wire the binary version into the install/binary check so the
	// doctor package stays decoupled from cmd-layer globals.
	doctor.BinaryVersion = version

	format := c.String("format")
	if format != "text" && format != "json" {
		return fmt.Errorf("invalid --format %q (use text or json)", format)
	}

	rep, exitCode, err := doctor.Run(doctor.Options{
		Format:      format,
		Strict:      c.Bool("strict"),
		Sections:    c.StringSlice("section"),
		SkipProject: c.Bool("skip-project"),
		NoColor:     c.Bool("no-color"),
		Out:         os.Stdout,
		Err:         os.Stderr,
	})
	if err != nil {
		return err
	}

	switch format {
	case "json":
		if err := doctor.RenderJSON(os.Stdout, rep); err != nil {
			return err
		}
	default:
		if err := doctor.RenderText(os.Stdout, rep, c.Bool("no-color")); err != nil {
			return err
		}
	}

	if exitCode != 0 {
		os.Exit(exitCode)
	}
	return nil
}
