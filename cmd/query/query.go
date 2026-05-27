// Package query implements the `mooncake query` top-level subcommand
// (epic-read-and-report D4). One-shot inspection of a JSON or YAML
// file by dotted path, without writing a plan.
//
// Behavior:
//
//	mooncake query ./package.json version
//	mooncake query ./config.yml service.port
//	mooncake query ./mooncake.lock 'tool[0].name'
//
// Format auto-detection: .json → json, .yml/.yaml → yaml. Override via
// --as. Scalar values print raw; structured values print compact JSON
// (--pretty for indented). Exit codes are agent-friendly: 0 found,
// 1 path-miss, 2 parse error.
package query

import (
	"encoding/json"
	"fmt"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/pathquery"
	"github.com/alehatsman/mooncake/internal/queryio"
)

// defaultQueryMaxBytes mirrors read_common.DefaultMaxBytes (4 MiB) so
// `mooncake query` and `read.json` / `read.yaml` refuse the same set of
// "this file is too big to slurp into memory" cases.
const defaultQueryMaxBytes int64 = 4 << 20

// Command registers `mooncake query <file> <path>`.
func Command() *cli.Command {
	return &cli.Command{
		Name:      "query",
		Usage:     "Read a JSON/YAML file and extract a value by dotted path",
		ArgsUsage: "<file> <path>",
		Description: `Read a JSON or YAML file and print the value at the given dotted path.

Path syntax matches read.json / read.yaml (spec-38): dotted keys
(a.b.c) and bracketed integer indices (a[0], a.b[3].c). Scalars print
raw; objects and arrays print as compact JSON unless --pretty is set.

Exit codes:
  0  path resolved, value printed
  1  path did not match (file parsed, key absent)
  2  file unreadable, oversize, or parse error`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "as",
				Usage: "Force format: json|yaml (default: auto-detect from extension)",
			},
			&cli.BoolFlag{
				Name:  "pretty",
				Usage: "Pretty-print structured output (2-space indent)",
			},
			&cli.Int64Flag{
				Name:  "max-bytes",
				Value: defaultQueryMaxBytes,
				Usage: "Refuse to load files larger than this size in bytes",
			},
		},
		Action: runQuery,
	}
}

func runQuery(c *cli.Context) error {
	if c.NArg() < 1 {
		return cli.Exit("query: missing <file> argument", 2)
	}
	path := c.Args().Get(0)
	queryPath := c.Args().Get(1) // empty path is allowed → returns whole document

	format, err := queryio.PickFormat(path, c.String("as"), "--as")
	if err != nil {
		return cli.Exit("query: "+err.Error(), 2)
	}

	if err := pathquery.Validate(queryPath); err != nil {
		return cli.Exit("query: invalid path: "+err.Error(), 2)
	}

	data, err := queryio.ReadBounded(path, c.Int64("max-bytes"))
	if err != nil {
		return cli.Exit("query: "+err.Error(), 2)
	}

	parsed, err := queryio.ParseDoc(data, format)
	if err != nil {
		return cli.Exit(fmt.Sprintf("query: parse %s: %v", path, err), 2)
	}

	value, found, err := pathquery.Extract(parsed, queryPath)
	if err != nil {
		return cli.Exit("query: "+err.Error(), 2)
	}
	if !found {
		return cli.Exit("", 1)
	}

	return printQueryValue(value, c.Bool("pretty"))
}

// printQueryValue writes the extracted value to stdout per the CLI
// contract: scalars raw, structured values JSON (compact by default,
// indented with --pretty).
func printQueryValue(v any, pretty bool) error {
	switch t := v.(type) {
	case string:
		fmt.Println(t)
		return nil
	case bool:
		fmt.Println(t)
		return nil
	case nil:
		fmt.Println("null")
		return nil
	case map[string]any, []any:
		var (
			out []byte
			err error
		)
		if pretty {
			out, err = json.MarshalIndent(v, "", "  ")
		} else {
			out, err = json.Marshal(v)
		}
		if err != nil {
			return cli.Exit("query: marshal output: "+err.Error(), 2)
		}
		fmt.Println(string(out))
		return nil
	default:
		// numbers (int, float64), and any other scalar
		fmt.Printf("%v\n", v)
		return nil
	}
}
