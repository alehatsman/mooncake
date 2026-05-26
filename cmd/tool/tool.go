// Package tool implements the `mooncake tool` CLI tree — read-only
// inspection of tools resolved through the project's lockfile
// (`mooncake tool which|list|env`).
package tool

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/alehatsman/mooncake/internal/actions/tool"
	"github.com/alehatsman/mooncake/internal/actions/tool/store"
	"github.com/alehatsman/mooncake/internal/lockfile"
	"github.com/urfave/cli/v2"
)

// Command groups the read-only `mooncake tool …` subcommands that
// inspect the local tool-action lockfile and install state.
// Spec 19. No install/upgrade verbs at this level — installation happens
// via `mooncake apply` against a config with `tool:` steps.
func Command() *cli.Command {
	return &cli.Command{
		Name:  "tool",
		Usage: "Inspect tools installed via the `tool` action",
		Description: `Read-only inspection of mooncake.lock and the tool install dir.

Tools are installed declaratively via 'mooncake apply' against a config
containing 'tool:' steps. These subcommands query what's already there.

Examples:
  mooncake tool which go
  mooncake tool list
  mooncake tool env --shell zsh`,
		Subcommands: []*cli.Command{
			toolWhichCommand(),
			toolListCommand(),
			toolEnvCommand(),
		},
	}
}

func toolWhichCommand() *cli.Command {
	return &cli.Command{
		Name:      "which",
		Usage:     "Print the absolute bin path for an installed tool",
		ArgsUsage: "<name>",
		Description: `Print the absolute path to the executable for <name>, resolved from
the nearest mooncake.lock (walking up from CWD). For mise-backed tools,
delegates to 'mise which <name> --version <version>'.

Exits non-zero if the tool is not declared in the lockfile, or is
declared but not installed.`,
		Action: toolWhichAction,
	}
}

func toolListCommand() *cli.Command {
	return &cli.Command{
		Name:  "list",
		Usage: "List tools declared in the nearest mooncake.lock",
		Description: `Print one row per lock entry: name, version, backend, installed status,
and absolute bin path. Reads the nearest mooncake.lock (walks up from CWD).`,
		Action: toolListAction,
	}
}

func toolEnvCommand() *cli.Command {
	return &cli.Command{
		Name:  "env",
		Usage: "Print shell export lines to put installed tool bins on PATH",
		Description: `Emit shell-specific 'export PATH=...' (or 'set -x PATH ...' for fish)
lines for each URL-backed tool in the lockfile. Mise-backed tools emit
a comment hint pointing at 'eval "$(mise activate)"' rather than a
PATH line — mise owns its own activation.

This is a string generator, not a shell runtime. Pipe into eval or
source from your shell rc.

Example:
  eval "$(mooncake tool env --shell zsh)"`,
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "shell",
				Value: "zsh",
				Usage: "Target shell: zsh | bash | fish",
			},
		},
		Action: toolEnvAction,
	}
}

// --- actions ------------------------------------------------------------

func toolWhichAction(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("usage: mooncake tool which <name>")
	}
	name := c.Args().First()

	lock, _, err := loadNearestLockfile()
	if err != nil {
		return err
	}

	entry, ok := findEntryByName(lock, name)
	if !ok {
		return fmt.Errorf("tool %q not declared in mooncake.lock", name)
	}

	binPath, err := locateEntry(c.Context, entry)
	if err != nil {
		return err
	}
	if binPath == "" {
		return fmt.Errorf("tool %s %s is declared but not installed (run 'mooncake apply' to install)", entry.Name, entry.Version)
	}
	fmt.Println(binPath)
	return nil
}

func toolListAction(c *cli.Context) error {
	lock, lockPath, err := loadNearestLockfile()
	if err != nil {
		return err
	}
	if len(lock.Entries) == 0 {
		fmt.Fprintln(os.Stderr, "no tools declared in", lockPath)
		return nil
	}

	entries := append([]lockfile.Entry(nil), lock.Entries...)
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].Name != entries[j].Name {
			return entries[i].Name < entries[j].Name
		}
		return entries[i].Version < entries[j].Version
	})

	for _, e := range entries {
		binPath, _ := locateEntry(c.Context, e)
		status := "missing"
		path := ""
		if binPath != "" {
			status = "installed"
			path = binPath
		}
		fmt.Printf("%-20s %-12s via %-16s [%s]  %s\n", e.Name, e.Version, e.Backend, status, path)
	}
	return nil
}

func toolEnvAction(c *cli.Context) error {
	shell := c.String("shell")
	if shell != "zsh" && shell != "bash" && shell != "fish" {
		return fmt.Errorf("unsupported shell %q (use zsh, bash, or fish)", shell)
	}

	lock, lockPath, err := loadNearestLockfile()
	if err != nil {
		return err
	}
	if len(lock.Entries) == 0 {
		fmt.Fprintln(os.Stderr, "# no tools declared in", lockPath)
		return nil
	}

	fmt.Printf("# mooncake tool env (shell=%s, lockfile=%s)\n", shell, lockPath)
	for _, e := range lock.Entries {
		if e.Backend == tool.BackendMise {
			fmt.Printf("# %s %s via mise — run 'eval \"$(mise activate %s)\"' to activate\n", e.Name, e.Version, shell)
			continue
		}
		binPath, _ := locateEntry(c.Context, e)
		if binPath == "" {
			fmt.Printf("# %s %s declared but not installed; skipping\n", e.Name, e.Version)
			continue
		}
		binDir := filepath.Dir(binPath)
		switch shell {
		case "fish":
			fmt.Printf("set -gx PATH %s $PATH  # %s %s\n", binDir, e.Name, e.Version)
		default: // zsh, bash
			fmt.Printf("export PATH=\"%s:$PATH\"  # %s %s\n", binDir, e.Name, e.Version)
		}
	}
	return nil
}

// --- helpers ------------------------------------------------------------

// loadNearestLockfile finds the nearest mooncake.lock walking up from
// CWD. Returns the loaded lockfile and the path it came from.
func loadNearestLockfile() (*lockfile.Lock, string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, "", fmt.Errorf("getwd: %w", err)
	}
	path := lockfile.Find(cwd)
	if path == "" {
		return nil, "", fmt.Errorf("no mooncake.lock found from %s upward — run 'mooncake apply' on a config with tool: steps first", cwd)
	}
	l, err := lockfile.Load(path)
	if err != nil {
		return nil, path, err
	}
	return l, path, nil
}

// findEntryByName returns the first lockfile entry matching name. If
// multiple versions are present, the highest version string wins
// (string comparison — v1 doesn't parse semver).
func findEntryByName(lock *lockfile.Lock, name string) (lockfile.Entry, bool) {
	var matches []lockfile.Entry
	for _, e := range lock.Entries {
		if e.Name == name {
			matches = append(matches, e)
		}
	}
	if len(matches) == 0 {
		return lockfile.Entry{}, false
	}
	if len(matches) == 1 {
		return matches[0], true
	}
	sort.Slice(matches, func(i, j int) bool { return matches[i].Version > matches[j].Version })
	return matches[0], true
}

// locateEntry resolves an entry to its absolute bin path. Returns
// ("", nil) when the tool is not installed; ("", err) only on
// transient backend errors that callers should surface.
func locateEntry(ctx context.Context, e lockfile.Entry) (string, error) {
	backend, err := tool.Get(e.Backend)
	if err != nil {
		// Unknown backend in lockfile: don't fail hard for `list` — report missing.
		return "", nil //nolint:nilerr // see locateEntry contract above
	}
	spec := tool.SpecFromLockEntry(e)
	installDir, err := store.InstallDir(e.Name, e.Version)
	if err != nil {
		return "", err
	}
	binPath, err := backend.Locate(ctx, spec, installDir)
	if err != nil {
		return "", err
	}
	if binPath == "" {
		return "", nil
	}
	if !filepath.IsAbs(binPath) {
		// Should not happen with current backends, but defensive: an
		// install dir-relative path is not useful for `which`.
		return "", nil
	}
	return binPath, nil
}
