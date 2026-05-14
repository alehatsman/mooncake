package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/alehatsman/mooncake/internal/config"
	"github.com/urfave/cli/v2"
)

// resolveConfigPath returns the explicit --config value if the flag was set,
// otherwise walks config.SearchPaths in the current working directory. If
// nothing matches, the friendly *config.ErrNoConfigFound is returned (carrying
// the searched directory) so the caller can render config.HintNoConfigFound.
func resolveConfigPath(c *cli.Context) (string, error) {
	if explicit := c.String("config"); explicit != "" {
		return explicit, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return config.DiscoverConfig(cwd)
}

// printNoConfigHintAndExit renders config.HintNoConfigFound for the given
// subcommand and exits with exitCodeValidationError. Returns true when it
// handled the error (caller can `return nil`).
func printNoConfigHintAndExit(err error, cmdName string) bool {
	var nfe *config.ErrNoConfigFound
	if !errors.As(err, &nfe) {
		return false
	}
	fmt.Fprint(os.Stderr, config.HintNoConfigFound(nfe, cmdName))
	os.Exit(exitCodeValidationError)
	return true
}
