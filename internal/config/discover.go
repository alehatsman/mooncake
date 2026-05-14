package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SearchPaths is the ordered list of relative paths checked for a project
// config when --config is not provided. Exported so callers (CLI handlers,
// `mooncake doctor`) can render the list in error messages.
var SearchPaths = []string{
	"mooncake.yml",
	"mooncake/main.yml",
}

// ErrNoConfigFound is returned by DiscoverConfig when no candidate exists.
// It carries the absolute search directory so callers can render a useful
// error.
type ErrNoConfigFound struct {
	Dir string
}

func (e *ErrNoConfigFound) Error() string {
	return "no mooncake config found in " + e.Dir
}

// DiscoverConfig returns the first existing regular file from SearchPaths,
// rooted at dir. Returns *ErrNoConfigFound if nothing matches. Directories
// matching a candidate name are skipped (a directory named "mooncake.yml"
// is pathological but should not crash discovery).
func DiscoverConfig(dir string) (string, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	for _, rel := range SearchPaths {
		candidate := filepath.Join(abs, rel)
		info, err := os.Stat(candidate)
		if err != nil {
			continue
		}
		if info.Mode().IsRegular() {
			return candidate, nil
		}
	}
	return "", &ErrNoConfigFound{Dir: abs}
}

// HintNoConfigFound returns the user-facing remediation message for an
// ErrNoConfigFound. cmdName is the subcommand the user invoked, used in
// the "point explicitly" suggestion (e.g. "apply", "plan", "validate").
func HintNoConfigFound(e *ErrNoConfigFound, cmdName string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "no mooncake config found in %s\n\n", e.Dir)
	b.WriteString("  searched:\n")
	for _, p := range SearchPaths {
		fmt.Fprintf(&b, "    ./%s\n", p)
	}
	b.WriteString("\n  scaffold one with:    mooncake init\n")
	fmt.Fprintf(&b, "  or point explicitly:  mooncake %s -c <path>\n", cmdName)
	return b.String()
}
