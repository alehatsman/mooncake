// Package cron implements the `mooncake cron` CLI subcommand for inspecting
// /etc/cron.d entries managed by mooncake (and optionally all entries).
package cron

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/urfave/cli/v2"
)

const (
	defaultCronDir = "/etc/cron.d"
	managedHeader  = "# Managed by mooncake os.cron"
)

// Command returns the `mooncake cron` command tree.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "cron",
		Usage: "Inspect and manage cron.d entries",
		Subcommands: []*cli.Command{
			listCmd(),
			removeCmd(),
		},
	}
}

func listCmd() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List cron.d entries managed by mooncake",
		Description: `Reads /etc/cron.d and prints entries managed by mooncake os.cron.
Use --all to also include files not managed by mooncake.

Exit codes:
  0  success (even if no entries found)
  2  directory unreadable`,
		Flags: []cli.Flag{
			&cli.BoolFlag{
				Name:    "all",
				Aliases: []string{"a"},
				Usage:   "Include files not managed by mooncake",
			},
			&cli.StringFlag{
				Name:  "dir",
				Value: defaultCronDir,
				Usage: "Path to cron.d directory",
			},
			&cli.BoolFlag{
				Name:  "json",
				Usage: "Output as JSON",
			},
		},
		Action: runList,
	}
}

// cronEntry holds a parsed cron.d entry.
type cronEntry struct {
	Name     string `json:"name"`
	Managed  bool   `json:"managed"`
	Schedule string `json:"schedule"`
	User     string `json:"user"`
	Command  string `json:"command"`
}

func runList(c *cli.Context) error {
	entries, err := readCronDir(c.String("dir"), c.Bool("all"))
	if err != nil {
		return cli.Exit(fmt.Sprintf("cron list: %v", err), 2)
	}
	if c.Bool("json") {
		return printJSON(c, entries)
	}
	return printTable(c, entries)
}

// readCronDir scans dir and returns parsed cron entries.
func readCronDir(dir string, showAll bool) ([]cronEntry, error) {
	infos, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("read %s: %w", dir, err)
	}

	var entries []cronEntry
	for _, info := range infos {
		if info.IsDir() {
			continue
		}
		path := filepath.Join(dir, info.Name())
		data, err := os.ReadFile(path) // #nosec G304
		if err != nil {
			continue
		}
		content := string(data)
		managed := strings.HasPrefix(content, managedHeader)
		if !managed && !showAll {
			continue
		}
		entries = append(entries, parseCronFile(info.Name(), managed, content)...)
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	return entries, nil
}

// parseCronFile extracts cron schedule lines from a cron.d file.
// Handles both standard 5-field schedules and @-shorthand (e.g. @reboot).
func parseCronFile(name string, managed bool, content string) []cronEntry {
	var out []cronEntry
	sc := bufio.NewScanner(strings.NewReader(content))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		// ENV=val lines: first token contains '=' and has no surrounding spaces
		if strings.Contains(fields[0], "=") {
			continue
		}
		var schedule, user, command string
		if strings.HasPrefix(fields[0], "@") {
			// @reboot / @hourly / @daily / … user command
			if len(fields) < 3 {
				continue
			}
			schedule = fields[0]
			user = fields[1]
			command = strings.Join(fields[2:], " ")
		} else {
			// min hour dom month dow user command
			if len(fields) < 7 {
				continue
			}
			schedule = strings.Join(fields[0:5], " ")
			user = fields[5]
			command = strings.Join(fields[6:], " ")
		}
		out = append(out, cronEntry{
			Name:     name,
			Managed:  managed,
			Schedule: schedule,
			User:     user,
			Command:  command,
		})
	}
	return out
}

func removeCmd() *cli.Command {
	return &cli.Command{
		Name:      "remove",
		Usage:     "Remove a mooncake-managed cron.d entry",
		ArgsUsage: "<name>",
		Description: `Deletes /etc/cron.d/<name> if it is managed by mooncake os.cron.
Use --force to remove files not managed by mooncake.

Exit codes:
  0  removed (or already absent)
  1  file not found
  2  error (unreadable, not managed without --force, etc.)`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "dir",
				Value: defaultCronDir,
				Usage: "Path to cron.d directory",
			},
			&cli.BoolFlag{
				Name:    "force",
				Aliases: []string{"f"},
				Usage:   "Remove even if not managed by mooncake",
			},
		},
		Action: runRemove,
	}
}

func runRemove(c *cli.Context) error {
	name := c.Args().First()
	if name == "" {
		return cli.Exit("cron remove: missing <name> argument", 2)
	}
	if strings.Contains(name, "/") || strings.Contains(name, "..") {
		return cli.Exit("cron remove: name must not contain path separators", 2)
	}

	path := filepath.Join(c.String("dir"), name)

	data, err := os.ReadFile(path) // #nosec G304
	if err != nil {
		if os.IsNotExist(err) {
			return cli.Exit(fmt.Sprintf("cron remove: %s: not found", name), 1)
		}
		return cli.Exit(fmt.Sprintf("cron remove: read %s: %v", name, err), 2)
	}

	if !strings.HasPrefix(string(data), managedHeader) && !c.Bool("force") {
		return cli.Exit(fmt.Sprintf("cron remove: %s is not managed by mooncake (use --force to remove anyway)", name), 2)
	}

	if err := os.Remove(path); err != nil {
		return cli.Exit(fmt.Sprintf("cron remove: %v", err), 2)
	}
	fmt.Fprintf(c.App.Writer, "removed %s\n", path)
	return nil
}

func printJSON(c *cli.Context, entries []cronEntry) error {
	if entries == nil {
		entries = []cronEntry{}
	}
	out, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return cli.Exit(fmt.Sprintf("cron list: marshal: %v", err), 2)
	}
	fmt.Fprintln(c.App.Writer, string(out))
	return nil
}

func printTable(c *cli.Context, entries []cronEntry) error {
	if len(entries) == 0 {
		fmt.Fprintln(c.App.Writer, "(no cron entries found)")
		return nil
	}
	fmt.Fprintf(c.App.Writer, "%-24s  %-7s  %-17s  %-8s  %s\n",
		"NAME", "MANAGED", "SCHEDULE", "USER", "COMMAND")
	fmt.Fprintf(c.App.Writer, "%s  %s  %s  %s  %s\n",
		strings.Repeat("-", 24), strings.Repeat("-", 7),
		strings.Repeat("-", 17), strings.Repeat("-", 8),
		strings.Repeat("-", 20))
	for _, e := range entries {
		managed := "no"
		if e.Managed {
			managed = "yes"
		}
		cmd := e.Command
		if len(cmd) > 40 {
			cmd = cmd[:37] + "..."
		}
		fmt.Fprintf(c.App.Writer, "%-24s  %-7s  %-17s  %-8s  %s\n",
			e.Name, managed, e.Schedule, e.User, cmd)
	}
	return nil
}
