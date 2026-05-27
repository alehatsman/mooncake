package kernel

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/cmd/cmdutil"
	"github.com/alehatsman/mooncake/internal/facts"
	"github.com/alehatsman/mooncake/internal/factsfmt"
)

// FactsCommand returns the `mooncake facts` cli.Command.
func FactsCommand() *cli.Command {
	return &cli.Command{
		Name:  "facts",
		Usage: "Display system facts",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "format",
				Aliases: []string{"f"},
				Value:   "text",
				Usage:   "Output format: text or json",
			},
			&cli.StringSliceFlag{
				Name:    "query",
				Aliases: []string{"q"},
				Usage:   "Query a specific fact by dot-path key (e.g. go.version). Repeatable.",
			},
		},
		Action: FactsAction,
	}
}

func FactsAction(c *cli.Context) error {
	// Collect facts (cached)
	f := facts.Collect()

	// --query mode: print specific values and exit
	if queries := c.StringSlice("query"); len(queries) > 0 {
		return cmdutil.QueryMap(f.ToMap(), queries)
	}

	format := c.String("format")
	if format != outputFormatText && format != outputFormatJSON {
		return fmt.Errorf("invalid format: %s (use 'text' or 'json')", format)
	}

	switch format {
	case outputFormatJSON:
		encoder := json.NewEncoder(os.Stdout)
		encoder.SetIndent("", "  ")
		return encoder.Encode(f)
	case outputFormatText:
		factsfmt.DisplayFacts(f)
		return nil
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// WriteFactsJSON is preserved for cmd/cmd_test.go's coverage of the
// snake_case marshal pattern. The live caller moved to
// internal/apply/runner.go's WriteFactsJSON in R1.1a; this cmd-side
// copy stays so the existing TestWriteFactsJSON* suite keeps pinning
// the contract.
//
//nolint:unused // covered by cmd/cmd_test.go; lint runs with tests:false.
func WriteFactsJSON(f *facts.Facts, path string) error {
	// MT-74: marshal via ToMap() so keys are snake_case, matching the
	// daemon's /v1/facts endpoint and the template scope (`{{ os }}`).
	// Direct json.Marshal(*Facts) would emit PascalCase Go field names.
	data, err := json.MarshalIndent(f.ToMap(), "", "  ")
	if err != nil {
		return fmt.Errorf("marshal facts: %w", err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}
