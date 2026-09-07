package kernel

import (
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/snapshot"
)

// StateCommand returns `mooncake state` — the single entry point for
// reading this machine's state.
//
// Three top-level commands used to answer overlapping versions of the
// same question: `snapshot` (the composite inventory), `facts` (static
// host identity), and `metrics` (live gauges). Each carried its own flag
// set and its own place in a 26-command list, so "how do I see what this
// box looks like?" had three answers and no obvious first one.
//
// They collapse into one noun with two drill-downs. Bare `state` is the
// composite snapshot — the view you want when you don't know what you're
// looking for. `state facts` and `state metrics` keep their own flags
// verbatim, and every view keeps the JSON shape it emitted before, so
// consumers change only the verb. See specs/cli-surface.md.
func StateCommand() *cli.Command {
	return &cli.Command{
		Name:     "state",
		Category: CategoryInspect,
		Usage:    "Read this machine's state (snapshot, facts, metrics)",
		Description: "Bare `state` prints the composite machine snapshot. " +
			"`state facts` reports static host facts; `state metrics` samples " +
			"live CPU/GPU/memory/load/network. All three honour --format json.",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   outputFormatText,
				Usage:   "Output format: text or json",
			},
			&cli.IntFlag{
				Name:  "budget",
				Value: 0,
				Usage: "Approximate token budget for text output (0 = unlimited, 1 token ~ 4 chars)",
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
		Action: snapshotAction,
		Subcommands: []*cli.Command{
			factsSubcommand(),
			metricsSubcommand(),
		},
	}
}

// snapshotAction is the body of the retired `mooncake snapshot` command,
// now the root action of `state`.
func snapshotAction(c *cli.Context) error {
	format := c.String("format")
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	f := facts.Collect()
	curr := snapshot.CollectSystem(f)

	if savePath := c.String("save"); savePath != "" {
		if err := snapshot.SaveSnapshot(savePath, curr); err != nil {
			return fmt.Errorf("failed to save snapshot: %w", err)
		}
		fmt.Printf("snapshot saved to %s\n", savePath)
		return nil
	}

	if diffPath := c.String("diff"); diffPath != "" {
		return renderSnapshotDiff(diffPath, curr, format)
	}

	if format == outputFormatJSON {
		data, err := curr.RenderJSON()
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Println(curr.RenderText(c.Int("budget")))
	return nil
}

// renderSnapshotDiff compares curr against a previously saved snapshot.
func renderSnapshotDiff(prevPath string, curr *snapshot.SystemSnapshot, format string) error {
	prev, err := snapshot.LoadSnapshot(prevPath)
	if err != nil {
		return err
	}
	d := snapshot.Compare(prev, curr)
	if format == outputFormatJSON {
		data, err := snapshot.RenderDiffJSON(d)
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}
	fmt.Println(snapshot.RenderDiffText(d))
	return nil
}
