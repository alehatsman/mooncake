// Package cmdutil hosts small helpers shared across cmd/ subsystem
// sub-packages (cmd/fleet, cmd/agentd, …). Anything that two or more
// sub-packages need to call belongs here; anything specific to one
// sub-package stays inside it.
//
// Three helpers live here today:
//
//   - ParseTags splits a comma-separated --tags flag value.
//   - ResolveConfigPath honors an explicit --config or auto-discovers.
//   - PrintNoConfigHintAndExit renders the no-config-found hint and
//     exits with the standard validation exit code.
package cmdutil

import (
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
