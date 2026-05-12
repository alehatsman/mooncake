package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/urfave/cli/v2"
)

// queryMap looks up one or more dot-path keys in a flat ToMap() and prints
// the results. Shared by `mooncake facts --query` and `mooncake metrics
// --query`.
//
// Key normalization: dots in the query are replaced with underscores to
// match the ToMap() key naming convention (e.g. `go.version` → `go_version`,
// `cpu.usage_pct` → `cpu_usage_pct`).
//
// Exits with code 1 (via cli.Exit) if any key is missing or empty.
func queryMap(m map[string]interface{}, queries []string) error {
	multi := len(queries) > 1
	missing := false

	for _, q := range queries {
		key := strings.ReplaceAll(q, ".", "_")
		val, ok := m[key]
		if !ok || val == nil || val == "" || val == false {
			if multi {
				fmt.Printf("%s=\n", q)
			}
			missing = true
			continue
		}

		var out string
		switch v := val.(type) {
		case string:
			out = v
		case bool:
			out = "true"
		case int, int64, float64:
			out = fmt.Sprintf("%v", v)
		default:
			b, err := json.Marshal(v)
			if err != nil {
				out = fmt.Sprintf("%v", v)
			} else {
				out = string(b)
			}
		}

		if multi {
			fmt.Printf("%s=%s\n", q, out)
		} else {
			fmt.Println(out)
		}
	}

	if missing {
		return cli.Exit("", 1)
	}
	return nil
}
