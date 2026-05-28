// Package snapshot implements the `mooncake snapshot` CLI — print a
// compact machine state snapshot (facts + system inventory).
package snapshot

import (
	"fmt"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/snapshot"
	"github.com/urfave/cli/v2"
)

// Output-format tokens duplicated locally (cmd/mooncake.go has the
// authoritative copies). Two strings; not worth promoting to cmdutil
// for one subpackage.
const (
	outputFormatJSON = "json"
	outputFormatText = "text"
)

func Command() *cli.Command {
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
			&cli.StringFlag{
				Name:  "diff",
				Usage: "Path to previous snapshot JSON file to diff against",
			},
			&cli.StringFlag{
				Name:  "save",
				Usage: "Save current snapshot to this file",
			},
		},
		Action: func(c *cli.Context) error {
			format := c.String("format")
			if format != outputFormatText && format != outputFormatJSON {
				return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
			}

			f := facts.Collect()
			curr := snapshot.CollectSystem(f)

			// Save current snapshot if requested
			if savePath := c.String("save"); savePath != "" {
				if err := snapshot.SaveSnapshot(savePath, curr); err != nil {
					return fmt.Errorf("failed to save snapshot: %w", err)
				}
				fmt.Printf("snapshot saved to %s\n", savePath)
				return nil
			}

			// Diff mode
			if diffPath := c.String("diff"); diffPath != "" {
				prev, err := snapshot.LoadSnapshot(diffPath)
				if err != nil {
					return err
				}
				d := snapshot.Compare(prev, curr)
				switch format {
				case outputFormatJSON:
					data, err := snapshot.RenderDiffJSON(d)
					if err != nil {
						return err
					}
					fmt.Println(string(data))
				default:
					fmt.Println(snapshot.RenderDiffText(d))
				}
				return nil
			}

			// Normal snapshot output
			switch format {
			case outputFormatJSON:
				data, err := curr.RenderJSON()
				if err != nil {
					return err
				}
				fmt.Println(string(data))
			default:
				fmt.Println(curr.RenderText(c.Int("budget")))
			}

			return nil
		},
	}
}
