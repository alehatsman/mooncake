// Package mod implements the `mooncake mod` CLI tree — fetch, cache,
// and register modules (Git repositories that export reusable
// components, spec-67). Successor to the marketplace surface in
// cmd/presets.go.
//
// Three subcommands ship in phase 1:
//
//	mooncake mod add <url>@<version> [--as <alias>]
//	mooncake mod cache list
//	mooncake mod cache clean
//
// `add` fetches the module, reads its index.yml for the default alias, and
// writes/updates the `modules:` block in the nearest mooncake.yml. The cache
// directory lives at ~/.cache/mooncake/modules (overridable via
// $MOONCAKE_MODULE_CACHE).
package mod

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v3"

	"github.com/alehatsman/mooncake/internal/modules"
)

func Command() *cli.Command {
	return &cli.Command{
		Name:  "mod",
		Usage: "Manage Git-native component modules",
		Subcommands: []*cli.Command{
			modAddCommand(),
			modCacheCommand(),
		},
	}
}

func modAddCommand() *cli.Command {
	return &cli.Command{
		Name:      "add",
		Usage:     "Fetch a module and add it to the playbook's modules: block",
		ArgsUsage: "<url>@<version>",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  "as",
				Usage: "Override the default alias (taken from the module's name: field)",
			},
			&cli.StringFlag{
				Name:  "playbook",
				Usage: "Path to the mooncake.yml to update (default: ./mooncake.yml)",
			},
		},
		Action: runModAdd,
	}
}

func modCacheCommand() *cli.Command {
	return &cli.Command{
		Name:  "cache",
		Usage: "Inspect and clean the on-disk module cache",
		Subcommands: []*cli.Command{
			{
				Name:   "list",
				Usage:  "List cached modules",
				Action: runModCacheList,
			},
			{
				Name:   "clean",
				Usage:  "Remove all cached modules",
				Action: runModCacheClean,
			},
		},
	}
}

func runModAdd(c *cli.Context) error {
	if c.NArg() != 1 {
		return fmt.Errorf("usage: mooncake mod add <url>@<version> [--as <alias>]")
	}
	refStr := c.Args().Get(0)
	ref, err := modules.ParseReference(refStr)
	if err != nil {
		return err
	}

	fetcher := newCLIFetcher()
	moduleRoot, err := fetcher.Fetch(context.Background(), ref)
	if err != nil {
		return err
	}
	// Subpath: the index.yml lives at the subpath root, not the repo root.
	if ref.Subpath != "" {
		moduleRoot = filepath.Join(moduleRoot, ref.Subpath)
	}
	idx, err := modules.LoadIndex(moduleRoot)
	if err != nil {
		return err
	}

	alias := c.String("as")
	if alias == "" {
		alias = idx.Name
	}
	if alias == "" {
		return fmt.Errorf("module has no name and no --as alias specified")
	}

	playbook := c.String("playbook")
	if playbook == "" {
		playbook = "mooncake.yml"
	}
	if err := upsertModulesEntry(playbook, alias, refStr); err != nil {
		return err
	}

	exports := make([]string, 0, len(idx.Exports))
	for name := range idx.Exports {
		exports = append(exports, name)
	}
	sort.Strings(exports)
	fmt.Printf("added %s (%s)\nexports: %s\n", alias, refStr, joinComma(exports))
	return nil
}

func runModCacheList(_ *cli.Context) error {
	root, err := cacheRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		fmt.Println("(no modules cached)")
		return nil
	}
	// Cache layout: <root>/<host>/<owner>/<repo>@<version>/
	entries, err := walkCache(root)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		fmt.Println("(no modules cached)")
		return nil
	}
	for _, e := range entries {
		fmt.Println(e)
	}
	return nil
}

func runModCacheClean(_ *cli.Context) error {
	root, err := cacheRoot()
	if err != nil {
		return err
	}
	if _, err := os.Stat(root); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(root)
}

func newCLIFetcher() *modules.Fetcher {
	root := os.Getenv("MOONCAKE_MODULE_CACHE")
	return &modules.Fetcher{Root: root}
}

func cacheRoot() (string, error) {
	if r := os.Getenv("MOONCAKE_MODULE_CACHE"); r != "" {
		return r, nil
	}
	return modules.DefaultCacheRoot()
}

// walkCache returns "<host>/<owner>/<repo>@<version>" entries for every
// cached module under root.
func walkCache(root string) ([]string, error) {
	var out []string
	hosts, err := os.ReadDir(root)
	if err != nil {
		return nil, err
	}
	for _, host := range hosts {
		if !host.IsDir() {
			continue
		}
		owners, err := os.ReadDir(filepath.Join(root, host.Name()))
		if err != nil {
			continue
		}
		for _, owner := range owners {
			if !owner.IsDir() {
				continue
			}
			repos, err := os.ReadDir(filepath.Join(root, host.Name(), owner.Name()))
			if err != nil {
				continue
			}
			for _, repo := range repos {
				if !repo.IsDir() {
					continue
				}
				out = append(out, host.Name()+"/"+owner.Name()+"/"+repo.Name())
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// upsertModulesEntry reads playbook, ensures `modules:` is a mapping
// containing `alias: ref`, and writes the file back. The file is created if
// it does not exist; in that case it contains only a modules: block.
//
// Comments and key ordering are NOT preserved — yaml.v3 round-trips through a
// map will rewrite the file. This is acceptable for phase 1.
func upsertModulesEntry(playbook, alias, ref string) error {
	var top map[string]interface{}
	data, err := os.ReadFile(playbook) // #nosec G304 -- path comes from --playbook flag (user-controlled CLI input)
	switch {
	case err == nil:
		if err := yaml.Unmarshal(data, &top); err != nil {
			return fmt.Errorf("parse %s: %w", playbook, err)
		}
		if top == nil {
			top = map[string]interface{}{}
		}
	case os.IsNotExist(err):
		top = map[string]interface{}{}
	default:
		return err
	}

	mods, _ := top["modules"].(map[string]interface{})
	if mods == nil {
		mods = map[string]interface{}{}
	}
	mods[alias] = ref
	top["modules"] = mods

	out, err := yaml.Marshal(top)
	if err != nil {
		return fmt.Errorf("marshal %s: %w", playbook, err)
	}
	return os.WriteFile(playbook, out, 0o644)
}

func joinComma(ss []string) string {
	out := ""
	for i, s := range ss {
		if i > 0 {
			out += ", "
		}
		out += s
	}
	return out
}
