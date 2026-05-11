package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/alehatsman/mooncake/internal/runlog"
	"github.com/urfave/cli/v2"
)

func lastCommand() *cli.Command {
	return &cli.Command{
		Name:  "last",
		Usage: "Print the most recent run summary",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
		},
		Action: func(c *cli.Context) error {
			entry, err := runlog.Last()
			if errors.Is(err, runlog.ErrNoHistory) {
				fmt.Println("No run history found (~/.mooncake/runs.jsonl)")
				return nil
			}
			if err != nil {
				return err
			}

			if c.String("format") == "json" {
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
		},
	}
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
