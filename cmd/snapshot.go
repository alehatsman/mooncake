package main

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/snapshot"
	"github.com/urfave/cli/v2"
)

func snapshotCommand() *cli.Command {
	return &cli.Command{
		Name:  "snapshot",
		Usage: "Print a compact machine state snapshot",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
			&cli.IntFlag{
				Name:  "budget",
				Value: 0,
				Usage: "Approximate token budget for text output (0 = unlimited, 1 token ≈ 4 chars)",
			},
		},
		Action: func(c *cli.Context) error {
			format := c.String("format")
			if format != outputFormatText && format != outputFormatJSON {
				return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
			}

			f := facts.Collect()
			snap := snapshot.CollectSystem(f)

			switch format {
			case outputFormatJSON:
				data, err := snap.RenderJSON()
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			default:
				fmt.Println(snap.RenderText(c.Int("budget")))
			}

			return nil
		},
	}
}
