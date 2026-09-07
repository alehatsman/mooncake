package cmdutil

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"
)

// Output-format tokens. The CLI accepts exactly these two everywhere a
// read command renders.
const (
	FormatText = "text"
	FormatJSON = "json"
)

// FormatFlag returns the standard `--format/-f` flag. Six read commands
// (`drift inspect`, `tool list`, `mod cache list`, `cron list`,
// `vault list`, `vault recipients list`) had no machine-readable output
// at all, so anything driving mooncake had to screen-scrape their
// human tables. They all take this flag now — declared once so the name,
// alias, default, and help text can't drift apart across six files.
// See specs/cli-surface.md.
func FormatFlag() *cli.StringFlag {
	return &cli.StringFlag{
		Name:    "format",
		Aliases: []string{"f"},
		Value:   FormatText,
		Usage:   "Output format: text or json",
	}
}

// WantJSON reports whether the caller asked for JSON, rejecting any
// value that is neither token. Returning an error rather than silently
// falling back to text matters for a script: a typo'd `--format jsonn`
// must fail loudly, not emit a table the caller then fails to parse.
func WantJSON(c *cli.Context) (bool, error) {
	switch f := c.String("format"); f {
	case "", FormatText:
		return false, nil
	case FormatJSON:
		return true, nil
	default:
		return false, fmt.Errorf("invalid format: %s (use 'text' or 'json')", f)
	}
}

// EmitJSON writes v to stdout as indented JSON with a trailing newline.
//
// Only the document goes to stdout — callers keep headers, hints, and
// progress on stderr — so `mooncake <cmd> --format json | jq` parses
// without filtering.
func EmitJSON(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
