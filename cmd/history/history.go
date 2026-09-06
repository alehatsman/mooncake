// Package history implements the `mooncake history` CLI — inspect
// past mooncake runs from the runlog.
package history

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/alehatsman/mooncake/internal/runlog"
	"github.com/alehatsman/mooncake/internal/utils"
	"github.com/urfave/cli/v2"
)

const outputFormatJSON = "json"

// Command replaces the old `mooncake last`. Bare invocation
// preserves last's behaviour (print the most recent run); subcommands
// add `list` and `show <index>` for browsing the full log.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "history",
		Usage: "Inspect past mooncake runs",
		Description: "Without subcommands, prints the most recent run summary " +
			"(equivalent to the previous `mooncake last`). Use `history list` " +
			"to browse multiple recent runs and `history show <N>` to view a " +
			"specific entry. Index is 1-based newest-first.",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "text", Usage: "Output format: text or json"},
		},
		Action: historyLastAction,
		Subcommands: []*cli.Command{
			{
				Name:  "list",
				Usage: "List recent runs",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "limit", Aliases: []string{"n"}, Value: 10, Usage: "Maximum runs to show (newest)"},
					&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "text", Usage: "Output format: text or json"},
				},
				Action: historyListAction,
			},
			{
				Name:  "gc",
				Usage: "Prune the run log, keeping recent entries",
				Description: "Rewrites ~/.mooncake/runs.jsonl in place, retaining only " +
					"the entries that pass the filters. With no flags, keeps the 500 " +
					"newest runs. Combining --keep and --older-than requires an entry " +
					"to satisfy both. The log also rolls automatically at 16 MiB; gc " +
					"is the explicit control on top of that.",
				Flags: []cli.Flag{
					&cli.IntFlag{Name: "keep", Aliases: []string{"n"}, Value: 500, Usage: "Number of newest runs to retain (0 = no count limit)"},
					&cli.DurationFlag{Name: "older-than", Usage: "Drop runs older than this duration (e.g. 90d is 2160h; 0 = no age limit)"},
					&cli.BoolFlag{Name: "dry-run", Usage: "Report what would be removed without rewriting the log"},
					&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "text", Usage: "Output format: text or json"},
				},
				Action: historyGCAction,
			},
			{
				Name:      "show",
				Usage:     "Show one historical run by index",
				ArgsUsage: "<index>",
				Description: "Index is 1-based newest-first. `history show 1` is " +
					"identical to bare `history`.",
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "format", Aliases: []string{"f"}, Value: "text", Usage: "Output format: text or json"},
				},
				Action: historyShowAction,
			},
		},
	}
}

func historyLastAction(c *cli.Context) error {
	entry, err := runlog.Last()
	if errors.Is(err, runlog.ErrNoHistory) {
		fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
		return nil
	}
	if err != nil {
		return err
	}
	return printHistoryEntry(entry, c.String("format"))
}

func historyListAction(c *cli.Context) error {
	entries, err := runlog.Recent(c.Int("limit"))
	if errors.Is(err, runlog.ErrNoHistory) {
		fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
		return nil
	}
	if err != nil {
		return err
	}

	if c.String("format") == outputFormatJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entries)
	}

	// Print oldest-of-window first so the newest sits at the bottom of
	// the terminal — easier to scan while scrolled to the prompt.
	for i := len(entries) - 1; i >= 0; i-- {
		printHistoryLine(i+1, entries[i])
	}
	return nil
}

func historyShowAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("history show requires exactly one argument: the 1-based newest-first index (try `mooncake history list`)")
	}
	idx, err := strconv.Atoi(c.Args().First())
	if err != nil {
		return fmt.Errorf("invalid index %q: must be an integer", c.Args().First())
	}
	entry, err := runlog.At(idx)
	if errors.Is(err, runlog.ErrNoHistory) {
		fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
		return nil
	}
	if errors.Is(err, runlog.ErrIndexOutOfRange) {
		all, _ := runlog.Recent(0)
		return fmt.Errorf("index %d out of range (1..%d). Run `mooncake history list` to see what's available", idx, len(all))
	}
	if err != nil {
		return err
	}
	return printHistoryEntry(entry, c.String("format"))
}

func historyGCAction(c *cli.Context) error {
	opts := runlog.PruneOptions{
		Keep:      c.Int("keep"),
		OlderThan: c.Duration("older-than"),
	}

	// --dry-run answers the same question without touching the file:
	// count what would survive, report, return.
	if c.Bool("dry-run") {
		res, err := runlog.PreviewPrune(opts)
		if errors.Is(err, runlog.ErrNoHistory) {
			fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
			return nil
		}
		if err != nil {
			return err
		}
		return printPruneResult(res, c.String("format"), true)
	}

	res, err := runlog.Prune(opts)
	if errors.Is(err, runlog.ErrNoHistory) {
		fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
		return nil
	}
	if err != nil {
		return err
	}
	return printPruneResult(res, c.String("format"), false)
}

func printPruneResult(res runlog.PruneResult, format string, dryRun bool) error {
	if format == outputFormatJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(struct {
			runlog.PruneResult
			DryRun bool `json:"dry_run"`
		}{res, dryRun})
	}
	verb := "removed"
	if dryRun {
		verb = "would remove"
	}
	fmt.Printf("%s %d of %d run(s); %d retained", verb, res.Removed, res.Before, res.After)
	if !dryRun {
		fmt.Printf(" (%s)", utils.HumanBytes(res.Bytes))
	}
	fmt.Println()
	return nil
}

func printHistoryEntry(entry runlog.Entry, format string) error {
	if format == outputFormatJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(entry)
	}
	fmt.Printf("Last run: %s  (config: %s)\n",
		entry.TS.Format("2006-01-02 15:04 UTC"), entry.Config)
	fmt.Printf("  changed=%d  ok=%d  skipped=%d  failed=%d  duration=%s\n",
		entry.Changed, entry.Ok, entry.Skipped, entry.Failed,
		fmtDuration(entry.DurationMs))
	return nil
}

// printHistoryLine renders one entry as a single line for `history list`.
func printHistoryLine(index int, e runlog.Entry) {
	fmt.Printf("#%-3d %s  config=%s  changed=%d  ok=%d  failed=%d  %s\n",
		index,
		e.TS.Format("2006-01-02 15:04 UTC"),
		e.Config,
		e.Changed, e.Ok, e.Failed,
		fmtDuration(e.DurationMs),
	)
}

func fmtDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	m := int(d.Minutes())
	s := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%02ds", m, s)
}
