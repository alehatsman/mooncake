// Package cmdutil hosts small helpers shared across cmd/ subsystem
// sub-packages (cmd/fleet, cmd/agentd, …). Anything that two or more
// sub-packages need to call belongs here; anything specific to one
// sub-package stays inside it.
package cmdutil

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v2"

	"github.com/alehatsman/mooncake/internal/config"
)

// ExitCodeValidationError mirrors cmd/mooncake.go's exitCodeValidationError
// (value 2). Duplicated here rather than imported because cmd/ imports
// cmd/cmdutil, not the reverse; the exit-code contract is part of the
// CLI's user-facing API, so the literal lives in two places by design.
const ExitCodeValidationError = 2

// ParseTags splits a comma-separated tag string into a slice of trimmed
// tags. Empty input or all-whitespace segments return nil so callers
// can treat "no tags" and "empty --tags=" identically.
func ParseTags(tagsStr string) []string {
	if tagsStr == "" {
		return nil
	}
	var tags []string
	for _, tag := range strings.Split(tagsStr, ",") {
		trimmed := strings.TrimSpace(tag)
		if trimmed != "" {
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// ResolveConfigPath returns the explicit --config value if the flag
// was set, otherwise walks config.SearchPaths in the current working
// directory. If nothing matches, the friendly *config.ErrNoConfigFound
// is returned (carrying the searched directory) so the caller can
// render config.HintNoConfigFound via PrintNoConfigHintAndExit.
func ResolveConfigPath(c *cli.Context) (string, error) {
	if explicit := c.String("config"); explicit != "" {
		return explicit, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return config.DiscoverConfig(cwd)
}

// PrintNoConfigHintAndExit renders config.HintNoConfigFound for the
// given subcommand and exits with ExitCodeValidationError. Returns
// true when it handled the error (caller can `return nil`).
func PrintNoConfigHintAndExit(err error, cmdName string) bool {
	var nfe *config.ErrNoConfigFound
	if !errors.As(err, &nfe) {
		return false
	}
	fmt.Fprint(os.Stderr, config.HintNoConfigFound(nfe, cmdName))
	os.Exit(ExitCodeValidationError)
	return true
}

// QueryMap looks up one or more dot-path keys in a flat ToMap() and
// prints the results. Shared by `mooncake facts --query` and
// `mooncake metrics --query`.
//
// Key normalization: dots in the query are replaced with underscores to
// match the ToMap() key naming convention (e.g. `go.version` →
// `go_version`, `cpu.usage_pct` → `cpu_usage_pct`).
//
// Exits with code 1 (via cli.Exit) if any key is missing or empty.
func QueryMap(m map[string]interface{}, queries []string) error {
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

// QueryMapJSON is the JSON counterpart to QueryMap: emits a single JSON
// object keyed by the original query strings (dot-form preserved) so the
// caller can `jq .cpu_usage_pct` regardless of how many keys were asked
// for. Missing keys yield a null value rather than being omitted, so the
// shape of the response is stable across invocations.
func QueryMapJSON(m map[string]interface{}, queries []string) error {
	missing := false
	out := make(map[string]interface{}, len(queries))
	for _, q := range queries {
		key := strings.ReplaceAll(q, ".", "_")
		val, ok := m[key]
		if !ok || val == nil {
			out[q] = nil
			missing = true
			continue
		}
		out[q] = val
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		return err
	}
	if missing {
		return cli.Exit("", 1)
	}
	return nil
}
