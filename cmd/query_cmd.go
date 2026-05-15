// Package main — `mooncake query` top-level subcommand (epic-read-and-
// report D4). One-shot inspection of a JSON or YAML file by dotted path,
// without writing a plan.
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
package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/pathquery"
)

// defaultQueryMaxBytes mirrors read_common.DefaultMaxBytes (4 MiB) so
// `mooncake query` and `read.json` / `read.yaml` refuse the same set of
// "this file is too big to slurp into memory" cases.
const defaultQueryMaxBytes int64 = 4 << 20

// queryCommand registers `mooncake query <file> <path>`.
func queryCommand() *cli.Command {
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

	format, err := pickFormat(path, c.String("as"))
	if err != nil {
		return cli.Exit("query: "+err.Error(), 2)
	}

	if err := pathquery.Validate(queryPath); err != nil {
		return cli.Exit("query: invalid path: "+err.Error(), 2)
	}

	data, err := readBoundedFile(path, c.Int64("max-bytes"))
	if err != nil {
		return cli.Exit("query: "+err.Error(), 2)
	}

	parsed, err := parseDoc(data, format)
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

// pickFormat resolves the file format, preferring an explicit override
// when set. Returns "json" or "yaml" or an error if neither override
// nor extension is decisive.
func pickFormat(path, override string) (string, error) {
	switch override {
	case "":
		// fall through to extension sniffing
	case "json", "yaml":
		return override, nil
	default:
		return "", fmt.Errorf("--as must be json or yaml (got %q)", override)
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".json":
		return "json", nil
	case ".yml", ".yaml":
		return "yaml", nil
	default:
		return "", fmt.Errorf("cannot infer format from extension %q; pass --as json|yaml", ext)
	}
}

// readBoundedFile mirrors read_common.readBounded: caps the read at
// limit+1 bytes so oversize files are detected without slurping them.
func readBoundedFile(path string, limit int64) ([]byte, error) {
	f, err := os.Open(path) // #nosec G304 — path is a CLI argument by design
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("file not found: %s", path)
		}
		return nil, fmt.Errorf("open %s: %w", path, err)
	}
	defer func() { _ = f.Close() }()

	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%s exceeds --max-bytes=%d", path, limit)
	}
	return data, nil
}

// parseDoc decodes data per format. YAML rejects multi-document files
// to match read.yaml's behavior (spec-38 Open Q3).
func parseDoc(data []byte, format string) (any, error) {
	var v any
	switch format {
	case "json":
		if err := json.Unmarshal(data, &v); err != nil {
			return nil, err
		}
		return v, nil
	case "yaml":
		dec := yaml.NewDecoder(bytes.NewReader(data))
		if err := dec.Decode(&v); err != nil {
			return nil, err
		}
		var second any
		switch err := dec.Decode(&second); {
		case errors.Is(err, io.EOF):
			return v, nil
		case err == nil:
			return nil, fmt.Errorf("multi-document YAML not supported")
		default:
			return nil, fmt.Errorf("trailing-document parse: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown format %q", format)
	}
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
